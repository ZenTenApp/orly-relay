import { useCallback, useEffect, useRef, useState } from 'react'
import { useRelayAdmin } from '@/providers/RelayAdminProvider'
import client from '@/services/client.service'
import { Button } from '@/components/ui/button'
import { toast } from 'sonner'
import { nip19 } from 'nostr-tools'

// ── Types ──

interface BlobDescriptor {
  sha256: string
  size: number
  type?: string
  uploaded?: number
  url?: string
}

interface UserStat {
  pubkey: string
  blob_count: number
  total_size_bytes: number
  profile?: { name?: string; picture?: string }
}

type SortField = 'date' | 'size'
type SortOrder = 'asc' | 'desc'

// ── Helpers ──

function getApiBase(): string {
  return window.location.origin
}

async function createBlossomAuth(verb: string, sha256Hex?: string): Promise<string> {
  const now = Math.floor(Date.now() / 1000)
  const expiration = now + 60
  const tags: string[][] = [
    ['t', verb],
    ['expiration', expiration.toString()]
  ]
  if (sha256Hex) {
    tags.push(['x', sha256Hex])
  }
  const event = await client.signer!.signEvent({
    kind: 24242,
    created_at: now,
    tags,
    content: `Blossom ${verb} operation`
  })
  return 'Nostr ' + btoa(JSON.stringify(event))
}

function formatSize(bytes: number | undefined): string {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let size = bytes
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024
    i++
  }
  return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

function formatDate(timestamp: number | undefined): string {
  if (!timestamp) return 'Unknown'
  return new Date(timestamp * 1000).toLocaleString()
}

function truncateHash(hash: string): string {
  if (!hash) return ''
  return `${hash.slice(0, 8)}...${hash.slice(-8)}`
}

function hexToNpub(hex: string): string {
  try {
    return nip19.npubEncode(hex)
  } catch {
    return hex
  }
}

function truncateNpub(npub: string): string {
  if (!npub) return ''
  return `${npub.slice(0, 12)}...${npub.slice(-8)}`
}

function sortBlobs(list: BlobDescriptor[], by: SortField, order: SortOrder): BlobDescriptor[] {
  if (!list.length) return list
  const sorted = [...list].sort((a, b) => {
    const cmp = by === 'date' ? (a.uploaded || 0) - (b.uploaded || 0) : (a.size || 0) - (b.size || 0)
    return order === 'desc' ? -cmp : cmp
  })
  return sorted
}

function getBlobUrl(blob: BlobDescriptor): string {
  if (blob.url) {
    if (blob.url.startsWith('http://') || blob.url.startsWith('https://')) return blob.url
    if (blob.url.startsWith('/')) return `${getApiBase()}${blob.url}`
    return `http://${blob.url}`
  }
  return `${getApiBase()}/blossom/${blob.sha256}`
}

function getThumbnailUrl(blob: BlobDescriptor): string {
  const base = getBlobUrl(blob)
  const sep = base.includes('?') ? '&' : '?'
  return `${base}${sep}w=128`
}

function mimeCategory(mimeType?: string): string {
  if (!mimeType) return 'unknown'
  if (mimeType.startsWith('image/')) return 'image'
  if (mimeType.startsWith('video/')) return 'video'
  if (mimeType.startsWith('audio/')) return 'audio'
  return 'file'
}

// ── Constants ──

const PAGE_SIZE = 40

// ── Component ──

