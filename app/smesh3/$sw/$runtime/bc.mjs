// TinyJS Runtime — Bus Bridge (Shell SW)
// Routes bus messages via postMessage to/from page clients.
// The page relays to satellite SW iframes.

let _onMessage = null;

export function Open(name, onMessage) {
  _onMessage = onMessage;
  // Bus messages from the page arrive as strings starting with '{'.
  self.addEventListener('message', (ev) => {
    const d = ev.data;
    if (typeof d === 'string' && d.length > 0 && d[0] === '{' && _onMessage) {
      _onMessage(d);
    }
  });
  return 1;
}

export function Send(bcId, msg) {
  self.clients.matchAll({ type: 'window' }).then(clients => {
    for (const c of clients) {
      try { c.postMessage(msg); } catch(e) {}
    }
  });
}

export function Close(bcId) {}
