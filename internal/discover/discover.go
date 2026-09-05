// Package discover findet ELTAKO-Gateways im lokalen Netz per mDNS.
//
// Die Geraete melden sich unter dem Dienst "_eltako._tcp.local" an --
// nicht nur Gateways, sondern auch Dimmer, Schaltaktoren und anderes.
// Unterschieden wird am TXT-Feld "md" (Modell).
//
// Das Paket bringt seinen eigenen, sehr kleinen DNS-Parser mit, statt
// eine Zeroconf-Bibliothek zu verwenden. Zwei Gruende: die erprobte
// Bibliothek fand auf einem Rechner mit mehreren virtuellen
// Netzwerkadaptern gar nichts, weil sie ueber die falsche Schnittstelle
// fragte; und das Programm haengt sonst an keiner Fremdabhaengigkeit
// ausser Wails.
package discover

import (
	"context"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

// service ist der Dienst, unter dem sich ELTAKO-Geraete anmelden.
const service = "_eltako._tcp.local"

// modellKennung entscheidet, was als Gateway gilt. Gesehen wurden
// "ZGW16-IP" im TXT-Feld; die Modellreihe umfasst ausserdem ZGW16WL-IP
// und ZGW16NI-IP.
const modellKennung = "ZGW16"

// Suchdauer ist das Zeitfenster, in dem Antworten gesammelt werden.
// Die Geraete antworten nicht gleichzeitig, sondern verteilt.
const Suchdauer = 3 * time.Second

// Gateway ist ein im Netz gefundenes ZGW.
type Gateway struct {
	IP     string `json:"ip"`     // aus dem A-Datensatz
	Host   string `json:"host"`   // mDNS-Name, etwa "zgw16-ip.local"
	Name   string `json:"name"`   // Geraetename, TXT-Feld "pn"
	Model  string `json:"model"`  // Modell, TXT-Feld "md"
	Serial string `json:"serial"` // Seriennummer, TXT-Feld "sn"
}

// Label ist die Zeile im Auswahlfeld: IP, mDNS-Name, Geraetename.
// Fehlende Angaben werden weggelassen statt als Leerstelle gezeigt.
func (g Gateway) Label() string {
	teile := []string{g.IP}
	if g.Host != "" {
		teile = append(teile, g.Host)
	}
	switch {
	case g.Name != "":
		teile = append(teile, g.Name)
	case g.Model != "":
		teile = append(teile, g.Model)
	}
	return strings.Join(teile, " - ")
}

// instanz sammelt, was ueber eine Dienstinstanz bekannt ist. Die
// Angaben kommen aus verschiedenen Datensaetzen und oft aus
// verschiedenen Paketen.
type instanz struct {
	host string
	txt  map[string]string
}

// collector fuegt die Datensaetze vieler Antworten zusammen.
type collector struct {
	mu        sync.Mutex
	instanzen map[string]*instanz
	adressen  map[string]string // Hostname -> IPv4
}

func newCollector() *collector {
	return &collector{
		instanzen: map[string]*instanz{},
		adressen:  map[string]string{},
	}
}

func (c *collector) hole(name string) *instanz {
	i, ok := c.instanzen[name]
	if !ok {
		i = &instanz{txt: map[string]string{}}
		c.instanzen[name] = i
	}
	return i
}

// add wertet eine empfangene Antwort aus. Unlesbare Pakete werden still
// verworfen: im Netz liegt allerlei, und ein einzelnes kaputtes Paket
// darf die Suche nicht beenden.
func (c *collector) add(msg []byte) {
	records, err := parseMessage(msg)
	if err != nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for _, r := range records {
		switch r.rtype {
		case typePTR:
			if !strings.HasSuffix(strings.ToLower(r.name), service) {
				continue
			}
			if ziel, _, err := readName(msg, r.rdataOff); err == nil {
				c.hole(ziel)
			}

		case typeSRV:
			// Aufbau: Prioritaet, Gewicht, Port, dann der Zielname.
			if len(r.rdata) < 7 {
				continue
			}
			if ziel, _, err := readName(msg, r.rdataOff+6); err == nil {
				c.hole(r.name).host = ziel
			}

		case typeTXT:
			i := c.hole(r.name)
			for k, v := range parseTXT(r.rdata) {
				i.txt[k] = v
			}

		case typeA:
			if len(r.rdata) == 4 {
				ip := net.IPv4(r.rdata[0], r.rdata[1], r.rdata[2], r.rdata[3])
				c.adressen[strings.ToLower(r.name)] = ip.String()
			}
		}
	}
}

// istGateway entscheidet anhand des Modells, ob ein gefundenes
// ELTAKO-Geraet ein Gateway ist.
func istGateway(modell string) bool {
	return strings.Contains(strings.ToUpper(modell), modellKennung)
}

// gateways liefert die gefundenen ZGW, nach IP sortiert und ohne
// Doppelte.
func (c *collector) gateways() []Gateway {
	return c.geraete(istGateway)
}

// geraete liefert alle gefundenen ELTAKO-Geraete, deren Modell dem
// Filter genuegt. Ein Filter von nil laesst alle durch; das nutzen die
// Tests, um auch Nicht-Gateways zu pruefen.
func (c *collector) geraete(filter func(modell string) bool) []Gateway {
	c.mu.Lock()
	defer c.mu.Unlock()

	nachIP := map[string]Gateway{}
	for _, i := range c.instanzen {
		modell := i.txt["md"]
		if filter != nil && !filter(modell) {
			continue
		}
		ip := c.adressen[strings.ToLower(i.host)]
		if ip == "" {
			// Ohne Adresse ist der Fund nutzlos: verbinden liesse sich
			// damit nur ueber die Namensaufloesung, und genau die soll
			// die Suche ja ersetzen.
			continue
		}
		nachIP[ip] = Gateway{
			IP:     ip,
			Host:   i.host,
			Name:   i.txt["pn"],
			Model:  modell,
			Serial: i.txt["sn"],
		}
	}

	out := make([]Gateway, 0, len(nachIP))
	for _, g := range nachIP {
		out = append(out, g)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].IP < out[b].IP })
	return out
}

