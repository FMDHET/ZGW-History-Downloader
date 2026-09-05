# ZGW History Downloader — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Eine einzelne Windows-`.exe` mit grafischer Oberfläche, die die History-Daten eines Eltako ZGW16WL-IP über dessen REST-API abruft und als Excel-taugliche CSV-Dateien speichert.

**Architecture:** Go-Backend und HTML-Oberfläche in einem Wails-v2-Fenster. Die Fachlogik liegt in drei Paketen unter `internal/`, die weder Wails noch einander kennen: `zgw` (HTTP-Client und Registerkatalog), `export` (CSV-Erzeugung), `config` (Einstellungen und DPAPI). `app.go` verdrahtet sie und meldet Fortschritt per Wails-Event an das Frontend.

**Tech Stack:** Go 1.27, Wails v2.15.0, Vanilla-JS + Vite (aus dem Wails-Template), keine Go-Fremdabhängigkeiten außer den von Wails mitgebrachten. DPAPI über `syscall.NewLazyDLL` gegen `crypt32.dll`.

**Spec:** [`docs/superpowers/specs/2026-09-05-zgw-history-downloader-design.md`](../specs/2026-09-05-zgw-history-downloader-design.md)

## Global Constraints

- Zielplattform ausschließlich `windows/amd64`. Kein macOS, kein Linux.
- Go-Modulname: `zgwhistory`. Go-Version im `go.mod`: `1.24` (Wails v2.15.0 verträgt sich damit; die lokale Toolchain 1.27 baut es).
- Die Fachpakete `internal/zgw` und `internal/export` importieren **niemals** `github.com/wailsapp/wails/v2` — das ist die Bedingung dafür, dass sie ohne GUI testbar sind.
- Alle für den Benutzer sichtbaren Texte sind deutsch. Das gilt auch für Fehlermeldungen, die aus den Fachpaketen nach oben gereicht werden.
- Basis-URL der API: `http://<host>/api`. Vorgabe-Host: `zgw16-ip.local`.
- Authentifizierung: HTTP-Header `accessToken: <UUID>`, **nicht** `Authorization: Bearer`.
- Erlaubte `timeFrame`-Werte: genau `1`, `14`, `365`, `1095`. Keine anderen.
- CSV-Format: UTF-8 **mit** BOM (`\xEF\xBB\xBF`), Feldtrenner `;`, Dezimaltrenner `,`, Zeilenende `\r\n`.
- Zeitstempel werden **nicht** in eine andere Zeitzone umgerechnet. Der Offset aus der API-Antwort bleibt erhalten.
- History-Werte werden **nicht** mit `scaling_factor` nachskaliert (siehe „Offene Punkte" in der Spec).
- Konfigurationspfad: `%APPDATA%\ZGW-History-Downloader\config.json`.
- Tests laufen mit `go test ./...`. Kein Test-Framework außer der Standardbibliothek.

---

### Task 1: Projektgerüst und Repository

**Files:**
- Create: das gesamte Wails-Gerüst im Projektstammverzeichnis (`main.go`, `app.go`, `go.mod`, `wails.json`, `frontend/`, `build/`)
- Create: `.gitignore`

**Interfaces:**
- Consumes: nichts
- Produces: ein baubares Wails-Projekt mit Modulnamen `zgwhistory`

- [ ] **Step 1: Git-Repository anlegen**

Im Projektstammverzeichnis `c:\Users\falkt\Documents\GitHub\FMDHET\ZGW-History-Downloader`:

```bash
git init
```

- [ ] **Step 2: Wails-Projekt in das aktuelle Verzeichnis erzeugen**

```bash
"$(go env GOPATH)/bin/wails.exe" init -n zgwhistory -t vanilla -d .
```

Erwartet: `main.go`, `app.go`, `go.mod`, `wails.json`, `frontend/`, `build/` liegen im Stammverzeichnis. Falls `-d .` einen Unterordner erzeugt statt in das aktuelle Verzeichnis zu schreiben, den Inhalt dieses Unterordners eine Ebene hoch verschieben und den leeren Ordner löschen.

- [ ] **Step 3: Modulname und Go-Version prüfen**

`go.mod` muss mit `module zgwhistory` beginnen. Falls nicht, korrigieren. Die `go`-Direktive auf `1.24` setzen, falls sie höher steht.

Run: `go mod tidy`
Expected: fehlerfrei

- [ ] **Step 4: `.gitignore` anlegen**

```gitignore
build/bin/
frontend/node_modules/
frontend/dist/
*.exe
```

- [ ] **Step 5: Bauen zur Kontrolle, dass die Toolchain vollständig ist**

Run: `"$(go env GOPATH)/bin/wails.exe" build -platform windows/amd64`
Expected: `build/bin/zgwhistory.exe` entsteht, Exit-Code 0. Der erste Lauf installiert die npm-Abhängigkeiten des Frontends und dauert einige Minuten.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "chore: Wails-Projektgeruest anlegen"
```

---

### Task 2: Datenmodelle und URL-Normalisierung

**Files:**
- Create: `internal/zgw/model.go`
- Create: `internal/zgw/url.go`
- Test: `internal/zgw/url_test.go`

**Interfaces:**
- Consumes: nichts
- Produces:
  - `zgw.Device`, `zgw.DeviceType`, `zgw.Register`, `zgw.HistoryPoint`, `zgw.History` (Structs, siehe unten)
  - `zgw.normalizeBaseURL(host string) (string, error)` — paketintern

- [ ] **Step 1: Fehlschlagenden Test schreiben**

`internal/zgw/url_test.go`:

```go
package zgw

import "testing"

func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"zgw16-ip.local", "http://zgw16-ip.local/api"},
		{"192.168.1.50", "http://192.168.1.50/api"},
		{"http://192.168.1.50", "http://192.168.1.50/api"},
		{"http://192.168.1.50/", "http://192.168.1.50/api"},
		{"http://192.168.1.50/api", "http://192.168.1.50/api"},
		{"https://zgw16-ip.local/api/", "https://zgw16-ip.local/api"},
		{"  zgw16-ip.local  ", "http://zgw16-ip.local/api"},
	}
	for _, c := range cases {
		got, err := normalizeBaseURL(c.in)
		if err != nil {
			t.Errorf("normalizeBaseURL(%q): unerwarteter Fehler %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("normalizeBaseURL(%q) = %q, erwartet %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeBaseURLRejectsEmpty(t *testing.T) {
	if _, err := normalizeBaseURL("   "); err == nil {
		t.Fatal("leere Adresse muss einen Fehler ergeben")
	}
}
```

- [ ] **Step 2: Test laufen lassen, Fehlschlag bestätigen**

Run: `go test ./internal/zgw/ -run TestNormalizeBaseURL -v`
Expected: FAIL — `undefined: normalizeBaseURL`

- [ ] **Step 3: `internal/zgw/url.go` schreiben**

```go
package zgw

import (
	"errors"
	"net/url"
	"strings"
)

// normalizeBaseURL macht aus einer Benutzereingabe wie "zgw16-ip.local"
// oder "http://192.168.1.50/" die vollstaendige API-Basisadresse
// "http://192.168.1.50/api".
func normalizeBaseURL(host string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", errors.New("keine Adresse angegeben")
	}
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	u, err := url.Parse(host)
	if err != nil {
		return "", errors.New("Adresse nicht lesbar: " + host)
	}
	if u.Host == "" {
		return "", errors.New("Adresse enthaelt keinen Hostnamen: " + host)
	}
	p := strings.TrimSuffix(u.Path, "/")
	if !strings.HasSuffix(p, "/api") {
		p += "/api"
	}
	u.Path = p
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
```

- [ ] **Step 4: `internal/zgw/model.go` schreiben**

```go
package zgw

// Device ist ein am Gateway angemeldeter Modbus-Zaehler,
// so wie ihn GET /devices liefert.
type Device struct {
	FriendlyId      string `json:"friendlyId"`
	DeviceTypeId    string `json:"deviceTypeId"`
	Manufacturer    string `json:"manufacturer"`
	BusType         int    `json:"busType"`
	BusAddress      int    `json:"busAddress"`
	FirstSeen       string `json:"firstSeen"`
	LastSeen        string `json:"lastSeen"`
	SelectedHistory []int  `json:"selectedHistory"`
}

type devicesResponse struct {
	Devices []Device `json:"devices"`
}

// Register beschreibt eine Messgroesse eines Zaehlertyps.
// Description ist in der API eine Liste einsprachiger Objekte,
// zum Beispiel [{"en":"Total active power"},{"de":"Gesamtwirkleistung"}].
type Register struct {
	Number      int                 `json:"number"`
	Description []map[string]string `json:"description"`
	Unit        string              `json:"unit"`
}

// DeviceType ist ein Zaehlertyp mit seiner vollstaendigen Registertabelle.
type DeviceType struct {
	DeviceTypeId string     `json:"deviceTypeId"`
	Name         string     `json:"name"`
	Registers    []Register `json:"registers"`
}

type deviceTypesResponse struct {
	DeviceTypes []DeviceType `json:"deviceTypes"`
}

// HistoryPoint ist ein einzelner aufgezeichneter Messwert.
type HistoryPoint struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

// History ist die Antwort von GET /devices/{addr}/history.
type History struct {
	Identifier int            `json:"identifier"`
	Values     []HistoryPoint `json:"values"`
}

// SystemInfo ist der fuer uns interessante Ausschnitt von GET /system.
type SystemInfo struct {
	Gateway struct {
		Type         string `json:"type"`
		Name         string `json:"name"`
		Version      string `json:"version"`
		SerialNumber string `json:"serialNumber"`
	} `json:"gateway"`
}
```

- [ ] **Step 5: Tests laufen lassen**

Run: `go test ./internal/zgw/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/zgw
git commit -m "feat(zgw): Datenmodelle und URL-Normalisierung"
```

---

### Task 3: API-Client mit Anmeldung

**Files:**
- Create: `internal/zgw/client.go`
- Test: `internal/zgw/client_test.go`

**Interfaces:**
- Consumes: `normalizeBaseURL`, `SystemInfo` aus Task 2
- Produces:
  - `zgw.NewClient(host, password string) (*Client, error)`
  - `(*Client).Login(ctx context.Context) error`
  - `(*Client).Token() string`
  - `(*Client).System(ctx context.Context) (SystemInfo, error)`
  - `zgw.ErrBadPassword`, `zgw.ErrUnauthorized`, `zgw.ErrUnreachable` (Sentinel-Fehler mit deutschem Text)

- [ ] **Step 1: Fehlschlagende Tests schreiben**

`internal/zgw/client_test.go`:

```go
package zgw

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

const testToken = "9BE80DF2-9799-467E-9D2C-AC736E8EB6C9"

// loginHandler beantwortet POST /api/login und prueft das Passwort.
func loginHandler(t *testing.T, want string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Login struct {
				Password string `json:"password"`
			} `json:"login"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if body.Login.Password != want {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"accessToken":"`+testToken+`"}`)
	}
}

func TestLoginStoresToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/login" {
			t.Errorf("unerwarteter Request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		loginHandler(t, "geheim")(w, r)
	}))
	defer srv.Close()

	c, err := NewClient(srv.URL, "geheim")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := c.Login(context.Background()); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if c.Token() != testToken {
		t.Fatalf("Token = %q, erwartet %q", c.Token(), testToken)
	}
}

