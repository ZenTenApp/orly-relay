import { useChat } from '@/providers/ChatProvider'
import { useNostr } from '@/providers/NostrProvider'
import { useFetchProfile } from '@/hooks/useFetchProfile'
import { useSecondaryPage } from '@/PageManager'
import { Pubkey } from '@/domain'
import {
  X,
  ExternalLink,
  MessageSquare,
  ShieldPlus,
  UserMinus,
  Ban,
  Copy,
  Check
} from 'lucide-react'
import { useState } from 'react'
import { Button } from '../ui/button'

export default function UserProfileModal({
  pubkeyHex,
  onClose
}: {
  pubkeyHex: string
  onClose: () => void
}) {
  const { profile } = useFetchProfile(pubkeyHex)
  const { pubkey } = useNostr()
  const { currentChannel, isOwnerOrMod, addMod, removeMember, blockUser, channelMods } = useChat()
  const { push } = useSecondaryPage()
  const [copied, setCopied] = useState(false)

  const pk = Pubkey.tryFromString(pubkeyHex)
  const npub = pk?.npub || ''
  const displayName = profile?.username || pk?.formatNpub(8) || pubkeyHex.slice(0, 12)
  const isOwner = currentChannel?.creator === pubkeyHex
  const isMod = channelMods.includes(pubkeyHex)
  const isSelf = pubkeyHex === pubkey

  const handleCopy = () => {
    navigator.clipboard.writeText(npub)
    setCopied(true)
    setTimeout(() => setCopied(false), 1500)
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div
        className="bg-background border rounded-lg w-80 overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Banner */}
        {profile?.banner ? (
          <div className="h-20 bg-muted">
            <img src={profile.banner} alt="" className="w-full h-full object-cover" />
          </div>
        ) : (
          <div className="h-20 bg-gradient-to-r from-primary/20 to-primary/5" />
        )}

        {/* Avatar + name */}
        <div className="px-4 -mt-6">
          <div className="size-12 rounded-full border-2 border-background overflow-hidden bg-muted">
            {profile?.avatar ? (
              <img src={profile.avatar} alt="" className="w-full h-full object-cover" />
            ) : (
              <div className="w-full h-full bg-primary/20" />
            )}
          </div>
        </div>

        <div className="px-4 pt-1 pb-3 space-y-2">
          <div>
            <div className="font-semibold text-sm flex items-center gap-1">
              {displayName}
              {isOwner && <span className="text-[10px] text-muted-foreground">(owner)</span>}
              {isMod && !isOwner && <span className="text-[10px] text-muted-foreground">(mod)</span>}
            </div>
            <button
              onClick={handleCopy}
              className="text-[10px] text-muted-foreground font-mono flex items-center gap-0.5 hover:text-foreground"
            >
              {npub.slice(0, 20)}...
              {copied ? <Check className="size-2.5" /> : <Copy className="size-2.5" />}
            </button>
          </div>

          {profile?.about && (
            <p className="text-xs text-muted-foreground line-clamp-2">{profile.about}</p>
          )}

          {/* Actions */}
          <div className="flex gap-1.5 pt-1">
            <Button
              variant="outline"
              size="sm"
              className="h-7 text-xs gap-1 flex-1"
              onClick={() => {
                onClose()
                push(`/users/${pubkeyHex}`)
              }}
            >
              <ExternalLink className="size-3" /> Profile
            </Button>
            {!isSelf && (
              <Button
                variant="outline"
                size="sm"
                className="h-7 text-xs gap-1 flex-1"
                onClick={() => {
                  onClose()
                  push(`/dm/${pubkeyHex}`)
                }}
              >
                <MessageSquare className="size-3" /> DM
              </Button>
            )}
          </div>

          {/* Mod actions */}
          {isOwnerOrMod && !isSelf && !isOwner && (
            <div className="flex gap-1.5 pt-0.5">
              {!isMod && (
                <Button
                  variant="outline"
                  size="sm"
                  className="h-7 text-xs gap-1 flex-1"
                  onClick={() => { addMod(pubkeyHex); onClose() }}
                >
                  <ShieldPlus className="size-3" /> Make Mod
                </Button>
              )}
              <Button
                variant="outline"
                size="sm"
                className="h-7 text-xs gap-1 flex-1"
                onClick={() => { removeMember(pubkeyHex); onClose() }}
              >
                <UserMinus className="size-3" /> Kick
              </Button>
              <Button
                variant="outline"
                size="sm"
                className="h-7 text-xs gap-1 flex-1 text-destructive border-destructive/30"
                onClick={() => { blockUser(pubkeyHex); onClose() }}
              >
                <Ban className="size-3" /> Block
              </Button>
            </div>
          )}
        </div>

        <Button
          variant="ghost"
          size="icon"
          className="absolute top-2 right-2 size-6 bg-background/80"
          onClick={onClose}
        >
          <X className="size-3.5" />
        </Button>
      </div>
    </div>
  )
}
