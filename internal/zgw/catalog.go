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
