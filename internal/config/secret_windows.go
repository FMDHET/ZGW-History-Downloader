//go:build windows

package config

import (
	"encoding/base64"
	"errors"
	"syscall"
	"unsafe"
)

var (
	crypt32           = syscall.NewLazyDLL("crypt32.dll")
	kernel32          = syscall.NewLazyDLL("kernel32.dll")
	procProtectData   = crypt32.NewProc("CryptProtectData")
	procUnprotectData = crypt32.NewProc("CryptUnprotectData")
	procLocalFree     = kernel32.NewProc("LocalFree")
)

// cryptprotectUIForbidden verbietet Windows, einen Dialog zu zeigen.
// Wir laufen in einer GUI und wollen keinen zweiten Prompt.
const cryptprotectUIForbidden = 0x1

type dataBlob struct {
	cbData uint32
	pbData *byte
}

func newBlob(d []byte) *dataBlob {
	if len(d) == 0 {
		return &dataBlob{}
	}
	return &dataBlob{cbData: uint32(len(d)), pbData: &d[0]}
}

func (b *dataBlob) bytes() []byte {
	if b.cbData == 0 || b.pbData == nil {
		return nil
	}
	out := make([]byte, b.cbData)
	copy(out, unsafe.Slice(b.pbData, b.cbData))
	return out
}

func (b *dataBlob) free() {
	if b.pbData != nil {
		procLocalFree.Call(uintptr(unsafe.Pointer(b.pbData)))
	}
}

// Protect verschluesselt einen Text mit der Windows-DPAPI, gebunden an
// das angemeldete Benutzerkonto, und gibt ihn base64-kodiert zurueck.
func Protect(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}
	in := newBlob([]byte(plaintext))
	var out dataBlob

	ret, _, err := procProtectData.Call(
		uintptr(unsafe.Pointer(in)),
		0, 0, 0, 0,
		uintptr(cryptprotectUIForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return "", errors.New("Passwort konnte nicht verschluesselt werden: " + err.Error())
	}
	defer out.free()
	return base64.StdEncoding.EncodeToString(out.bytes()), nil
}

// Unprotect kehrt Protect um. Ein leerer Eingabewert ergibt einen
// leeren Ausgabewert ohne Fehler.
func Unprotect(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", errors.New("gespeichertes Passwort ist unlesbar")
	}
	in := newBlob(raw)
	var out dataBlob

	ret, _, err := procUnprotectData.Call(
		uintptr(unsafe.Pointer(in)),
		0, 0, 0, 0,
		uintptr(cryptprotectUIForbidden),
		uintptr(unsafe.Pointer(&out)),
	)
	if ret == 0 {
		return "", errors.New("gespeichertes Passwort konnte nicht entschluesselt werden: " + err.Error())
	}
	defer out.free()
	return string(out.bytes()), nil
}