func TestLoginWithWrongPassword(t *testing.T) {
	srv := httptest.NewServer(loginHandler(t, "geheim"))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "falsch")
	err := c.Login(context.Background())
	if !errors.Is(err, ErrBadPassword) {
		t.Fatalf("Login = %v, erwartet ErrBadPassword", err)
	}
}

func TestSystemSendsAccessTokenHeader(t *testing.T) {
	var seen atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login":
			loginHandler(t, "geheim")(w, r)
		case "/api/system":
			seen.Store(r.Header.Get("accessToken"))
			io.WriteString(w, `{"gateway":{"type":"ZGW16WL-IP","name":"Keller","version":"1.2.3","serialNumber":"AB-12"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "geheim")
	if err := c.Login(context.Background()); err != nil {
		t.Fatalf("Login: %v", err)
	}
	info, err := c.System(context.Background())
	if err != nil {
		t.Fatalf("System: %v", err)
	}
	if info.Gateway.Name != "Keller" {
		t.Errorf("Gateway.Name = %q, erwartet \"Keller\"", info.Gateway.Name)
	}
	if seen.Load() != testToken {
		t.Errorf("Header accessToken = %v, erwartet %q", seen.Load(), testToken)
	}
}

func TestRelogsInAfterTokenExpiry(t *testing.T) {
	var logins, systemCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login":
			atomic.AddInt32(&logins, 1)
			loginHandler(t, "geheim")(w, r)
		case "/api/system":
			// Der erste Datenabruf laeuft in das abgelaufene Token.
			if atomic.AddInt32(&systemCalls, 1) == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			io.WriteString(w, `{"gateway":{"name":"Keller"}}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	c, _ := NewClient(srv.URL, "geheim")
	if err := c.Login(context.Background()); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if _, err := c.System(context.Background()); err != nil {
		t.Fatalf("System nach Ablauf: %v", err)
	}
	if logins != 2 {
		t.Errorf("logins = %d, erwartet 2 (Erstanmeldung plus Neuanmeldung)", logins)
	}
	if systemCalls != 2 {
		t.Errorf("systemCalls = %d, erwartet 2 (Fehlschlag plus Wiederholung)", systemCalls)
	}
}

func TestUnreachableHostGivesGermanError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close() // Port ist jetzt zu.

	c, _ := NewClient(url, "geheim")
	err := c.Login(context.Background())
	if !errors.Is(err, ErrUnreachable) {
		t.Fatalf("Login = %v, erwartet ErrUnreachable", err)
	}
}
```

- [ ] **Step 2: Tests laufen lassen, Fehlschlag bestätigen**

Run: `go test ./internal/zgw/ -v`
Expected: FAIL — `undefined: NewClient`, `undefined: ErrBadPassword`

- [ ] **Step 3: `internal/zgw/client.go` schreiben**

```go
package zgw

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Sentinel-Fehler. Der Text ist bereits die Meldung, die der Benutzer sieht.
var (
	ErrBadPassword  = errors.New("Passwort falsch")
	ErrUnauthorized = errors.New("nicht angemeldet")
	ErrUnreachable  = errors.New("Gateway nicht erreichbar")
)

// requestTimeout begrenzt jede einzelne Anfrage an das Gateway.
const requestTimeout = 30 * time.Second

// Client spricht mit der REST-API eines ZGW16-IP.
//
// Das Zugangstoken des Geraets verfaellt nach 15 Minuten. Der Client haelt
// deshalb das Passwort und meldet sich bei einem 401 selbsttaetig neu an;
// fuer Aufrufer ist das unsichtbar.
type Client struct {
	baseURL  string
	password string
	http     *http.Client

	mu    sync.Mutex
	token string
}

// NewClient erzeugt einen Client. host darf "zgw16-ip.local",
// "192.168.1.50" oder eine vollstaendige URL sein.
func NewClient(host, password string) (*Client, error) {
	base, err := normalizeBaseURL(host)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL:  base,
		password: password,
		http:     &http.Client{Timeout: requestTimeout},
	}, nil
}

// Token gibt das aktuelle Zugangstoken zurueck.
func (c *Client) Token() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token
}

func (c *Client) setToken(t string) {
	c.mu.Lock()
	c.token = t
	c.mu.Unlock()
}

// Login holt ein neues Zugangstoken.
func (c *Client) Login(ctx context.Context) error {
	payload, err := json.Marshal(map[string]any{
		"login": map[string]string{"password": c.password},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/login", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusBadRequest {
		return ErrBadPassword
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Anmeldung fehlgeschlagen (HTTP %d)", resp.StatusCode)
	}

	var out struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return fmt.Errorf("Antwort der Anmeldung nicht lesbar: %v", err)
	}
	if out.AccessToken == "" {
		return errors.New("Anmeldung lieferte kein Zugangstoken")
	}
	c.setToken(out.AccessToken)
	return nil
}

// get ruft einen Endpunkt ab und meldet sich bei abgelaufenem Token
// einmal neu an, bevor es aufgibt.
func (c *Client) get(ctx context.Context, path string, out any) error {
	err := c.doGet(ctx, path, out)
	if !errors.Is(err, ErrUnauthorized) {
		return err
	}
	if err := c.Login(ctx); err != nil {
		return err
	}
	return c.doGet(ctx, path, out)
}

func (c *Client) doGet(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("accessToken", c.Token())
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return ErrUnauthorized
	case resp.StatusCode != http.StatusOK:
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("Gateway antwortete mit HTTP %d: %s", resp.StatusCode, bytes.TrimSpace(body))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("Antwort nicht lesbar: %v", err)
	}
	return nil
}

