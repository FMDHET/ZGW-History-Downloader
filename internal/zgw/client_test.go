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