// frage baut die PTR-Anfrage nach dem ELTAKO-Dienst.
func frage() []byte {
	b := []byte{0, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	for _, label := range strings.Split(service, ".") {
		b = append(b, byte(len(label)))
		b = append(b, label...)
	}
	return append(b, 0, 0, typePTR, 0, 1)
}

// ipv4Schnittstellen liefert die Adressen aller aktiven Schnittstellen.
//
// Ueber alle zu fragen ist keine Vorsicht, sondern noetig: auf einem
// Rechner mit virtuellen Adaptern (VMware, Hyper-V, WSL) landet eine
// Multicast-Anfrage sonst auf einem Netz ohne Geraete, und die Suche
// bleibt ohne Ergebnis.
func ipv4Schnittstellen() []net.IP {
	var out []net.IP
	ifaces, err := net.Interfaces()
	if err != nil {
		return out
	}
	for _, ifi := range ifaces {
		if ifi.Flags&net.FlagUp == 0 || ifi.Flags&net.FlagLoopback != 0 {
			continue
		}
		addrs, err := ifi.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok {
				if v4 := ipn.IP.To4(); v4 != nil {
					out = append(out, v4)
				}
			}
		}
	}
	return out
}

// Find sucht Gateways im Netz. Es wird ueber jede Schnittstelle gefragt
// und bis zum Ablauf von Suchdauer oder bis zum Abbruch des Kontexts
// gesammelt.
//
// Findet die Suche nichts, ist das kein Fehler: das Ergebnis ist dann
// einfach leer. Ein Fehler kommt nur, wenn ueberhaupt keine Anfrage
// gestellt werden konnte.
func Find(ctx context.Context) ([]Gateway, error) {
	ctx, cancel := context.WithTimeout(ctx, Suchdauer)
	defer cancel()

	c := newCollector()
	gruppe := &net.UDPAddr{IP: net.IPv4(224, 0, 0, 251), Port: 5353}
	anfrage := frage()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var gesendet int
	var letzterFehler error

	for _, local := range ipv4Schnittstellen() {
		wg.Add(1)
		go func(local net.IP) {
			defer wg.Done()

			conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: local, Port: 0})
			if err != nil {
				mu.Lock()
				letzterFehler = err
				mu.Unlock()
				return
			}
			defer conn.Close()

			if _, err := conn.WriteToUDP(anfrage, gruppe); err != nil {
				mu.Lock()
				letzterFehler = err
				mu.Unlock()
				return
			}
			mu.Lock()
			gesendet++
			mu.Unlock()

			// Der Kontext beendet das Warten, indem er die Frist auf
			// sofort setzt und das Lesen damit aufweckt.
			stop := make(chan struct{})
			defer close(stop)
			go func() {
				select {
				case <-ctx.Done():
					conn.SetReadDeadline(time.Now())
				case <-stop:
				}
			}()

			buf := make([]byte, 9000)
			for {
				conn.SetReadDeadline(time.Now().Add(Suchdauer))
				n, _, err := conn.ReadFromUDP(buf)
				if err != nil {
					return
				}
				paket := make([]byte, n)
				copy(paket, buf[:n])
				c.add(paket)
			}
		}(local)
	}

	wg.Wait()

	if gesendet == 0 {
		if letzterFehler != nil {
			return nil, letzterFehler
		}
		return nil, errKeineSchnittstelle
	}
	return c.gateways(), nil
}