// System liefert Typ, Name und Firmwarestand des Gateways.
func (c *Client) System(ctx context.Context) (SystemInfo, error) {
	var info SystemInfo
	err := c.get(ctx, "/system", &info)
	return info, err
}
```

- [ ] **Step 4: Tests laufen lassen**

Run: `go test ./internal/zgw/ -v`
Expected: PASS, alle fünf Tests

- [ ] **Step 5: Commit**

```bash
git add internal/zgw
git commit -m "feat(zgw): API-Client mit Anmeldung und automatischer Neuanmeldung"
```

---

### Task 4: Zähler, Zählertypen und History abrufen

**Files:**
- Modify: `internal/zgw/client.go` (drei Methoden anhängen)
- Test: `internal/zgw/client_data_test.go`

**Interfaces:**
- Consumes: `(*Client).get` aus Task 3
- Produces:
  - `(*Client).Devices(ctx context.Context) ([]Device, error)`
  - `(*Client).DeviceTypes(ctx context.Context) ([]DeviceType, error)`
  - `(*Client).History(ctx context.Context, busAddress, register, timeFrame int) (History, error)`
  - `zgw.ValidTimeFrames = []int{1, 14, 365, 1095}`

- [ ] **Step 1: Fehlschlagende Tests schreiben**

`internal/zgw/client_data_test.go`:

```go
package zgw

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// dataServer beantwortet Anmeldung, Geraeteliste, Typenliste und History
// mit den Beispieldaten aus der OpenAPI-Spezifikation.
func dataServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/login":
			loginHandler(t, "geheim")(w, r)
		case "/api/devices":
			io.WriteString(w, `{"devices":[
				{"friendlyId":"Garage","deviceTypeId":"7f43","busAddress":4,"selectedHistory":[30053,30073]},
				{"friendlyId":"","deviceTypeId":"7f43","busAddress":7,"selectedHistory":[30073]}
			]}`)
		case "/api/devices/types":
			io.WriteString(w, `{"deviceTypes":[{"deviceTypeId":"7f43","name":"DSZ15DZMOD","registers":[
				{"number":30053,"unit":"Watt","description":[{"en":"Total active power"},{"de":"Gesamtwirkleistung"}]},
				{"number":30073,"unit":"kWh","description":[{"en":"Total imported energy"}]}
			]}]}`)
		case "/api/devices/4/history":
			if got, want := r.URL.Query().Get("identifier"), "30073"; got != want {
				t.Errorf("identifier = %q, erwartet %q", got, want)
			}
			if got, want := r.URL.Query().Get("timeFrame"), "14"; got != want {
				t.Errorf("timeFrame = %q, erwartet %q", got, want)
			}
			io.WriteString(w, `{"identifier":30073,"values":[
				{"timestamp":"2023-08-19T13:18:19.743+02:00","value":25.6},
				{"timestamp":"2023-08-19T13:18:25.743+02:00","value":29.6}
			]}`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func connectedClient(t *testing.T, url string) *Client {
	t.Helper()
	c, err := NewClient(url, "geheim")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if err := c.Login(context.Background()); err != nil {
		t.Fatalf("Login: %v", err)
	}
	return c
}

func TestDevices(t *testing.T) {
	srv := dataServer(t)
	defer srv.Close()

	devices, err := connectedClient(t, srv.URL).Devices(context.Background())
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	if len(devices) != 2 {
		t.Fatalf("len(devices) = %d, erwartet 2", len(devices))
	}
	if devices[0].FriendlyId != "Garage" || devices[0].BusAddress != 4 {
		t.Errorf("devices[0] = %+v", devices[0])
	}
	if len(devices[0].SelectedHistory) != 2 {
		t.Errorf("SelectedHistory = %v, erwartet zwei Eintraege", devices[0].SelectedHistory)
	}
}

func TestDeviceTypes(t *testing.T) {
	srv := dataServer(t)
	defer srv.Close()

	types, err := connectedClient(t, srv.URL).DeviceTypes(context.Background())
	if err != nil {
		t.Fatalf("DeviceTypes: %v", err)
	}
	if len(types) != 1 || types[0].Name != "DSZ15DZMOD" {
		t.Fatalf("types = %+v", types)
	}
	if len(types[0].Registers) != 2 || types[0].Registers[0].Unit != "Watt" {
		t.Errorf("Registers = %+v", types[0].Registers)
	}
}

func TestHistory(t *testing.T) {
	srv := dataServer(t)
	defer srv.Close()

	h, err := connectedClient(t, srv.URL).History(context.Background(), 4, 30073, 14)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if h.Identifier != 30073 {
		t.Errorf("Identifier = %d, erwartet 30073", h.Identifier)
	}
	if len(h.Values) != 2 || h.Values[0].Value != 25.6 {
		t.Fatalf("Values = %+v", h.Values)
	}
	if h.Values[0].Timestamp != "2023-08-19T13:18:19.743+02:00" {
		t.Errorf("Timestamp = %q", h.Values[0].Timestamp)
	}
}

func TestHistoryRejectsUnknownTimeFrame(t *testing.T) {
	srv := dataServer(t)
	defer srv.Close()

	_, err := connectedClient(t, srv.URL).History(context.Background(), 4, 30073, 7)
	if err == nil {
		t.Fatal("timeFrame 7 ist nicht erlaubt und muss abgelehnt werden")
	}
}
```

- [ ] **Step 2: Tests laufen lassen, Fehlschlag bestätigen**

Run: `go test ./internal/zgw/ -run "TestDevices|TestDeviceTypes|TestHistory" -v`
Expected: FAIL — `c.Devices undefined`

- [ ] **Step 3: Methoden an `internal/zgw/client.go` anhängen**

```go
// ValidTimeFrames sind die vier Zeitstufen, die das Geraet kennt.
// 1 Tag in 15-Minuten-Schritten, 14 Tage taeglich, 365 Tage monatlich,
// 1095 Tage jaehrlich.
var ValidTimeFrames = []int{1, 14, 365, 1095}

func validTimeFrame(tf int) bool {
	for _, v := range ValidTimeFrames {
		if v == tf {
			return true
		}
	}
	return false
}

// Devices liefert alle am Gateway bekannten Zaehler.
func (c *Client) Devices(ctx context.Context) ([]Device, error) {
	var out devicesResponse
	if err := c.get(ctx, "/devices", &out); err != nil {
		return nil, err
	}
	return out.Devices, nil
}

// DeviceTypes liefert die Registertabellen aller bekannten Zaehlertypen.
func (c *Client) DeviceTypes(ctx context.Context) ([]DeviceType, error) {
	var out deviceTypesResponse
	if err := c.get(ctx, "/devices/types", &out); err != nil {
		return nil, err
	}
	return out.DeviceTypes, nil
}

// History liefert die aufgezeichneten Werte eines Registers.
func (c *Client) History(ctx context.Context, busAddress, register, timeFrame int) (History, error) {
	var out History
	if !validTimeFrame(timeFrame) {
		return out, fmt.Errorf("unzulaessiger Zeitraum %d Tage", timeFrame)
	}
	path := fmt.Sprintf("/devices/%d/history?identifier=%d&timeFrame=%d", busAddress, register, timeFrame)
	err := c.get(ctx, path, &out)
	return out, err
}
```

- [ ] **Step 4: Tests laufen lassen**

Run: `go test ./internal/zgw/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/zgw
git commit -m "feat(zgw): Zaehler, Zaehlertypen und History abrufen"
```

---

### Task 5: Registerkatalog aus Zählern und Typen

**Files:**
- Create: `internal/zgw/catalog.go`
- Test: `internal/zgw/catalog_test.go`

**Interfaces:**
- Consumes: `Device`, `DeviceType`, `Register` aus Task 2
- Produces:
  - `zgw.RecordedRegister` mit Feldern `Number int`, `Name string`, `Unit string`
  - `(RecordedRegister).ColumnHeader() string`
  - `zgw.MeterInfo` mit Feldern `BusAddress int`, `Name string`, `TypeName string`, `Registers []RecordedRegister`
  - `(MeterInfo).RegisterByNumber(n int) RecordedRegister`
  - `zgw.BuildCatalog(devices []Device, types []DeviceType) []MeterInfo`

- [ ] **Step 1: Fehlschlagende Tests schreiben**

`internal/zgw/catalog_test.go`:

```go
package zgw

import "testing"

func testTypes() []DeviceType {
	return []DeviceType{{
		DeviceTypeId: "7f43",
		Name:         "DSZ15DZMOD",
		Registers: []Register{
			{Number: 30053, Unit: "Watt", Description: []map[string]string{{"en": "Total active power"}, {"de": "Gesamtwirkleistung"}}},
			{Number: 30073, Unit: "kWh", Description: []map[string]string{{"en": "Total imported energy"}}},
			{Number: 30075, Unit: "", Description: nil},
		},
	}}
}

func TestBuildCatalogPrefersGermanNames(t *testing.T) {
	devices := []Device{{FriendlyId: "Garage", DeviceTypeId: "7f43", BusAddress: 4, SelectedHistory: []int{30053, 30073, 30075}}}

	meters := BuildCatalog(devices, testTypes())
	if len(meters) != 1 {
		t.Fatalf("len(meters) = %d, erwartet 1", len(meters))
	}
	m := meters[0]
	if m.Name != "Garage" || m.BusAddress != 4 || m.TypeName != "DSZ15DZMOD" {
		t.Fatalf("MeterInfo = %+v", m)
	}
	if len(m.Registers) != 3 {
		t.Fatalf("len(Registers) = %d, erwartet 3", len(m.Registers))
	}
	if m.Registers[0].Name != "Gesamtwirkleistung" {
		t.Errorf("deutscher Name bevorzugt: %q", m.Registers[0].Name)
	}
	if m.Registers[1].Name != "Total imported energy" {
		t.Errorf("englischer Name als Rueckfall: %q", m.Registers[1].Name)
	}
	if m.Registers[2].Name != "Register 30075" {
		t.Errorf("Registernummer als letzter Rueckfall: %q", m.Registers[2].Name)
	}
}

func TestBuildCatalogNamesUnnamedMeters(t *testing.T) {
	devices := []Device{{FriendlyId: "", DeviceTypeId: "7f43", BusAddress: 7, SelectedHistory: []int{30053}}}

	meters := BuildCatalog(devices, testTypes())
	if meters[0].Name != "Zaehler_7" {
		t.Errorf("Name = %q, erwartet \"Zaehler_7\"", meters[0].Name)
	}
}

func TestBuildCatalogHandlesUnknownRegisterAndType(t *testing.T) {
	devices := []Device{
		{FriendlyId: "A", DeviceTypeId: "7f43", BusAddress: 1, SelectedHistory: []int{99999}},
		{FriendlyId: "B", DeviceTypeId: "unbekannt", BusAddress: 2, SelectedHistory: []int{30053}},
	}

	meters := BuildCatalog(devices, testTypes())
	if meters[0].Registers[0].Name != "Register 99999" {
		t.Errorf("unbekanntes Register: %q", meters[0].Registers[0].Name)
	}
	if meters[1].TypeName != "unbekannt" {
		t.Errorf("unbekannter Typ: TypeName = %q", meters[1].TypeName)
	}
	if meters[1].Registers[0].Name != "Register 30053" {
		t.Errorf("Register eines unbekannten Typs: %q", meters[1].Registers[0].Name)
	}
}

func TestColumnHeader(t *testing.T) {
	withUnit := RecordedRegister{Number: 30053, Name: "Gesamtwirkleistung", Unit: "Watt"}
	if got, want := withUnit.ColumnHeader(), "Gesamtwirkleistung [Watt]"; got != want {
		t.Errorf("ColumnHeader = %q, erwartet %q", got, want)
	}
	withoutUnit := RecordedRegister{Number: 30075, Name: "Register 30075"}
	if got, want := withoutUnit.ColumnHeader(), "Register 30075"; got != want {
		t.Errorf("ColumnHeader = %q, erwartet %q", got, want)
	}
}

func TestRegisterByNumber(t *testing.T) {
	m := MeterInfo{Registers: []RecordedRegister{{Number: 30053, Name: "Gesamtwirkleistung"}}}
	if m.RegisterByNumber(30053).Name != "Gesamtwirkleistung" {
		t.Error("bekanntes Register nicht gefunden")
	}
	if got, want := m.RegisterByNumber(1).Name, "Register 1"; got != want {
		t.Errorf("unbekanntes Register = %q, erwartet %q", got, want)
	}
}
```

- [ ] **Step 2: Tests laufen lassen, Fehlschlag bestätigen**

Run: `go test ./internal/zgw/ -run "TestBuildCatalog|TestColumnHeader|TestRegisterByNumber" -v`
Expected: FAIL — `undefined: BuildCatalog`

- [ ] **Step 3: `internal/zgw/catalog.go` schreiben**

```go
package zgw

import "fmt"

// RecordedRegister ist ein Register, das das Gateway fuer einen Zaehler
// tatsaechlich aufzeichnet, angereichert um Klartextname und Einheit.
type RecordedRegister struct {
	Number int    `json:"number"`
	Name   string `json:"name"`
	Unit   string `json:"unit"`
}

// ColumnHeader ist die Spaltenueberschrift in der CSV.
func (r RecordedRegister) ColumnHeader() string {
	if r.Unit == "" {
		return r.Name
	}
	return r.Name + " [" + r.Unit + "]"
}

// MeterInfo fasst einen Zaehler mit seinen aufgezeichneten Registern
// zusammen. Das ist die Form, die die Oberflaeche anzeigt.
type MeterInfo struct {
	BusAddress int                `json:"busAddress"`
	Name       string             `json:"name"`
	TypeName   string             `json:"typeName"`
	Registers  []RecordedRegister `json:"registers"`
}

// RegisterByNumber findet ein Register. Fuer eine unbekannte Nummer
// entsteht ein Platzhalter, damit Aufrufer keinen Sonderfall brauchen.
func (m MeterInfo) RegisterByNumber(n int) RecordedRegister {
	for _, r := range m.Registers {
		if r.Number == n {
			return r
		}
	}
	return RecordedRegister{Number: n, Name: fallbackRegisterName(n)}
}

func fallbackRegisterName(n int) string {
	return fmt.Sprintf("Register %d", n)
}

// registerName bevorzugt die deutsche Beschreibung, faellt auf die
// englische zurueck und zuletzt auf die nackte Registernummer.
func registerName(r Register) string {
	var english string
	for _, entry := range r.Description {
		if de, ok := entry["de"]; ok && de != "" {
			return de
		}
		if en, ok := entry["en"]; ok && en != "" && english == "" {
			english = en
		}
	}
	if english != "" {
		return english
	}
	return fallbackRegisterName(r.Number)
}

// BuildCatalog verknuepft die Zaehlerliste mit den Registertabellen der
// Zaehlertypen. Unbekannte Typen und unbekannte Register fuehren zu
// Platzhaltern, nicht zu Fehlern: was das Gateway aufzeichnet, soll
// abrufbar bleiben, auch wenn die Beschreibung fehlt.
func BuildCatalog(devices []Device, types []DeviceType) []MeterInfo {
	byType := make(map[string]DeviceType, len(types))
	for _, t := range types {
		byType[t.DeviceTypeId] = t
	}

	meters := make([]MeterInfo, 0, len(devices))
	for _, d := range devices {
		t, known := byType[d.DeviceTypeId]

		typeName := t.Name
		if !known || typeName == "" {
			typeName = "unbekannt"
		}

		name := d.FriendlyId
		if name == "" {
			name = fmt.Sprintf("Zaehler_%d", d.BusAddress)
		}

		byNumber := make(map[int]Register, len(t.Registers))
		for _, r := range t.Registers {
			byNumber[r.Number] = r
		}

		regs := make([]RecordedRegister, 0, len(d.SelectedHistory))
		for _, n := range d.SelectedHistory {
			if r, ok := byNumber[n]; ok {
				regs = append(regs, RecordedRegister{Number: n, Name: registerName(r), Unit: r.Unit})
				continue
			}
			regs = append(regs, RecordedRegister{Number: n, Name: fallbackRegisterName(n)})
		}

		meters = append(meters, MeterInfo{
			BusAddress: d.BusAddress,
			Name:       name,
			TypeName:   typeName,
			Registers:  regs,
		})
	}
	return meters
}
```

- [ ] **Step 4: Tests laufen lassen**

Run: `go test ./internal/zgw/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/zgw
git commit -m "feat(zgw): Registerkatalog aus Zaehlern und Typentabelle"
```

---

### Task 6: Dateinamen

**Files:**
- Create: `internal/export/filename.go`
- Test: `internal/export/filename_test.go`

**Interfaces:**
- Consumes: nichts
- Produces:
  - `export.SafeName(s string) string`
  - `export.FileName(meter string, timeFrame int, t time.Time) string`

- [ ] **Step 1: Fehlschlagenden Test schreiben**

`internal/export/filename_test.go`:

```go
package export

import (
	"testing"
	"time"
)

func TestSafeName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Garage", "Garage"},
		{"Wohnung 3", "Wohnung 3"},
		{`A/B\C:D*E?F"G<H>I|J`, "A_B_C_D_E_F_G_H_I_J"},
		{"  Rand  ", "Rand"},
		{"Zeilen\numbruch", "Zeilen_umbruch"},
		{"", "unbenannt"},
		{"///", "unbenannt"},
	}
	for _, c := range cases {
		if got := SafeName(c.in); got != c.want {
			t.Errorf("SafeName(%q) = %q, erwartet %q", c.in, got, c.want)
		}
	}
}

