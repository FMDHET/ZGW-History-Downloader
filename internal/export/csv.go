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
