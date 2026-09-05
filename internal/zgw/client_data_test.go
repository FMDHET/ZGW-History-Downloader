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
