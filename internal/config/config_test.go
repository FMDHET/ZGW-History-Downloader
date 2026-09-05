package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadWithoutFileGivesDefaults(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	c := Load()
	if c.Host != "zgw16-ip.local" {
		t.Errorf("Host = %q, erwartet \"zgw16-ip.local\"", c.Host)
	}
	if c.Remember {
		t.Error("Remember muss ohne Konfigurationsdatei falsch sein")
	}
}

func TestSaveThenLoad(t *testing.T) {
	t.Setenv("APPDATA", t.TempDir())

	want := Config{Host: "192.168.1.50", OutputDir: `C:\Export`, Remember: true, Secret: "abc"}
	if err := Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got := Load()
	if got != want {
		t.Errorf("Load = %+v, erwartet %+v", got, want)
	}
}

func TestLoadIgnoresBrokenFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)

	p, err := Path()
	if err != nil {
		t.Fatalf("Path: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(p, []byte("{kaputt"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if c := Load(); c.Host != "zgw16-ip.local" {
		t.Errorf("beschaedigte Datei muss zu Vorgaben fuehren, Host = %q", c.Host)
	}
}

func TestProtectRoundTrip(t *testing.T) {
	enc, err := Protect("geheim")
	if err != nil {
		t.Fatalf("Protect: %v", err)
	}
	if enc == "geheim" {
		t.Error("das Passwort darf nicht im Klartext abgelegt werden")
	}
	dec, err := Unprotect(enc)
	if err != nil {
		t.Fatalf("Unprotect: %v", err)
	}
	if dec != "geheim" {
		t.Errorf("Unprotect = %q, erwartet \"geheim\"", dec)
	}
}

func TestUnprotectEmptyIsEmpty(t *testing.T) {
	got, err := Unprotect("")
	if err != nil {
		t.Fatalf("Unprotect(\"\"): %v", err)
	}
	if got != "" {
		t.Errorf("Unprotect(\"\") = %q, erwartet \"\"", got)
	}
}

func TestUnprotectGarbageFails(t *testing.T) {
	if _, err := Unprotect("kein base64 !!!"); err == nil {
		t.Fatal("unlesbare Ablage muss einen Fehler ergeben")
	}
}
