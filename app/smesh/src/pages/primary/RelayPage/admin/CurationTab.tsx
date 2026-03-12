import { useCallback, useEffect, useState } from 'react'
import relayAdmin from '@/services/relay-admin.service'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { toast } from 'sonner'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface TrustedEntry {
  pubkey: string
  note?: string
}

interface BlacklistedEntry {
  pubkey: string
  reason?: string
}

interface UnclassifiedEntry {
  pubkey: string
  event_count: number
}

interface SpamEntry {
  event_id: string
  pubkey: string
  reason?: string
}

interface BlockedIP {
  ip: string
  reason?: string
  expires_at?: string
}

interface UserEvent {
  id: string
  kind: number
  content: string
  created_at: number
}

type Tab = 'trusted' | 'blacklist' | 'unclassified' | 'spam' | 'ips' | 'settings'
type UserCategory = 'trusted' | 'blacklisted' | 'unclassified'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatPubkey(pk: string): string {
  if (!pk || pk.length < 16) return pk
  return `${pk.slice(0, 8)}...${pk.slice(-8)}`
}

function formatDate(ts: number | string | undefined): string {
  if (!ts) return ''
  const n = typeof ts === 'string' ? Date.parse(ts) : ts
  return new Date(n).toLocaleString()
}

const KIND_NAMES: Record<number, string> = {
  0: 'Metadata',
  1: 'Text Note',
  3: 'Follow List',
  4: 'Encrypted DM',
  6: 'Repost',
  7: 'Reaction',
  14: 'Chat Message',
  1063: 'File Metadata',
  10002: 'Relay List',
  30023: 'Long-form',
  30078: 'App Data',
}

function kindName(k: number): string {
  return KIND_NAMES[k] ?? `Kind ${k}`
}

function truncateContent(c: string, maxLines = 6): string {
  if (!c) return ''
  const lines = c.split('\n')
  if (lines.length <= maxLines && c.length <= maxLines * 100) return c
  let t = lines.slice(0, maxLines).join('\n')
  if (t.length > maxLines * 100) t = t.substring(0, maxLines * 100)
  return t
}

function isContentTruncated(c: string, maxLines = 6): boolean {
  if (!c) return false
  const lines = c.split('\n')
  return lines.length > maxLines || c.length > maxLines * 100
}

// Wrapper that extracts .result from NIP-86 JSON-RPC response
async function nip86(method: string, params: unknown[] = []): Promise<unknown> {
  const res = await relayAdmin.nip86Request(method, params)
  if (res.error) throw new Error(String(res.error))
  return res.result
}

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function TabButton({
  active,
  onClick,
  children,
}: {
  active: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button
      onClick={onClick}
      className={cn(
        'px-3 py-2 text-sm border-b-2 transition-colors',
        active
          ? 'border-primary text-primary'
          : 'border-transparent text-muted-foreground hover:text-foreground hover:bg-accent/20'
      )}
    >
      {children}
    </button>
  )
}

// ---------------------------------------------------------------------------
// User Detail Panel
// ---------------------------------------------------------------------------

