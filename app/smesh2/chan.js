// chan.js — channel primitives for async message passing
// replaces callback maps + stale closures with linear async flow

// broadcast channel: multiple listeners, each recv() gets the next send()
export function chan() {
  const waiters = []
  return {
    send(val) {
      for (const w of waiters.splice(0)) w(val)
    },
    recv() {
      return new Promise(resolve => waiters.push(resolve))
    }
  }
}

// multiplexed channel: keyed by request ID, one-shot per key, with timeout
export function mux() {
  const pending = new Map()
  return {
    send(id, val) {
      const resolve = pending.get(id)
      if (resolve) { pending.delete(id); resolve(val) }
    },
    recv(id, timeoutMs = 15000) {
      return new Promise((resolve, reject) => {
        const timer = setTimeout(() => {
          pending.delete(id)
          reject(new Error('timeout'))
        }, timeoutMs)
        pending.set(id, (val) => { clearTimeout(timer); resolve(val) })
      })
    }
  }
}
