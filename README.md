# ZGW History Downloader

Lädt die gespeicherten Verbrauchs-Historien eines Eltako ZGW16WL-IP über
dessen REST-API und legt sie als Excel-taugliche CSV-Dateien ab.

Eine einzelne `.exe`, keine Installation, keine Laufzeitumgebung auf dem
Zielrechner nötig.

## Benutzung

`zgwhistory.exe` starten.

1. Passwort eingeben, *Verbinden*. Die Adresse trägt das Programm in der
   Regel selbst ein — siehe unten.
2. Zähler, Register und Zeitstufen auswählen — alles ist vorausgewählt.
3. Zielordner wählen und *Herunterladen*.

Je Zähler und Zeitstufe entsteht eine Datei
`ZGW_<Zähler>_<Stufe>d_<Datum-Uhrzeit>.csv`.

Ein laufender Export lässt sich jederzeit abbrechen; bereits geschriebene
Dateien bleiben liegen.

## Das Gateway wird selbst gefunden

Beim Start sucht das Programm per mDNS im lokalen Netz. Gefundene
Gateways stehen im Adressfeld als Auswahlliste bereit, beschriftet mit
**IP – mDNS-Name – Gerätename**:

```
192.168.177.218 - zgw16-ip.local - ZGW16WL-IP
```

Ausgewählt wird die IP-Adresse. Das Verbinden hängt damit nicht an der
Namensauflösung, die je nach Netz und Windows-Einstellung unterschiedlich
zuverlässig ist.

Bei genau einem Fund trägt das Programm die Adresse selbst ein —
allerdings nur, wenn dort noch die Vorgabe `zgw16-ip.local` steht. Eine
von dir eingetragene und gemerkte Adresse wird nie überschrieben. Der
Knopf *Suchen* wiederholt die Suche jederzeit.

Findet die Suche nichts, sagt das Programm das und du trägst die Adresse
von Hand ein. Das kommt vor, wenn im Netz Multicast gefiltert wird oder
das Gateway in einem anderen Subnetz hängt; ein aktives VPN kann
ebenfalls dazwischenfunken.

### Warum die Suche nach Modell filtert

ELTAKO-Geräte melden sich alle unter dem Dienst `_eltako._tcp.local` an,
nicht nur Gateways. Im Testnetz antworten sechs Geräte auf die Anfrage:

```
  192.168.177.194 - EG-Wohnzimmer-ELTAKO-EUD64NPN-IPM
  192.168.177.203 - EG-Küche-EUD64NPN-IPM
* 192.168.177.218 - zgw16-ip.local - ZGW16WL-IP
  192.168.177.241 - EG-Esszimmer-EUD64NPN-IPM
  192.168.177.242 - DG-Bad-ESR64NP-IPM
  192.168.177.249 - DG-Flur-ESR64NP-IPM
```

Angeboten wird nur, was im TXT-Feld `md` ein `ZGW16` trägt. Sonst
stünden Dimmer und Schaltaktoren mit zur Auswahl.

Gefragt wird über **jede** Netzwerkschnittstelle. Auf Rechnern mit
virtuellen Adaptern (VMware, Hyper-V, WSL) geht eine Anfrage sonst in ein
Netz ohne Geräte, und die Suche bleibt ohne Ergebnis.

## Zeitstufen — und was nicht geht

**Ein freier Zeitraum „von … bis …" ist mit diesem Gerät nicht möglich.**
Die API kennt nur vier feste Stufen, die zugleich die Auflösung
bestimmen:

| Stufe | Zeitraum | Auflösung |
| --- | --- | --- |
| 1 | ein Tag | 15 Minuten |
| 14 | zwei Wochen | 24 Stunden |
| 365 | ein Jahr | ein Monat |
| 1095 | drei Jahre | ein Jahr |

Feiner als 15 Minuten geht nicht, und Werte älter als ein Tag gibt es
nur in der gröberen Auflösung der jeweiligen Stufe. Wer eine lückenlose
15-Minuten-Historie über den Tag hinaus will, muss die 1-Tages-Stufe
regelmäßig abholen und selbst zusammenführen.

## Dateiformat

UTF-8 mit BOM, Semikolon als Feldtrenner, Komma als Dezimalzeichen —
deutsches Excel öffnet die Dateien per Doppelklick korrekt.

Echte Ausgabe eines DSZ15DZMOD:

```
Zeitstempel;Zeitstempel_ISO;Total active power [Watt];Total imported active energy [kWh];Total exported active energy [kWh]
04.09.2026 08:32:00;2026-09-04T08:32:00+0200;884;2290,06;5279,42
04.09.2026 08:47:00;2026-09-04T08:47:00+0200;2106;2290,54;5279,42
04.09.2026 09:01:00;2026-09-04T09:01:00+0200;;;5279,42
04.09.2026 09:16:00;2026-09-04T09:16:00+0200;3517;;
```

Spalte A ist die deutsche Zeitdarstellung, Spalte B die unveränderte
Zeitangabe des Geräts. Die zweite Spalte löst die Zweideutigkeit der
Stunde auf, die bei der Umstellung von Sommer- auf Winterzeit doppelt
vorkommt und die Spalte A allein nicht auflösen kann.

Die Werte kommen fertig skaliert vom Gerät und werden nicht
umgerechnet.

### Warum Zellen leer bleiben

Zeile 3 und 4 des Beispiels zeigen es: **die Register eines Zählers
teilen sich die Zeitstempel nicht.** Das Gateway zeichnet nicht jede
Messgröße zu jedem Zeitpunkt auf.

