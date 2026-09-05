package discover

import (
	"context"
	"os"
	"testing"
)

// TestFindImEchtenNetz sucht tatsaechlich im Netz. Er laeuft nur, wenn
// ZGW_LIVE gesetzt ist, damit der normale Testlauf ohne Netz auskommt
// und keine Multicast-Pakete verschickt.
//
//	ZGW_LIVE=1 go test ./internal/discover/ -run Echten -v
func TestFindImEchtenNetz(t *testing.T) {
	if os.Getenv("ZGW_LIVE") == "" {
		t.Skip("ZGW_LIVE nicht gesetzt, keine Suche im echten Netz")
	}

	gateways, err := Find(context.Background())
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(gateways) == 0 {
		t.Fatal("kein Gateway gefunden")
	}
	for _, g := range gateways {
		t.Logf("gefunden: %s", g.Label())
		if g.IP == "" {
			t.Errorf("Fund ohne IP: %+v", g)
		}
		if !istGateway(g.Model) {
			t.Errorf("Fund ist kein Gateway: %+v", g)
		}
	}
}
