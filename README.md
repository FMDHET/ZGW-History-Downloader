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
