import { useFetchProfile } from '@/hooks/useFetchProfile'
import { Pubkey } from '@/domain'
import { useMemo } from 'react'

export default function MentionPopup({
  participants,
  query,
  onSelect,
  position
}: {
  participants: string[]
  query: string // text after @ (lowercase)
  onSelect: (pubkey: string, displayName: string) => void
  position: { bottom: number; left: number }
}) {
  const filtered = useMemo(() => {
    if (!query) return participants.slice(0, 8)
    return participants.filter((pk) => {
      const npub = Pubkey.tryFromString(pk)?.formatNpub(20) || ''
      return pk.toLowerCase().includes(query) || npub.toLowerCase().includes(query)
    }).slice(0, 8)
  }, [participants, query])

  if (filtered.length === 0) return null

  return (
    <div
      className="absolute z-30 bg-popover border rounded shadow-md max-h-48 overflow-y-auto w-56"
      style={{ bottom: position.bottom, left: position.left }}
    >
      {filtered.map((pk) => (
        <MentionItem key={pk} pubkey={pk} query={query} onSelect={onSelect} />
      ))}
    </div>
  )
}

function MentionItem({
  pubkey,
  query,
  onSelect
}: {
  pubkey: string
  query: string
  onSelect: (pubkey: string, displayName: string) => void
}) {
  const { profile } = useFetchProfile(pubkey)
  const pk = Pubkey.tryFromString(pubkey)
  const displayName = profile?.username || pk?.formatNpub(8) || pubkey.slice(0, 12)
  const npub = pk?.npub || ''

  // Profile-based filtering (name match)
  const nameMatch = !query || displayName.toLowerCase().includes(query)
  if (!nameMatch && !pubkey.toLowerCase().includes(query)) return null

  return (
    <button
      className="w-full px-3 py-1.5 text-left hover:bg-muted flex items-center gap-2 text-xs"
      onClick={() => onSelect(pubkey, displayName)}
    >
      <div className="size-5 rounded-full bg-muted overflow-hidden shrink-0">
        {profile?.avatar && <img src={profile.avatar} alt="" className="w-full h-full object-cover" />}
      </div>
      <span className="font-medium truncate">{displayName}</span>
      <span className="text-muted-foreground text-[10px] truncate">{npub.slice(0, 16)}...</span>
    </button>
  )
}
