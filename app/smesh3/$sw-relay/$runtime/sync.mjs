// TinyJS Runtime — Sync Primitives
// WaitGroup, Mutex, Once, RWMutex.

export class WaitGroup {
  constructor() {
    this.counter = 0;
    this.waiters = [];
  }

  add(delta) {
    this.counter += delta;
    if (this.counter < 0) {
      throw new Error('sync: negative WaitGroup counter');
    }
    if (this.counter === 0) {
      const w = this.waiters.splice(0);
      for (const resolve of w) resolve();
    }
  }

  done() {
    this.add(-1);
  }

  async wait() {
    if (this.counter === 0) return;
    return new Promise(resolve => {
      this.waiters.push(resolve);
    });
  }
}

export class Mutex {
  constructor() {
    this.locked = false;
    this.waiters = [];
  }

  async lock() {
    if (!this.locked) {
      this.locked = true;
      return;
    }
    return new Promise(resolve => {
      this.waiters.push(resolve);
    });
  }

  unlock() {
    if (!this.locked) {
      throw new Error('sync: unlock of unlocked mutex');
    }
    if (this.waiters.length > 0) {
      const next = this.waiters.shift();
      queueMicrotask(next);
    } else {
      this.locked = false;
    }
  }
}

export class RWMutex {
  constructor() {
    this.readers = 0;
    this.writing = false;
    this.readWaiters = [];
    this.writeWaiters = [];
  }

  async rLock() {
    if (!this.writing && this.writeWaiters.length === 0) {
      this.readers++;
      return;
    }
    return new Promise(resolve => {
      this.readWaiters.push(resolve);
    });
  }

  rUnlock() {
    this.readers--;
    if (this.readers === 0 && this.writeWaiters.length > 0) {
      this.writing = true;
      const next = this.writeWaiters.shift();
      queueMicrotask(next);
    }
  }

  async lock() {
    if (!this.writing && this.readers === 0) {
      this.writing = true;
      return;
    }
    return new Promise(resolve => {
      this.writeWaiters.push(resolve);
    });
  }

  unlock() {
    if (!this.writing) {
      throw new Error('sync: unlock of unlocked RWMutex');
    }
    this.writing = false;

    // Wake waiting readers first (reader preference).
    if (this.readWaiters.length > 0) {
      const readers = this.readWaiters.splice(0);
      this.readers = readers.length;
      for (const resolve of readers) queueMicrotask(resolve);
    } else if (this.writeWaiters.length > 0) {
      this.writing = true;
      const next = this.writeWaiters.shift();
      queueMicrotask(next);
    }
  }
}

export class Once {
  constructor() {
    this.called = false;
    this.result = undefined;
  }

  do(fn) {
    if (this.called) return;
    this.called = true;
    this.result = fn();
  }
}
