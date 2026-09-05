// Package config liest und schreibt die Einstellungen des Programms.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// DefaultHost ist der mDNS-Name, unter dem ein ZGW16-IP ab Werk erreichbar ist.
const DefaultHost = "zgw16-ip.local"

const appFolder = "ZGW-History-Downloader"

// Config ist der Inhalt von config.json. Secret ist das mit
// Protect verschluesselte Passwort, niemals Klartext.
type Config struct {
	Host      string `json:"host"`
	OutputDir string `json:"outputDir"`
	Remember  bool   `json:"rememberPassword"`
	Secret    string `json:"password"`
}

// Path liefert den vollstaendigen Pfad der Konfigurationsdatei.
func Path() (string, error) {
	dir := os.Getenv("APPDATA")
	if dir == "" {
		var err error
		dir, err = os.UserConfigDir()
		if err != nil {
			return "", errors.New("Konfigurationsverzeichnis nicht gefunden")
		}
	}
	return filepath.Join(dir, appFolder, "config.json"), nil
}

// Load liest die Konfiguration. Fehlt die Datei oder ist sie
// beschaedigt, gelten die Vorgabewerte; das ist kein Fehlerfall.
func Load() Config {
	defaults := Config{Host: DefaultHost}

	p, err := Path()
	if err != nil {
		return defaults
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return defaults
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return defaults
	}
	if c.Host == "" {
		c.Host = DefaultHost
	}
	return c
}

// Save schreibt die Konfiguration und legt das Verzeichnis bei Bedarf an.
func Save(c Config) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}
