/**
 * Relay Admin Service
 *
 * Provides NIP-98 authenticated HTTP calls to ORLY relay management endpoints.
 * Only works when smesh is served from the same origin as the relay (embedded mode).
 */
import client from '@/services/client.service'

class RelayAdminService {
  static instance: RelayAdminService

  constructor() {
    if (!RelayAdminService.instance) {
      RelayAdminService.instance = this
    }
    return RelayAdminService.instance
  }

  private getApiBase(): string {
    return window.location.origin
  }

  private async authGet(path: string): Promise<Response> {
    const url = `${this.getApiBase()}${path}`
    const auth = await client.signHttpAuth(url, 'GET')
    return fetch(url, { headers: { Authorization: auth } })
  }

  private async authPost(path: string, body?: BodyInit, contentType?: string): Promise<Response> {
    const url = `${this.getApiBase()}${path}`
    const auth = await client.signHttpAuth(url, 'POST')
    const headers: Record<string, string> = { Authorization: auth }
    if (contentType) headers['Content-Type'] = contentType
    return fetch(url, { method: 'POST', headers, body })
  }

  private async authPut(path: string, body: BodyInit, contentType: string): Promise<Response> {
    const url = `${this.getApiBase()}${path}`
    const auth = await client.signHttpAuth(url, 'PUT')
    return fetch(url, {
      method: 'PUT',
      headers: { Authorization: auth, 'Content-Type': contentType },
      body
    })
  }

  private async authDelete(path: string): Promise<Response> {
    const url = `${this.getApiBase()}${path}`
    const auth = await client.signHttpAuth(url, 'DELETE')
    return fetch(url, { method: 'DELETE', headers: { Authorization: auth } })
  }

  // ==================== Embedded Mode Detection ====================

  async isEmbeddedInRelay(): Promise<boolean> {
    try {
      const res = await fetch(this.getApiBase(), {
        headers: { Accept: 'application/nostr+json' }
      })
      if (!res.ok) return false
      const data = await res.json()
      return !!(data && data.name)
    } catch {
      return false
    }
  }

  // ==================== Role & ACL ====================

  async fetchUserRole(): Promise<string> {
    try {
      const res = await this.authGet('/api/role')
      if (res.ok) {
        const data = await res.json()
        return data.role || ''
      }
    } catch (e) {
      console.error('fetchUserRole error:', e)
    }
    return ''
  }

  async fetchACLMode(): Promise<string> {
    try {
      const res = await fetch(`${this.getApiBase()}/api/acl-mode`)
      if (res.ok) {
        const data = await res.json()
        return data.mode || ''
      }
    } catch (e) {
      console.error('fetchACLMode error:', e)
    }
    return ''
  }

  // ==================== Relay Info (NIP-11) ====================

  async fetchRelayInfo(): Promise<Record<string, unknown> | null> {
    try {
      const res = await fetch(this.getApiBase(), {
        headers: { Accept: 'application/nostr+json' }
      })
      if (res.ok) return await res.json()
    } catch (e) {
      console.error('fetchRelayInfo error:', e)
    }
    return null
  }

  // ==================== Sprocket API ====================

  async loadSprocketConfig(): Promise<Record<string, unknown>> {
    const res = await this.authGet('/api/sprocket/config')
    if (!res.ok) throw new Error(`Failed to load config: ${res.statusText}`)
    return await res.json()
  }

  async loadSprocketStatus(): Promise<Record<string, unknown>> {
    const res = await this.authGet('/api/sprocket/status')
    if (!res.ok) throw new Error(`Failed to load status: ${res.statusText}`)
    return await res.json()
  }

  async loadSprocketScript(): Promise<string> {
    const res = await this.authGet('/api/sprocket')
    if (res.status === 404) return ''
    if (!res.ok) throw new Error(`Failed to load sprocket: ${res.statusText}`)
    return await res.text()
  }

  async saveSprocketScript(script: string): Promise<Record<string, unknown>> {
    const res = await this.authPut('/api/sprocket', script, 'text/plain')
    if (!res.ok) throw new Error(`Failed to save: ${res.statusText}`)
    return await res.json()
  }

