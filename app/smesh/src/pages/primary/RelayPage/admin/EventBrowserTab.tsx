import { useCallback, useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { toast } from 'sonner'
import { useNostr } from '@/providers/NostrProvider'
import { getKindName } from '@/lib/event-kinds'
import { SimplePool, type Event, type Filter } from 'nostr-tools'
import { cn } from '@/lib/utils'

interface FilterState {
  kinds: string
  authors: string
  ids: string
  since: string
  until: string
  limit: string
}

const EMPTY_FILTER: FilterState = {
  kinds: '',
  authors: '',
  ids: '',
  since: '',
  until: '',
  limit: '50',
}

function getRelayWsUrl(): string {
  const loc = window.location
  const proto = loc.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${loc.host}`
}

function truncate(s: string, n: number): string {
  if (!s) return ''
  return s.length > n ? s.slice(0, n) + '...' : s
}

function truncatePubkey(pk: string): string {
  if (!pk) return ''
  return pk.slice(0, 8) + '...' + pk.slice(-8)
}

function buildFilter(state: FilterState): Filter {
  const filter: Filter = {}
  if (state.kinds.trim()) {
    filter.kinds = state.kinds
      .split(',')
      .map((s) => parseInt(s.trim(), 10))
      .filter((n) => !isNaN(n))
  }
  if (state.authors.trim()) {
    filter.authors = state.authors
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)
  }
  if (state.ids.trim()) {
    filter.ids = state.ids
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean)
  }
  if (state.since) {
    filter.since = Math.floor(new Date(state.since).getTime() / 1000)
  }
  if (state.until) {
    filter.until = Math.floor(new Date(state.until).getTime() / 1000)
  }
  const limit = parseInt(state.limit, 10)
  filter.limit = !isNaN(limit) && limit > 0 ? limit : 50
  return filter
}

export default function EventBrowserTab() {
  const { pubkey, publish } = useNostr()
  const [filterState, setFilterState] = useState<FilterState>(EMPTY_FILTER)
  const [jsonMode, setJsonMode] = useState(false)
  const [jsonText, setJsonText] = useState('')
  const [jsonError, setJsonError] = useState('')
  const [events, setEvents] = useState<Event[]>([])
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set())
  const [isLoading, setIsLoading] = useState(false)
  const poolRef = useRef<SimplePool | null>(null)

  useEffect(() => {
    poolRef.current = new SimplePool()
    return () => {
      poolRef.current?.close(poolRef.current ? [getRelayWsUrl()] : [])
    }
  }, [])

  const updateField = (field: keyof FilterState, value: string) => {
    setFilterState((prev) => ({ ...prev, [field]: value }))
  }

  const toggleJsonMode = () => {
    if (!jsonMode) {
      const filter = buildFilter(filterState)
      setJsonText(JSON.stringify(filter, null, 2))
      setJsonError('')
    }
    setJsonMode(!jsonMode)
  }

  const queryEvents = useCallback(async () => {
    setIsLoading(true)
    setEvents([])
    try {
      let filter: Filter
      if (jsonMode) {
        try {
          filter = JSON.parse(jsonText)
          setJsonError('')
        } catch (e) {
          setJsonError(e instanceof Error ? e.message : 'Invalid JSON')
          setIsLoading(false)
          return
        }
      } else {
        filter = buildFilter(filterState)
      }

      if (!filter.limit) filter.limit = 50

      const pool = poolRef.current
      if (!pool) {
        toast.error('Pool not initialized')
        setIsLoading(false)
        return
      }

      const relayUrl = getRelayWsUrl()
      const collected: Event[] = []

      await new Promise<void>((resolve) => {
        const sub = pool.subscribeMany([relayUrl], filter, {
          onevent(evt: Event) {
            collected.push(evt)
          },
          oneose() {
            sub.close()
            resolve()
          },
        })
        // Safety timeout
        setTimeout(() => {
          sub.close()
          resolve()
        }, 15000)
      })

      collected.sort((a, b) => b.created_at - a.created_at)
      setEvents(collected)
      toast.success(`Found ${collected.length} events`)
    } catch (e) {
      toast.error(`Query failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setIsLoading(false)
    }
  }, [filterState, jsonMode, jsonText])

  const toggleExpand = (id: string) => {
    setExpandedIds((prev) => {
      const next = new Set(prev)
      if (next.has(id)) {
        next.delete(id)
      } else {
        next.add(id)
      }
      return next
    })
  }

  const copyEvent = (event: Event) => {
    navigator.clipboard.writeText(JSON.stringify(event))
    toast.success('Copied to clipboard')
  }

  const deleteEvent = async (event: Event) => {
    if (!pubkey) {
      toast.error('Login required to delete events')
      return
    }
    if (!confirm(`Delete event ${event.id.slice(0, 16)}...?`)) return

    try {
      const tags: string[][] = [['k', String(event.kind)]]
      tags.push(['e', event.id])

      const draft = {
        kind: 5,
        content: 'Request for deletion of the event.',
        tags,
        created_at: Math.floor(Date.now() / 1000),
      }
      await publish(draft)
      toast.success('Deletion request published')
      setEvents((prev) => prev.filter((e) => e.id !== event.id))
    } catch (e) {
      toast.error(`Delete failed: ${e instanceof Error ? e.message : String(e)}`)
    }
  }

  const clearFilter = () => {
    setFilterState(EMPTY_FILTER)
    setJsonText('')
    setJsonError('')
  }

  return (
    <div className="p-4 space-y-4 w-full">
      <div className="flex items-center justify-between">
        <h3 className="text-lg font-semibold">Event Browser</h3>
        <div className="flex items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            onClick={toggleJsonMode}
            className={cn(jsonMode && 'bg-accent')}
          >
            {'</>'}
          </Button>
          <Button variant="outline" size="sm" onClick={clearFilter}>
            Clear
          </Button>
          <Button size="sm" onClick={queryEvents} disabled={isLoading}>
            {isLoading ? 'Querying...' : 'Query'}
          </Button>
        </div>
      </div>

      {jsonMode ? (
        <div className="space-y-2">
          <textarea
            value={jsonText}
            onChange={(e) => setJsonText(e.target.value)}
            className="w-full rounded-md border bg-card p-3 font-mono text-sm min-h-[160px] resize-y"
            placeholder='{"kinds": [1], "limit": 50}'
          />
          {jsonError && (
            <div className="text-sm text-destructive">{jsonError}</div>
          )}
        </div>
      ) : (
        <div className="rounded-lg bg-card p-4 space-y-3">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div className="space-y-1">
              <label className="text-sm font-medium text-muted-foreground">Kinds</label>
              <input
                type="text"
                value={filterState.kinds}
                onChange={(e) => updateField('kinds', e.target.value)}
                placeholder="1, 0, 3"
                className="w-full rounded-md border bg-background px-3 py-2 text-sm"
              />
            </div>
            <div className="space-y-1">
              <label className="text-sm font-medium text-muted-foreground">Limit</label>
              <input
                type="number"
                value={filterState.limit}
                onChange={(e) => updateField('limit', e.target.value)}
                placeholder="50"
                min="1"
                className="w-full rounded-md border bg-background px-3 py-2 text-sm"
              />
            </div>
          </div>
          <div className="space-y-1">
            <label className="text-sm font-medium text-muted-foreground">Authors (hex pubkeys, comma-separated)</label>
            <input
              type="text"
              value={filterState.authors}
              onChange={(e) => updateField('authors', e.target.value)}
              placeholder="abc123..., def456..."
              className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
            />
          </div>
          <div className="space-y-1">
            <label className="text-sm font-medium text-muted-foreground">Event IDs (hex, comma-separated)</label>
            <input
              type="text"
              value={filterState.ids}
              onChange={(e) => updateField('ids', e.target.value)}
              placeholder="abc123..., def456..."
              className="w-full rounded-md border bg-background px-3 py-2 text-sm font-mono"
            />
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div className="space-y-1">
              <label className="text-sm font-medium text-muted-foreground">Since</label>
              <input
                type="datetime-local"
                value={filterState.since}
                onChange={(e) => updateField('since', e.target.value)}
                className="w-full rounded-md border bg-background px-3 py-2 text-sm"
              />
            </div>
            <div className="space-y-1">
              <label className="text-sm font-medium text-muted-foreground">Until</label>
              <input
                type="datetime-local"
                value={filterState.until}
                onChange={(e) => updateField('until', e.target.value)}
                className="w-full rounded-md border bg-background px-3 py-2 text-sm"
              />
            </div>
          </div>
        </div>
      )}

      <div className="text-xs text-muted-foreground">
        {events.length} events loaded from {getRelayWsUrl()}
      </div>

      <div className="space-y-1">
        {events.length === 0 && !isLoading && (
          <div className="text-center py-8 text-muted-foreground">
            No events. Enter a filter and press Query.
          </div>
        )}

        {events.map((event) => (
          <div key={event.id} className="rounded-md bg-card border">
            <div
              className="flex items-center gap-3 px-3 py-2 cursor-pointer hover:bg-accent/20"
              onClick={() => toggleExpand(event.id)}
            >
              <div className="shrink-0 min-w-[100px]">
                <span className="font-mono text-xs text-muted-foreground">
                  {truncatePubkey(event.pubkey)}
                </span>
              </div>
              <div className="flex items-center gap-1.5 shrink-0">
                <span
                  className={cn(
                    'rounded px-1.5 py-0.5 font-mono text-xs font-semibold',
                    event.kind === 5
                      ? 'bg-destructive text-destructive-foreground'
                      : 'bg-secondary text-secondary-foreground'
                  )}
                >
                  {event.kind}
                </span>
                <span className="text-xs text-muted-foreground">
                  {getKindName(event.kind)}
                </span>
              </div>
              <div className="flex-1 min-w-0">
                <span className="text-xs text-muted-foreground truncate block">
                  {truncate(event.content, 80)}
                </span>
              </div>
              <span className="text-xs text-muted-foreground shrink-0">
                {new Date(event.created_at * 1000).toLocaleString()}
              </span>
              {pubkey && event.kind !== 5 && (
                <Button
                  variant="ghost-destructive"
                  size="sm"
                  onClick={(e) => {
                    e.stopPropagation()
                    deleteEvent(event)
                  }}
                  className="shrink-0 h-7 px-2 text-xs"
                >
                  Delete
                </Button>
              )}
            </div>
            {expandedIds.has(event.id) && (
              <div className="border-t px-3 py-2 relative">
                <pre className="text-xs font-mono overflow-x-auto whitespace-pre-wrap break-all bg-background p-3 rounded">
                  {JSON.stringify(event, null, 2)}
                </pre>
                <Button
                  variant="outline"
                  size="sm"
                  className="absolute top-4 right-5"
                  onClick={(e) => {
                    e.stopPropagation()
                    copyEvent(event)
                  }}
                >
                  Copy
                </Button>
              </div>
            )}
          </div>
        ))}

        {isLoading && (
          <div className="text-center py-8 text-muted-foreground">Loading events...</div>
        )}
      </div>
    </div>
  )
}
