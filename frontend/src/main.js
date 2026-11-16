import "./style.css";
import {Events, Window} from "@wailsio/runtime";
import {Service} from "../bindings/github.com/a27259954/btaudio";
import {LANGUAGES, setLanguage, t} from "./i18n.js";

const $ = (id) => document.getElementById(id);
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({"&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;"}[c]));

let state = {devices: []};
let lang = "";
let toastTimer;

function toast(msg) {
  const el = $("toast");
  el.textContent = msg;
  el.classList.add("show");
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.classList.remove("show"), 2000);
}

function applyLanguage(code) {
  if (code === lang) return;
  lang = code;
  setLanguage(code);
  document.querySelectorAll("[data-i18n]").forEach((el) => { el.textContent = t(el.dataset.i18n); });
  document.querySelectorAll("[data-i18n-title]").forEach((el) => { el.title = t(el.dataset.i18nTitle); });
  $("opt-lang").innerHTML = [["", t("langAuto")], ...LANGUAGES]
    .map(([v, name]) => `<option value="${v}">${esc(name)}</option>`).join("");
}

function renderHero() {
  const devs = state.devices || [];
  const connected = devs.filter((d) => d.status === "connected");
  const connecting = devs.some((d) => d.status === "connecting");
  $("hero").dataset.status = connected.length ? "connected" : connecting ? "connecting" : "idle";
  $("hero-title").textContent = connected.length
    ? connected.map((d) => d.name).join(", ")
    : t(connecting ? "connecting" : "notConnected");
  $("output").textContent = state.output || "—";
  $("btn-fix").disabled = !connected.length;
}

function renderDevices() {
  const devs = state.devices || [];
  if (!devs.length) {
    $("devices").innerHTML = `<div class="card empty"><span>${t("noPhones")}</span><button class="btn" data-act="bt">${t("btSettings")}</button></div>`;
    return;
  }
  $("devices").innerHTML = devs.map((d) => {
    const id = esc(d.id);
    const err = d.status === "idle" && d.error ? `<div class="device-error">${esc(t("err_" + d.error))}</div>` : "";
    const btn = d.status === "idle"
      ? `<button class="btn" data-act="connect" data-id="${id}">${t("connect")}</button>`
      : `<button class="btn ghost" data-act="disconnect" data-id="${id}">${t(d.status === "connecting" ? "cancel" : "disconnect")}</button>`;
    return `
      <div class="card device" data-status="${d.status}">
        <span class="dot"></span>
        <div class="device-text"><div class="device-name" title="${esc(d.name)}">${esc(d.name)}</div>${err}</div>
        <label class="auto" title="${t("autoTip")}">${t("auto")}<input type="checkbox" class="switch sm" data-act="auto" data-id="${id}" ${d.auto ? "checked" : ""}/></label>
        ${btn}
      </div>`;
  }).join("");
}

function render(s) {
  state = s || state;
  applyLanguage(state.lang || "en");
  renderHero();
  renderDevices();
  $("opt-autostart").checked = !!state.autostart;
  $("opt-lang").value = state.langSetting || "";
  $("version").textContent = state.version && state.version !== "dev" ? state.version : "";
  const err = $("bt-error");
  err.hidden = !state.error;
  err.textContent = state.error ? t("listError") : "";
  fitWindow();
}

// Size the window to its content, so one phone leaves no gap and a few
// phones need no scrolling.
let windowHeight = 0;
function fitWindow() {
  requestAnimationFrame(() => {
    const h = Math.ceil(document.querySelector(".titlebar").offsetHeight + document.querySelector(".content").offsetHeight);
    if (h !== windowHeight) {
      windowHeight = h;
      Window.SetSize(360, h);
    }
  });
}

$("devices").addEventListener("click", (e) => {
  const el = e.target.closest("[data-act]");
  if (!el) return;
  if (el.dataset.act === "connect") Service.Connect(el.dataset.id);
  if (el.dataset.act === "disconnect") Service.Disconnect(el.dataset.id);
  if (el.dataset.act === "bt") Service.OpenBluetoothSettings();
});
$("devices").addEventListener("change", (e) => {
  if (e.target.dataset.act === "auto") Service.SetAuto(e.target.dataset.id, e.target.checked);
});

$("btn-fix").onclick = () => { Service.Repair(); toast(t("repairing")); };
$("btn-min").onclick = () => Window.Minimise();
$("btn-close").onclick = () => Window.Hide();
$("opt-autostart").onchange = (e) => Service.SetAutostart(e.target.checked).catch(() => {
  toast(t("saveFailed"));
  e.target.checked = !e.target.checked;
});
$("opt-lang").onchange = (e) => Service.SetLanguage(e.target.value);

Events.On("state", (ev) => render(ev.data));
Events.On("level", (ev) => {
  const v = Math.min(1, Math.sqrt(Math.max(0, ev.data || 0)) * 1.15);
  $("meter-fill").style.width = `${(v * 100).toFixed(1)}%`;
});
Service.GetState().then(render);