  async restartSprocket(): Promise<Record<string, unknown>> {
    const res = await this.authPost('/api/sprocket/restart')
    if (!res.ok) throw new Error(`Failed to restart: ${res.statusText}`)
    return await res.json()
  }

  async deleteSprocket(): Promise<Record<string, unknown>> {
    const res = await this.authDelete('/api/sprocket')
    if (!res.ok) throw new Error(`Failed to delete: ${res.statusText}`)
    return await res.json()
  }

  async loadSprocketVersions(): Promise<Array<Record<string, unknown>>> {
    const res = await this.authGet('/api/sprocket/versions')
    if (!res.ok) throw new Error(`Failed to load versions: ${res.statusText}`)
    return await res.json()
  }

  async loadSprocketVersion(version: string): Promise<string> {
    const res = await this.authGet(`/api/sprocket/versions/${encodeURIComponent(version)}`)
    if (!res.ok) throw new Error(`Failed to load version: ${res.statusText}`)
    return await res.text()
  }

  async deleteSprocketVersion(filename: string): Promise<Record<string, unknown>> {
    const res = await this.authDelete(`/api/sprocket/versions/${encodeURIComponent(filename)}`)
    if (!res.ok) throw new Error(`Failed to delete version: ${res.statusText}`)
    return await res.json()
  }

  // ==================== Policy API ====================

  async loadPolicyConfig(): Promise<Record<string, unknown>> {
    const res = await this.authGet('/api/policy/config')
    if (!res.ok) throw new Error(`Failed to load policy config: ${res.statusText}`)
    return await res.json()
  }

  async loadPolicy(): Promise<Record<string, unknown>> {
    const res = await this.authGet('/api/policy')
    if (!res.ok) throw new Error(`Failed to load policy: ${res.statusText}`)
    return await res.json()
  }

  async validatePolicy(policyJson: string): Promise<Record<string, unknown>> {
    const res = await this.authPost('/api/policy/validate', policyJson, 'application/json')
    return await res.json()
  }

  async fetchPolicyFollows(): Promise<string[]> {
    const res = await this.authGet('/api/policy/follows')
    if (!res.ok) throw new Error(`Failed to fetch follows: ${res.statusText}`)
    const data = await res.json()
    return data.follows || []
  }

  // ==================== Export/Import API ====================

  async exportEvents(authorPubkeys: string[] = []): Promise<Blob> {
    const res = await this.authPost(
      '/api/export',
      JSON.stringify({ pubkeys: authorPubkeys }),
      'application/json'
    )
    if (!res.ok) throw new Error(`Export failed: ${res.statusText}`)
    return await res.blob()
  }

  async importEvents(file: File): Promise<Record<string, unknown>> {
    const formData = new FormData()
    formData.append('file', file)
    const url = `${this.getApiBase()}/api/import`
    const auth = await client.signHttpAuth(url, 'POST')
    const res = await fetch(url, {
      method: 'POST',
      headers: { Authorization: auth },
      body: formData
    })
    if (!res.ok) throw new Error(`Import failed: ${res.statusText}`)
    return await res.json()
  }

  // ==================== Log API ====================

  async getLogs(cursor?: string, limit = 100): Promise<Record<string, unknown>> {
    const params = new URLSearchParams()
    if (cursor) params.set('cursor', cursor)
    params.set('limit', String(limit))
    const res = await this.authGet(`/api/logs?${params}`)
    if (!res.ok) throw new Error(`Failed to get logs: ${res.statusText}`)
    return await res.json()
  }

  async clearLogs(): Promise<Record<string, unknown>> {
    const res = await this.authPost('/api/logs/clear')
    if (!res.ok) throw new Error(`Failed to clear logs: ${res.statusText}`)
    return await res.json()
  }

  async getLogLevel(): Promise<string> {
    const res = await this.authGet('/api/logs/level')
    if (!res.ok) throw new Error(`Failed to get log level: ${res.statusText}`)
    const data = await res.json()
    return data.level || 'info'
  }

  async setLogLevel(level: string): Promise<Record<string, unknown>> {
    const res = await this.authPost(
      '/api/logs/level',
      JSON.stringify({ level }),
      'application/json'
    )
    if (!res.ok) throw new Error(`Failed to set log level: ${res.statusText}`)
    return await res.json()
  }

