package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zgwhistory/internal/zgw"
)

// TestGegenEchtesGateway laeuft nur, wenn ZGW_HOST und ZGW_PASSWORD
// gesetzt sind, und prueft dann den vollstaendigen Weg vom Login bis zur
// geschriebenen CSV an echter Hardware.
//
//	ZGW_HOST=192.168.1.50 ZGW_PASSWORD=geheim go test . -run Echtes -v
//
// Ohne die beiden Variablen wird der Test uebersprungen, damit der
// normale Testlauf kein Geraet braucht.
func TestGegenEchtesGateway(t *testing.T) {
	host := os.Getenv("ZGW_HOST")
	password := os.Getenv("ZGW_PASSWORD")
	if host == "" || password == "" {
		t.Skip("ZGW_HOST und ZGW_PASSWORD nicht gesetzt, kein Geraet zum Pruefen")
	}

	ctx := context.Background()

	client, err := zgw.NewClient(host, password)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := client.Login(ctx); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if client.Token() == "" {
		t.Fatal("nach dem Login liegt kein Token vor")
	}

	info, err := client.System(ctx)
	if err != nil {
		t.Fatalf("System: %v", err)
	}
	t.Logf("Gateway: %s (%s), Firmware %s", info.Gateway.Name, info.Gateway.Type, info.Gateway.Version)

	devices, err := client.Devices(ctx)
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	if len(devices) == 0 {
		t.Fatal("das Gateway meldet keine Zaehler")
	}

	types, err := client.DeviceTypes(ctx)
	if err != nil {
		t.Fatalf("DeviceTypes: %v", err)
	}

	meters := zgw.BuildCatalog(devices, types)
	registers := 0
	for _, m := range meters {
		t.Logf("Zaehler %d %q (%s), %d aufgezeichnete Register", m.BusAddress, m.Name, m.TypeName, len(m.Registers))
		registers += len(m.Registers)
	}
	if registers == 0 {
		t.Fatal("kein einziges Register wird aufgezeichnet")
	}

	// Ein vollstaendiger Export ueber alle Zaehler und alle vier
	// Zeitstufen. Ohne ZGW_OUT landet er in einem Ordner, den der Test
	// wieder aufraeumt; mit ZGW_OUT bleiben die Dateien zum Ansehen liegen.
	dir := os.Getenv("ZGW_OUT")
	if dir == "" {
		dir = t.TempDir()
	}
	selection := make([]MeterSelection, 0, len(meters))
	for _, m := range meters {
		nums := make([]int, 0, len(m.Registers))
		for _, r := range m.Registers {
			nums = append(nums, r.Number)
		}
		selection = append(selection, MeterSelection{BusAddress: m.BusAddress, Registers: nums})
	}

	a := &App{client: client, meters: meters}
	var summary Summary
	a.emit = func(name string, data any) {
		switch name {
		case "export:done":
			summary = data.(Summary)
		case "export:log":
			m := data.(map[string]any)
			if m["level"] == "error" {
				t.Errorf("Protokoll: %s", m["message"])
			} else {
				t.Logf("Protokoll: %s", m["message"])
			}
		}
	}

	a.runExport(ctx, client, meters, ExportRequest{
		Meters:     selection,
		TimeFrames: zgw.ValidTimeFrames,
		OutputDir:  dir,
	})

	t.Logf("Bilanz: %d Abrufe, %d Dateien, %d Fehler, %d ohne Daten",
		summary.Total, summary.Written, summary.Failed, summary.Skipped)

	if summary.Failed > 0 {
		t.Errorf("%d Abrufe sind fehlgeschlagen", summary.Failed)
	}
	if summary.Written == 0 {
		t.Fatal("es wurde keine einzige Datei geschrieben")
	}

	// Die Dateien muessen echte Datenzeilen enthalten, nicht nur
	// Ueberschriften: genau das war der Fehlerfall, als die Zeitstempel
	// nicht gelesen werden konnten.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("ReadFile %s: %v", e.Name(), err)
		}
		lines := strings.Count(strings.TrimRight(string(data), "\r\n"), "\n") + 1
		if lines < 2 {
			t.Errorf("%s enthaelt nur die Ueberschrift, keine Werte", e.Name())
			continue
		}
		t.Logf("%s: %d Zeilen", e.Name(), lines)
	}
}
