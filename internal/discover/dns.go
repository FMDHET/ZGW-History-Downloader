package discover

import (
	"errors"
	"strings"
)

// Datensatztypen, die uns interessieren.
const (
	typeA   = 1
	typePTR = 12
	typeTXT = 16
	typeSRV = 33
)

var (
	errKaputt             = errors.New("Paket nicht lesbar")
	errKeineSchnittstelle = errors.New("keine aktive Netzwerkschnittstelle gefunden")
)

// maxSpruenge begrenzt, wie oft ein Name einem Rueckwaertszeiger folgen
// darf. Ohne diese Grenze bringt ein Zeiger auf sich selbst den Parser
// zum Stillstand -- ein Paket aus dem Netz darf das nicht koennen.
const maxSpruenge = 32

// record ist ein Datensatz aus einer DNS-Antwort.
type record struct {
	name  string
	rtype uint16
	rdata []byte
	// rdataOff ist die Position der Nutzdaten im Gesamtpaket. SRV und PTR
	// enthalten darin Namen, die per Zeiger auf fruehere Stellen des
	// Pakets verweisen; ohne diese Position sind sie nicht aufloesbar.
	rdataOff int
}

// readName liest einen DNS-Namen ab off und folgt dabei
// Kompressionszeigern. Zurueck kommen der Name in Punktschreibweise und
// die Position hinter dem Namen im laufenden Datenstrom.
func readName(msg []byte, off int) (string, int, error) {
	var labels []string
	var ende int
	gesprungen := false
	spruenge := 0

	for {
		if off < 0 || off >= len(msg) {
			return "", 0, errKaputt
		}
		l := int(msg[off])

		switch {
		case l == 0:
			off++
			if !gesprungen {
				ende = off
			}
			return strings.Join(labels, "."), ende, nil

		case l&0xc0 == 0xc0:
			// Rueckwaertszeiger: die unteren 14 Bit sind die Zieladresse.
			if off+1 >= len(msg) {
				return "", 0, errKaputt
			}
			ziel := int(msg[off]&0x3f)<<8 | int(msg[off+1])
			if !gesprungen {
				ende = off + 2
				gesprungen = true
			}
			spruenge++
			if spruenge > maxSpruenge {
				return "", 0, errKaputt
			}
			off = ziel

		case l > 63:
			return "", 0, errKaputt

		default:
			if off+1+l > len(msg) {
				return "", 0, errKaputt
			}
			labels = append(labels, string(msg[off+1:off+1+l]))
			off += 1 + l
		}
	}
}

// parseMessage zerlegt eine DNS-Antwort in ihre Datensaetze. Alle
// Abschnitte werden gelesen: mDNS-Geraete legen SRV, TXT und A gern in
// den Zusatzteil statt in den Antwortteil.
//
// Ein abgeschnittenes Paket ist kein Grund, alles zu verwerfen -- was
// bis dahin gelesen wurde, wird zurueckgegeben.
func parseMessage(msg []byte) ([]record, error) {
	if len(msg) < 12 {
		return nil, errKaputt
	}
	fragen := int(msg[4])<<8 | int(msg[5])
	anzahl := (int(msg[6])<<8 | int(msg[7])) +
		(int(msg[8])<<8 | int(msg[9])) +
		(int(msg[10])<<8 | int(msg[11]))

	off := 12
	for i := 0; i < fragen; i++ {
		_, next, err := readName(msg, off)
		if err != nil {
			return nil, err
		}
		off = next + 4 // Typ und Klasse
		if off > len(msg) {
			return nil, errKaputt
		}
	}

	records := make([]record, 0, anzahl)
	for i := 0; i < anzahl; i++ {
		name, next, err := readName(msg, off)
		if err != nil {
			return records, nil
		}
		off = next
		if off+10 > len(msg) {
			return records, nil
		}
		rtype := uint16(msg[off])<<8 | uint16(msg[off+1])
		rdlen := int(msg[off+8])<<8 | int(msg[off+9])
		off += 10
		if off+rdlen > len(msg) {
			return records, nil
		}
		records = append(records, record{
			name:     name,
			rtype:    rtype,
			rdata:    msg[off : off+rdlen],
			rdataOff: off,
		})
		off += rdlen
	}
	return records, nil
}

// parseTXT zerlegt die Zeichenkettenliste eines TXT-Datensatzes in
// Schluessel und Wert. Eintraege ohne "=" werden uebergangen.
func parseTXT(rdata []byte) map[string]string {
	out := map[string]string{}
	for i := 0; i < len(rdata); {
		l := int(rdata[i])
		if l == 0 || i+1+l > len(rdata) {
			break
		}
		eintrag := string(rdata[i+1 : i+1+l])
		if k, v, ok := strings.Cut(eintrag, "="); ok {
			out[k] = v
		}
		i += 1 + l
	}
	return out
}
