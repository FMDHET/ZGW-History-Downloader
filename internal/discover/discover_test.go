package discover

import (
	"encoding/hex"
	"testing"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("Pruefdaten nicht lesbar: %v", err)
	}
	return b
}

func TestGatewayAusEchtemPaket(t *testing.T) {
	c := newCollector()
	c.add(mustHex(t, paketZGW))

	got := c.gateways()
	if len(got) != 1 {
		t.Fatalf("erwartet genau ein Gateway, bekam %d: %+v", len(got), got)
	}
	g := got[0]

	if g.IP != "192.168.177.218" {
		t.Errorf("IP = %q, erwartet \"192.168.177.218\"", g.IP)
	}
	if g.Host != "zgw16-ip.local" {
		t.Errorf("Host = %q, erwartet \"zgw16-ip.local\"", g.Host)
	}
	if g.Name != "ZGW16WL-IP" {
		t.Errorf("Name = %q, erwartet \"ZGW16WL-IP\" (TXT-Feld pn)", g.Name)
	}
	if g.Model != "ZGW16-IP" {
		t.Errorf("Model = %q, erwartet \"ZGW16-IP\" (TXT-Feld md)", g.Model)
	}
	if g.Serial != "792827CD-F823-482D-9E99-43F44D084021" {
		t.Errorf("Serial = %q", g.Serial)
	}
}

// Die Beschriftung im Auswahlfeld: IP, mDNS-Name, Geraetename.
func TestLabel(t *testing.T) {
	g := Gateway{IP: "192.168.177.218", Host: "zgw16-ip.local", Name: "ZGW16WL-IP"}
	if got, want := g.Label(), "192.168.177.218 - zgw16-ip.local - ZGW16WL-IP"; got != want {
		t.Errorf("Label = %q, erwartet %q", got, want)
	}
}

func TestLabelOhneGeraetenamen(t *testing.T) {
	g := Gateway{IP: "192.168.1.9", Host: "zgw16-ip.local", Model: "ZGW16-IP"}
	if got, want := g.Label(), "192.168.1.9 - zgw16-ip.local - ZGW16-IP"; got != want {
		t.Errorf("ohne pn muss das Modell einspringen: %q, erwartet %q", got, want)
	}
}

func TestLabelNurIP(t *testing.T) {
	g := Gateway{IP: "192.168.1.9"}
	if got, want := g.Label(), "192.168.1.9"; got != want {
		t.Errorf("Label = %q, erwartet %q", got, want)
	}
}

// Am selben Dienst _eltako._tcp haengen auch Dimmer und Schaltaktoren.
// Die duerfen nicht in der Auswahl landen.
func TestAndereEltakoGeraeteWerdenAussortiert(t *testing.T) {
	c := newCollector()
	c.add(mustHex(t, paketAnderesGeraet))

	if got := c.gateways(); len(got) != 0 {
		t.Fatalf("erwartet kein Gateway, bekam %+v", got)
	}
}

// Dieses Paket ist gross genug, dass seine Namenszeiger ueber Offset 255
// hinausgehen (etwa c101). Ein Parser, der beim Zeiger nur das untere
// Byte auswertet, liest hier Unsinn -- und faellt sonst nirgends auf,
// weil im kurzen Gateway-Paket alle Zeiger darunter liegen.
func TestZeigerJenseitsVonOffset255(t *testing.T) {
	c := newCollector()
	c.add(mustHex(t, paketAnderesGeraet))

	alle := c.geraete(nil)
	if len(alle) != 1 {
		t.Fatalf("erwartet ein Geraet, bekam %d: %+v", len(alle), alle)
	}
	g := alle[0]
	if g.IP != "192.168.177.249" {
		t.Errorf("IP = %q, erwartet \"192.168.177.249\"", g.IP)
	}
	if g.Host != "eltako-ahmggdmray.local" {
		t.Errorf("Host = %q, erwartet \"eltako-ahmggdmray.local\"", g.Host)
	}
	if g.Name != "DG-Flur-ESR64NP-IPM" {
		t.Errorf("Name = %q, erwartet \"DG-Flur-ESR64NP-IPM\"", g.Name)
	}
	if g.Model != "ESR64NP-IPM" {
		t.Errorf("Model = %q, erwartet \"ESR64NP-IPM\"", g.Model)
	}
}

func TestGatewayWirdZwischenAnderenGeraetenGefunden(t *testing.T) {
	c := newCollector()
	c.add(mustHex(t, paketAnderesGeraet))
	c.add(mustHex(t, paketZGW))

	got := c.gateways()
	if len(got) != 1 {
		t.Fatalf("erwartet genau ein Gateway, bekam %d: %+v", len(got), got)
	}
	if got[0].Model != "ZGW16-IP" {
		t.Errorf("falsches Geraet ausgewaehlt: %+v", got[0])
	}
}

// Dasselbe Geraet antwortet ueber jede Schnittstelle, ueber die gefragt
// wurde. Es darf trotzdem nur einmal in der Liste stehen.
func TestMehrfachEmpfangeneAntwortenWerdenEntdoppelt(t *testing.T) {
	c := newCollector()
	c.add(mustHex(t, paketZGW))
	c.add(mustHex(t, paketZGW))
	c.add(mustHex(t, paketZGW))

	if got := c.gateways(); len(got) != 1 {
		t.Fatalf("erwartet ein Gateway, bekam %d", len(got))
	}
}

func TestKaputtePaketeStuerzenNichtAb(t *testing.T) {
	voll := mustHex(t, paketZGW)
	c := newCollector()

	c.add(nil)
	c.add([]byte{1, 2, 3})
	c.add(voll[:20])                                        // mitten im Namen abgeschnitten
	c.add(voll[:len(voll)-5])                               // im letzten Datensatz abgeschnitten
	c.add([]byte{0, 0, 0x84, 0, 0, 0, 0, 0xff, 0, 0, 0, 0}) // 255 Antworten behauptet, keine geliefert

	// Nach all dem Schrott muss ein gutes Paket weiterhin gelesen werden.
	c.add(voll)
	if got := c.gateways(); len(got) != 1 {
		t.Fatalf("nach kaputten Paketen wurde das Gateway nicht mehr gefunden: %+v", got)
	}
}

func TestNamensZeigerImKreisBeendetSich(t *testing.T) {
	// Ein Zeiger auf sich selbst darf keine Endlosschleife ergeben.
	msg := []byte{
		0, 0, 0x84, 0, 0, 0, 0, 1, 0, 0, 0, 0,
		0xc0, 0x0c, // Name an Offset 12 zeigt auf Offset 12
		0, 12, 0, 1, 0, 0, 0, 60, 0, 0,
	}
	c := newCollector()
	done := make(chan struct{})
	go func() {
		c.add(msg)
		close(done)
	}()
	<-done // laeuft der Parser endlos, blockiert der Test hier und faellt in den Zeitablauf
}
