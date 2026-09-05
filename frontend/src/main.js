import {
  LoadSettings,
  Connect,
  Discover,
  ChooseFolder,
  StartExport,
  CancelExport,
} from "../wailsjs/go/main/App";
import { EventsOn } from "../wailsjs/runtime/runtime";

const $ = (id) => document.getElementById(id);

// Vorgabeadresse. Nur wenn genau die im Feld steht, darf ein Fund sie
// überschreiben — eine selbst eingetragene Adresse bleibt unangetastet.
const DEFAULT_HOST = "zgw16-ip.local";

// Beschriftung der vier Zeitstufen, die die API kennt.
const TIMEFRAMES = [
  { days: 1, label: "1 Tag — 15-Minuten-Werte" },
  { days: 14, label: "14 Tage — Tageswerte" },
  { days: 365, label: "1 Jahr — Monatswerte" },
  { days: 1095, label: "3 Jahre — Jahreswerte" },
];

let meters = [];
let exporting = false;

function log(message, level = "info") {
  const line = document.createElement("span");
  line.className = level;
  line.textContent = message + "\n";
  $("log").appendChild(line);
  $("log").scrollTop = $("log").scrollHeight;
}

function setState(text, kind = "") {
  const el = $("connection-state");
  el.textContent = text;
  el.className = "state " + kind;
}

function renderTimeframes(selected) {
  $("timeframes").innerHTML = "";
  for (const tf of TIMEFRAMES) {
    const label = document.createElement("label");
    label.className = "inline";
    const box = document.createElement("input");
    box.type = "checkbox";
    box.dataset.days = String(tf.days);
    box.checked = selected.includes(tf.days);
    label.append(box, document.createTextNode(" " + tf.label));
    $("timeframes").appendChild(label);
  }
}

function renderMeters() {
  const host = $("meters");
  host.innerHTML = "";
  for (const m of meters) {
    const details = document.createElement("details");
    details.className = "meter";
    details.open = true;

    const summary = document.createElement("summary");
    const box = document.createElement("input");
    box.type = "checkbox";
    box.checked = true;
    box.dataset.meter = String(m.busAddress);
    box.addEventListener("click", (e) => e.stopPropagation());
    box.addEventListener("change", () => {
      details
        .querySelectorAll("input[data-register]")
        .forEach((r) => (r.checked = box.checked));
    });
    summary.append(
      box,
      document.createTextNode(` Adresse ${m.busAddress} · ${m.name} `)
    );
    const type = document.createElement("span");
    type.className = "type";
    type.textContent = `(${m.typeName})`;
    summary.appendChild(type);
    details.appendChild(summary);

    const list = document.createElement("div");
    list.className = "registers";
    for (const r of m.registers || []) {
      const label = document.createElement("label");
      label.className = "inline";
      const rb = document.createElement("input");
      rb.type = "checkbox";
      rb.checked = true;
      rb.dataset.register = String(r.number);
      rb.dataset.meter = String(m.busAddress);
      const unit = r.unit ? ` [${r.unit}]` : "";
      label.append(rb, document.createTextNode(` ${r.name}${unit}`));
      list.appendChild(label);
    }
    if (!(m.registers || []).length) {
      const empty = document.createElement("span");
      empty.className = "hint";
      empty.textContent = "Für diesen Zähler zeichnet das Gateway nichts auf.";
      list.appendChild(empty);
    }
    details.appendChild(list);
    host.appendChild(details);
  }
}

function collectRequest() {
  const selection = [];
  for (const m of meters) {
    const registers = [
      ...document.querySelectorAll(
        `input[data-register][data-meter="${m.busAddress}"]:checked`
      ),
    ].map((el) => Number(el.dataset.register));
    if (registers.length) {
      selection.push({ busAddress: m.busAddress, registers });
    }
  }
  const timeFrames = [
    ...document.querySelectorAll("#timeframes input:checked"),
  ].map((el) => Number(el.dataset.days));

  return { meters: selection, timeFrames, outputDir: $("outdir").value };
}

function setExporting(active) {
  exporting = active;
  $("download").textContent = active ? "Abbrechen" : "Herunterladen";
  $("connect").disabled = active;
  $("browse").disabled = active;
}