function UserDetail({
  pubkey,
  category,
  onClose,
  onChanged,
}: {
  pubkey: string
  category: UserCategory
  onClose: () => void
  onChanged: () => void
}) {
  const [events, setEvents] = useState<UserEvent[]>([])
  const [total, setTotal] = useState(0)
  const [loadingEvents, setLoadingEvents] = useState(false)
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [busy, setBusy] = useState(false)

  const loadEvents = useCallback(
    async (offset: number) => {
      if (loadingEvents) return
      setLoadingEvents(true)
      try {
        const res = (await nip86('geteventsforpubkey', [pubkey, 100, offset])) as {
          events?: UserEvent[]
          total?: number
        } | null
        if (res) {
          if (offset === 0) {
            setEvents(res.events ?? [])
          } else {
            setEvents((prev) => [...prev, ...(res.events ?? [])])
          }
          setTotal(res.total ?? 0)
        }
      } catch (e) {
        toast.error(`Failed to load events: ${e instanceof Error ? e.message : String(e)}`)
      } finally {
        setLoadingEvents(false)
      }
    },
    [pubkey, loadingEvents]
  )

  useEffect(() => {
    loadEvents(0)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pubkey])

  const act = async (method: string, params: unknown[], msg: string) => {
    setBusy(true)
    try {
      await nip86(method, params)
      toast.success(msg)
      onChanged()
      onClose()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  const deleteAll = async () => {
    if (!confirm(`Delete ALL ${total} events from this user? This cannot be undone.`)) return
    setBusy(true)
    try {
      const res = (await nip86('deleteeventsforpubkey', [pubkey])) as { deleted?: number } | null
      toast.success(`Deleted ${res?.deleted ?? 0} events`)
      setEvents([])
      setTotal(0)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="rounded-lg border bg-card p-4 space-y-4">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-2 border-b pb-3">
        <div className="flex items-center gap-3 flex-wrap">
          <Button variant="outline" size="sm" onClick={onClose}>
            &larr; Back
          </Button>
          <span className="font-semibold">User Events</span>
          <code className="rounded bg-muted px-2 py-0.5 text-xs" title={pubkey}>
            {formatPubkey(pubkey)}
          </code>
          <span className="text-xs text-green-500 font-medium">{total} events</span>
        </div>

        <div className="flex gap-2 flex-wrap">
          {category === 'trusted' && (
            <>
              <Button
                variant="destructive"
                size="sm"
                disabled={busy}
                onClick={() => act('untrustpubkey', [pubkey], 'Trust removed')}
              >
                Remove Trust
              </Button>
              <Button
                variant="destructive"
                size="sm"
                disabled={busy}
                onClick={() => act('blacklistpubkey', [pubkey, ''], 'Blacklisted')}
              >
                Blacklist
              </Button>
            </>
          )}
          {category === 'blacklisted' && (
            <>
              <Button
                variant="destructive"
                size="sm"
                disabled={busy || total === 0}
                onClick={deleteAll}
                className="bg-red-900 hover:bg-red-950"
              >
                Delete All Events
              </Button>
              <Button
                size="sm"
                disabled={busy}
                onClick={() => act('unblacklistpubkey', [pubkey], 'Removed from blacklist')}
              >
                Remove from Blacklist
              </Button>
              <Button
                size="sm"
                disabled={busy}
                onClick={() => act('trustpubkey', [pubkey, ''], 'Trusted')}
              >
                Trust
              </Button>
            </>
          )}
          {category === 'unclassified' && (
            <>
              <Button
                size="sm"
                disabled={busy}
                onClick={() => act('trustpubkey', [pubkey, ''], 'Trusted')}
              >
                Trust
              </Button>
              <Button
                variant="destructive"
                size="sm"
                disabled={busy}
                onClick={() => act('blacklistpubkey', [pubkey, ''], 'Blacklisted')}
              >
                Blacklist
              </Button>
            </>
          )}
        </div>
      </div>

      {/* Events */}
      <div className="max-h-[600px] overflow-y-auto space-y-2">
        {loadingEvents && events.length === 0 ? (
          <p className="text-center py-8 text-muted-foreground">Loading events...</p>
        ) : events.length === 0 ? (
          <p className="text-center py-8 text-muted-foreground italic">No events found.</p>
        ) : (
          events.map((ev) => {
            const isExp = !!expanded[ev.id]
            const isTrunc = isContentTruncated(ev.content)
            return (
              <div key={ev.id} className="rounded-md border bg-background p-3 space-y-1">
                <div className="flex flex-wrap items-center gap-2 text-xs">
                  <span className="rounded bg-primary/80 text-primary-foreground px-2 py-0.5 font-medium">
                    {kindName(ev.kind)}
                  </span>
                  <code className="text-muted-foreground" title={ev.id}>
                    {formatPubkey(ev.id)}
                  </code>
                  <span className="text-muted-foreground/60">
                    {formatDate(ev.created_at * 1000)}
                  </span>
                </div>
                <pre
                  className={cn(
                    'whitespace-pre-wrap break-words text-sm bg-card rounded p-2',
                    !isExp && 'max-h-[150px] overflow-hidden'
                  )}
                >
                  {isExp || !isTrunc
                    ? ev.content || '(empty)'
                    : `${truncateContent(ev.content)}...`}
                </pre>
                {isTrunc && (
                  <button
                    className="text-xs text-primary hover:underline"
                    onClick={() =>
                      setExpanded((prev) => ({ ...prev, [ev.id]: !prev[ev.id] }))
                    }
                  >
                    {isExp ? 'Show less' : 'Show more'}
                  </button>
                )}
              </div>
            )
          })
        )}

        {events.length > 0 && events.length < total && (
          <div className="text-center py-3">
            <Button
              variant="outline"
              size="sm"
              disabled={loadingEvents}
              onClick={() => loadEvents(events.length)}
            >
              {loadingEvents ? 'Loading...' : `Load more (${events.length} of ${total})`}
            </Button>
          </div>
        )}
      </div>
    </div>
  )
}

// ---------------------------------------------------------------------------
// Main component
// ---------------------------------------------------------------------------

export default function CurationTab() {
  const [activeTab, setActiveTab] = useState<Tab>('trusted')
  const [loading, setLoading] = useState(false)

  // Data
  const [trusted, setTrusted] = useState<TrustedEntry[]>([])
  const [blacklisted, setBlacklisted] = useState<BlacklistedEntry[]>([])
  const [unclassified, setUnclassified] = useState<UnclassifiedEntry[]>([])
  const [spam, setSpam] = useState<SpamEntry[]>([])
  const [blockedIPs, setBlockedIPs] = useState<BlockedIP[]>([])

  // Add-form inputs
  const [newTrustedPk, setNewTrustedPk] = useState('')
  const [newTrustedNote, setNewTrustedNote] = useState('')
  const [newBlackPk, setNewBlackPk] = useState('')
  const [newBlackReason, setNewBlackReason] = useState('')

  // User detail
  const [selectedUser, setSelectedUser] = useState<string | null>(null)
  const [selectedCategory, setSelectedCategory] = useState<UserCategory>('unclassified')

  // Settings
  const [dailyLimit, setDailyLimit] = useState(50)
  const [firstBanHours, setFirstBanHours] = useState(1)
  const [secondBanHours, setSecondBanHours] = useState(168)

  // -----------------------------------------------------------------------
  // Loaders
  // -----------------------------------------------------------------------

  const loadTrusted = useCallback(async () => {
    try {
      const r = (await nip86('listtrustedpubkeys')) as TrustedEntry[] | null
      setTrusted(r ?? [])
    } catch {
      setTrusted([])
    }
  }, [])

  const loadBlacklisted = useCallback(async () => {
    try {
      const r = (await nip86('listblacklistedpubkeys')) as BlacklistedEntry[] | null
      setBlacklisted(r ?? [])
    } catch {
      setBlacklisted([])
    }
  }, [])

  const loadUnclassified = useCallback(async () => {
    try {
      const r = (await nip86('listunclassifiedusers')) as UnclassifiedEntry[] | null
      setUnclassified(r ?? [])
    } catch {
      setUnclassified([])
    }
  }, [])

  const loadSpam = useCallback(async () => {
    try {
      const r = (await nip86('listspamevents')) as SpamEntry[] | null
      setSpam(r ?? [])
    } catch {
      setSpam([])
    }
  }, [])

  const loadIPs = useCallback(async () => {
    try {
      const r = (await nip86('listblockedips')) as BlockedIP[] | null
      setBlockedIPs(r ?? [])
    } catch {
      setBlockedIPs([])
    }
  }, [])

  const loadConfig = useCallback(async () => {
    try {
      const r = (await nip86('getcuratingconfig')) as Record<string, unknown> | null
      if (r) {
        setDailyLimit((r.daily_limit as number) ?? 50)
        setFirstBanHours((r.first_ban_hours as number) ?? 1)
        setSecondBanHours((r.second_ban_hours as number) ?? 168)
      }
    } catch {
      /* keep defaults */
    }
  }, [])

  const loadAll = useCallback(async () => {
    setLoading(true)
    await Promise.all([loadTrusted(), loadBlacklisted(), loadUnclassified(), loadSpam(), loadIPs(), loadConfig()])
    setLoading(false)
  }, [loadTrusted, loadBlacklisted, loadUnclassified, loadSpam, loadIPs, loadConfig])

  useEffect(() => {
    loadAll()
  }, [loadAll])

  // -----------------------------------------------------------------------
  // Actions
  // -----------------------------------------------------------------------

  const trustPubkey = async (pk: string, note: string) => {
    if (!pk) return
    try {
      await nip86('trustpubkey', [pk, note])
      toast.success('Pubkey trusted')
      setNewTrustedPk('')
      setNewTrustedNote('')
      await Promise.all([loadTrusted(), loadUnclassified()])
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const untrustPubkey = async (pk: string) => {
    try {
      await nip86('untrustpubkey', [pk])
      toast.success('Trust removed')
      await loadTrusted()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const blacklistPubkey = async (pk: string, reason: string) => {
    if (!pk) return
    try {
      await nip86('blacklistpubkey', [pk, reason])
      toast.success('Pubkey blacklisted')
      setNewBlackPk('')
      setNewBlackReason('')
      await Promise.all([loadBlacklisted(), loadUnclassified()])
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const unblacklistPubkey = async (pk: string) => {
    try {
      await nip86('unblacklistpubkey', [pk])
      toast.success('Removed from blacklist')
      await loadBlacklisted()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const unmarkSpam = async (eventId: string) => {
    try {
      await nip86('unmarkspam', [eventId])
      toast.success('Spam mark removed')
      await loadSpam()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const deleteEvent = async (eventId: string) => {
    if (!confirm('Permanently delete this event?')) return
    try {
      await nip86('deleteevent', [eventId])
      toast.success('Event deleted')
      await loadSpam()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const unblockIP = async (ip: string) => {
    try {
      await nip86('unblockip', [ip])
      toast.success('IP unblocked')
      await loadIPs()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const scanDatabase = async () => {
    try {
      const res = (await nip86('scanpubkeys')) as {
        total_pubkeys?: number
        total_events?: number
        skipped?: number
      } | null
      toast.success(
        `Scanned: ${res?.total_pubkeys ?? 0} pubkeys, ${res?.total_events ?? 0} events (${res?.skipped ?? 0} skipped)`
      )
      await loadUnclassified()
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    }
  }

  const saveSettings = async () => {
    setLoading(true)
    try {
      await nip86('updatecuratingconfig', [
        { daily_limit: dailyLimit, first_ban_hours: firstBanHours, second_ban_hours: secondBanHours },
      ])
      toast.success('Settings updated')
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }

  // -----------------------------------------------------------------------
  // Detail view
  // -----------------------------------------------------------------------

  if (selectedUser) {
    return (
      <div className="p-4 w-full">
        <UserDetail
          pubkey={selectedUser}
          category={selectedCategory}
          onClose={() => setSelectedUser(null)}
          onChanged={loadAll}
        />
      </div>
    )
  }

  // -----------------------------------------------------------------------
  // Tabs
  // -----------------------------------------------------------------------

  return (
    <div className="p-4 space-y-4 w-full">
      <h3 className="text-lg font-semibold">Curation Mode</h3>

      {/* Tab bar */}
      <div className="flex flex-wrap border-b">
        <TabButton active={activeTab === 'trusted'} onClick={() => setActiveTab('trusted')}>
          Trusted ({trusted.length})
        </TabButton>
        <TabButton active={activeTab === 'blacklist'} onClick={() => setActiveTab('blacklist')}>
          Blacklist ({blacklisted.length})
        </TabButton>
        <TabButton active={activeTab === 'unclassified'} onClick={() => setActiveTab('unclassified')}>
          Unclassified ({unclassified.length})
        </TabButton>
        <TabButton active={activeTab === 'spam'} onClick={() => setActiveTab('spam')}>
          Spam ({spam.length})
        </TabButton>
        <TabButton active={activeTab === 'ips'} onClick={() => setActiveTab('ips')}>
          Blocked IPs ({blockedIPs.length})
        </TabButton>
        <TabButton active={activeTab === 'settings'} onClick={() => setActiveTab('settings')}>
          Settings
        </TabButton>
      </div>

      {/* ===== Trusted ===== */}
      {activeTab === 'trusted' && (
        <div className="rounded-lg border bg-card p-4 space-y-3">
          <div>
            <h4 className="font-medium">Trusted Publishers</h4>
            <p className="text-xs text-muted-foreground">
              Trusted users can publish unlimited events without rate limiting.
            </p>
          </div>

          <div className="flex flex-wrap gap-2">
            <input
              className="flex-1 min-w-[200px] rounded-md border bg-background px-3 py-1.5 text-sm"
              placeholder="Pubkey (64 hex chars)"
              value={newTrustedPk}
              onChange={(e) => setNewTrustedPk(e.target.value)}
            />
            <input
              className="flex-1 min-w-[120px] rounded-md border bg-background px-3 py-1.5 text-sm"
              placeholder="Note (optional)"
              value={newTrustedNote}
              onChange={(e) => setNewTrustedNote(e.target.value)}
            />
            <Button size="sm" disabled={loading || !newTrustedPk} onClick={() => trustPubkey(newTrustedPk, newTrustedNote)}>
              Trust
            </Button>
          </div>

          <div className="rounded-md border max-h-[400px] overflow-y-auto divide-y">
            {trusted.length === 0 ? (
              <p className="text-center py-6 text-muted-foreground italic">No trusted pubkeys yet.</p>
            ) : (
              trusted.map((item) => (
                <div
                  key={item.pubkey}
                  className="flex items-center justify-between gap-2 px-3 py-2 cursor-pointer hover:bg-accent/20 transition-colors"
                  onClick={() => {
                    setSelectedUser(item.pubkey)
                    setSelectedCategory('trusted')
                  }}
                >
                  <div className="min-w-0">
                    <code className="text-sm" title={item.pubkey}>
                      {formatPubkey(item.pubkey)}
                    </code>
                    {item.note && (
                      <p className="text-xs text-muted-foreground truncate">{item.note}</p>
                    )}
                  </div>
                  <Button
                    variant="destructive"
                    size="sm"
                    onClick={(e) => {
                      e.stopPropagation()
                      untrustPubkey(item.pubkey)
                    }}
                  >
                    Remove
                  </Button>
                </div>
              ))
            )}
          </div>
        </div>
      )}

      {/* ===== Blacklist ===== */}
      {activeTab === 'blacklist' && (
        <div className="rounded-lg border bg-card p-4 space-y-3">
          <div>
            <h4 className="font-medium">Blacklisted Publishers</h4>
            <p className="text-xs text-muted-foreground">
              Blacklisted users cannot publish any events.
            </p>
          </div>

          <div className="flex flex-wrap gap-2">
            <input
              className="flex-1 min-w-[200px] rounded-md border bg-background px-3 py-1.5 text-sm"
              placeholder="Pubkey (64 hex chars)"
              value={newBlackPk}
              onChange={(e) => setNewBlackPk(e.target.value)}
            />
            <input
              className="flex-1 min-w-[120px] rounded-md border bg-background px-3 py-1.5 text-sm"
              placeholder="Reason (optional)"
              value={newBlackReason}
              onChange={(e) => setNewBlackReason(e.target.value)}
            />
            <Button
              variant="destructive"
              size="sm"
              disabled={loading || !newBlackPk}
              onClick={() => blacklistPubkey(newBlackPk, newBlackReason)}
            >
              Blacklist
            </Button>
          </div>

          <div className="rounded-md border max-h-[400px] overflow-y-auto divide-y">
            {blacklisted.length === 0 ? (
              <p className="text-center py-6 text-muted-foreground italic">No blacklisted pubkeys.</p>
            ) : (
              blacklisted.map((item) => (
                <div
                  key={item.pubkey}
                  className="flex items-center justify-between gap-2 px-3 py-2 cursor-pointer hover:bg-accent/20 transition-colors"
                  onClick={() => {
                    setSelectedUser(item.pubkey)
                    setSelectedCategory('blacklisted')
                  }}
                >
                  <div className="min-w-0">
                    <code className="text-sm" title={item.pubkey}>
                      {formatPubkey(item.pubkey)}
                    </code>
                    {item.reason && (
                      <p className="text-xs text-muted-foreground truncate">{item.reason}</p>
                    )}
                  </div>
                  <Button
                    size="sm"
                    onClick={(e) => {
                      e.stopPropagation()
                      unblacklistPubkey(item.pubkey)
                    }}
                  >
                    Remove
                  </Button>
                </div>
              ))
            )}
          </div>
        </div>
      )}

      {/* ===== Unclassified ===== */}
      {activeTab === 'unclassified' && (
        <div className="rounded-lg border bg-card p-4 space-y-3">
          <div>
            <h4 className="font-medium">Unclassified Users</h4>
            <p className="text-xs text-muted-foreground">
              Users who have posted events but haven't been classified. Sorted by event count.
            </p>
          </div>

          <div className="flex gap-2">
            <Button variant="outline" size="sm" disabled={loading} onClick={loadUnclassified}>
              Refresh
            </Button>
            <Button variant="secondary" size="sm" disabled={loading} onClick={scanDatabase}>
              Scan Database
            </Button>
          </div>

          <div className="rounded-md border max-h-[400px] overflow-y-auto divide-y">
            {unclassified.length === 0 ? (
              <p className="text-center py-6 text-muted-foreground italic">No unclassified users.</p>
            ) : (
              unclassified.map((user) => (
                <div
                  key={user.pubkey}
                  className="flex items-center justify-between gap-2 px-3 py-2 cursor-pointer hover:bg-accent/20 transition-colors"
                  onClick={() => {
                    setSelectedUser(user.pubkey)
                    setSelectedCategory('unclassified')
                  }}
                >
                  <div className="min-w-0">
                    <code className="text-sm" title={user.pubkey}>
                      {formatPubkey(user.pubkey)}
                    </code>
                    <span className="ml-2 text-xs text-green-500 font-medium">
                      {user.event_count} events
                    </span>
                  </div>
                  <div className="flex gap-1.5 shrink-0">
                    <Button
                      size="sm"
                      onClick={(e) => {
                        e.stopPropagation()
                        trustPubkey(user.pubkey, '')
                      }}
                    >
                      Trust
                    </Button>
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={(e) => {
                        e.stopPropagation()
                        blacklistPubkey(user.pubkey, '')
                      }}
                    >
                      Blacklist
                    </Button>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      )}

      {/* ===== Spam ===== */}
      {activeTab === 'spam' && (
        <div className="rounded-lg border bg-card p-4 space-y-3">
          <div>
            <h4 className="font-medium">Spam Events</h4>
            <p className="text-xs text-muted-foreground">
              Events flagged as spam are hidden from query results but remain in the database.
            </p>
          </div>

          <Button variant="outline" size="sm" disabled={loading} onClick={loadSpam}>
            Refresh
          </Button>

          <div className="rounded-md border max-h-[400px] overflow-y-auto divide-y">
            {spam.length === 0 ? (
              <p className="text-center py-6 text-muted-foreground italic">No spam events flagged.</p>
            ) : (
              spam.map((ev) => (
                <div key={ev.event_id} className="flex items-center justify-between gap-2 px-3 py-2">
                  <div className="min-w-0">
                    <code className="text-sm" title={ev.event_id}>
                      {formatPubkey(ev.event_id)}
                    </code>
                    <span className="ml-2 text-xs text-muted-foreground" title={ev.pubkey}>
                      by {formatPubkey(ev.pubkey)}
                    </span>
                    {ev.reason && (
                      <p className="text-xs text-muted-foreground truncate">{ev.reason}</p>
                    )}
                  </div>
                  <div className="flex gap-1.5 shrink-0">
                    <Button size="sm" onClick={() => unmarkSpam(ev.event_id)}>
                      Unmark
                    </Button>
                    <Button variant="destructive" size="sm" onClick={() => deleteEvent(ev.event_id)}>
                      Delete
                    </Button>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      )}

      {/* ===== Blocked IPs ===== */}
      {activeTab === 'ips' && (
        <div className="rounded-lg border bg-card p-4 space-y-3">
          <div>
            <h4 className="font-medium">Blocked IP Addresses</h4>
            <p className="text-xs text-muted-foreground">
              IP addresses blocked due to rate limit violations.
            </p>
          </div>

          <Button variant="outline" size="sm" disabled={loading} onClick={loadIPs}>
            Refresh
          </Button>

          <div className="rounded-md border max-h-[400px] overflow-y-auto divide-y">
            {blockedIPs.length === 0 ? (
              <p className="text-center py-6 text-muted-foreground italic">No blocked IPs.</p>
            ) : (
              blockedIPs.map((ip) => (
                <div key={ip.ip} className="flex items-center justify-between gap-2 px-3 py-2">
                  <div className="min-w-0">
                    <code className="text-sm">{ip.ip}</code>
                    {ip.reason && (
                      <span className="ml-2 text-xs text-muted-foreground">{ip.reason}</span>
                    )}
                    {ip.expires_at && (
                      <span className="ml-2 text-xs text-muted-foreground/60">
                        Expires: {formatDate(ip.expires_at)}
                      </span>
                    )}
                  </div>
                  <Button size="sm" onClick={() => unblockIP(ip.ip)}>
                    Unblock
                  </Button>
                </div>
              ))
            )}
          </div>
        </div>
      )}

      {/* ===== Settings ===== */}
      {activeTab === 'settings' && (
        <div className="rounded-lg border bg-card p-4 space-y-4">
          <div>
            <h4 className="font-medium">Rate Limiting</h4>
            <p className="text-xs text-muted-foreground">
              Configure rate limits for unclassified users and IP ban durations.
            </p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <label className="space-y-1">
              <span className="text-sm font-medium">Daily Event Limit</span>
              <input
                type="number"
                min={1}
                className="w-full rounded-md border bg-background px-3 py-1.5 text-sm"
                value={dailyLimit}
                onChange={(e) => setDailyLimit(Number(e.target.value))}
              />
            </label>
            <label className="space-y-1">
              <span className="text-sm font-medium">First Ban (hours)</span>
              <input
                type="number"
                min={1}
                className="w-full rounded-md border bg-background px-3 py-1.5 text-sm"
                value={firstBanHours}
                onChange={(e) => setFirstBanHours(Number(e.target.value))}
              />
            </label>
            <label className="space-y-1">
              <span className="text-sm font-medium">Second+ Ban (hours)</span>
              <input
                type="number"
                min={1}
                className="w-full rounded-md border bg-background px-3 py-1.5 text-sm"
                value={secondBanHours}
                onChange={(e) => setSecondBanHours(Number(e.target.value))}
              />
            </label>
          </div>

          <div>
            <Button size="sm" disabled={loading} onClick={saveSettings}>
              {loading ? 'Saving...' : 'Save Settings'}
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
