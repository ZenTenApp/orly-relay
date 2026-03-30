// TinyJS Runtime — Bus Bridge (Relay SW)
// Routes bus messages via MessagePort to/from the page.
// Listener registered at top level to catch port before Open() runs.

// Global error handler — catches errors and forwards to page via bus port.
self.addEventListener('error', (e) => {
  console.error('relay-sw uncaught:', e.message, e.filename, e.lineno);
  if (self._busPort) {
    self._busPort.postMessage('{"from":"relay","to":"shell","msg":["LOG","relay","UNCAUGHT: ' + String(e.message).replace(/"/g, '\\"').replace(/\n/g, ' ') + ' at ' + String(e.filename).replace(/"/g, '') + ':' + e.lineno + '"]}');
  }
});
self.addEventListener('unhandledrejection', (e) => {
  var msg = e.reason ? (e.reason.message || String(e.reason)) : 'unknown';
  console.error('relay-sw unhandled rejection:', msg);
  if (self._busPort) {
    self._busPort.postMessage('{"from":"relay","to":"shell","msg":["LOG","relay","REJECTION: ' + String(msg).replace(/"/g, '\\"').replace(/\n/g, ' ') + '"]}');
  }
});

let _onMessage = null;
let _queue = [];

// waitUntil with a 25s promise — tells browser "I'm busy, don't terminate."
// With 10s keepalive interval from page, there are always 2-3 overlapping
// promises. Browser MUST keep SW alive while any waitUntil promise is pending.
function _holdOpen(ev) {
  if (ev.waitUntil) ev.waitUntil(new Promise(r => setTimeout(r, 25000)));
}

// Capture port immediately — before goroutine scheduler calls Open().
self.addEventListener('message', (ev) => {
  if (ev.data && ev.data.type === 'bus-port' && ev.ports && ev.ports[0]) {
    self._busPort = ev.ports[0];
    self._busPort.onmessage = _portHandler;
    // Flush queued messages.
    for (const m of _queue) self._busPort.postMessage(m);
    _queue = [];
    _holdOpen(ev);
    return;
  }
  // Keepalive from page — extend SW lifetime, nothing else needed.
  if (ev.data === 'keepalive') {
    _holdOpen(ev);
    return;
  }
  // Direct string message (fallback).
  const d = ev.data;
  if (typeof d === 'string' && d.length > 0 && d[0] === '{' && _onMessage) {
    _holdOpen(ev);
    try {
      _onMessage(d);
    } catch (e) {
      console.error('relay-sw: direct msg handler CRASH:', e.message, e.stack);
    }
  }
});

function _portHandler(pev) {
  const d = pev.data;
  // Health check: page sends PING, we respond PONG to prove we're alive.
  if (d === 'PING') {
    if (self._busPort) self._busPort.postMessage('PONG');
    return;
  }
  if (typeof d === 'string' && d.length > 0 && d[0] === '{' && _onMessage) {
    try {
      _onMessage(d);
    } catch (e) {
      console.error('relay-sw: bus handler CRASH:', e.message, e.stack);
      // Forward crash to shell SW via port so it reaches the page console.
      if (self._busPort) {
        self._busPort.postMessage('{"from":"relay","to":"shell","msg":["LOG","relay","BUS CRASH: ' + String(e.message).replace(/"/g, '\\"') + '"]}');
      }
    }
  }
}

export function Open(name, onMessage) {
  _onMessage = onMessage;
  // If port arrived before Open(), wire up handler now.
  if (self._busPort) {
    self._busPort.onmessage = _portHandler;
  }
  return 1;
}

export function Send(bcId, msg) {
  if (self._busPort) {
    try { self._busPort.postMessage(msg); }
    catch (e) { console.error('relay bus send failed:', e.message); }
  } else {
    _queue.push(msg);
  }
}

export function Close(bcId) {}
