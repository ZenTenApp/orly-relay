/**
 * Detects the user's external IP address for per-network relay stats keying.
 * Queries multiple public IP services in parallel, uses first response.
 * Caches result for session lifetime.
 */

const IP_SERVICES = [
  {
    url: 'https://api.ipify.org?format=json',
    parse: (text: string) => {
      try { return JSON.parse(text).ip as string } catch { return null }
    }
  },
  {
    url: 'https://ifconfig.me/ip',
    parse: (text: string) => text.trim()
  },
  {
    url: 'https://icanhazip.com',
    parse: (text: string) => text.trim()
  }
]

/** Convert dotted-quad IPv4 string to 8-char hex string (e.g. "c0a80001") */
export function ipToHex(ip: string): string {
  const parts = ip.split('.')
  if (parts.length !== 4) return '00000000'
  return parts
    .map((p) => {
      const n = parseInt(p, 10)
      return (isNaN(n) || n < 0 || n > 255) ? '00' : n.toString(16).padStart(2, '0')
    })
    .join('')
}

/** Convert 8-char hex string to dotted-quad IPv4 */
export function hexToIp(hex: string): string {
  if (hex.length !== 8) return '0.0.0.0'
  const parts: number[] = []
  for (let i = 0; i < 8; i += 2) {
    parts.push(parseInt(hex.slice(i, i + 2), 16))
  }
  return parts.join('.')
}

/** Convert dotted-quad to 4-byte Uint8Array */
export function ipToBytes(ip: string): Uint8Array {
  const parts = ip.split('.')
  const bytes = new Uint8Array(4)
  for (let i = 0; i < 4; i++) {
    const n = parseInt(parts[i], 10)
    bytes[i] = isNaN(n) ? 0 : n & 0xff
  }
  return bytes
}

/** Convert 4-byte Uint8Array to dotted-quad */
export function bytesToIp(bytes: Uint8Array): string {
  return `${bytes[0]}.${bytes[1]}.${bytes[2]}.${bytes[3]}`
}

const IPV4_REGEX = /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$/

class NetworkIdentityService {
  private cachedIp: string | null = null
  private detectPromise: Promise<string | null> | null = null

  /** Get current external IP hex key. Returns null if detection failed. */
  getCurrentIpHex(): string | null {
    return this.cachedIp ? ipToHex(this.cachedIp) : null
  }

  /** Get current external IP as dotted-quad. Returns null if detection failed. */
  getCurrentIp(): string | null {
    return this.cachedIp
  }

  /** Detect external IP. Call once at session start. Returns cached result on subsequent calls. */
  async detect(): Promise<string | null> {
    if (this.cachedIp) return this.cachedIp
    if (this.detectPromise) return this.detectPromise

    this.detectPromise = this.raceDetect()
    this.cachedIp = await this.detectPromise
    this.detectPromise = null
    return this.cachedIp
  }

  private async raceDetect(): Promise<string | null> {
    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), 5000)

    try {
      const result = await Promise.any(
        IP_SERVICES.map(async (service) => {
          const resp = await fetch(service.url, { signal: controller.signal })
          if (!resp.ok) throw new Error(`${service.url}: ${resp.status}`)
          const text = await resp.text()
          const ip = service.parse(text)
          if (!ip || !IPV4_REGEX.test(ip)) throw new Error(`${service.url}: invalid IP "${ip}"`)
          return ip
        })
      )
      return result
    } catch {
      return null
    } finally {
      clearTimeout(timeout)
    }
  }
}

const networkIdentityService = new NetworkIdentityService()
export default networkIdentityService
