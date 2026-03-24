// TinyJS Runtime — Channels
// Go channels with buffered/unbuffered semantics, close, and select.

export class Channel {
  constructor(bufferSize = 0) {
    this.bufferSize = bufferSize;
    this.buffer = [];
    this.closed = false;
    this.sendWaiters = []; // [{value, resolve, reject}]
    this.recvWaiters = []; // [{resolve, reject}]
  }

  // Send a value. Blocks (awaits) if buffer full and no receiver waiting.
  async send(value) {
    if (this.closed) {
      throw new Error('send on closed channel');
    }

    // If a receiver is waiting, hand off directly.
    if (this.recvWaiters.length > 0) {
      const waiter = this.recvWaiters.shift();
      waiter.resolve({ value, ok: true });
      return;
    }

    // If buffer has space, enqueue.
    if (this.buffer.length < this.bufferSize) {
      this.buffer.push(value);
      return;
    }

    // Block until a receiver arrives.
    return new Promise((resolve, reject) => {
      this.sendWaiters.push({ value, resolve, reject });
    });
  }

  // Receive a value. Blocks (awaits) if buffer empty and no sender waiting.
  async recv() {
    // If buffer has data, return it. Then unblock a sender if waiting.
    if (this.buffer.length > 0) {
      const value = this.buffer.shift();
      if (this.sendWaiters.length > 0) {
        const sender = this.sendWaiters.shift();
        this.buffer.push(sender.value);
        sender.resolve();
      }
      return { value, ok: true };
    }

    // If a sender is waiting (unbuffered or buffer was empty), take directly.
    if (this.sendWaiters.length > 0) {
      const sender = this.sendWaiters.shift();
      sender.resolve();
      return { value: sender.value, ok: true };
    }

    // If closed and nothing buffered, return zero value.
    if (this.closed) {
      return { value: undefined, ok: false };
    }

    // Block until a sender arrives or channel closes.
    return new Promise((resolve) => {
      this.recvWaiters.push({ resolve });
    });
  }

  // Close the channel.
  close() {
    if (this.closed) {
      throw new Error('close of closed channel');
    }
    this.closed = true;

    // Wake all waiting receivers with zero value.
    while (this.recvWaiters.length > 0) {
      const waiter = this.recvWaiters.shift();
      waiter.resolve({ value: undefined, ok: false });
    }

    // Panic on waiting senders.
    while (this.sendWaiters.length > 0) {
      const sender = this.sendWaiters.shift();
      sender.reject(new Error('send on closed channel'));
    }
  }

  // Non-blocking try-send. Returns true if sent.
  trySend(value) {
    if (this.closed) throw new Error('send on closed channel');

    if (this.recvWaiters.length > 0) {
      const waiter = this.recvWaiters.shift();
      waiter.resolve({ value, ok: true });
      return true;
    }

    if (this.buffer.length < this.bufferSize) {
      this.buffer.push(value);
      return true;
    }

    return false;
  }

  // Non-blocking try-recv. Returns {value, ok, received}.
  tryRecv() {
    if (this.buffer.length > 0) {
      const value = this.buffer.shift();
      if (this.sendWaiters.length > 0) {
        const sender = this.sendWaiters.shift();
        this.buffer.push(sender.value);
        sender.resolve();
      }
      return { value, ok: true, received: true };
    }

    if (this.sendWaiters.length > 0) {
      const sender = this.sendWaiters.shift();
      sender.resolve();
      return { value: sender.value, ok: true, received: true };
    }

    if (this.closed) {
      return { value: undefined, ok: false, received: true };
    }

    return { value: undefined, ok: false, received: false };
  }
}

// Select statement implementation.
// cases: [{ch, dir: 'send'|'recv', value?, id}]
// hasDefault: boolean
// Returns: {id, value, ok}
export async function select(cases, hasDefault) {
  // Shuffle cases for fairness (Go spec requires pseudo-random selection).
  const shuffled = cases.map((c, i) => ({ ...c, origIndex: i }));
  for (let i = shuffled.length - 1; i > 0; i--) {
    const j = Math.floor(Math.random() * (i + 1));
    [shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]];
  }

  // Try non-blocking first.
  for (const c of shuffled) {
    if (c.dir === 'send') {
      if (c.ch.trySend(c.value)) {
        return { id: c.id, value: undefined, ok: true };
      }
    } else {
      const result = c.ch.tryRecv();
      if (result.received) {
        return { id: c.id, value: result.value, ok: result.ok };
      }
    }
  }

  // Default case.
  if (hasDefault) {
    return { id: -1, value: undefined, ok: false };
  }

  // Block: race all cases.
  return new Promise((resolve) => {
    const cleanup = [];
    let resolved = false;

    for (const c of shuffled) {
      if (c.dir === 'recv') {
        const waiter = {
          resolve: (result) => {
            if (resolved) return;
            resolved = true;
            // Remove other waiters.
            for (const fn of cleanup) fn();
            resolve({ id: c.id, value: result.value, ok: result.ok });
          }
        };
        c.ch.recvWaiters.push(waiter);
        cleanup.push(() => {
          const idx = c.ch.recvWaiters.indexOf(waiter);
          if (idx >= 0) c.ch.recvWaiters.splice(idx, 1);
        });
      } else {
        const waiter = {
          value: c.value,
          resolve: () => {
            if (resolved) return;
            resolved = true;
            for (const fn of cleanup) fn();
            resolve({ id: c.id, value: undefined, ok: true });
          },
          reject: (err) => {
            if (resolved) return;
            resolved = true;
            for (const fn of cleanup) fn();
            // Propagate error (send on closed channel).
            throw err;
          }
        };
        c.ch.sendWaiters.push(waiter);
        cleanup.push(() => {
          const idx = c.ch.sendWaiters.indexOf(waiter);
          if (idx >= 0) c.ch.sendWaiters.splice(idx, 1);
        });
      }
    }
  });
}
