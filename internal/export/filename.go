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
