# ZGW History Downloader — Design

Stand: 2026-09-05

## Zweck

Ein Windows-Programm mit grafischer Oberfläche, das die im Eltako
ZGW16WL-IP gespeicherten Verbrauchs-Historien ausliest und als
Excel-taugliche CSV-Dateien ablegt. Es läuft als einzelne `.exe` ohne
Installation und ohne vorinstallierte Laufzeitumgebung.

## Nicht-Ziele

- Keine Konfiguration des Gateways (Passwort setzen, MQTT, Modbus,
  Firmware-Update, Gerät löschen). Das Programm liest nur.
- Keine dauerhafte Datenbank und keine Fortschreibung über die
  Zeitfenster der API hinaus. Jeder Abruf erzeugt eigenständige Dateien.
- Keine Diagramme oder Auswertungen im Programm.
- Kein Betrieb auf macOS oder Linux.

## Die API des Geräts

Grundlage ist die offizielle Spezifikation
`ELTAKO-ZGW16-IP-OpenAPI.yaml`, Version 2.4.1, aus dem Repository
[Eltako/eltako-openapi-specifications](https://github.com/Eltako/eltako-openapi-specifications).

Basis-URL ist `http://<host>/api`, Standardname im Netz
`zgw16-ip.local`. Verwendete Endpunkte:

| Endpunkt | Zweck |
| --- | --- |
| `POST /login` | Body `{"login":{"password":"…"}}`, liefert `{"accessToken":"<UUID>"}` |
| `GET /devices` | Alle Zähler mit `busAddress`, `deviceTypeId`, `friendlyId`, `selectedHistory` |
| `GET /devices/types` | Registertabelle je Zählertyp: `number`, `description` (de/en), `unit`, `scaling_factor` |
| `GET /devices/{busAddress}/history` | Query `identifier=<Registernummer>` und `timeFrame=<1\|14\|365\|1095>` |

Die Authentifizierung erfolgt über den Header `accessToken: <UUID>`,
nicht über `Authorization: Bearer`.

Die History-Antwort hat die Form:

```json
{ "identifier": 30073,
  "values": [ { "timestamp": "2023-08-19T13:18:19.743+02:00", "value": 25.6 } ] }
```

### Zwei Eigenheiten, die das Design prägen

**Das Token verfällt nach 15 Minuten.** Ein Vollexport über 16 Zähler,
je drei aufgezeichnete Register und alle vier Zeitstufen sind rund 200
Anfragen und kann diese Frist überschreiten. Der Client hält deshalb
das Passwort im Speicher, meldet sich bei einem `401` selbsttätig neu
an und wiederholt die Anfrage genau einmal.

**Der Zeitraum ist nicht frei wählbar.** `timeFrame` kennt nur vier
feste Stufen, die zugleich die Auflösung bestimmen:

| Stufe | Zeitraum | Auflösung |
| --- | --- | --- |
| `1` | ein Tag | 15 Minuten |
| `14` | zwei Wochen | 24 Stunden |
| `365` | ein Jahr | ein Monat |
| `1095` | drei Jahre | ein Jahr |

Ein beliebiges Start- und Enddatum in feiner Auflösung ist über die API
nicht abrufbar. Die Oberfläche bildet deshalb genau diese vier Stufen
als Mehrfachauswahl ab und verspricht nichts, was das Gerät nicht
liefern kann.

## Architektur

Go-Backend und HTML-Oberfläche in einem Wails-v2-Fenster. Die
Fachlogik liegt außerhalb der Wails-Schicht, damit sie ohne laufende
GUI testbar bleibt.

```
zgw-history-downloader/
  internal/zgw/      API-Client: Login, Devices, DeviceTypes, History
  internal/export/   CSV-Writer, breite Tabelle im Excel-DE-Dialekt
  internal/config/   Konfiguration lesen/schreiben, DPAPI
  app.go             Wails-Bindings, Orchestrierung, Fortschritts-Events
  main.go            Fensteraufbau
  frontend/          Eine Seite, drei Abschnitte, Vanilla-JS
```

### `internal/zgw`

Kapselt alles, was mit HTTP zu tun hat. Nach außen bietet der `Client`
`Login`, `Devices`, `DeviceTypes` und `History`. Er hält Adresse,
Passwort und aktuelles Token; die Neuanmeldung bei `401` passiert
innerhalb des Clients und ist für Aufrufer unsichtbar.

Der Client kennt weder CSV noch Wails. Abhängigkeiten sind
ausschließlich die Standardbibliothek. Getestet wird gegen einen
`httptest.Server`, der die Beispielantworten der OpenAPI-Spec ausliefert.

Zusätzlich stellt das Paket die Verknüpfung von Zähler und
Registertabelle bereit: aus `deviceTypeId` und `selectedHistory` wird je
Zähler eine Liste aufgezeichneter Register mit deutschem Klartextnamen
und Einheit. Fehlt eine deutsche Beschreibung, greift die englische;
fehlt auch die, die nackte Registernummer.

### `internal/export`

Nimmt je Zähler und Zeitstufe die abgerufenen Register entgegen und
schreibt eine CSV. Kennt kein HTTP.

Zeitstempel verschiedener Register desselben Zählers sollten
übereinstimmen, garantiert ist das nicht. Der Writer bildet deshalb die
Vereinigungsmenge aller Zeitstempel, sortiert sie aufsteigend und füllt
je Spalte den passenden Wert ein. Fehlt zu einem Zeitstempel ein Wert,
bleibt die Zelle leer — Zeilen können nicht gegeneinander verrutschen.

### `internal/config`

Liest und schreibt `%APPDATA%\ZGW-History-Downloader\config.json` mit
Adresse, zuletzt gewähltem Zielordner, dem Merken-Haken und dem
verschlüsselten Passwort. Die Verschlüsselung nutzt die
Windows-DPAPI (`CryptProtectData` / `CryptUnprotectData` über
`golang.org/x/sys/windows`) mit `CRYPTPROTECT_UI_FORBIDDEN`, gebunden
an das Benutzerkonto.

Schlägt das Entschlüsseln fehl — etwa weil die Datei von einem anderen
Konto oder Rechner stammt — gilt das nicht als Fehler: das
Passwortfeld bleibt einfach leer.

Eine fehlende oder beschädigte Konfigurationsdatei führt zu den
Vorgabewerten, nie zu einem Absturz.

### `app.go`

Die an das Frontend gebundenen Methoden:

| Methode | Wirkung |
| --- | --- |
| `LoadSettings()` | Gespeicherte Konfiguration für das Formular |
| `Connect(host, password, remember)` | Anmelden, Zähler und Register laden, Konfiguration schreiben |
| `ChooseFolder()` | Nativer Ordner-Dialog |
| `Export(auswahl)` | Läuft im Hintergrund, meldet Fortschritt per Event |
| `CancelExport()` | Bricht einen laufenden Export ab |

Der Export läuft in einer Goroutine und sendet
`export:progress` (Zähler, Gesamtzahl, Klartextmeldung) sowie
`export:done` (Zusammenfassung) an das Frontend. Abbruch erfolgt über
einen `context.Context`.

## Oberfläche

Ein Fenster, 900 × 700, drei Abschnitte untereinander. Sprache Deutsch.

**Verbindung.** Adressfeld mit Vorgabe `zgw16-ip.local`, Passwortfeld,
Haken „Passwort merken", Schaltfläche *Verbinden*. Nach erfolgreicher
Anmeldung erscheint der Gerätename des Gateways.

**Auswahl.** Je gefundenem Zähler eine Zeile mit Busadresse, Name und
Typ, aufklappbar zu den aufgezeichneten Registern als Checkboxen mit
Klartext und Einheit. Alles ist vorausgewählt. Darunter die vier
Zeitstufen, ebenfalls als Checkboxen, mit `1` vorausgewählt.

**Export.** Zielordner mit Durchsuchen-Schaltfläche, Schaltfläche
*Herunterladen*, Fortschrittsbalken, Protokollfenster und nach dem Lauf
eine Zusammenfassung.

Während eines Exports sind die Eingaben gesperrt und die Schaltfläche
wechselt zu *Abbrechen*.

## Dateien und Format

Je Zähler und Zeitstufe eine Datei:

```
ZGW_<Zählername>_<Stufe>d_<JJJJMMTT-hhmmss>.csv
```

`<Zählername>` ist `friendlyId`, bereinigt um Zeichen, die in
Windows-Dateinamen unzulässig sind. Ist das Feld leer, greift
`Zaehler_<Busadresse>`.

Kodierung UTF-8 mit BOM, Semikolon als Feldtrenner, Komma als
Dezimalzeichen — so öffnet deutsches Excel die Datei per Doppelklick
korrekt. Aufbau:

```
Zeitstempel;Zeitstempel_ISO;Gesamtwirkleistung [Watt];Bezug [kWh]
05.09.2026 08:15:00;2026-09-05T08:15:00.000+02:00;1234,5;5678,90
```

Spalte A ist die deutsche Ortszeit-Darstellung, Spalte B die
unveränderte Zeitangabe der API. Die zweite Spalte ist Absicht: sie löst
die Zweideutigkeit der Stunde auf, die bei der Umstellung von Sommer-
auf Winterzeit doppelt vorkommt, und Spalte A allein nicht auflösen kann.

Ein Semikolon im Registernamen wird durch das übliche
Anführungszeichen-Quoting entschärft.

## Fehlerverhalten

Ein einzelner fehlgeschlagener Abruf beendet den Export nicht. Fehler
werden gesammelt und am Ende zusammengefasst („17 von 192 Abrufen
fehlgeschlagen"), das Protokoll nennt jeden Fall einzeln mit Zähler,
Register und Zeitstufe.

Für die häufigen Fälle gibt es Klartext statt HTTP-Codes:

| Lage | Meldung |
| --- | --- |
| DNS oder Verbindung scheitert | Gateway nicht erreichbar, mit Hinweis auf Adresse und Netz |
| `401` bei der Anmeldung | Passwort falsch |
| `401` bei Daten | still neu anmelden, einmal wiederholen, erst dann melden |
| Leere `values` | Kein Datensatz; Register wird übersprungen und im Protokoll vermerkt |
| Kein Register liefert Daten | Keine Datei schreiben, im Protokoll vermerken |

Ein Zeitüberschreitung je Anfrage liegt bei 30 Sekunden.

## Testen

`internal/zgw` und `internal/export` werden testgetrieben entwickelt.

- Client gegen `httptest.Server`: erfolgreiche Anmeldung, falsches
  Passwort, Zähler- und Typenabruf, History-Abruf, Neuanmeldung nach
  `401`, Netzwerkfehler, leere Antwort.
- Verknüpfung Zähler ↔ Registertabelle: deutscher Name bevorzugt,
  englischer als Rückfall, unbekanntes Register.
- Writer gegen erwartete Ausgabe: Zahlenformat, BOM, Trennzeichen,
  auseinanderlaufende Zeitstempel, Sonderzeichen im Spaltennamen,
  Dateinamensbereinigung.
- `internal/config` nur plattformabhängig auf Windows: Schreiben und
  Lesen im Umlauf, fehlende Datei, unlesbares Passwort.

Die Oberfläche wird nicht automatisiert getestet, sondern am Gerät geprüft.

## Bauen und Ausliefern

```
wails build -platform windows/amd64 -webview2 embed -ldflags "-s -w"
```

Ergebnis ist eine einzelne `.exe` von etwa 10 MB. `-webview2 embed`
bettet den WebView2-Bootstrapper ein: fehlt die Runtime auf einem
älteren Windows 10, bietet das Programm die Nachinstallation an, statt
wortlos nicht zu starten. Auf Windows 11 ist die Runtime immer vorhanden.

## Offene Punkte, am Gerät zu klären

**Skalierung.** Die Registertabelle nennt einen `scaling_factor`, etwa
100. Das History-Beispiel der Spec liefert jedoch bereits `25.6`, also
einen fertig skalierten Wert. Die Umsetzung rechnet deshalb **nicht**
nach. Liegen die Werte am echten Gerät um den Faktor daneben, ist das
sofort erkennbar und an einer Stelle korrigierbar.

**Nicht aufgezeichnete Register.** Ob die API für ein Register ohne
Aufzeichnung eine leere Liste oder einen Fehler liefert, sagt die Spec
nicht. Das oben beschriebene Fehlerverhalten deckt beide Fälle ab, ohne
dass die Antwort vorab bekannt sein muss.
