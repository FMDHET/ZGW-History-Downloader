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
	Host       string `json:"host"`
	Password   string `json:"password"`
	Remember   bool   `json:"remember"`
	OutputDir  string `json:"outputDir"`
	TimeFrames []int  `json:"timeFrames"`
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

	// emit schickt ein Ereignis an die Oberflaeche. Im Betrieb ist das
	// der Wails-Emitter; Tests setzen hier eine eigene Funktion ein und
	// koennen den Export dadurch ohne laufende GUI pruefen.
	emit func(name string, data any)

	mu      sync.Mutex
	client  *zgw.Client
	meters  []zgw.MeterInfo
	cancel  context.CancelFunc
	running bool
}

func NewApp() *App {
	a := &App{}
	a.emit = func(name string, data any) {
		wailsruntime.EventsEmit(a.ctx, name, data)
	}
	return a
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
	a.emit("export:progress", map[string]any{
		"done": done, "total": total, "message": message,
	})
}

func (a *App) logLine(level, message string) {
	a.emit("export:log", map[string]any{
		"level": level, "message": message,
	})
}

func (a *App) finish(s Summary) {
	a.emit("export:done", s)
}
