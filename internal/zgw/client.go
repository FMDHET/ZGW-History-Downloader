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
		if ctx.Err() != nil {
			return ctx.Err()
		}
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
