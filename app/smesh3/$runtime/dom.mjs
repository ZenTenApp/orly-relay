// TinyJS Runtime — DOM Bridge
// Provides Go-callable browser DOM operations.
// Elements are tracked by integer handles.
// Function names match Go signatures (PascalCase).

const _elements = new Map();
let _nextId = 1;
const _callbacks = new Map();
let _nextCb = 1;

// Bootstrap: map handle 0 to document.body (available after DOMContentLoaded).
function ensureBody() {
  if (!_elements.has(0) && typeof document !== 'undefined' && document.body) {
    _elements.set(0, document.body);
  }
}

// Element creation and lookup.
export function CreateElement(tag) {
  const el = document.createElement(tag);
  const id = _nextId++;
  _elements.set(id, el);
  return id;
}

export function CreateTextNode(text) {
  const node = document.createTextNode(text);
  const id = _nextId++;
  _elements.set(id, node);
  return id;
}

export function GetElementById(id) {
  const el = document.getElementById(id);
  if (!el) return -1;
  const handle = _nextId++;
  _elements.set(handle, el);
  return handle;
}

export function QuerySelector(sel) {
  const el = document.querySelector(sel);
  if (!el) return -1;
  const handle = _nextId++;
  _elements.set(handle, el);
  return handle;
}

export function Body() {
  ensureBody();
  return 0;
}

// Tree manipulation.
export function AppendChild(parentId, childId) {
  const parent = _elements.get(parentId);
  const child = _elements.get(childId);
  if (parent && child) parent.appendChild(child);
}

export function RemoveChild(parentId, childId) {
  const parent = _elements.get(parentId);
  const child = _elements.get(childId);
  if (parent && child) parent.removeChild(child);
}

export function FirstChild(parentId) {
  const parent = _elements.get(parentId);
  if (parent && parent.firstChild) {
    const id = _nextId++;
    _elements.set(id, parent.firstChild);
    return id;
  }
  return 0;
}

export function NextSibling(elId) {
  const el = _elements.get(elId);
  if (el && el.nextSibling) {
    const id = _nextId++;
    _elements.set(id, el.nextSibling);
    return id;
  }
  return 0;
}

export function InsertBefore(parentId, newId, refId) {
  const parent = _elements.get(parentId);
  const newEl = _elements.get(newId);
  const ref = refId >= 0 ? _elements.get(refId) : null;
  if (parent && newEl) parent.insertBefore(newEl, ref);
}

export function ReplaceChild(parentId, newId, oldId) {
  const parent = _elements.get(parentId);
  const newEl = _elements.get(newId);
  const oldEl = _elements.get(oldId);
  if (parent && newEl && oldEl) parent.replaceChild(newEl, oldEl);
}

// Properties and attributes.
export function SetAttribute(elId, name, value) {
  const el = _elements.get(elId);
  if (el) el.setAttribute(name, value);
}

export function RemoveAttribute(elId, name) {
  const el = _elements.get(elId);
  if (el) el.removeAttribute(name);
}

export function SetTextContent(elId, text) {
  const el = _elements.get(elId);
  if (el) el.textContent = text;
}

export function SetInnerHTML(elId, html) {
  const el = _elements.get(elId);
  if (el) el.innerHTML = html;
}

export function SetStyle(elId, prop, value) {
  const el = _elements.get(elId);
  if (el) el.style[prop] = value;
}

export function SetProperty(elId, prop, value) {
  const el = _elements.get(elId);
  if (el) el[prop] = value;
}

export function GetProperty(elId, prop) {
  const el = _elements.get(elId);
  if (el) return String(el[prop] ?? '');
  return '';
}

export function AddClass(elId, cls) {
  const el = _elements.get(elId);
  if (el && el.classList) el.classList.add(cls);
}

export function RemoveClass(elId, cls) {
  const el = _elements.get(elId);
  if (el && el.classList) el.classList.remove(cls);
}

// Events.
export function AddEventListener(elId, event, callbackId) {
  const el = _elements.get(elId);
  const cb = _callbacks.get(callbackId);
  if (el && cb) el.addEventListener(event, cb);
}

export function RemoveEventListener(elId, event, callbackId) {
  const el = _elements.get(elId);
  const cb = _callbacks.get(callbackId);
  if (el && cb) el.removeEventListener(event, cb);
}

// Register a Go function as a JS callback. Returns callback ID.
export function RegisterCallback(fn) {
  const id = _nextCb++;
  _callbacks.set(id, fn);
  return id;
}

export function ReleaseCallback(id) {
  _callbacks.delete(id);
}

// Scheduling.
export function RequestAnimationFrame(fn) {
  if (typeof window !== 'undefined') {
    window.requestAnimationFrame(fn);
  } else {
    setTimeout(fn, 16);
  }
}

export function SetTimeout(fn, ms) {
  return setTimeout(fn, ms);
}

export function SetInterval(fn, ms) {
  return setInterval(fn, ms);
}

export function ClearInterval(id) {
  clearInterval(id);
}

export function ClearTimeout(id) {
  clearTimeout(id);
}

// Cleanup: release element handle.
export function ReleaseElement(id) {
  _elements.delete(id);
}

// Fetch a URL as text, call fn with result.
export function FetchText(url, fn) {
  fetch(url).then(r => r.text()).then(t => { if (fn) fn(t); });
}

// Fetch NIP-11 relay info document with Accept header.
export function FetchRelayInfo(url, fn) {
  fetch(url, { headers: { 'Accept': 'application/nostr+json' } })
    .then(r => r.text())
    .then(t => { if (fn) fn(t); })
    .catch(() => { if (fn) fn(''); });
}

// Log a message to the browser console.
export function ConsoleLog(msg) {
  console.log('[sm3sh]', msg);
}

// Get raw element (for advanced use within JS runtime only).
export function getRawElement(id) {
  return _elements.get(id);
}
