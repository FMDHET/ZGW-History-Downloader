package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"zgwhistory/internal/zgw"
)

// gatewayStub spielt ein ZGW16-IP mit zwei Zaehlern.
//
// Adresse 4 liefert fuer 30053 echte Werte und fuer 30073 eine leere
// Liste. Adresse 7 scheitert beim ersten Register mit HTTP 500, liefert
// beim zweiten aber Werte: das prueft die Zusage der Spezifikation, dass
// ein einzelner Fehlschlag den Lauf nicht beendet.
func gatewayStub(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Query().Get("identifier")
		switch {
		case r.URL.Path == "/api/login":
			io.WriteString(w, `{"login":{"accessToken":"TOKEN"}}`)
		case r.URL.Path == "/api/devices/4/history" && id == "30053":
			io.WriteString(w, `{"identifier":30053,"values":[
				{"timestamp":"2026-09-05T08:00:00.000+02:00","value":1234.5},
				{"timestamp":"2026-09-05T08:15:00.000+02:00","value":1240}
			]}`)
		case r.URL.Path == "/api/devices/4/history" && id == "30073":
			io.WriteString(w, `{"identifier":30073,"values":[]}`)
		case r.URL.Path == "/api/devices/7/history" && id == "30053":
			w.WriteHeader(http.StatusInternalServerError)
		case r.URL.Path == "/api/devices/7/history" && id == "30075":
			io.WriteString(w, `{"identifier":30075,"values":[
				{"timestamp":"2026-09-05T08:00:00.000+02:00","value":42}
			]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func testMeters() []zgw.MeterInfo {
	return []zgw.MeterInfo{
		{BusAddress: 4, Name: "Garage", TypeName: "DSZ15DZMOD", Registers: []zgw.RecordedRegister{
			{Number: 30053, Name: "Gesamtwirkleistung", Unit: "Watt"},
			{Number: 30073, Name: "Bezug", Unit: "kWh"},
		}},
		{BusAddress: 7, Name: "Keller", TypeName: "DSZ15DZMOD", Registers: []zgw.RecordedRegister{
			{Number: 30053, Name: "Gesamtwirkleistung", Unit: "Watt"},
			{Number: 30075, Name: "Einspeisung", Unit: "kWh"},
		}},
	}
}

// newTestApp baut eine App mit angemeldetem Client und einem Emitter,
// der die Ereignisse einsammelt statt sie an eine GUI zu schicken.
func newTestApp(t *testing.T, url string) (*App, *[]Summary, func() []string) {
	t.Helper()

	client, err := zgw.NewClient(url, "geheim")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.Login(context.Background()); err != nil {
		t.Fatalf("Login: %v", err)
	}

	var mu sync.Mutex
	var summaries []Summary
	var logs []string

	a := &App{client: client, meters: testMeters()}
	a.emit = func(name string, data any) {
		mu.Lock()
		defer mu.Unlock()
		switch name {
		case "export:done":
			summaries = append(summaries, data.(Summary))
		case "export:log":
			m := data.(map[string]any)
			logs = append(logs, m["level"].(string)+": "+m["message"].(string))
		}
	}
	return a, &summaries, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), logs...)
	}
}

func TestRunExportWritesFileAndCountsOutcomes(t *testing.T) {
	srv := gatewayStub(t)
	defer srv.Close()

	dir := t.TempDir()
	a, summaries, logs := newTestApp(t, srv.URL)

	a.runExport(context.Background(), a.client, a.meters, ExportRequest{
		Meters: []MeterSelection{
			{BusAddress: 4, Registers: []int{30053, 30073}},
			{BusAddress: 7, Registers: []int{30053, 30075}},
		},
		TimeFrames: []int{14},
		OutputDir:  dir,
	})

	if len(*summaries) != 1 {
		t.Fatalf("erwartet genau eine Bilanz, bekam %d", len(*summaries))
	}
	s := (*summaries)[0]
	if s.Total != 4 {
		t.Errorf("Total = %d, erwartet 4 (zwei Zaehler mit je zwei Registern)", s.Total)
	}
	if s.Written != 2 {
		t.Errorf("Written = %d, erwartet 2 (je eine Datei pro Zaehler)", s.Written)
	}
	if s.Failed != 1 {
		t.Errorf("Failed = %d, erwartet 1 (Keller/30053 antwortet mit HTTP 500)", s.Failed)
	}
	if s.Skipped != 1 {
		t.Errorf("Skipped = %d, erwartet 1 (Garage/Bezug hat keine Daten)", s.Skipped)
	}
	if s.Cancelled {
		t.Error("der Lauf wurde nicht abgebrochen")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("erwartet zwei Dateien, bekam %v", names(entries))
	}

	byMeter := map[string]string{}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("ReadFile %s: %v", e.Name(), err)
		}
		switch {
		case strings.HasPrefix(e.Name(), "ZGW_Garage_14d_"):
			byMeter["Garage"] = string(data)
		case strings.HasPrefix(e.Name(), "ZGW_Keller_14d_"):
			byMeter["Keller"] = string(data)
		default:
			t.Errorf("unerwarteter Dateiname %q", e.Name())
		}
		if !strings.HasSuffix(e.Name(), ".csv") {
			t.Errorf("Datei ohne .csv-Endung: %q", e.Name())
		}
	}

	// Nur das Register mit Daten wird zur Spalte, "Bezug" fehlt zu Recht.
	wantGarage := "\xef\xbb\xbf" +
		"Zeitstempel;Zeitstempel_ISO;Gesamtwirkleistung [Watt]\r\n" +
		"05.09.2026 08:00:00;2026-09-05T08:00:00.000+02:00;1234,5\r\n" +
		"05.09.2026 08:15:00;2026-09-05T08:15:00.000+02:00;1240\r\n"
	if byMeter["Garage"] != wantGarage {
		t.Errorf("Garage-Datei\n%q\nerwartet\n%q", byMeter["Garage"], wantGarage)
	}

	// Der Fehlschlag bei 30053 darf 30075 nicht mitreissen.
	wantKeller := "\xef\xbb\xbf" +
		"Zeitstempel;Zeitstempel_ISO;Einspeisung [kWh]\r\n" +
		"05.09.2026 08:00:00;2026-09-05T08:00:00.000+02:00;42\r\n"
	if byMeter["Keller"] != wantKeller {
		t.Errorf("Keller-Datei\n%q\nerwartet\n%q", byMeter["Keller"], wantKeller)
	}

	if len(logs()) == 0 {
		t.Error("Fehler und uebersprungene Register muessen im Protokoll auftauchen")
	}
}

func names(entries []os.DirEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

func TestRunExportStopsOnCancel(t *testing.T) {
	srv := gatewayStub(t)
	defer srv.Close()

	dir := t.TempDir()
	a, summaries, _ := newTestApp(t, srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Abbruch vor dem ersten Abruf.

	a.runExport(ctx, a.client, a.meters, ExportRequest{
		Meters:     []MeterSelection{{BusAddress: 4, Registers: []int{30053}}},
		TimeFrames: []int{14},
		OutputDir:  dir,
	})

	if len(*summaries) != 1 || !(*summaries)[0].Cancelled {
		t.Fatalf("Bilanz muss den Abbruch melden, bekam %+v", *summaries)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("nach sofortigem Abbruch darf keine Datei entstehen, bekam %v", entries)
	}
}