export default function BlossomAdminTab() {
  const { isAdmin, isOwner } = useRelayAdmin()

  // Admin user listing
  const [userStats, setUserStats] = useState<UserStat[]>([])
  const [isLoadingUsers, setIsLoadingUsers] = useState(false)

  // Selected user blob listing
  const [selectedUser, setSelectedUser] = useState<UserStat | null>(null)
  const [userBlobs, setUserBlobs] = useState<BlobDescriptor[]>([])
  const [isLoadingBlobs, setIsLoadingBlobs] = useState(false)

  // Sort
  const [sortBy, setSortBy] = useState<SortField>('date')
  const [sortOrder, setSortOrder] = useState<SortOrder>('desc')

  // Pagination
  const [visibleCount, setVisibleCount] = useState(PAGE_SIZE)
  const sentinelRef = useRef<HTMLDivElement>(null)

  // Selection for bulk delete
  const [selectedHashes, setSelectedHashes] = useState<Set<string>>(new Set())
  const [isDeletingSelected, setIsDeletingSelected] = useState(false)

  const [error, setError] = useState('')

  const canAdmin = isAdmin || isOwner

  // ── Data loading ──

  const fetchUserStats = useCallback(async () => {
    setIsLoadingUsers(true)
    setError('')
    try {
      const auth = await createBlossomAuth('admin')
      const url = `${getApiBase()}/blossom/admin/users`
      const res = await fetch(url, { headers: { Authorization: auth } })
      if (!res.ok) throw new Error(`Failed to load user stats: ${res.statusText}`)
      const data: UserStat[] = await res.json()

      // Resolve profiles in the background
      const statsWithProfiles = data.map((s) => ({ ...s }))
      setUserStats(statsWithProfiles)

      for (const stat of statsWithProfiles) {
        client
          .fetchProfile(stat.pubkey)
          .then((profile) => {
            if (profile) {
              stat.profile = { name: profile.username, picture: profile.avatar }
              setUserStats((prev) => [...prev])
            }
          })
          .catch(() => {})
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load user stats')
    } finally {
      setIsLoadingUsers(false)
    }
  }, [])

  const loadUserBlobs = useCallback(async (userPubkey: string) => {
    setIsLoadingBlobs(true)
    setError('')
    try {
      const url = `${getApiBase()}/blossom/list/${userPubkey}`
      const res = await fetch(url)
      if (!res.ok) throw new Error(`Failed to load blobs: ${res.statusText}`)
      const data: BlobDescriptor[] = await res.json()
      setUserBlobs(data)
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load blobs')
    } finally {
      setIsLoadingBlobs(false)
    }
  }, [])

  // Load user stats on mount
  useEffect(() => {
    if (canAdmin) {
      fetchUserStats()
    }
  }, [canAdmin, fetchUserStats])

  // Load blobs when a user is selected
  useEffect(() => {
    if (selectedUser) {
      setVisibleCount(PAGE_SIZE)
      setSelectedHashes(new Set())
      loadUserBlobs(selectedUser.pubkey)
    }
  }, [selectedUser, loadUserBlobs])

  // Reset visible count on sort change
  useEffect(() => {
    setVisibleCount(PAGE_SIZE)
  }, [sortBy, sortOrder])

  // Intersection observer for infinite scroll
  useEffect(() => {
    const el = sentinelRef.current
    if (!el) return
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting) {
          setVisibleCount((prev) => prev + PAGE_SIZE)
        }
      },
      { rootMargin: '0px 0px 150% 0px' }
    )
    observer.observe(el)
    return () => observer.disconnect()
  }, [selectedUser])

  // ── Derived data ──

  const sortedBlobs = sortBlobs(userBlobs, sortBy, sortOrder)
  const displayBlobs = sortedBlobs.slice(0, visibleCount)
  const hasMore = visibleCount < sortedBlobs.length

  const totalStorage = userStats.reduce((sum, u) => sum + (u.total_size_bytes || 0), 0)
  const totalBlobs = userStats.reduce((sum, u) => sum + (u.blob_count || 0), 0)

  // ── Actions ──

  const toggleSort = (field: SortField) => {
    if (sortBy === field) {
      setSortOrder((prev) => (prev === 'desc' ? 'asc' : 'desc'))
    } else {
      setSortBy(field)
      setSortOrder('desc')
    }
  }

  const toggleSelection = (hash: string) => {
    setSelectedHashes((prev) => {
      const next = new Set(prev)
      if (next.has(hash)) next.delete(hash)
      else next.add(hash)
      return next
    })
  }

  const deleteBlob = async (blob: BlobDescriptor) => {
    if (!confirm(`Delete blob ${truncateHash(blob.sha256)}?`)) return
    try {
      const auth = await createBlossomAuth('delete', blob.sha256)
      const url = `${getApiBase()}/blossom/${blob.sha256}`
      const res = await fetch(url, { method: 'DELETE', headers: { Authorization: auth } })
      if (!res.ok) throw new Error(`Delete failed: ${res.statusText}`)
      toast.success('Blob deleted')
      if (selectedUser) loadUserBlobs(selectedUser.pubkey)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Delete failed')
    }
  }

  const deleteSelectedBlobs = async () => {
    if (selectedHashes.size === 0) return
    if (!confirm(`Delete ${selectedHashes.size} selected file(s)? This cannot be undone.`)) return

    setIsDeletingSelected(true)
    let deleted = 0
    let failed = 0

    for (const hash of selectedHashes) {
      try {
        const auth = await createBlossomAuth('delete', hash)
        const url = `${getApiBase()}/blossom/${hash}`
        const res = await fetch(url, { method: 'DELETE', headers: { Authorization: auth } })
        if (res.ok) deleted++
        else failed++
      } catch {
        failed++
      }
    }

    setSelectedHashes(new Set())
    setIsDeletingSelected(false)

    if (failed > 0) {
      toast.warning(`Deleted ${deleted}, failed ${failed}`)
    } else {
      toast.success(`Deleted ${deleted} file(s)`)
    }

    if (selectedUser) loadUserBlobs(selectedUser.pubkey)
  }

  const handleRefresh = () => {
    if (selectedUser) {
      loadUserBlobs(selectedUser.pubkey)
    } else {
      fetchUserStats()
    }
  }

  // ── Render ──

  if (!canAdmin) {
    return (
      <div className="p-4 text-muted-foreground text-center">
        Admin access required.
      </div>
    )
  }

  // Selected user blob list view
  if (selectedUser) {
    return (
      <div className="p-4 space-y-3 w-full">
        {/* Header */}
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="sm" onClick={() => setSelectedUser(null)}>
              &larr; Back
            </Button>
            <div className="flex items-center gap-2">
              {selectedUser.profile?.picture && (
                <img
                  src={selectedUser.profile.picture}
                  alt=""
                  className="w-6 h-6 rounded-full object-cover"
                />
              )}
              <span className="font-semibold truncate max-w-[200px]">
                {selectedUser.profile?.name || truncateNpub(hexToNpub(selectedUser.pubkey))}
              </span>
            </div>
          </div>
          <div className="flex items-center gap-2 flex-wrap">
            {selectedHashes.size > 0 && (
              <Button
                variant="destructive"
                size="sm"
                onClick={deleteSelectedBlobs}
                disabled={isDeletingSelected}
              >
                {isDeletingSelected
                  ? 'Deleting...'
                  : `Delete Selected (${selectedHashes.size})`}
              </Button>
            )}
            <Button variant="ghost" size="sm" onClick={() => toggleSort('date')}>
              Date {sortBy === 'date' ? (sortOrder === 'desc' ? '\u2193' : '\u2191') : ''}
            </Button>
            <Button variant="ghost" size="sm" onClick={() => toggleSort('size')}>
              Size {sortBy === 'size' ? (sortOrder === 'desc' ? '\u2193' : '\u2191') : ''}
            </Button>
            <Button size="sm" onClick={handleRefresh} disabled={isLoadingBlobs}>
              {isLoadingBlobs ? 'Loading...' : 'Refresh'}
            </Button>
          </div>
        </div>

        {error && (
          <div className="rounded-md bg-destructive/10 text-destructive p-3 text-sm">{error}</div>
        )}

        <div className="text-xs text-muted-foreground">
          {sortedBlobs.length} blob(s) &middot; {formatSize(selectedUser.total_size_bytes)}
        </div>

        {/* Blob list */}
        {isLoadingBlobs && displayBlobs.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">Loading blobs...</div>
        ) : displayBlobs.length === 0 ? (
          <div className="text-center py-8 text-muted-foreground">
            No files found for this user.
          </div>
        ) : (
          <div className="space-y-1">
            {displayBlobs.map((blob) => (
              <div
                key={blob.sha256}
                className={`flex items-center gap-2 rounded-md px-3 py-2 ${
                  selectedHashes.has(blob.sha256)
                    ? 'bg-primary/10 ring-1 ring-primary/30'
                    : 'bg-card'
                }`}
              >
                <input
                  type="checkbox"
                  checked={selectedHashes.has(blob.sha256)}
                  onChange={() => toggleSelection(blob.sha256)}
                  className="shrink-0"
                />
                <div className="w-10 h-10 shrink-0 rounded overflow-hidden bg-muted flex items-center justify-center">
                  {mimeCategory(blob.type) === 'image' ? (
                    <img
                      src={getThumbnailUrl(blob)}
                      alt=""
                      className="w-full h-full object-cover"
                      loading="lazy"
                    />
                  ) : (
                    <span className="text-xs text-muted-foreground">
                      {blob.type?.split('/')[1]?.slice(0, 4) || '?'}
                    </span>
                  )}
                </div>
                <div className="flex-1 min-w-0">
                  <div className="font-mono text-xs truncate" title={blob.sha256}>
                    {truncateHash(blob.sha256)}
                  </div>
                  <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
                    <span>{formatSize(blob.size)}</span>
                    <span>{blob.type || 'unknown'}</span>
                    <span>{formatDate(blob.uploaded)}</span>
                  </div>
                </div>
                <Button
                  variant="ghost"
                  size="sm"
                  className="shrink-0 text-destructive hover:text-destructive hover:bg-destructive/10"
                  onClick={(e) => {
                    e.stopPropagation()
                    deleteBlob(blob)
                  }}
                >
                  Delete
                </Button>
              </div>
            ))}
            {hasMore && (
              <div ref={sentinelRef} className="text-center py-4 text-xs text-muted-foreground">
                Loading more...
              </div>
            )}
          </div>
        )}
      </div>
    )
  }

  // User stats list view (default)
  return (
    <div className="p-4 space-y-3 w-full">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-lg font-semibold">Blossom Storage Admin</h3>
        <Button size="sm" onClick={handleRefresh} disabled={isLoadingUsers}>
          {isLoadingUsers ? 'Loading...' : 'Refresh'}
        </Button>
      </div>

      {error && (
        <div className="rounded-md bg-destructive/10 text-destructive p-3 text-sm">{error}</div>
      )}

      {/* Summary stats */}
      <div className="flex gap-4 text-sm">
        <div className="rounded-lg bg-card p-3 flex-1 text-center">
          <div className="text-2xl font-bold">{userStats.length}</div>
          <div className="text-muted-foreground">Users</div>
        </div>
        <div className="rounded-lg bg-card p-3 flex-1 text-center">
          <div className="text-2xl font-bold">{totalBlobs}</div>
          <div className="text-muted-foreground">Blobs</div>
        </div>
        <div className="rounded-lg bg-card p-3 flex-1 text-center">
          <div className="text-2xl font-bold">{formatSize(totalStorage)}</div>
          <div className="text-muted-foreground">Total</div>
        </div>
      </div>

      {/* User list */}
      {isLoadingUsers ? (
        <div className="text-center py-8 text-muted-foreground">Loading user statistics...</div>
      ) : userStats.length === 0 ? (
        <div className="text-center py-8 text-muted-foreground">
          No users have uploaded files yet.
        </div>
      ) : (
        <div className="space-y-1">
          {userStats.map((stat) => (
            <button
              key={stat.pubkey}
              className="w-full flex items-center gap-3 rounded-md bg-card px-3 py-2.5 text-left hover:bg-accent transition-colors"
              onClick={() => setSelectedUser(stat)}
            >
              <div className="w-9 h-9 rounded-full overflow-hidden bg-muted shrink-0 flex items-center justify-center">
                {stat.profile?.picture ? (
                  <img
                    src={stat.profile.picture}
                    alt=""
                    className="w-full h-full object-cover"
                  />
                ) : (
                  <span className="text-xs text-muted-foreground">?</span>
                )}
              </div>
              <div className="flex-1 min-w-0">
                <div className="font-medium truncate">
                  {stat.profile?.name || truncateNpub(hexToNpub(stat.pubkey))}
                </div>
                <div className="text-xs text-muted-foreground font-mono truncate">
                  {truncateNpub(hexToNpub(stat.pubkey))}
                </div>
              </div>
              <div className="text-right shrink-0">
                <div className="text-sm font-medium">{stat.blob_count} files</div>
                <div className="text-xs text-muted-foreground">
                  {formatSize(stat.total_size_bytes)}
                </div>
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