func TestFileName(t *testing.T) {
	stamp := time.Date(2026, 9, 5, 8, 15, 30, 0, time.UTC)
	got := FileName("Garage", 14, stamp)
	want := "ZGW_Garage_14d_20260905-081530.csv"
	if got != want {
		t.Errorf("FileName = %q, erwartet %q", got, want)
	}
}
```

- [ ] **Step 2: Test laufen lassen, Fehlschlag bestätigen**

Run: `go test ./internal/export/ -v`
Expected: FAIL — `undefined: SafeName`

- [ ] **Step 3: `internal/export/filename.go` schreiben**

```go
package export

import (
	"fmt"
	"strings"
	"time"
)

// SafeName ersetzt alles, was Windows in Dateinamen nicht zulaesst,
// durch einen Unterstrich.
func SafeName(s string) string {
	const forbidden = `<>:"/\|?*`
	var b strings.Builder
	for _, r := range s {
		if r < 0x20 || strings.ContainsRune(forbidden, r) {
			b.WriteRune('_')
			continue
		}
		b.WriteRune(r)
	}
	out := strings.Trim(b.String(), " .")
	if strings.Trim(out, "_") == "" {
		return "unbenannt"
	}
	return out
}

// FileName baut den Dateinamen fuer einen Zaehler und eine Zeitstufe.
func FileName(meter string, timeFrame int, t time.Time) string {
	return fmt.Sprintf("ZGW_%s_%dd_%s.csv", SafeName(meter), timeFrame, t.Format("20060102-150405"))
}
```

- [ ] **Step 4: Test laufen lassen**

Run: `go test ./internal/export/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/export
git commit -m "feat(export): Dateinamen erzeugen und bereinigen"
```

---

### Task 7: CSV-Writer

**Files:**
- Create: `internal/export/csv.go`
- Test: `internal/export/csv_test.go`

**Interfaces:**
- Consumes: nichts aus anderen Tasks
- Produces:
  - `export.Point` mit Feldern `Time time.Time`, `Raw string`, `Value float64`
  - `export.NewPoint(iso string, value float64) (Point, error)`
  - `export.Series` mit Feldern `Header string`, `Points []Point`
  - `export.Write(w io.Writer, series []Series) error`
  - `export.WriteFile(path string, series []Series) error`

- [ ] **Step 1: Fehlschlagende Tests schreiben**

`internal/export/csv_test.go`:

```go
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
```

- [ ] **Step 2: Tests laufen lassen, Fehlschlag bestätigen**

Run: `go test ./internal/export/ -v`
Expected: FAIL — `undefined: NewPoint`, `undefined: Write`

- [ ] **Step 3: `internal/export/csv.go` schreiben**

```go
package export