  // ==================== NIP-86 Management ====================

  async nip86Request(method: string, params: unknown[] = []): Promise<Record<string, unknown>> {
    const res = await this.authPost(
      '/api/nip86',
      JSON.stringify({ method, params }),
      'application/json'
    )
    if (!res.ok) throw new Error(`NIP-86 ${method} failed: ${res.statusText}`)
    return await res.json()
  }

  // ==================== WireGuard API ====================

  async fetchWireGuardStatus(): Promise<Record<string, unknown>> {
    try {
      const res = await fetch(`${this.getApiBase()}/api/wireguard/status`)
      if (res.ok) return await res.json()
    } catch (e) {
      console.error('fetchWireGuardStatus error:', e)
    }
    return { wireguard_enabled: false, bunker_enabled: false, available: false }
  }

  async getWireGuardConfig(): Promise<Record<string, unknown>> {
    const res = await this.authGet('/api/wireguard/config')
    if (!res.ok) throw new Error(await res.text() || `Failed: ${res.statusText}`)
    return await res.json()
  }

  async regenerateWireGuard(): Promise<Record<string, unknown>> {
    const res = await this.authPost('/api/wireguard/regenerate')
    if (!res.ok) throw new Error(await res.text() || `Failed: ${res.statusText}`)
    return await res.json()
  }

  async getWireGuardAudit(): Promise<Record<string, unknown>> {
    const res = await this.authGet('/api/wireguard/audit')
    if (!res.ok) throw new Error(await res.text() || `Failed: ${res.statusText}`)
    return await res.json()
  }

  // ==================== NRC API ====================

  async fetchNRCConfig(): Promise<Record<string, unknown>> {
    try {
      const res = await fetch(`${this.getApiBase()}/api/nrc/config`)
      if (res.ok) return await res.json()
    } catch (e) {
      console.error('fetchNRCConfig error:', e)
    }
    return { enabled: false, badger_required: true }
  }

  async fetchNRCConnections(): Promise<Record<string, unknown>> {
    const res = await this.authGet('/api/nrc/connections')
    if (!res.ok) throw new Error(await res.text() || `Failed: ${res.statusText}`)
    return await res.json()
  }

  async createNRCConnection(label: string): Promise<Record<string, unknown>> {
    const res = await this.authPost(
      '/api/nrc/connections',
      JSON.stringify({ label }),
      'application/json'
    )
    if (!res.ok) throw new Error(await res.text() || `Failed: ${res.statusText}`)
    return await res.json()
  }

  async deleteNRCConnection(connId: string): Promise<Record<string, unknown>> {
    const res = await this.authDelete(`/api/nrc/connections/${connId}`)
    if (!res.ok) throw new Error(await res.text() || `Failed: ${res.statusText}`)
    return await res.json()
  }

  async getNRCConnectionURI(connId: string): Promise<Record<string, unknown>> {
    const res = await this.authGet(`/api/nrc/connections/${connId}/uri`)
    if (!res.ok) throw new Error(await res.text() || `Failed: ${res.statusText}`)
    return await res.json()
  }

  // ==================== Bunker API ====================

  async fetchBunkerInfo(): Promise<Record<string, unknown>> {
    try {
      const res = await fetch(`${this.getApiBase()}/api/bunker/info`)
      if (res.ok) return await res.json()
    } catch (e) {
      console.error('fetchBunkerInfo error:', e)
    }
    return {}
  }

  async fetchBunkerURL(): Promise<string> {
    try {
      const res = await this.authGet('/api/bunker/url')
      if (res.ok) {
        const data = await res.json()
        return data.url || ''
      }
    } catch (e) {
      console.error('fetchBunkerURL error:', e)
    }
    return ''
  }

  // ==================== Events API ====================

  async fetchMyEvents(cursor?: string, limit = 50): Promise<Record<string, unknown>> {
    const params = new URLSearchParams()
    if (cursor) params.set('cursor', cursor)
    params.set('limit', String(limit))
    const res = await this.authGet(`/api/events/mine?${params}`)
    if (!res.ok) throw new Error(`Failed: ${res.statusText}`)
    return await res.json()
  }
}

const relayAdmin = new RelayAdminService()
export default relayAdmin
