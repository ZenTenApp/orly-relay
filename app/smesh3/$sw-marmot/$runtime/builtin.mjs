// TinyJS Runtime — Builtin Operations
// Slice, map, string operations that mirror Go's builtin functions.

// --- Slices ---

export class Slice {
  constructor(array, offset, length, capacity) {
    this.$array = array;     // backing array (JS Array)
    this.$offset = offset;   // start offset into backing array
    this.$length = length;   // number of accessible elements
    this.$capacity = capacity; // max elements before realloc
  }

  get(i) {
    if (i < 0 || i >= this.$length) {
      throw new Error(`runtime error: index out of range [${i}] with length ${this.$length}`);
    }
    return this.$array[this.$offset + i];
  }

  set(i, v) {
    if (i < 0 || i >= this.$length) {
      throw new Error(`runtime error: index out of range [${i}] with length ${this.$length}`);
    }
    this.$array[this.$offset + i] = v;
  }

  addr(i) {
    if (i < 0 || i >= this.$length) {
      throw new Error(`runtime error: index out of range [${i}] with length ${this.$length}`);
    }
    const arr = this.$array;
    const idx = this.$offset + i;
    return {
      $get: () => arr[idx],
      $set: (v) => { arr[idx] = v; }
    };
  }
}

// Make a new slice.
export function makeSlice(len, cap, zero) {
  if (cap === undefined || cap < len) cap = len;
  const arr = new Array(cap);
  for (let i = 0; i < cap; i++) arr[i] = zero !== undefined ? zero : 0;
  return new Slice(arr, 0, len, cap);
}

// Slice a slice: s[low:high:max]
export function sliceSlice(s, low, high, max) {
  if (low === undefined) low = 0;
  if (high === undefined) high = s.$length;
  if (max === undefined) max = s.$capacity;

  if (low < 0 || high < low || max < high || max > s.$capacity) {
    throw new Error(`runtime error: slice bounds out of range [${low}:${high}:${max}] with capacity ${s.$capacity}`);
  }

  return new Slice(s.$array, s.$offset + low, high - low, max - low);
}

// Append to slice.
export function append(s, ...elems) {
  if (s === null) {
    s = new Slice([], 0, 0, 0);
  }

  const needed = s.$length + elems.length;

  if (needed <= s.$capacity) {
    // Fits in existing backing array.
    for (let i = 0; i < elems.length; i++) {
      s.$array[s.$offset + s.$length + i] = elems[i];
    }
    return new Slice(s.$array, s.$offset, needed, s.$capacity);
  }

  // Need to grow. Go growth strategy: double until 256, then grow by 25%.
  let newCap = s.$capacity;
  if (newCap === 0) newCap = 1;
  while (newCap < needed) {
    if (newCap < 256) {
      newCap *= 2;
    } else {
      newCap += Math.floor(newCap / 4);
    }
  }

  const newArr = new Array(newCap);
  for (let i = 0; i < s.$length; i++) {
    newArr[i] = s.$array[s.$offset + i];
  }
  for (let i = 0; i < elems.length; i++) {
    newArr[s.$length + i] = elems[i];
  }

  return new Slice(newArr, 0, needed, newCap);
}

// Append a string's UTF-8 bytes to a byte slice: append([]byte, string...)
export function appendString(dst, s) {
  const bytes = utf8Bytes(s);
  const elems = [];
  for (let i = 0; i < bytes.length; i++) elems.push(bytes[i]);
  return append(dst, ...elems);
}

// Append slice to slice: append(a, b...)
export function appendSlice(dst, src) {
  const elems = [];
  for (let i = 0; i < src.$length; i++) {
    elems.push(src.$array[src.$offset + i]);
  }
  return append(dst, ...elems);
}

// Copy from src to dst. Returns number of elements copied.
export function copy(dst, src) {
  // Handle string source — copy UTF-8 bytes.
  if (typeof src === 'string') {
    const bytes = utf8Bytes(src);
    const n = Math.min(dst.$length, bytes.length);
    for (let i = 0; i < n; i++) {
      dst.$array[dst.$offset + i] = bytes[i];
    }
    return n;
  }

  const n = Math.min(dst.$length, src.$length);
  // Handle overlapping slices.
  if (dst.$array === src.$array && dst.$offset > src.$offset) {
    for (let i = n - 1; i >= 0; i--) {
      dst.$array[dst.$offset + i] = src.$array[src.$offset + i];
    }
  } else {
    for (let i = 0; i < n; i++) {
      dst.$array[dst.$offset + i] = src.$array[src.$offset + i];
    }
  }
  return n;
}