async function connect() {
  $("connect").disabled = true;
  setState("Verbinde …");
  try {
    const res = await Connect(
      $("host").value,
      $("password").value,
      $("remember").checked
    );
    meters = res.meters || [];
    setState(`Verbunden mit ${res.gateway} · ${meters.length} Zähler`, "ok");
    renderMeters();
    $("selection-card").hidden = false;
    $("export-card").hidden = false;
  } catch (err) {
    meters = [];
    $("selection-card").hidden = true;
    $("export-card").hidden = true;
    setState(String(err), "error");
  } finally {
    $("connect").disabled = false;
  }
}

function setSearchState(text, kind = "") {
  const el = $("search-state");
  el.textContent = text;
  el.className = "state small " + kind;
}

// search sucht per mDNS nach Gateways und füllt die Auswahlliste.
// Bei genau einem Fund wird die Adresse eingetragen, sonst bleibt die
// Wahl beim Benutzer.
async function search() {
  $("search").disabled = true;
  setSearchState("Suche im Netz …");
  try {
    const found = (await Discover()) || [];
    const list = $("gateways");
    list.innerHTML = "";
    for (const g of found) {
      const opt = document.createElement("option");
      // Ausgewählt wird die IP; die Beschriftung nennt zusätzlich
      // mDNS-Namen und Gerätenamen.
      opt.value = g.ip;
      const rest = [g.host, g.name || g.model].filter(Boolean).join(" - ");
      if (rest) opt.label = rest;
      list.appendChild(opt);
    }

    if (found.length === 0) {
      setSearchState("Kein Gateway gefunden. Adresse bitte von Hand eintragen.");
      return;
    }

    const current = $("host").value.trim();
    if (found.length === 1) {
      if (current === "" || current === DEFAULT_HOST) {
        $("host").value = found[0].ip;
        setSearchState(`Gefunden: ${found[0].host || found[0].ip}`, "ok");
      } else {
        setSearchState(`1 Gateway gefunden (${found[0].ip}), Auswahl über das Feld`, "ok");
      }
      return;
    }
    setSearchState(`${found.length} Gateways gefunden — bitte im Feld auswählen`, "ok");
  } catch (err) {
    setSearchState(String(err), "error");
  } finally {
    $("search").disabled = false;
  }
}

async function browse() {
  try {
    const dir = await ChooseFolder();
    if (dir) $("outdir").value = dir;
  } catch (err) {
    log(String(err), "error");
  }
}

async function download() {
  if (exporting) {
    CancelExport();
    return;
  }
  const req = collectRequest();
  if (!req.meters.length) return log("Kein Register ausgewählt.", "warn");
  if (!req.timeFrames.length) return log("Kein Zeitraum ausgewählt.", "warn");
  if (!req.outputDir) return log("Kein Zielordner gewählt.", "warn");

  $("log").innerHTML = "";
  $("progress").value = 0;
  setExporting(true);
  try {
    await StartExport(req);
  } catch (err) {
    log(String(err), "error");
    setExporting(false);
  }
}

EventsOn("export:progress", (p) => {
  $("progress").max = p.total || 1;
  $("progress").value = p.done || 0;
  $("progress-text").textContent = `${p.done}/${p.total} · ${p.message}`;
});

EventsOn("export:log", (l) => log(l.message, l.level));

EventsOn("export:done", (s) => {
  setExporting(false);
  $("progress-text").textContent = s.message;
  log(s.message, s.failed > 0 ? "warn" : "info");
});

$("connect").addEventListener("click", connect);
$("search").addEventListener("click", search);
$("browse").addEventListener("click", browse);
$("download").addEventListener("click", download);
$("password").addEventListener("keydown", (e) => {
  if (e.key === "Enter") connect();
});

LoadSettings().then((s) => {
  $("host").value = s.host || DEFAULT_HOST;
  $("password").value = s.password || "";
  $("remember").checked = !!s.remember;
  $("outdir").value = s.outputDir || "";
  renderTimeframes([1]);
  // Beim Start im Hintergrund suchen, damit im Normalfall nichts
  // eingetippt werden muss.
  search();
});
