//go:build !windows

package config

import (
	"encoding/base64"
	"errors"
)

// Protect ist auf Nicht-Windows-Systemen keine echte Verschluesselung.
// Das Programm wird ausschliesslich fuer Windows ausgeliefert; diese
// Fassung existiert nur, damit die Tests ueberall uebersetzen.
func Protect(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	return "plain:" + base64.StdEncoding.EncodeToString([]byte(plaintext)), nil
}

// Unprotect kehrt Protect um.
func Unprotect(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	if len(encoded) < 6 || encoded[:6] != "plain:" {
		return "", errors.New("gespeichertes Passwort ist unlesbar")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded[6:])
	if err != nil {
		return "", errors.New("gespeichertes Passwort ist unlesbar")
	}
	return string(raw), nil
}