// Len.
export function len(v) {
  if (v === null || v === undefined) return 0;
  if (typeof v === 'string') return utf8Bytes(v).length;
  if (v instanceof Slice) return v.$length;
  if (v instanceof Map) return v.size;
  if (v instanceof GoMap) return v.size();
  if (Array.isArray(v)) return v.length;
  return 0;
}

// Cap.
export function cap(v) {
  if (v === null || v === undefined) return 0;
  if (v instanceof Slice) return v.$capacity;
  if (Array.isArray(v)) return v.length;
  return 0;
}

// --- Maps ---

// Go maps need special key handling (struct keys use deep equality).
export class GoMap {
  constructor() {
    this.entries = []; // [{key, value, hash}]
  }

  get(key) {
    for (const e of this.entries) {
      if (deepEqual(e.key, key)) return { value: e.value, ok: true };
    }
    return { value: undefined, ok: false };
  }

  set(key, value) {
    for (const e of this.entries) {
      if (deepEqual(e.key, key)) {
        e.value = value;
        return;
      }
    }
    this.entries.push({ key, value });
  }

  delete(key) {
    const idx = this.entries.findIndex(e => deepEqual(e.key, key));
    if (idx >= 0) this.entries.splice(idx, 1);
  }

  has(key) {
    return this.entries.some(e => deepEqual(e.key, key));
  }

  size() {
    return this.entries.length;
  }

  // Iterate: calls fn(key, value) for each entry.
  // Order is randomized per Go spec.
  forEach(fn) {
    // Randomize iteration order.
    const shuffled = [...this.entries];
    for (let i = shuffled.length - 1; i > 0; i--) {
      const j = Math.floor(Math.random() * (i + 1));
      [shuffled[i], shuffled[j]] = [shuffled[j], shuffled[i]];
    }
    for (const e of shuffled) {
      fn(e.key, e.value);
    }
  }
}

// For simple key types (string, number, bool), use native Map.
export function makeMap(keyKind) {
  if (keyKind === 'string' || keyKind === 'int' || keyKind === 'float64' || keyKind === 'bool') {
    return new Map();
  }
  return new GoMap();
}

// Map lookup with comma-ok.
export function mapLookup(m, key) {
  if (m === null || m === undefined) return { value: undefined, ok: false };
  if (m instanceof Map) {
    if (m.has(key)) return { value: m.get(key), ok: true };
    return { value: undefined, ok: false };
  }
  if (m instanceof GoMap) return m.get(key);
  return { value: undefined, ok: false };
}

// Map update.
export function mapUpdate(m, key, value) {
  if (m === null || m === undefined) {
    throw new Error('assignment to entry in nil map');
  }
  if (m instanceof Map) {
    m.set(key, value);
  } else if (m instanceof GoMap) {
    m.set(key, value);
  }
}

// Map delete.
export function mapDelete(m, key) {
  if (m === null || m === undefined) return;
  if (m instanceof Map) {
    m.delete(key);
  } else if (m instanceof GoMap) {
    m.delete(key);
  }
}

// --- Strings (UTF-8 byte semantics) ---

const _enc = new TextEncoder();
const _dec = new TextDecoder();

// One-string cache: amortizes TextEncoder cost when a loop indexes the same string.
let _cacheStr = '';
let _cacheBytes = new Uint8Array(0);

export function utf8Bytes(s) {
  if (s !== _cacheStr) {
    _cacheStr = s;
    _cacheBytes = _enc.encode(s);
  }
  return _cacheBytes;
}

// UTF-8 byte length of a string.
export function byteLen(s) {
  return utf8Bytes(s).length;
}

// UTF-8 byte at byte position i.
export function stringByteAt(s, i) {
  return utf8Bytes(s)[i];
}

// for range over string — returns (byteIndex, rune) using UTF-8.
export function stringRange(s) {
  const bytes = utf8Bytes(s);
  return {
    $bytes: bytes,
    $pos: 0,
    next() {
      if (this.$pos >= this.$bytes.length) return [false, 0, 0];
      const start = this.$pos;
      const b0 = this.$bytes[this.$pos];
      let cp;
      if (b0 < 0x80) {
        cp = b0; this.$pos += 1;
      } else if (b0 < 0xE0) {
        cp = ((b0 & 0x1F) << 6) | (this.$bytes[this.$pos + 1] & 0x3F);
        this.$pos += 2;
      } else if (b0 < 0xF0) {
        cp = ((b0 & 0x0F) << 12) | ((this.$bytes[this.$pos + 1] & 0x3F) << 6) |
             (this.$bytes[this.$pos + 2] & 0x3F);
        this.$pos += 3;
      } else {
        cp = ((b0 & 0x07) << 18) | ((this.$bytes[this.$pos + 1] & 0x3F) << 12) |
             ((this.$bytes[this.$pos + 2] & 0x3F) << 6) | (this.$bytes[this.$pos + 3] & 0x3F);
        this.$pos += 4;
      }
      return [true, start, cp];
    }
  };
}

