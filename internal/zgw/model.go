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
