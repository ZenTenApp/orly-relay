// TinyJS Runtime — Goroutine Scheduler
// Cooperative async/await scheduler with microtask queue.

const runQueue = [];
let running = false;
let goroutineId = 0;

class Goroutine {
  constructor(id, fn) {
    this.id = id;
    this.fn = fn;
    this.done = false;
    this.error = null;
  }
}

// Spawn a new goroutine.
export function spawn(fn) {
  const id = ++goroutineId;
  const g = new Goroutine(id, fn);
  runQueue.push(g);
  scheduleRun();
  return g;
}

let scheduled = false;

function scheduleRun() {
  if (!scheduled) {
    scheduled = true;
    // Use queueMicrotask for tight scheduling, setTimeout(0) for fairness.
    queueMicrotask(tick);
  }
}

async function tick() {
  scheduled = false;
  if (running) return;
  running = true;

  while (runQueue.length > 0) {
    const g = runQueue.shift();
    if (g.done) continue;

    try {
      const result = g.fn();
      if (result instanceof Promise) {
        await result;
      }
      g.done = true;
    } catch (e) {
      g.done = true;
      g.error = e;
      // Unhandled panic in goroutine — print and exit like Go does.
      if (typeof e === 'object' && e !== null && e.name === 'GoPanic') {
        console.error(`goroutine ${g.id}: panic: ${e.message}`);
      } else {
        console.error(`goroutine ${g.id}: unhandled error:`, e);
      }
      if (typeof process !== 'undefined' && process.exit) process.exit(2);
    }
  }

  running = false;
}

// Yield current goroutine to let others run.
export function gosched() {
  return new Promise(resolve => {
    queueMicrotask(resolve);
  });
}

// Sleep for a duration (nanoseconds, but we convert to ms).
export function sleep(ns) {
  const ms = Math.max(0, ns / 1_000_000);
  return new Promise(resolve => setTimeout(resolve, ms));
}

// Run the main function and wait for all goroutines.
export async function runMain(mainFn) {
  try {
    const result = mainFn();
    if (result instanceof Promise) await result;
  } catch (e) {
    console.error('marmot-sw runMain error:', e);
    if (e && e.name === 'GoPanic') {
      console.error(`panic: ${e.message}`);
      console.error(e.stack);
      try { if(self._busPort)self._busPort.postMessage('{"from":"marmot","to":"shell","msg":["LOG","marmot","PANIC: '+String(e.message).replace(/"/g,'\\"')+'"]}'); } catch(_){}
      if (typeof process !== 'undefined' && process.exit) process.exit(2);
    }
    throw e;
  }

  // Drain remaining goroutines.
  while (runQueue.length > 0) {
    await tick();
    // Small yield to let any newly-spawned goroutines enqueue.
    await new Promise(r => setTimeout(r, 0));
  }
}
