import { useCallback, useEffect, useState } from 'react'
import relayAdmin from '@/services/relay-admin.service'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { toast } from 'sonner'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface PubkeyEntry {
  pubkey: string
  reason?: string
}

interface EventEntry {
  id: string
  reason?: string
}

interface IPEntry {
  ip: string
  reason?: string
}

interface RelayConfig {
  relay_name: string
  relay_description: string
  relay_icon: string
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

async function nip86(method: string, params: unknown[] = []): Promise<unknown> {
  const res = await relayAdmin.nip86Request(method, params)
  if ((res as { error?: string }).error) {
    throw new Error((res as { error: string }).error)
  }
  return (res as { result?: unknown }).result
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function ListShell({
  children,
  empty
}: {
  children: React.ReactNode
  empty: string
}) {
  const hasChildren = Array.isArray(children) ? children.length > 0 : !!children
  return (
    <div className="rounded-lg border border-border bg-card max-h-72 overflow-y-auto">
      {hasChildren ? children : (
        <div className="py-8 text-center text-sm text-muted-foreground italic">{empty}</div>
      )}
    </div>
  )
}

function ListRow({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-3 border-b border-border px-3 py-2 last:border-b-0 text-sm">
      {children}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Banned Pubkeys
// ---------------------------------------------------------------------------

function BannedPubkeysSection() {
  const [items, setItems] = useState<PubkeyEntry[]>([])
  const [pubkey, setPubkey] = useState('')
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    try {
      const r = await nip86('listbannedpubkeys')
      setItems(Array.isArray(r) ? r : [])
    } catch (e) {
      toast.error(`Load banned pubkeys: ${e instanceof Error ? e.message : String(e)}`)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const ban = async () => {
    if (!pubkey.trim()) return
    setBusy(true)
    try {
      await nip86('banpubkey', [pubkey.trim(), reason.trim()])
      toast.success('Pubkey banned')
      setPubkey('')
      setReason('')
      await load()
    } catch (e) {
      toast.error(`Ban failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-3">
      <h3 className="text-base font-semibold">Banned Pubkeys</h3>
      <div className="flex flex-wrap gap-2">
        <Input className="flex-1 min-w-[200px]" placeholder="Pubkey (64 hex chars)" value={pubkey} onChange={(e) => setPubkey(e.target.value)} />
        <Input className="flex-1 min-w-[140px]" placeholder="Reason (optional)" value={reason} onChange={(e) => setReason(e.target.value)} />
        <Button size="sm" onClick={ban} disabled={busy}>Ban Pubkey</Button>
      </div>
      <ListShell empty="No banned pubkeys.">
        {items.map((it, i) => (
          <ListRow key={i}>
            <span className="font-mono text-xs break-all flex-1">{it.pubkey}</span>
            {it.reason && <span className="text-muted-foreground italic text-xs">{it.reason}</span>}
          </ListRow>
        ))}
      </ListShell>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Allowed Pubkeys
// ---------------------------------------------------------------------------

function AllowedPubkeysSection() {
  const [items, setItems] = useState<PubkeyEntry[]>([])
  const [pubkey, setPubkey] = useState('')
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    try {
      const r = await nip86('listallowedpubkeys')
      setItems(Array.isArray(r) ? r : [])
    } catch (e) {
      toast.error(`Load allowed pubkeys: ${e instanceof Error ? e.message : String(e)}`)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const allow = async () => {
    if (!pubkey.trim()) return
    setBusy(true)
    try {
      await nip86('allowpubkey', [pubkey.trim(), reason.trim()])
      toast.success('Pubkey allowed')
      setPubkey('')
      setReason('')
      await load()
    } catch (e) {
      toast.error(`Allow failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-3">
      <h3 className="text-base font-semibold">Allowed Pubkeys</h3>
      <div className="flex flex-wrap gap-2">
        <Input className="flex-1 min-w-[200px]" placeholder="Pubkey (64 hex chars)" value={pubkey} onChange={(e) => setPubkey(e.target.value)} />
        <Input className="flex-1 min-w-[140px]" placeholder="Reason (optional)" value={reason} onChange={(e) => setReason(e.target.value)} />
        <Button size="sm" onClick={allow} disabled={busy}>Allow Pubkey</Button>
      </div>
      <ListShell empty="No allowed pubkeys.">
        {items.map((it, i) => (
          <ListRow key={i}>
            <span className="font-mono text-xs break-all flex-1">{it.pubkey}</span>
            {it.reason && <span className="text-muted-foreground italic text-xs">{it.reason}</span>}
          </ListRow>
        ))}
      </ListShell>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Banned Events
// ---------------------------------------------------------------------------

function BannedEventsSection() {
  const [items, setItems] = useState<EventEntry[]>([])
  const [eventId, setEventId] = useState('')
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    try {
      const r = await nip86('listbannedevents')
      setItems(Array.isArray(r) ? r : [])
    } catch (e) {
      toast.error(`Load banned events: ${e instanceof Error ? e.message : String(e)}`)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const ban = async () => {
    if (!eventId.trim()) return
    setBusy(true)
    try {
      await nip86('banevent', [eventId.trim(), reason.trim()])
      toast.success('Event banned')
      setEventId('')
      setReason('')
      await load()
    } catch (e) {
      toast.error(`Ban failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-3">
      <h3 className="text-base font-semibold">Banned Events</h3>
      <div className="flex flex-wrap gap-2">
        <Input className="flex-1 min-w-[200px]" placeholder="Event ID (64 hex chars)" value={eventId} onChange={(e) => setEventId(e.target.value)} />
        <Input className="flex-1 min-w-[140px]" placeholder="Reason (optional)" value={reason} onChange={(e) => setReason(e.target.value)} />
        <Button size="sm" onClick={ban} disabled={busy}>Ban Event</Button>
      </div>
      <ListShell empty="No banned events.">
        {items.map((it, i) => (
          <ListRow key={i}>
            <span className="font-mono text-xs break-all flex-1">{it.id}</span>
            {it.reason && <span className="text-muted-foreground italic text-xs">{it.reason}</span>}
          </ListRow>
        ))}
      </ListShell>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Allowed Events (allow only, no list endpoint)
// ---------------------------------------------------------------------------

function AllowedEventsSection() {
  const [eventId, setEventId] = useState('')
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)

  const allow = async () => {
    if (!eventId.trim()) return
    setBusy(true)
    try {
      await nip86('allowevent', [eventId.trim(), reason.trim()])
      toast.success('Event allowed')
      setEventId('')
      setReason('')
    } catch (e) {
      toast.error(`Allow failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-3">
      <h3 className="text-base font-semibold">Allow Event</h3>
      <div className="flex flex-wrap gap-2">
        <Input className="flex-1 min-w-[200px]" placeholder="Event ID (64 hex chars)" value={eventId} onChange={(e) => setEventId(e.target.value)} />
        <Input className="flex-1 min-w-[140px]" placeholder="Reason (optional)" value={reason} onChange={(e) => setReason(e.target.value)} />
        <Button size="sm" onClick={allow} disabled={busy}>Allow Event</Button>
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Allowed Kinds
// ---------------------------------------------------------------------------

function AllowedKindsSection() {
  const [items, setItems] = useState<number[]>([])
  const [kindInput, setKindInput] = useState('')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    try {
      const r = await nip86('listallowedkinds')
      setItems(Array.isArray(r) ? r : [])
    } catch (e) {
      toast.error(`Load allowed kinds: ${e instanceof Error ? e.message : String(e)}`)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const addKind = async () => {
    const num = parseInt(kindInput, 10)
    if (isNaN(num)) {
      toast.error('Invalid kind number')
      return
    }
    setBusy(true)
    try {
      await nip86('allowkind', [num])
      toast.success(`Kind ${num} allowed`)
      setKindInput('')
      await load()
    } catch (e) {
      toast.error(`Allow kind failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setBusy(false)
    }
  }

  const removeKind = async (kind: number) => {
    setBusy(true)
    try {
      await nip86('disallowkind', [kind])
      toast.success(`Kind ${kind} disallowed`)
      await load()
    } catch (e) {
      toast.error(`Disallow kind failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-3">
      <h3 className="text-base font-semibold">Allowed Event Kinds</h3>
      <div className="flex flex-wrap gap-2">
        <Input className="w-40" type="number" placeholder="Kind number" value={kindInput} onChange={(e) => setKindInput(e.target.value)} />
        <Button size="sm" onClick={addKind} disabled={busy}>Allow Kind</Button>
      </div>
      <ListShell empty="No allowed kinds configured. All kinds are allowed by default.">
        {items.map((kind) => (
          <ListRow key={kind}>
            <span className="font-mono text-xs flex-1">Kind {kind}</span>
            <Button variant="ghost-destructive" size="sm" onClick={() => removeKind(kind)} disabled={busy}>Remove</Button>
          </ListRow>
        ))}
      </ListShell>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Blocked IPs
// ---------------------------------------------------------------------------

function BlockedIPsSection() {
  const [items, setItems] = useState<IPEntry[]>([])
  const [ip, setIp] = useState('')
  const [reason, setReason] = useState('')
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    try {
      const r = await nip86('listblockedips')
      setItems(Array.isArray(r) ? r : [])
    } catch (e) {
      toast.error(`Load blocked IPs: ${e instanceof Error ? e.message : String(e)}`)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const block = async () => {
    if (!ip.trim()) return
    setBusy(true)
    try {
      await nip86('blockip', [ip.trim(), reason.trim()])
      toast.success('IP blocked')
      setIp('')
      setReason('')
      await load()
    } catch (e) {
      toast.error(`Block failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setBusy(false)
    }
  }

  const unblock = async (addr: string) => {
    setBusy(true)
    try {
      await nip86('unblockip', [addr])
      toast.success('IP unblocked')
      await load()
    } catch (e) {
      toast.error(`Unblock failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-3">
      <h3 className="text-base font-semibold">Blocked IPs</h3>
      <div className="flex flex-wrap gap-2">
        <Input className="flex-1 min-w-[160px]" placeholder="IP address" value={ip} onChange={(e) => setIp(e.target.value)} />
        <Input className="flex-1 min-w-[140px]" placeholder="Reason (optional)" value={reason} onChange={(e) => setReason(e.target.value)} />
        <Button size="sm" onClick={block} disabled={busy}>Block IP</Button>
      </div>
      <ListShell empty="No blocked IPs.">
        {items.map((it, i) => (
          <ListRow key={i}>
            <span className="font-mono text-xs flex-1">{it.ip}</span>
            {it.reason && <span className="text-muted-foreground italic text-xs">{it.reason}</span>}
            <Button variant="ghost-destructive" size="sm" onClick={() => unblock(it.ip)} disabled={busy}>Unblock</Button>
          </ListRow>
        ))}
      </ListShell>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Events Needing Moderation
// ---------------------------------------------------------------------------

function ModerationSection() {
  const [items, setItems] = useState<EventEntry[]>([])
  const [busy, setBusy] = useState(false)

  const load = useCallback(async () => {
    setBusy(true)
    try {
      const r = await nip86('listeventsneedingmoderation')
      setItems(Array.isArray(r) ? r : [])
    } catch (e) {
      toast.error(`Load moderation queue: ${e instanceof Error ? e.message : String(e)}`)
      setItems([])
    } finally {
      setBusy(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const approve = async (id: string) => {
    setBusy(true)
    try {
      await nip86('allowevent', [id, 'Approved from moderation queue'])
      toast.success('Event approved')
      await load()
    } catch (e) {
      toast.error(`Approve failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setBusy(false)
    }
  }

  const reject = async (id: string) => {
    setBusy(true)
    try {
      await nip86('banevent', [id, 'Rejected from moderation queue'])
      toast.success('Event rejected')
      await load()
    } catch (e) {
      toast.error(`Reject failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <h3 className="text-base font-semibold">Events Needing Moderation</h3>
        <Button variant="outline" size="sm" onClick={load} disabled={busy}>
          {busy ? 'Loading...' : 'Refresh'}
        </Button>
      </div>
      <ListShell empty="No events need moderation.">
        {items.map((it, i) => (
          <ListRow key={i}>
            <span className="font-mono text-xs break-all flex-1">{it.id}</span>
            {it.reason && <span className="text-muted-foreground italic text-xs">{it.reason}</span>}
            <div className="flex gap-1 shrink-0">
              <Button size="sm" onClick={() => approve(it.id)} disabled={busy}>Allow</Button>
              <Button variant="destructive" size="sm" onClick={() => reject(it.id)} disabled={busy}>Ban</Button>
            </div>
          </ListRow>
        ))}
      </ListShell>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Relay Config
// ---------------------------------------------------------------------------

function RelayConfigSection() {
  const [config, setConfig] = useState<RelayConfig>({
    relay_name: '',
    relay_description: '',
    relay_icon: ''
  })
  const [busy, setBusy] = useState(false)

  const fetchInfo = useCallback(async () => {
    setBusy(true)
    try {
      const info = await relayAdmin.fetchRelayInfo()
      if (info) {
        setConfig({
          relay_name: (info.name as string) || '',
          relay_description: (info.description as string) || '',
          relay_icon: (info.icon as string) || ''
        })
      }
    } catch (e) {
      toast.error(`Fetch relay info: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setBusy(false)
    }
  }, [])

  useEffect(() => { fetchInfo() }, [fetchInfo])

  const save = async () => {
    setBusy(true)
    try {
      const updates: Promise<unknown>[] = []
      if (config.relay_name) updates.push(nip86('changerelayname', [config.relay_name]))
      if (config.relay_description) updates.push(nip86('changerelaydescription', [config.relay_description]))
      if (config.relay_icon) updates.push(nip86('changerelayicon', [config.relay_icon]))

      if (updates.length === 0) {
        toast.info('No changes to update')
        return
      }

      await Promise.all(updates)
      toast.success('Relay configuration updated')
      await fetchInfo()
    } catch (e) {
      toast.error(`Update failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-base font-semibold">Relay Configuration</h3>
        <Button variant="outline" size="sm" onClick={fetchInfo} disabled={busy}>Refresh</Button>
      </div>
      <div className="space-y-3">
        <div className="space-y-1">
          <label className="text-sm font-medium" htmlFor="acl-relay-name">Relay Name</label>
          <Input
            id="acl-relay-name"
            placeholder="Enter relay name"
            value={config.relay_name}
            onChange={(e) => setConfig((c) => ({ ...c, relay_name: e.target.value }))}
          />
        </div>
        <div className="space-y-1">
          <label className="text-sm font-medium" htmlFor="acl-relay-desc">Relay Description</label>
          <textarea
            id="acl-relay-desc"
            className="flex w-full rounded-lg border border-input bg-background px-3 py-2 text-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:border-ring min-h-[80px] resize-y"
            placeholder="Enter relay description"
            value={config.relay_description}
            onChange={(e) => setConfig((c) => ({ ...c, relay_description: e.target.value }))}
          />
        </div>
        <div className="space-y-1">
          <label className="text-sm font-medium" htmlFor="acl-relay-icon">Relay Icon URL</label>
          <Input
            id="acl-relay-icon"
            type="url"
            placeholder="Enter icon URL"
            value={config.relay_icon}
            onChange={(e) => setConfig((c) => ({ ...c, relay_icon: e.target.value }))}
          />
        </div>
      </div>
      <Button onClick={save} disabled={busy}>
        {busy ? 'Saving...' : 'Save Configuration'}
      </Button>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

export default function ManagedACLTab() {
  return (
    <div className="p-4 space-y-4 w-full">
      <div>
        <h2 className="text-lg font-semibold">Managed ACL Configuration</h2>
        <p className="text-sm text-muted-foreground">NIP-86 relay management</p>
        <div className="mt-2 rounded-md bg-yellow-500/10 border border-yellow-500/30 px-3 py-2 text-sm">
          <span className="font-semibold">Owner only</span> -- this interface is restricted to relay owners.
        </div>
      </div>

      <Tabs defaultValue="pubkeys">
        <TabsList className="flex flex-wrap h-auto gap-1">
          <TabsTrigger value="pubkeys">Pubkeys</TabsTrigger>
          <TabsTrigger value="events">Events</TabsTrigger>
          <TabsTrigger value="ips">IPs</TabsTrigger>
          <TabsTrigger value="kinds">Kinds</TabsTrigger>
          <TabsTrigger value="moderation">Moderation</TabsTrigger>
          <TabsTrigger value="relay">Relay Config</TabsTrigger>
        </TabsList>

        <TabsContent value="pubkeys" className="space-y-6">
          <BannedPubkeysSection />
          <AllowedPubkeysSection />
        </TabsContent>

        <TabsContent value="events" className="space-y-6">
          <BannedEventsSection />
          <AllowedEventsSection />
        </TabsContent>

        <TabsContent value="ips">
          <BlockedIPsSection />
        </TabsContent>

        <TabsContent value="kinds">
          <AllowedKindsSection />
        </TabsContent>

        <TabsContent value="moderation">
          <ModerationSection />
        </TabsContent>

        <TabsContent value="relay">
          <RelayConfigSection />
        </TabsContent>
      </Tabs>
    </div>
  )
}