// String to byte slice.
export function stringToBytes(s) {
  const bytes = _enc.encode(s);
  const arr = Array.from(bytes);
  return new Slice(arr, 0, arr.length, arr.length);
}

// Byte slice to string.
export function bytesToString(sl) {
  if (typeof sl === 'string') return sl;
  const bytes = new Uint8Array(sl.$length);
  for (let i = 0; i < sl.$length; i++) {
    bytes[i] = sl.$array[sl.$offset + i];
  }
  return _dec.decode(bytes);
}

// String to rune slice.
export function stringToRunes(s) {
  const runes = [...s].map(c => c.codePointAt(0));
  return new Slice(runes, 0, runes.length, runes.length);
}

// Rune slice to string.
export function runesToString(sl) {
  let s = '';
  for (let i = 0; i < sl.$length; i++) {
    s += String.fromCodePoint(sl.$array[sl.$offset + i]);
  }
  return s;
}

// String concatenation (Go's + operator for strings).
export function stringConcat(a, b) {
  return a + b;
}

// String comparison.
export function stringCompare(a, b) {
  if (a < b) return -1;
  if (a > b) return 1;
  return 0;
}

// String slice: s[low:high] — operates on UTF-8 byte boundaries.
export function stringSlice(s, low, high) {
  const bytes = utf8Bytes(s);
  if (low === undefined) low = 0;
  if (high === undefined) high = bytes.length;
  return _dec.decode(bytes.subarray(low, high));
}

// --- Deep equality for map keys ---

function deepEqual(a, b) {
  if (a === b) return true;
  if (a === null || b === null) return a === b;
  if (typeof a !== typeof b) return false;
  if (typeof a !== 'object') return false;

  const keysA = Object.keys(a).filter(k => !k.startsWith('$'));
  const keysB = Object.keys(b).filter(k => !k.startsWith('$'));
  if (keysA.length !== keysB.length) return false;
  return keysA.every(k => deepEqual(a[k], b[k]));
}

// Clone a Go value type (array or struct). Slices get a new backing array copy.
// Structs get shallow field copies (nested value types need recursive clone).
export function cloneValue(v) {
  if (v === null || v === undefined || typeof v !== 'object') return v;
  if (v instanceof Slice) {
    const newArr = new Array(v.$capacity);
    for (let i = 0; i < v.$length; i++) {
      const elem = v.$array[v.$offset + i];
      newArr[i] = (typeof elem === 'object' && elem !== null) ? cloneValue(elem) : elem;
    }
    return new Slice(newArr, 0, v.$length, v.$capacity);
  }
  if (Array.isArray(v)) {
    return v.map(e => (typeof e === 'object' && e !== null) ? cloneValue(e) : e);
  }
  // Struct: shallow clone with recursive value cloning.
  const obj = {};
  for (const key of Object.keys(v)) {
    if (key.startsWith('$')) continue; // skip $get/$set/$value
    const val = v[key];
    obj[key] = (typeof val === 'object' && val !== null) ? cloneValue(val) : val;
  }
  return obj;
}

// 64-bit bitwise operations using Number arithmetic (safe up to 2^53).
// JS bitwise operators truncate to 32-bit signed int; these preserve precision.
export function int64or(x, y) {
  // Split into high (above bit 32) and low (below bit 32) parts.
  const xhi = Math.trunc(x / 0x100000000);
  const xlo = x - xhi * 0x100000000;
  const yhi = Math.trunc(y / 0x100000000);
  const ylo = y - yhi * 0x100000000;
  return ((xhi | yhi) >>> 0) * 0x100000000 + ((xlo | ylo) >>> 0);
}

export function int64and(x, y) {
  const xhi = Math.trunc(x / 0x100000000);
  const xlo = x - xhi * 0x100000000;
  const yhi = Math.trunc(y / 0x100000000);
  const ylo = y - yhi * 0x100000000;
  return ((xhi & yhi) >>> 0) * 0x100000000 + ((xlo & ylo) >>> 0);
}

export function int64xor(x, y) {
  const xhi = Math.trunc(x / 0x100000000);
  const xlo = x - xhi * 0x100000000;
  const yhi = Math.trunc(y / 0x100000000);
  const ylo = y - yhi * 0x100000000;
  return ((xhi ^ yhi) >>> 0) * 0x100000000 + ((xlo ^ ylo) >>> 0);
}
