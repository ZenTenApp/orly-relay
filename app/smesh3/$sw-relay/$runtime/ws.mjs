// TinyJS Runtime — WebSocket Bridge
// Provides Go-callable WebSocket operations.
// Function names match Go signatures (PascalCase).

const _conns = new Map();
let _nextId = 1;

// Open a WebSocket connection. Returns connection ID.
// Callback params are Go closures compiled to JS functions.
export function Dial(url, onMessage, onOpen, onClose, onError) {
  const id = _nextId++;
  const ws = new WebSocket(url);
  const conn = { ws, id, closed: false };
  _conns.set(id, conn);

  ws.onopen = () => {
    if (onOpen) onOpen(id);
  };

  ws.onmessage = (ev) => {
    if (onMessage) {
      try {
        onMessage(id, String(ev.data));
      } catch (e) {
        console.error('relay-sw: WS onmessage CRASH:', e.message, e.stack);
      }
    }
  };

  ws.onclose = (ev) => {
    conn.closed = true;
    if (onClose) onClose(id, ev.code, ev.reason);
  };

  ws.onerror = (ev) => {
    if (onError) onError(id);
  };

  return id;
}

// Send a string message on a connection.
export function Send(connId, msg) {
  const conn = _conns.get(connId);
  if (conn && !conn.closed && conn.ws.readyState === WebSocket.OPEN) {
    conn.ws.send(msg);
    return true;
  }
  return false;
}

// Close a connection.
export function Close(connId) {
  const conn = _conns.get(connId);
  if (conn) {
    conn.closed = true;
    conn.ws.close();
    _conns.delete(connId);
  }
}

// Get connection readyState.
export function ReadyState(connId) {
  const conn = _conns.get(connId);
  if (!conn) return -1;
  return conn.ws.readyState;
}