import (
	"encoding/csv"
	"errors"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// utf8BOM sorgt dafuer, dass Excel die Datei als UTF-8 erkennt.
const utf8BOM = "\xef\xbb\xbf"

// germanTimeLayout ist die Darstellung, die deutsches Excel als
// Datum-Uhrzeit einliest.
const germanTimeLayout = "02.01.2006 15:04:05"

// Point ist ein Messwert. Raw haelt die unveraenderte Zeitangabe der
// API, damit die zweite Spalte der CSV die Zweideutigkeit der
// Zeitumstellung aufloesen kann.
type Point struct {
	Time  time.Time
	Raw   string
	Value float64
}

// NewPoint liest einen Zeitstempel im Format der API, zum Beispiel
// "2026-09-05T08:15:00.000+02:00". Der Zonenversatz bleibt erhalten;
// es wird nicht in Ortszeit oder UTC umgerechnet.
func NewPoint(iso string, value float64) (Point, error) {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return Point{}, err
	}
	return Point{Time: t, Raw: iso, Value: value}, nil
}

// Series ist eine Messreihe, also eine Spalte der spaeteren Tabelle.
type Series struct {
	Header string
	Points []Point
}

type row struct {
	when time.Time
	raw  string
	vals map[int]float64
}

// Write erzeugt eine breite Tabelle: eine Zeile je Zeitstempel, eine
// Spalte je Messreihe.
//
// Die Zeitstempel verschiedener Register desselben Zaehlers sollten
// uebereinstimmen, garantiert ist das nicht. Deshalb wird ueber die
// Vereinigungsmenge aller Zeitstempel gebaut und eine fehlende Zelle
// bleibt leer, statt dass Zeilen gegeneinander verrutschen.
func Write(w io.Writer, series []Series) error {
	if len(series) == 0 {
		return errors.New("keine Messreihen zum Schreiben")
	}

	rows := make(map[int64]*row)
	for i, s := range series {
		for _, p := range s.Points {
			key := p.Time.UnixNano()
			r, ok := rows[key]
			if !ok {
				r = &row{when: p.Time, raw: p.Raw, vals: make(map[int]float64, len(series))}
				rows[key] = r
			}
			r.vals[i] = p.Value
		}
	}

	keys := make([]int64, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool { return keys[a] < keys[b] })

	if _, err := io.WriteString(w, utf8BOM); err != nil {
		return err
	}

	cw := csv.NewWriter(w)
	cw.Comma = ';'
	cw.UseCRLF = true

	header := make([]string, 0, len(series)+2)
	header = append(header, "Zeitstempel", "Zeitstempel_ISO")
	for _, s := range series {
		header = append(header, s.Header)
	}
	if err := cw.Write(header); err != nil {
		return err
	}

	record := make([]string, len(header))
	for _, k := range keys {
		r := rows[k]
		record[0] = r.when.Format(germanTimeLayout)
		record[1] = r.raw
		for i := range series {
			if v, ok := r.vals[i]; ok {
				record[i+2] = germanNumber(v)
				continue
			}
			record[i+2] = ""
		}
		if err := cw.Write(record); err != nil {
			return err
		}
	}

	cw.Flush()
	return cw.Error()
}

// WriteFile schreibt die Tabelle in eine Datei.
func WriteFile(path string, series []Series) error {
	if len(series) == 0 {
		return errors.New("keine Messreihen zum Schreiben")
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := Write(f, series); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// germanNumber gibt die Zahl mit Komma als Dezimaltrenner aus, ohne
// unnoetige Nachkommastellen.
func germanNumber(v float64) string {
	return strings.Replace(strconv.FormatFloat(v, 'f', -1, 64), ".", ",", 1)
}
```

- [ ] **Step 4: Tests laufen lassen**

Run: `go test ./internal/export/ -v`
Expected: PASS, alle acht Tests

- [ ] **Step 5: Commit**

```bash
git add internal/export
git commit -m "feat(export): CSV-Writer im deutschen Excel-Dialekt"
```

---

### Task 8: Konfiguration und Passwortablage

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/secret_windows.go`
- Create: `internal/config/secret_other.go`
- Test: `internal/config/config_test.go`

**Interfaces:**
- Consumes: nichts
- Produces:
  - `config.Config` mit Feldern `Host string`, `OutputDir string`, `Remember bool`, `Secret string`
  - `config.Load() Config`
  - `config.Save(c Config) error`
  - `config.Path() (string, error)`
  - `config.Protect(plaintext string) (string, error)`
  - `config.Unprotect(encoded string) (string, error)`

- [ ] **Step 1: Fehlschlagende Tests schreiben**

`internal/config/config_test.go`:

```go
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
```

- [ ] **Step 2: Tests laufen lassen, Fehlschlag bestätigen**

Run: `go test ./internal/config/ -v`
Expected: FAIL — `undefined: Load`

- [ ] **Step 3: `internal/config/config.go` schreiben**

```go
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
```

- [ ] **Step 4: `internal/config/secret_windows.go` schreiben**

```go
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
```

- [ ] **Step 5: `internal/config/secret_other.go` schreiben**

Nur damit `go vet ./...` und Tests auch außerhalb von Windows übersetzen. Das Programm selbst wird ausschließlich für Windows gebaut.

```go
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
```

- [ ] **Step 6: Tests laufen lassen**

Run: `go test ./internal/config/ -v`
Expected: PASS, alle sechs Tests

- [ ] **Step 7: Gesamtlauf**

Run: `go test ./... && go vet ./...`
Expected: alles PASS, keine vet-Meldungen

- [ ] **Step 8: Commit**

```bash
git add internal/config
git commit -m "feat(config): Einstellungen mit DPAPI-geschuetztem Passwort"
```

---

### Task 9: Wails-Bindings und Export-Ablauf

**Files:**
- Modify: `app.go` (Inhalt des Templates vollständig ersetzen)
- Modify: `main.go` (Fenstertitel und -größe)

**Interfaces:**
- Consumes: alles aus den Tasks 3 bis 8
- Produces (an das Frontend gebunden):
  - `LoadSettings() Settings`
  - `Connect(host, password string, remember bool) (ConnectResult, error)`
  - `ChooseFolder() (string, error)`
  - `StartExport(req ExportRequest) error`
  - `CancelExport()`
  - Events `export:progress`, `export:log`, `export:done`

- [ ] **Step 1: `app.go` schreiben**

Dieser Task hat keinen eigenen Unit-Test: er verdrahtet nur getestete Bausteine und braucht eine laufende Wails-Umgebung. Geprüft wird er in Task 11 am Gerät.

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"zgwhistory/internal/config"
	"zgwhistory/internal/export"
	"zgwhistory/internal/zgw"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Settings ist das, was die Oberflaeche beim Start in das Formular fuellt.
type Settings struct {
	Host      string `json:"host"`
	Password  string `json:"password"`
	Remember  bool   `json:"remember"`
	OutputDir string `json:"outputDir"`
	TimeFrames []int `json:"timeFrames"`
}

// ConnectResult ist die Antwort auf eine erfolgreiche Anmeldung.
type ConnectResult struct {
	Gateway string          `json:"gateway"`
	Meters  []zgw.MeterInfo `json:"meters"`
}

// MeterSelection ist die Auswahl fuer genau einen Zaehler.
type MeterSelection struct {
	BusAddress int   `json:"busAddress"`
	Registers  []int `json:"registers"`
}

// ExportRequest ist der komplette Auftrag aus der Oberflaeche.
type ExportRequest struct {
	Meters     []MeterSelection `json:"meters"`
	TimeFrames []int            `json:"timeFrames"`
	OutputDir  string           `json:"outputDir"`
}

// Summary ist die Bilanz eines Laufs.
type Summary struct {
	Total     int    `json:"total"`
	Written   int    `json:"written"`
	Failed    int    `json:"failed"`
	Skipped   int    `json:"skipped"`
	Cancelled bool   `json:"cancelled"`
	Message   string `json:"message"`
}

// App haelt den Zustand zwischen Oberflaeche und Geraet.
type App struct {
	ctx context.Context

	mu      sync.Mutex
	client  *zgw.Client
	meters  []zgw.MeterInfo
	cancel  context.CancelFunc
	running bool
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// LoadSettings liefert die gespeicherten Einstellungen. Ein nicht
// entschluesselbares Passwort ist kein Fehler: das Feld bleibt leer.
func (a *App) LoadSettings() Settings {
	c := config.Load()
	s := Settings{
		Host:       c.Host,
		Remember:   c.Remember,
		OutputDir:  c.OutputDir,
		TimeFrames: zgw.ValidTimeFrames,
	}
	if c.Remember {
		if pw, err := config.Unprotect(c.Secret); err == nil {
			s.Password = pw
		}
	}
	return s
}

// Connect meldet sich am Gateway an und laedt Zaehler und Register.
func (a *App) Connect(host, password string, remember bool) (ConnectResult, error) {
	var res ConnectResult

	client, err := zgw.NewClient(host, password)
	if err != nil {
		return res, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	defer cancel()

	if err := client.Login(ctx); err != nil {
		return res, err
	}
	info, err := client.System(ctx)
	if err != nil {
		return res, err
	}
	devices, err := client.Devices(ctx)
	if err != nil {
		return res, err
	}
	if len(devices) == 0 {
		return res, errors.New("das Gateway meldet keine Zaehler. Modbus-RTU-Einstellungen pruefen.")
	}
	types, err := client.DeviceTypes(ctx)
	if err != nil {
		return res, err
	}

	meters := zgw.BuildCatalog(devices, types)

	a.mu.Lock()
	a.client = client
	a.meters = meters
	a.mu.Unlock()

	a.saveSettings(host, password, remember)

	res.Gateway = fmt.Sprintf("%s (%s)", info.Gateway.Name, info.Gateway.Type)
	res.Meters = meters
	return res, nil
}

func (a *App) saveSettings(host, password string, remember bool) {
	c := config.Load()
	c.Host = host
	c.Remember = remember
	if remember {
		if secret, err := config.Protect(password); err == nil {
			c.Secret = secret
		}
	} else {
		c.Secret = ""
	}
	_ = config.Save(c)
}

// ChooseFolder oeffnet den nativen Ordnerdialog.
func (a *App) ChooseFolder() (string, error) {
	dir, err := wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title: "Zielordner fuer die CSV-Dateien waehlen",
	})
	if err != nil {
		return "", err
	}
	if dir != "" {
		c := config.Load()
		c.OutputDir = dir
		_ = config.Save(c)
	}
	return dir, nil
}

// StartExport nimmt den Auftrag an und arbeitet ihn im Hintergrund ab.
func (a *App) StartExport(req ExportRequest) error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return errors.New("es laeuft bereits ein Export")
	}
	if a.client == nil {
		a.mu.Unlock()
		return errors.New("nicht mit dem Gateway verbunden")
	}
	if req.OutputDir == "" {
		a.mu.Unlock()
		return errors.New("kein Zielordner gewaehlt")
	}
	if len(req.Meters) == 0 || len(req.TimeFrames) == 0 {
		a.mu.Unlock()
		return errors.New("nichts ausgewaehlt")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	a.running = true
	client := a.client
	meters := a.meters
	a.mu.Unlock()

	go a.runExport(ctx, client, meters, req)
	return nil
}

// CancelExport bricht einen laufenden Export ab.
func (a *App) CancelExport() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
}

