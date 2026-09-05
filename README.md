# ZGW History Downloader

Lädt die History-Daten eines Eltako ZGW16WL-IP über dessen REST-API und
speichert sie als Excel-taugliche CSV-Dateien.

## Benutzung

`zgwhistory.exe` starten — keine Installation nötig.

1. Adresse des Gateways (Vorgabe `zgw16-ip.local`) und Passwort eingeben,
   *Verbinden*.
2. Zähler, Register und Zeitstufen auswählen.
3. Zielordner wählen und *Herunterladen*.

Je Zähler und Zeitstufe entsteht eine Datei
`ZGW_<Zähler>_<Stufe>d_<Datum-Uhrzeit>.csv`.

## Zeitstufen

Die API kennt keinen freien Datumsbereich, sondern vier feste Stufen, die
zugleich die Auflösung bestimmen:

| Stufe | Zeitraum | Auflösung |
| --- | --- | --- |
| 1 | ein Tag | 15 Minuten |
| 14 | zwei Wochen | 24 Stunden |
| 365 | ein Jahr | ein Monat |
| 1095 | drei Jahre | ein Jahr |

## Dateiformat

UTF-8 mit BOM, Semikolon als Feldtrenner, Komma als Dezimalzeichen —
deutsches Excel öffnet die Dateien per Doppelklick korrekt.

```
Zeitstempel;Zeitstempel_ISO;Gesamtwirkleistung [Watt];Bezug [kWh]
05.09.2026 08:15:00;2026-09-05T08:15:00.000+02:00;1234,5;5678,90
```

Spalte A ist die deutsche Zeitdarstellung, Spalte B die unveränderte
Zeitangabe der API. Die zweite Spalte löst die Zweideutigkeit der bei der
Zeitumstellung doppelt vorkommenden Stunde auf, die Spalte A allein nicht
auflösen kann.

Zeichnet das Gateway für ein Register nichts auf, entfällt dessen Spalte.
Liefert kein Register eines Zählers Werte, entsteht für ihn keine Datei.

## Einstellungen

`%APPDATA%\ZGW-History-Downloader\config.json`. Ist „Passwort merken"
gesetzt, wird das Passwort mit der Windows-DPAPI verschlüsselt und ist nur
von demselben Benutzerkonto auf demselben Rechner lesbar.

## Selbst bauen

Voraussetzungen: Go, Node und die Wails-CLI:

    go install github.com/wailsapp/wails/v2/cmd/wails@latest
    wails build -platform windows/amd64 -webview2 embed -ldflags "-s -w"

`-webview2 embed` legt den WebView2-Bootstrapper mit in die Datei, damit
sie auch auf Rechnern ohne installierte Runtime startet.

Tests:

    go test ./...

Zusätzlich gibt es einen Test gegen echte Hardware. Ohne die beiden
Umgebungsvariablen wird er übersprungen:

    ZGW_HOST=192.168.1.50 ZGW_PASSWORD=geheim go test . -run Echtes -v

Er meldet sich an, liest Zähler und Register, exportiert alle vier
Zeitstufen und prüft, dass die Dateien echte Werte enthalten. Mit
`ZGW_OUT=C:\irgendwo` bleiben die erzeugten CSV-Dateien zum Ansehen
liegen.

## Geprüft gegen

Ein ZGW16WL-IP mit Firmware **3.0.99-rc.1**, drei Zählern (DSZ16DZ(E),
DSZ15DZMOD, DSZ16WDZ(E)) und 20 aufgezeichneten Registern: 80 Abrufe,
10 Dateien, keine Fehler.

Die Firmware weicht an zwei Stellen von der Spezifikation 2.4.1 ab —
die Anmeldung antwortet verschachtelt, und Zeitstempel tragen den
Zonenversatz ohne Doppelpunkt (`+0200` statt `+02:00`). Das Programm
liest beide Formen. Einzelheiten in
[`docs/superpowers/specs/`](docs/superpowers/specs/).

Registerbeschreibungen liefert die Firmware nur auf Englisch, deshalb
lauten die Spaltenüberschriften etwa `Total active power [Watt]`.

## Aufbau

| Pfad | Zweck |
| --- | --- |
| `internal/zgw` | REST-Client, Anmeldung, Registerkatalog |
| `internal/export` | CSV-Erzeugung und Dateinamen |
| `internal/config` | Einstellungen und DPAPI |
| `app.go` | Wails-Bindings und Export-Ablauf |
| `frontend/` | Oberfläche |

`internal/zgw` und `internal/export` kennen weder Wails noch einander und
sind deshalb ohne GUI testbar.

## Grundlage

Offizielle Gerätespezifikation:
[Eltako/eltako-openapi-specifications](https://github.com/Eltako/eltako-openapi-specifications),
Datei `zgw16ip/ELTAKO-ZGW16-IP-OpenAPI.yaml` (v2.4.1).

Entwurf und Umsetzungsplan liegen unter [`docs/superpowers/`](docs/superpowers/).
