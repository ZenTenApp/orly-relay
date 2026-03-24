// TinyJS Runtime — Core
// Provides panic/recover/defer, program initialization, and console I/O.

export class GoPanic extends Error {
  constructor(value) {
    super(typeof value === 'string' ? value : JSON.stringify(value));
    this.goValue = value;
    this.name = 'GoPanic';
  }
}

// Defer stack. Each goroutine has its own.
export class DeferStack {
  constructor() {
    this.frames = [];
  }

  push(fn) {
    this.frames.push(fn);
  }

  run() {
    let panicked = null;
    while (this.frames.length > 0) {
      const fn = this.frames.pop();
      try {
        fn();
      } catch (e) {
        panicked = e;
      }
    }
    if (panicked) throw panicked;
  }
}

// Recover returns the panic value if called inside a deferred function
// during a panic. Otherwise returns null.
let currentPanicValue = null;
let recovering = false;

export function setPanic(val) {
  currentPanicValue = val;
}

export function recover() {
  if (recovering) {
    const val = currentPanicValue;
    currentPanicValue = null;
    recovering = false;
    return val;
  }
  return null;
}

// Panic halts the current goroutine. Deferred functions run first.
export function panic(value) {
  throw new GoPanic(value);
}

// Run deferred functions, handling panic/recover.
export async function runDefers(deferStack) {
  const frames = [...deferStack.frames].reverse();
  deferStack.frames = [];
  let lastPanic = null;

  for (const fn of frames) {
    try {
      recovering = lastPanic !== null;
      if (recovering) {
        currentPanicValue = lastPanic instanceof GoPanic ? lastPanic.goValue : lastPanic;
      }
      const result = fn();
      if (result instanceof Promise) await result;
      if (recovering && currentPanicValue === null) {
        lastPanic = null; // recovered
      }
      recovering = false;
    } catch (e) {
      recovering = false;
      lastPanic = e;
    }
  }

  if (lastPanic) throw lastPanic;
}

// Synchronous version of defer stack runner for generated code.
// Takes an array of deferred functions (already in push order; will pop from end).
export function runDeferStack(defers, panicValue) {
  let lastPanic = panicValue || null;
  while (defers.length) {
    const fn = defers.pop();
    try {
      recovering = lastPanic !== null;
      if (recovering) {
        currentPanicValue = lastPanic instanceof GoPanic ? lastPanic.goValue : lastPanic;
      }
      fn();
      // If recover() was called, it clears currentPanicValue to null.
      // Check if the panic was recovered (currentPanicValue cleared by recover()).
      if (currentPanicValue === null && lastPanic !== null) {
        lastPanic = null; // recovered
      }
      recovering = false;
    } catch (e) {
      recovering = false;
      lastPanic = e;
    }
  }
  currentPanicValue = null;
  recovering = false;
  if (lastPanic) throw lastPanic;
}

// Zero values for Go types.
export function zeroValue(typeId) {
  switch (typeId) {
    case 'bool': return false;
    case 'int': case 'int8': case 'int16': case 'int32': case 'int64':
    case 'uint': case 'uint8': case 'uint16': case 'uint32': case 'uint64':
    case 'float32': case 'float64':
    case 'uintptr':
      return 0;
    case 'complex64': case 'complex128':
      return { re: 0, im: 0 };
    case 'string':
      return '';
    default:
      return null;
  }
}

// Println — maps to fmt.Println for the runtime.
export function println(...args) {
  console.log(args.map(a => {
    if (typeof a === 'string') return a;
    if (typeof a === 'boolean') return a ? 'true' : 'false';
    if (a === null || a === undefined) return '<nil>';
    return String(a);
  }).join(' '));
}

// Print without newline.
export function print(...args) {
  const text = args.map(String).join(' ');
  if (typeof process !== 'undefined' && process.stdout) {
    process.stdout.write(text);
  } else {
    console.log(text);
  }
}

// Program exit.
export function exit(code) {
  if (typeof process !== 'undefined' && process.exit) {
    process.exit(code);
  } else {
    throw new Error('exit: ' + code);
  }
}

// Nil check helper.
export function nilCheck(ptr, op) {
  if (ptr === null || ptr === undefined) {
    panic('runtime error: invalid memory address or nil pointer dereference');
  }
}

// Bounds check helper.
export function boundsCheck(index, length) {
  if (index < 0 || index >= length) {
    panic(`runtime error: index out of range [${index}] with length ${length}`);
  }
}

// Slice bounds check.
export function sliceBoundsCheck(low, high, max) {
  if (low < 0 || high < low || max < high) {
    panic(`runtime error: slice bounds out of range [${low}:${high}:${max}]`);
  }
}