func (a *App) runExport(ctx context.Context, client *zgw.Client, meters []zgw.MeterInfo, req ExportRequest) {
	defer func() {
		a.mu.Lock()
		a.running = false
		a.cancel = nil
		a.mu.Unlock()
	}()

	byAddress := make(map[int]zgw.MeterInfo, len(meters))
	for _, m := range meters {
		byAddress[m.BusAddress] = m
	}

	total := 0
	for _, sel := range req.Meters {
		total += len(sel.Registers) * len(req.TimeFrames)
	}

	stamp := time.Now()
	var done, written, failed, skipped int

	for _, sel := range req.Meters {
		meter, ok := byAddress[sel.BusAddress]
		if !ok {
			meter = zgw.MeterInfo{BusAddress: sel.BusAddress, Name: fmt.Sprintf("Zaehler_%d", sel.BusAddress)}
		}

		for _, tf := range req.TimeFrames {
			var series []export.Series

			for _, regNum := range sel.Registers {
				if ctx.Err() != nil {
					a.finish(Summary{Total: total, Written: written, Failed: failed, Skipped: skipped,
						Cancelled: true, Message: "Export abgebrochen."})
					return
				}
				reg := meter.RegisterByNumber(regNum)
				done++
				a.progress(done, total, fmt.Sprintf("%s · %s · %d Tage", meter.Name, reg.Name, tf))

				h, err := client.History(ctx, sel.BusAddress, regNum, tf)
				if err != nil {
					failed++
					a.logLine("error", fmt.Sprintf("%s · %s · %d Tage: %v", meter.Name, reg.Name, tf, err))
					continue
				}
				if len(h.Values) == 0 {
					skipped++
					a.logLine("warn", fmt.Sprintf("%s · %s · %d Tage: keine Daten aufgezeichnet", meter.Name, reg.Name, tf))
					continue
				}

				points := make([]export.Point, 0, len(h.Values))
				var unreadable int
				for _, v := range h.Values {
					p, err := export.NewPoint(v.Timestamp, v.Value)
					if err != nil {
						unreadable++
						continue
					}
					points = append(points, p)
				}
				if unreadable > 0 {
					a.logLine("warn", fmt.Sprintf("%s · %s · %d Tage: %d Werte mit unlesbarem Zeitstempel uebersprungen",
						meter.Name, reg.Name, tf, unreadable))
				}
				if len(points) == 0 {
					skipped++
					continue
				}
				series = append(series, export.Series{Header: reg.ColumnHeader(), Points: points})
			}

			if len(series) == 0 {
				a.logLine("warn", fmt.Sprintf("%s · %d Tage: keine Datei geschrieben, kein Register lieferte Werte", meter.Name, tf))
				continue
			}

			name := export.FileName(meter.Name, tf, stamp)
			path := filepath.Join(req.OutputDir, name)
			if err := export.WriteFile(path, series); err != nil {
				failed++
				a.logLine("error", fmt.Sprintf("%s konnte nicht geschrieben werden: %v", name, err))
				continue
			}
			written++
			a.logLine("info", "Geschrieben: "+name)
		}
	}

	msg := fmt.Sprintf("Fertig. %d Datei(en) geschrieben.", written)
	if failed > 0 {
		msg += fmt.Sprintf(" %d von %d Abrufen fehlgeschlagen.", failed, total)
	}
	if skipped > 0 {
		msg += fmt.Sprintf(" %d Register ohne Daten.", skipped)
	}
	a.finish(Summary{Total: total, Written: written, Failed: failed, Skipped: skipped, Message: msg})
}

func (a *App) progress(done, total int, message string) {
	wailsruntime.EventsEmit(a.ctx, "export:progress", map[string]any{
		"done": done, "total": total, "message": message,
	})
}

func (a *App) logLine(level, message string) {
	wailsruntime.EventsEmit(a.ctx, "export:log", map[string]any{
		"level": level, "message": message,
	})
}

func (a *App) finish(s Summary) {
	wailsruntime.EventsEmit(a.ctx, "export:done", s)
}
```

- [ ] **Step 2: `main.go` anpassen**

Im vom Template erzeugten `main.go` nur die Fensteroptionen ändern:

```go
	err := wails.Run(&options.App{
		Title:  "ZGW History Downloader",
		Width:  980,
		Height: 760,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 250, G: 250, B: 250, A: 1},
		OnStartup:        app.startup,
		Bind: []any{
			app,
		},
	})
```

- [ ] **Step 3: Übersetzen**

Run: `go build ./... && go vet ./...`
Expected: fehlerfrei

- [ ] **Step 4: Bindings erzeugen und Gesamttest**

Run: `"$(go env GOPATH)/bin/wails.exe" build -platform windows/amd64`
Expected: Exit-Code 0; `frontend/wailsjs/go/main/App.js` enthält `LoadSettings`, `Connect`, `ChooseFolder`, `StartExport`, `CancelExport`

Run: `go test ./...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app.go main.go frontend/wailsjs
git commit -m "feat(app): Wails-Bindings und Export-Ablauf"
```

---

### Task 10: Oberfläche

**Files:**
- Modify: `frontend/index.html` (Inhalt vollständig ersetzen)
- Modify: `frontend/src/main.js` (Inhalt vollständig ersetzen)
- Modify: `frontend/src/style.css` (Inhalt vollständig ersetzen)
- Delete: `frontend/src/app.css`, `frontend/src/assets/` (falls vom Template angelegt und nicht mehr referenziert)

**Interfaces:**
- Consumes: die Bindings und Events aus Task 9
- Produces: die fertige Oberfläche

- [ ] **Step 1: `frontend/index.html` schreiben**

```html
<!DOCTYPE html>
<html lang="de">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>ZGW History Downloader</title>
    <link rel="stylesheet" href="./src/style.css" />
  </head>
  <body>
    <main>
      <section class="card">
        <h2>1 · Verbindung</h2>
        <div class="grid">
          <label for="host">Adresse des Gateways</label>
          <input id="host" type="text" placeholder="zgw16-ip.local" />
          <label for="password">Passwort</label>
          <input id="password" type="password" />
          <span></span>
          <label class="inline"><input id="remember" type="checkbox" /> Passwort merken</label>
        </div>
        <div class="actions">
          <button id="connect" class="primary">Verbinden</button>
          <span id="connection-state" class="state"></span>
        </div>
      </section>

      <section class="card" id="selection-card" hidden>
        <h2>2 · Auswahl</h2>
        <div id="meters"></div>
        <h3>Zeitraum</h3>
        <p class="hint">
          Das Gateway kennt nur diese vier Stufen. Je Stufe und Zähler entsteht eine eigene Datei.
        </p>
        <div id="timeframes" class="timeframes"></div>
      </section>

      <section class="card" id="export-card" hidden>
        <h2>3 · Export</h2>
        <div class="actions">
          <input id="outdir" type="text" readonly placeholder="Kein Zielordner gewählt" />
          <button id="browse">Durchsuchen …</button>
        </div>
        <div class="actions">
          <button id="download" class="primary">Herunterladen</button>
          <span id="progress-text" class="state"></span>
        </div>
        <progress id="progress" value="0" max="1"></progress>
        <pre id="log" aria-live="polite"></pre>
      </section>
    </main>
    <script type="module" src="./src/main.js"></script>
  </body>
