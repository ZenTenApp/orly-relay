import { useChat } from '@/providers/ChatProvider'
import { useFetchProfile } from '@/hooks/useFetchProfile'
import { useSecondaryPage } from '@/PageManager'
import { Pubkey } from '@/domain'
import { X, MessageSquare, Shield, Crown } from 'lucide-react'
import { Button } from '../ui/button'

export default function MemberListPanel({ onClose }: { onClose: () => void }) {
  const { currentChannel, channelParticipants, channelMods } = useChat()

  if (!currentChannel) return null

  // Sort: owner first, then mods, then everyone else
  const sorted = [...channelParticipants].sort((a, b) => {
    const aOwner = a === currentChannel.creator ? 0 : 1
    const bOwner = b === currentChannel.creator ? 0 : 1
    if (aOwner !== bOwner) return aOwner - bOwner
    const aMod = channelMods.includes(a) ? 0 : 1
    const bMod = channelMods.includes(b) ? 0 : 1
    return aMod - bMod
  })

  return (
    <div className="absolute inset-y-0 right-0 z-20 w-56 bg-background border-l overflow-y-auto">
      <div className="flex items-center justify-between p-2 border-b">
        <span className="text-xs font-semibold">Members ({sorted.length})</span>
        <Button variant="ghost" size="icon" className="size-6" onClick={onClose}>
          <X className="size-3.5" />
        </Button>
      </div>
      <div className="py-1">
        {sorted.map((pk) => (
          <MemberItem
            key={pk}
            pubkey={pk}
            isOwner={pk === currentChannel.creator}
            isMod={channelMods.includes(pk)}
          />
        ))}
      </div>
    </div>
  )
}

function MemberItem({
  pubkey,
  isOwner,
  isMod
}: {
  pubkey: string
  isOwner: boolean
  isMod: boolean
}) {
  const { profile } = useFetchProfile(pubkey)
  const { push } = useSecondaryPage()
  const pk = Pubkey.tryFromString(pubkey)
  const displayName = profile?.username || pk?.formatNpub(8) || pubkey.slice(0, 12)

  return (
    <div className="group flex items-center gap-2 px-2 py-1 hover:bg-muted/50 text-xs">
      <div className="size-5 rounded-full bg-muted overflow-hidden shrink-0">
        {profile?.avatar && <img src={profile.avatar} alt="" className="w-full h-full object-cover" />}
      </div>
      <span className="font-medium truncate flex-1">{displayName}</span>
      {isOwner && <span title="Owner"><Crown className="size-3 text-primary shrink-0" /></span>}
      {isMod && !isOwner && <span title="Mod"><Shield className="size-3 text-muted-foreground shrink-0" /></span>}
      <button
        className="opacity-0 group-hover:opacity-100 text-muted-foreground hover:text-foreground shrink-0"
        onClick={() => push(`/dm/${pubkey}`)}
        title="Send DM"
      >
        <MessageSquare className="size-3" />
      </button>
    </div>
  )
}
