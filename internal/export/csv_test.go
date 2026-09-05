package export

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

const bom = "\xef\xbb\xbf"

func mustPoint(t *testing.T, iso string, v float64) Point {
	t.Helper()
	p, err := NewPoint(iso, v)
	if err != nil {
		t.Fatalf("NewPoint(%q): %v", iso, err)
	}
	return p
}

func TestNewPointRejectsUnparsableTimestamp(t *testing.T) {
	if _, err := NewPoint("gestern", 1); err == nil {
		t.Fatal("unlesbarer Zeitstempel muss einen Fehler ergeben")
	}
}

func TestWriteGermanExcelDialect(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, []Series{{
		Header: "Gesamtwirkleistung [Watt]",
		Points: []Point{
			mustPoint(t, "2026-09-05T08:15:00.000+02:00", 1234.5),
			mustPoint(t, "2026-09-05T08:30:00.000+02:00", 1240),
		},
	}})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	want := bom +
		"Zeitstempel;Zeitstempel_ISO;Gesamtwirkleistung [Watt]\r\n" +
		"05.09.2026 08:15:00;2026-09-05T08:15:00.000+02:00;1234,5\r\n" +
		"05.09.2026 08:30:00;2026-09-05T08:30:00.000+02:00;1240\r\n"
	if buf.String() != want {
		t.Errorf("Write ergab\n%q\nerwartet\n%q", buf.String(), want)
	}
}

func TestWriteFillsGapsWithEmptyCells(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, []Series{
		{Header: "A", Points: []Point{
			mustPoint(t, "2026-09-05T08:00:00.000+02:00", 1),
			mustPoint(t, "2026-09-05T08:30:00.000+02:00", 3),
		}},
		{Header: "B", Points: []Point{
			mustPoint(t, "2026-09-05T08:15:00.000+02:00", 2),
		}},
	})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	want := bom +
		"Zeitstempel;Zeitstempel_ISO;A;B\r\n" +
		"05.09.2026 08:00:00;2026-09-05T08:00:00.000+02:00;1;\r\n" +
		"05.09.2026 08:15:00;2026-09-05T08:15:00.000+02:00;;2\r\n" +
		"05.09.2026 08:30:00;2026-09-05T08:30:00.000+02:00;3;\r\n"
	if buf.String() != want {
		t.Errorf("Write ergab\n%q\nerwartet\n%q", buf.String(), want)
	}
}

func TestWriteSortsUnorderedInput(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, []Series{{Header: "A", Points: []Point{
		mustPoint(t, "2026-09-05T09:00:00.000+02:00", 2),
		mustPoint(t, "2026-09-05T08:00:00.000+02:00", 1),
	}}})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	want := bom +
		"Zeitstempel;Zeitstempel_ISO;A\r\n" +
		"05.09.2026 08:00:00;2026-09-05T08:00:00.000+02:00;1\r\n" +
		"05.09.2026 09:00:00;2026-09-05T09:00:00.000+02:00;2\r\n"
	if buf.String() != want {
		t.Errorf("Write ergab\n%q\nerwartet\n%q", buf.String(), want)
	}
}

func TestWriteQuotesSeparatorInHeader(t *testing.T) {
	var buf bytes.Buffer
	err := Write(&buf, []Series{{Header: "Wirk;leistung [W]", Points: []Point{
		mustPoint(t, "2026-09-05T08:00:00.000+02:00", 1),
	}}})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"Wirk;leistung [W]"`)) {
		t.Errorf("Semikolon in der Ueberschrift wurde nicht maskiert:\n%s", buf.String())
	}
}

func TestWriteKeepsOriginalOffset(t *testing.T) {
	// Ein Zeitstempel mit +01:00 darf nicht in eine andere Zone gerechnet werden.
	var buf bytes.Buffer
	err := Write(&buf, []Series{{Header: "A", Points: []Point{
		mustPoint(t, "2026-01-15T23:45:00.000+01:00", 7),
	}}})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !bytes.Contains(buf.Bytes(), []byte("15.01.2026 23:45:00")) {
		t.Errorf("Ortszeit wurde umgerechnet:\n%s", buf.String())
	}
}

func TestWriteRejectsEmptySeries(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, nil); err == nil {
		t.Fatal("ohne Messreihen darf keine Datei entstehen")
	}
}

func TestWriteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.csv")
	err := WriteFile(path, []Series{{Header: "A", Points: []Point{
		mustPoint(t, "2026-09-05T08:00:00.000+02:00", 1),
	}}})
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !bytes.HasPrefix(data, []byte(bom)) {
		t.Error("Datei beginnt nicht mit dem UTF-8-BOM")
	}
}