</html>
```

- [ ] **Step 2: `frontend/src/style.css` schreiben**

```css
:root {
  --bg: #fafafa;
  --card: #ffffff;
  --line: #d8d8d8;
  --text: #1c1c1c;
  --muted: #666;
  --accent: #0b6ab0;
  --error: #b3261e;
  --warn: #8a5a00;
}

* { box-sizing: border-box; }

body {
  margin: 0;
  padding: 16px;
  background: var(--bg);
  color: var(--text);
  font: 14px/1.5 "Segoe UI", system-ui, sans-serif;
}

main { display: flex; flex-direction: column; gap: 16px; }

.card {
  background: var(--card);
  border: 1px solid var(--line);
  border-radius: 8px;
  padding: 16px;
}

h2 { margin: 0 0 12px; font-size: 15px; }
h3 { margin: 16px 0 4px; font-size: 14px; }

.hint { margin: 0 0 8px; color: var(--muted); font-size: 13px; }

.grid {
  display: grid;
  grid-template-columns: 200px 1fr;
  gap: 8px 12px;
  align-items: center;
}

.inline { display: flex; align-items: center; gap: 6px; }

input[type="text"], input[type="password"] {
  padding: 6px 8px;
  border: 1px solid var(--line);
  border-radius: 4px;
  font: inherit;
}

