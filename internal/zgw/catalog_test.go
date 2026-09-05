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
