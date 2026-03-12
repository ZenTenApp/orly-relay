import { useCallback, useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { toast } from 'sonner'
import { useNostr } from '@/providers/NostrProvider'
import { eventKinds, getKindName, type EventKind } from '@/lib/event-kinds'
import { SimplePool, type Event, type Filter } from 'nostr-tools'

function getRelayWsUrl(): string {
  const loc = window.location
  const proto = loc.protocol === 'https:' ? 'wss:' : 'ws:'
  return `${proto}//${loc.host}`
}

function getReplaceableKinds(): EventKind[] {
  return eventKinds.filter(
    (ek) => ek.isReplaceable || ek.isAddressable || ek.kind === 0 || ek.kind === 3
  )
}

export default function RecoveryTab() {
  const { pubkey, publish } = useNostr()
  const [selectedKind, setSelectedKind] = useState<number | ''>('')
  const [customKind, setCustomKind] = useState('')
  const [events, setEvents] = useState<Event[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [isReposting, setIsReposting] = useState<string | null>(null)
  const poolRef = useRef<SimplePool | null>(null)

  const replaceableKinds = getReplaceableKinds()

  useEffect(() => {
    poolRef.current = new SimplePool()
    return () => {
      poolRef.current?.close(poolRef.current ? [getRelayWsUrl()] : [])
    }
  }, [])

  const activeKind = selectedKind !== '' ? selectedKind : customKind ? parseInt(customKind, 10) : null

  const loadEvents = useCallback(async () => {
    if (activeKind === null || isNaN(activeKind)) return
    setIsLoading(true)
    setEvents([])

    try {
      const pool = poolRef.current
      if (!pool) {
        toast.error('Pool not initialized')
        setIsLoading(false)
        return
      }

      const relayUrl = getRelayWsUrl()
      const filter: Filter = {
        kinds: [activeKind],
        limit: 200,
      }

      if (pubkey) {
        filter.authors = [pubkey]
      }

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
        setTimeout(() => {
          sub.close()
          resolve()
        }, 15000)
      })

      collected.sort((a, b) => b.created_at - a.created_at)
      setEvents(collected)

      if (collected.length === 0) {
        toast.info('No events found for this kind')
      } else {
        toast.success(`Found ${collected.length} versions`)
      }
    } catch (e) {
      toast.error(`Query failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setIsLoading(false)
    }
  }, [activeKind, pubkey])

  // Auto-load when kind changes
  useEffect(() => {
    if (activeKind !== null && !isNaN(activeKind)) {
      loadEvents()
    }
  }, [activeKind]) // eslint-disable-line react-hooks/exhaustive-deps

  const isCurrentVersion = (_event: Event, index: number): boolean => {
    return index === 0
  }

  const repostEvent = async (event: Event) => {
    if (!pubkey) {
      toast.error('Login required')
      return
    }
    if (!confirm('Republish this old version? It will become the current version.')) return

    setIsReposting(event.id)
    try {
      const draft = {
        kind: event.kind,
        content: event.content,
        tags: event.tags,
        created_at: Math.floor(Date.now() / 1000),
      }
      await publish(draft)
      toast.success('Event republished successfully')
      // Reload to see updated order
      await loadEvents()
    } catch (e) {
      toast.error(`Repost failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setIsReposting(null)
    }
  }

  const copyEvent = (event: Event) => {
    navigator.clipboard.writeText(JSON.stringify(event))
    toast.success('Copied to clipboard')
  }

  return (
    <div className="p-4 space-y-4 w-full max-w-4xl">
      <div>
        <h3 className="text-lg font-semibold">Event Recovery</h3>
        <p className="text-sm text-muted-foreground mt-1">
          Search and recover old versions of replaceable events. A wise pelican once said:
          every version tells a story, even the ones you regret.
        </p>
      </div>

      <div className="rounded-lg bg-card p-4 space-y-3">
        <div className="space-y-1">
          <label className="text-sm font-medium text-muted-foreground">
            Select Replaceable Kind
          </label>
          <select
            value={selectedKind}
            onChange={(e) => {
              const val = e.target.value
              setSelectedKind(val === '' ? '' : parseInt(val, 10))
              if (val !== '') setCustomKind('')
            }}
            className="w-full rounded-md border bg-background px-3 py-2 text-sm"
          >
            <option value="">Choose a replaceable kind...</option>
            {replaceableKinds.map((ek) => (
              <option key={ek.kind} value={ek.kind}>
                {ek.name} ({ek.kind})
              </option>
            ))}
          </select>
        </div>

        <div className="space-y-1">
          <label className="text-sm font-medium text-muted-foreground">
            Or enter custom kind number
          </label>
          <input
            type="number"
            value={customKind}
            onChange={(e) => {
              setCustomKind(e.target.value)
              if (e.target.value) setSelectedKind('')
            }}
            placeholder="e.g., 10001"
            min="0"
            className="w-full rounded-md border bg-background px-3 py-2 text-sm"
          />
        </div>

        <Button size="sm" onClick={loadEvents} disabled={isLoading || activeKind === null}>
          {isLoading ? 'Loading...' : 'Search Versions'}
        </Button>
      </div>

      {events.length > 0 && (
        <div className="text-xs text-muted-foreground">
          {events.length} version{events.length !== 1 ? 's' : ''} found for kind{' '}
          {activeKind} ({getKindName(activeKind!)})
        </div>
      )}

      <div className="space-y-3">
        {events.map((event, idx) => {
          const isCurrent = isCurrentVersion(event, idx)
          return (
            <div
              key={`${event.id}-${idx}`}
              className={
                isCurrent
                  ? 'rounded-lg border bg-card p-4'
                  : 'rounded-lg border border-yellow-600/50 bg-yellow-900/10 p-4'
              }
            >
              <div className="flex flex-wrap items-center justify-between gap-2 mb-3">
                <div className="space-y-0.5">
                  {isCurrent && (
                    <span className="text-sm font-semibold text-primary">Current Version</span>
                  )}
                  <div className="text-xs text-muted-foreground">
                    {new Date(event.created_at * 1000).toLocaleString()}
                  </div>
                  <div className="text-xs font-mono text-muted-foreground">
                    {event.id.slice(0, 16)}...
                  </div>
                </div>
                <div className="flex items-center gap-2 flex-wrap">
                  {!isCurrent && (
                    <Button
                      size="sm"
                      onClick={() => repostEvent(event)}
                      disabled={isReposting === event.id}
                    >
                      {isReposting === event.id ? 'Reposting...' : 'Repost'}
                    </Button>
                  )}
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => copyEvent(event)}
                  >
                    Copy JSON
                  </Button>
                </div>
              </div>
              <pre className="text-xs font-mono overflow-x-auto whitespace-pre-wrap break-all bg-background p-3 rounded max-h-[300px] overflow-y-auto">
                {JSON.stringify(event, null, 2)}
              </pre>
            </div>
          )
        })}

        {events.length === 0 && !isLoading && activeKind !== null && (
          <div className="text-center py-8 text-muted-foreground">
            No events found for this kind.
          </div>
        )}

        {isLoading && (
          <div className="text-center py-8 text-muted-foreground">Loading events...</div>
        )}
      </div>
    </div>
  )
}