.actions { display: flex; align-items: center; gap: 8px; margin-top: 12px; }
.actions input[type="text"] { flex: 1; background: #f3f3f3; }

button {
  padding: 6px 14px;
  border: 1px solid var(--line);
  border-radius: 4px;
  background: #f3f3f3;
  font: inherit;
  cursor: pointer;
}
button.primary { background: var(--accent); border-color: var(--accent); color: #fff; }
button:disabled { opacity: 0.5; cursor: default; }

.state { color: var(--muted); }
.state.error { color: var(--error); }
.state.ok { color: #1c6b32; }

.meter { border: 1px solid var(--line); border-radius: 6px; margin-bottom: 8px; }
.meter > summary { padding: 8px 10px; cursor: pointer; font-weight: 600; }
.meter .registers { padding: 0 10px 10px 28px; display: flex; flex-direction: column; gap: 4px; }
.meter .type { color: var(--muted); font-weight: 400; }

.timeframes { display: flex; flex-wrap: wrap; gap: 16px; }

progress { width: 100%; height: 10px; margin-top: 10px; }

#log {
  margin: 10px 0 0;
  padding: 8px;
  height: 180px;
  overflow: auto;
  background: #1e1e1e;
  color: #ddd;
  border-radius: 4px;
  font: 12px/1.5 Consolas, monospace;
  white-space: pre-wrap;
}
#log .error { color: #ff8a80; }
#log .warn { color: #ffd180; }
```

- [ ] **Step 3: `frontend/src/main.js` schreiben**

```js
import {
  LoadSettings,
  Connect,
  ChooseFolder,
  StartExport,
  CancelExport,
} from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime/runtime";

const $ = (id) => document.getElementById(id);

// Beschriftung der vier Zeitstufen, die die API kennt.
const TIMEFRAMES = [
  { days: 1, label: "1 Tag — 15-Minuten-Werte" },
  { days: 14, label: "14 Tage — Tageswerte" },
  { days: 365, label: "1 Jahr — Monatswerte" },
  { days: 1095, label: "3 Jahre — Jahreswerte" },
];

let meters = [];
let exporting = false;

function log(message, level = "info") {
  const line = document.createElement("span");
  line.className = level;
  line.textContent = message + "\n";
  $("log").appendChild(line);
  $("log").scrollTop = $("log").scrollHeight;
}

function setState(text, kind = "") {
  const el = $("connection-state");
  el.textContent = text;
  el.className = "state " + kind;
}

function renderTimeframes(selected) {
  $("timeframes").innerHTML = "";
  for (const tf of TIMEFRAMES) {
    const label = document.createElement("label");
    label.className = "inline";
    const box = document.createElement("input");
    box.type = "checkbox";
    box.dataset.days = String(tf.days);
    box.checked = selected.includes(tf.days);
    label.append(box, document.createTextNode(" " + tf.label));
    $("timeframes").appendChild(label);
  }
}

function renderMeters() {
  const host = $("meters");
  host.innerHTML = "";
  for (const m of meters) {
    const details = document.createElement("details");
    details.className = "meter";
    details.open = true;

    const summary = document.createElement("summary");
    const box = document.createElement("input");
    box.type = "checkbox";
    box.checked = true;
    box.dataset.meter = String(m.busAddress);
    box.addEventListener("click", (e) => e.stopPropagation());
    box.addEventListener("change", () => {
      details
        .querySelectorAll("input[data-register]")
        .forEach((r) => (r.checked = box.checked));
    });
    summary.append(
      box,
      document.createTextNode(` Adresse ${m.busAddress} · ${m.name} `)
    );
    const type = document.createElement("span");
    type.className = "type";
    type.textContent = `(${m.typeName})`;
    summary.appendChild(type);
    details.appendChild(summary);

    const list = document.createElement("div");
    list.className = "registers";
    for (const r of m.registers || []) {
      const label = document.createElement("label");
      label.className = "inline";
      const rb = document.createElement("input");
      rb.type = "checkbox";
      rb.checked = true;
      rb.dataset.register = String(r.number);
      rb.dataset.meter = String(m.busAddress);
      const unit = r.unit ? ` [${r.unit}]` : "";
      label.append(rb, document.createTextNode(` ${r.name}${unit}`));
      list.appendChild(label);
    }
    if (!(m.registers || []).length) {
      const empty = document.createElement("span");
      empty.className = "hint";
      empty.textContent = "Für diesen Zähler zeichnet das Gateway nichts auf.";
      list.appendChild(empty);
    }
    details.appendChild(list);
    host.appendChild(details);
  }
}

function collectRequest() {
  const selection = [];
  for (const m of meters) {
    const registers = [
      ...document.querySelectorAll(
        `input[data-register][data-meter="${m.busAddress}"]:checked`
      ),
    ].map((el) => Number(el.dataset.register));
    if (registers.length) {
      selection.push({ busAddress: m.busAddress, registers });
    }
  }
  const timeFrames = [
    ...document.querySelectorAll("#timeframes input:checked"),
  ].map((el) => Number(el.dataset.days));

  return { meters: selection, timeFrames, outputDir: $("outdir").value };
}

function setExporting(active) {
  exporting = active;
  $("download").textContent = active ? "Abbrechen" : "Herunterladen";
  $("connect").disabled = active;
  $("browse").disabled = active;
}

async function connect() {
  $("connect").disabled = true;
  setState("Verbinde …");
  try {
    const res = await Connect(
      $("host").value,
      $("password").value,
      $("remember").checked
    );
    meters = res.meters || [];
    setState(`Verbunden mit ${res.gateway} · ${meters.length} Zähler`, "ok");
    renderMeters();
    $("selection-card").hidden = false;
    $("export-card").hidden = false;
  } catch (err) {
    meters = [];
    $("selection-card").hidden = true;
    $("export-card").hidden = true;
    setState(String(err), "error");
  } finally {
    $("connect").disabled = false;
  }
}

async function browse() {
  try {
    const dir = await ChooseFolder();
    if (dir) $("outdir").value = dir;
  } catch (err) {
    log(String(err), "error");
  }
}

async function download() {
  if (exporting) {
    CancelExport();
    return;
  }
  const req = collectRequest();
  if (!req.meters.length) return log("Kein Register ausgewählt.", "warn");
  if (!req.timeFrames.length) return log("Kein Zeitraum ausgewählt.", "warn");
  if (!req.outputDir) return log("Kein Zielordner gewählt.", "warn");

  $("log").innerHTML = "";
  $("progress").value = 0;
  setExporting(true);
  try {
    await StartExport(req);
  } catch (err) {
    log(String(err), "error");
    setExporting(false);
  }
}

EventsOn("export:progress", (p) => {
  $("progress").max = p.total || 1;
  $("progress").value = p.done || 0;
  $("progress-text").textContent = `${p.done}/${p.total} · ${p.message}`;
});

EventsOn("export:log", (l) => log(l.message, l.level));

EventsOn("export:done", (s) => {
  setExporting(false);
  $("progress-text").textContent = s.message;
  log(s.message, s.failed > 0 ? "warn" : "info");
});

$("connect").addEventListener("click", connect);
$("browse").addEventListener("click", browse);
$("download").addEventListener("click", download);
$("password").addEventListener("keydown", (e) => {
  if (e.key === "Enter") connect();
});

LoadSettings().then((s) => {
  $("host").value = s.host || "zgw16-ip.local";
  $("password").value = s.password || "";
  $("remember").checked = !!s.remember;
  $("outdir").value = s.outputDir || "";
  renderTimeframes([1]);
});
```

- [ ] **Step 4: Nicht mehr benötigte Template-Reste entfernen**

Falls `frontend/src/app.css` und `frontend/src/assets/` vom Template stammen und nirgends mehr referenziert werden, löschen. `frontend/index.html` darf nach Step 1 nur noch `./src/style.css` und `./src/main.js` laden.

- [ ] **Step 5: Bauen und starten**

Run: `"$(go env GOPATH)/bin/wails.exe" build -platform windows/amd64`
Expected: Exit-Code 0

Dann `build/bin/zgwhistory.exe` starten. Erwartet: das Fenster öffnet sich, zeigt Abschnitt 1 mit `zgw16-ip.local` im Adressfeld, Abschnitte 2 und 3 sind noch verborgen.

- [ ] **Step 6: Commit**

```bash
git add frontend
git commit -m "feat(ui): Oberflaeche mit Verbindung, Auswahl und Export"
```

---

### Task 11: Abnahme am Gerät und Auslieferungs-Build

**Files:**
- Create: `README.md`
- Modify: gegebenenfalls `app.go` (nur falls die Skalierungsprüfung es verlangt)

**Interfaces:**
- Consumes: das fertige Programm aus Task 10
- Produces: `build/bin/zgwhistory.exe` als Auslieferungsstand, `README.md`

- [ ] **Step 1: Gegen das echte Gateway verbinden**

Programm starten, Adresse und Passwort eingeben, *Verbinden*.

Erwartet: Gerätename erscheint, die tatsächlich angeschlossenen Zähler werden mit deutschen Registernamen und Einheiten gelistet.

Schlägt es fehl, ist die Meldung deutsch und benennt die Ursache (nicht erreichbar / Passwort falsch).

- [ ] **Step 2: Export über alle vier Zeitstufen**

Alle Zähler, alle Register, alle vier Zeitstufen ankreuzen, Zielordner wählen, *Herunterladen*.

Erwartet: Fortschrittsbalken läuft, je Zähler und Stufe entsteht eine Datei, die Zusammenfassung nennt die Anzahl.

- [ ] **Step 3: Skalierung prüfen — der offene Punkt aus der Spec**

Eine erzeugte Datei in Excel öffnen und die Werte gegen die Anzeige der ELTAKO-Connect-App oder die Weboberfläche des Gateways halten.

Stimmen die Größenordnungen, bleibt es dabei: keine Nachskalierung.

Weichen sie um genau den `scaling_factor` des Registers ab (typisch 100), dann in `runExport` in `app.go` beim Erzeugen der Punkte durch den Faktor teilen. Dafür muss `zgw.RecordedRegister` das Feld `ScalingFactor int` bekommen, das `BuildCatalog` aus `Register` übernimmt — `Register` braucht dann `ScalingFactor int \`json:"scaling_factor"\`` in `model.go`. Diese Änderung erst vornehmen, wenn die Messung sie belegt, und mit einem Test in `catalog_test.go` absichern, dass der Faktor durchgereicht wird.

- [ ] **Step 4: Abbruch prüfen**

Einen großen Export starten und während des Laufs *Abbrechen* drücken.

Erwartet: der Lauf endet zügig, die Zusammenfassung meldet „Export abgebrochen.", bereits geschriebene Dateien bleiben liegen.

- [ ] **Step 5: Passwort-Ablage prüfen**

Mit gesetztem Haken „Passwort merken" verbinden, Programm schließen, neu starten.

Erwartet: Adresse und Passwort sind vorbelegt. `%APPDATA%\ZGW-History-Downloader\config.json` enthält unter `password` einen Base64-Block, **nicht** das Klartextpasswort.

Dann den Haken entfernen, erneut verbinden, Programm neu starten: das Passwortfeld ist leer und `password` in der Datei ist `""`.

- [ ] **Step 6: `README.md` schreiben**

```markdown
# ZGW History Downloader

Lädt die History-Daten eines Eltako ZGW16WL-IP über dessen REST-API und
speichert sie als Excel-taugliche CSV-Dateien.

## Benutzung

`zgwhistory.exe` starten — keine Installation nötig.

1. Adresse des Gateways (Vorgabe `zgw16-ip.local`) und Passwort eingeben,
   *Verbinden*.
2. Zähler, Register und Zeitstufen auswählen.
3. Zielordner wählen und *Herunterladen*.

Je Zähler und Zeitstufe entsteht eine Datei
`ZGW_<Zähler>_<Stufe>d_<Datum-Uhrzeit>.csv`.

## Zeitstufen

Die API kennt keinen freien Datumsbereich, sondern vier feste Stufen:

| Stufe | Zeitraum | Auflösung |
| --- | --- | --- |
| 1 | ein Tag | 15 Minuten |
| 14 | zwei Wochen | 24 Stunden |
| 365 | ein Jahr | ein Monat |
| 1095 | drei Jahre | ein Jahr |

## Dateiformat

UTF-8 mit BOM, Semikolon als Feldtrenner, Komma als Dezimalzeichen —
deutsches Excel öffnet die Dateien per Doppelklick korrekt.

Spalte A ist die deutsche Zeitdarstellung, Spalte B die unveränderte
Zeitangabe der API. Die zweite Spalte löst die Zweideutigkeit der bei der
Zeitumstellung doppelt vorkommenden Stunde auf.

## Einstellungen

`%APPDATA%\ZGW-History-Downloader\config.json`. Ist „Passwort merken"
gesetzt, wird das Passwort mit der Windows-DPAPI verschlüsselt und ist
nur von demselben Benutzerkonto auf demselben Rechner lesbar.

## Selbst bauen

Voraussetzungen: Go, Node, Wails-CLI (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`).

    wails build -platform windows/amd64 -webview2 embed -ldflags "-s -w"

## Grundlage

Offizielle Gerätespezifikation:
[Eltako/eltako-openapi-specifications](https://github.com/Eltako/eltako-openapi-specifications),
Datei `zgw16ip/ELTAKO-ZGW16-IP-OpenAPI.yaml`.
```

- [ ] **Step 7: Auslieferungs-Build**

Run: `"$(go env GOPATH)/bin/wails.exe" build -platform windows/amd64 -webview2 embed -ldflags "-s -w"`
Expected: `build/bin/zgwhistory.exe` entsteht. `-webview2 embed` legt den WebView2-Bootstrapper mit hinein, damit die Datei auch auf Rechnern ohne installierte Runtime startet.

- [ ] **Step 8: Abschließender Gesamttest**

Run: `go test ./... && go vet ./...`
Expected: PASS, keine Meldungen

- [ ] **Step 9: Commit**

```bash
git add -A
git commit -m "docs: README und Auslieferungsstand"
```

---

## Selbstprüfung des Plans

**Abdeckung der Spec**

| Abschnitt der Spec | Task |
| --- | --- |
| API-Endpunkte, Token-Ablauf | 3, 4 |
| Vier feste Zeitstufen | 4 (Prüfung), 10 (Anzeige) |
| `internal/zgw` | 2, 3, 4, 5 |
| `internal/export` | 6, 7 |
| `internal/config` mit DPAPI | 8 |
| `app.go`, Events, Abbruch | 9 |
| Oberfläche, drei Abschnitte | 10 |
| Dateien und CSV-Format | 6, 7 |
| Fehlerverhalten | 3 (Sentinels), 9 (Sammeln und Bilanz) |
| Testen | 2 bis 8 |
| Bauen und Ausliefern | 1, 11 |
| Offener Punkt Skalierung | 11, Step 3 |
| Offener Punkt nicht aufgezeichnete Register | 5, 9 (Platzhalter und leere Antwort) |

**Namenskonsistenz** — die Bezeichner, die über Taskgrenzen hinweg benutzt werden, sind an einer Stelle definiert und überall gleich geschrieben: `normalizeBaseURL`, `NewClient`, `Login`, `Token`, `System`, `Devices`, `DeviceTypes`, `History`, `ValidTimeFrames`, `BuildCatalog`, `MeterInfo`, `RecordedRegister`, `ColumnHeader`, `RegisterByNumber`, `SafeName`, `FileName`, `NewPoint`, `Series`, `Write`, `WriteFile`, `Load`, `Save`, `Path`, `Protect`, `Unprotect`.

**Bekannte Abweichung** — Task 9 hat keinen Unit-Test. Das ist Absicht: `app.go` enthält keine eigene Logik, sondern verdrahtet die in den Tasks 3 bis 8 getesteten Bausteine, und ein Test dafür bräuchte eine laufende Wails-Umgebung. Die Abnahme erfolgt in Task 11 am echten Gerät.