Die Tabelle wird deshalb über die Vereinigungsmenge aller Zeitstempel
gebaut. Fehlt zu einem Zeitpunkt ein Wert, bleibt die Zelle leer. Das
ist kein Fehler, sondern der einzige Weg, bei dem die Werte einer Zeile
verlässlich zusammengehören — würde man die Spalten stumpf
nebeneinanderlegen, verrutschten sie still gegeneinander.

Zeichnet das Gateway für ein Register gar nichts auf, entfällt dessen
Spalte. Liefert kein einziges Register eines Zählers Werte, entsteht für
ihn keine Datei; das Protokoll sagt warum.

### Spaltennamen sind englisch

Die Firmware liefert die Registerbeschreibungen ausschließlich auf
Englisch — über drei Zählertypen und 198 Register kein einziger
deutscher Eintrag. Deshalb steht dort `Total active power [Watt]` und
nicht `Gesamtwirkleistung [Watt]`. Das Programm bevorzugt eine deutsche
Beschreibung, sobald das Gerät eine liefert.

## Einstellungen

`%APPDATA%\ZGW-History-Downloader\config.json`.

Ist „Passwort merken" gesetzt, wird das Passwort mit der Windows-DPAPI
verschlüsselt und ist nur von demselben Benutzerkonto auf demselben
Rechner lesbar. Ohne den Haken landet nichts Geheimes auf der Platte.

## Geprüft gegen echte Hardware

Ein ZGW16WL-IP mit Firmware **3.0.99-rc.1**, drei Zählern (DSZ16DZ(E),
DSZ15DZMOD, DSZ16WDZ(E)) und 20 aufgezeichneten Registern. Vollexport
über alle vier Zeitstufen: 80 Abrufe, 10 Dateien, keine Fehler.

Dass 35 der 80 Abrufe leer zurückkamen, ist kein Mangel — für die
Dreijahresstufe hat das Gerät schlicht noch keine Daten.

### Abweichungen der Firmware von der Spezifikation

Die offizielle Spezifikation beschreibt Version 2.4.1. Firmware 3.0.99
weicht an zwei Stellen ab, beide am Gerät nachgemessen:

| Was | Spezifikation | Gerät |
| --- | --- | --- |
| Antwort auf `/login` | flach `{"accessToken":…}` im Endpunkt-Beispiel, verschachtelt im Schema | verschachtelt `{"login":{"accessToken":…}}` |
| Zeitstempel | `2026-09-05T08:15:00.000+02:00` | `2026-09-04T08:32:00+0200` |

Das Programm liest jeweils beide Formen. Ohne die zweite Anpassung wäre
jeder einzelne Messwert verworfen worden und keine Datei entstanden.

Hintergrund in
[`docs/superpowers/specs/`](docs/superpowers/specs/).

## Selbst bauen

Voraussetzungen: Go, Node und die Wails-CLI.

    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    wails build -platform windows/amd64 -webview2 embed -ldflags "-s -w"

`-webview2 embed` legt den WebView2-Bootstrapper mit in die Datei, damit
sie auch auf Rechnern ohne installierte Runtime startet. Ergebnis ist
eine `.exe` von rund 14 MB unter `build\bin\`.

## Tests

    go test ./...

Läuft ohne Gerät und ohne Netz gegen nachgebaute Antworten.

Dazu ein Test gegen echte Hardware, der ohne die beiden
Umgebungsvariablen übersprungen wird:

    ZGW_HOST=192.168.1.50 ZGW_PASSWORD=geheim go test . -run Echtes -v

Er meldet sich an, liest Zähler und Register, exportiert alle vier
Zeitstufen und prüft, dass die Dateien echte Werte enthalten und nicht
nur Überschriften. Mit `ZGW_OUT=C:\irgendwo` bleiben die erzeugten
CSV-Dateien zum Ansehen liegen.

Die mDNS-Suche lässt sich getrennt davon gegen das echte Netz prüfen:

    ZGW_LIVE=1 go test ./internal/discover/ -run Echten -v

Die Parsertests selbst brauchen kein Netz. Sie laufen gegen echte, im
Netz aufgezeichnete Antwortpakete, die als Prüfdaten im Paket liegen.

## Aufbau

| Pfad | Zweck |
| --- | --- |
| `internal/zgw` | REST-Client, Anmeldung, Registerkatalog |
| `internal/export` | CSV-Erzeugung und Dateinamen |
| `internal/config` | Einstellungen und DPAPI |
| `internal/discover` | mDNS-Suche nach Gateways |
| `app.go` | Wails-Bindings und Export-Ablauf |
| `frontend/` | Oberfläche |
| `integration_test.go` | Test gegen echte Hardware |

Ausser Wails hängt das Programm an keiner Fremdabhängigkeit; auch der
mDNS-Teil bringt seinen eigenen kleinen DNS-Parser mit.

`internal/zgw` und `internal/export` kennen weder Wails noch einander.
Deshalb lässt sich die gesamte Fachlogik ohne GUI prüfen — der
Export-Ablauf in `app.go` ebenfalls, weil der Ereignis-Emitter
austauschbar ist.

## Zugangstoken

Das Token des Geräts verfällt nach 15 Minuten. Ein Vollexport kann
länger dauern, deshalb meldet sich der Client bei Bedarf selbsttätig neu
an. Davon ist im Betrieb nichts zu merken.

## Grundlage

Offizielle Gerätespezifikation:
[Eltako/eltako-openapi-specifications](https://github.com/Eltako/eltako-openapi-specifications),
Datei `zgw16ip/ELTAKO-ZGW16-IP-OpenAPI.yaml` (v2.4.1).

Entwurf und Umsetzungsplan liegen unter
[`docs/superpowers/`](docs/superpowers/).
