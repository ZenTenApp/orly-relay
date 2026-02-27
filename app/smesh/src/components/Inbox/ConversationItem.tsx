import UserAvatar from '@/components/UserAvatar'
import { useKeyboardNavigable } from '@/hooks/useKeyboardNavigable'
import { formatTimestamp } from '@/lib/timestamp'
import { cn } from '@/lib/utils'
import client from '@/services/client.service'
import { TConversation, TProfile } from '@/types'
import { Lock, Users, X } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'

interface ConversationItemProps {
  conversation: TConversation
  isActive: boolean
  isFollowing: boolean
  onClick: () => void
  onClose?: () => void
  navIndex?: number
}

export default function ConversationItem({
  conversation,
  isActive,
  isFollowing,
  onClick,
  onClose,
  navIndex
}: ConversationItemProps) {
  const [profile, setProfile] = useState<TProfile | null>(null)
  const buttonRef = useRef<HTMLButtonElement>(null)

  const handleActivate = useCallback(() => {
    buttonRef.current?.click()
  }, [])

  const { ref: navRef, isSelected } = useKeyboardNavigable(1, navIndex ?? 0, {
    meta: { type: 'sidebar', onActivate: handleActivate }
  })

  useEffect(() => {
    const fetchProfileData = async () => {
      try {
        const profileData = await client.fetchProfile(conversation.partnerPubkey)
        if (profileData) {
          setProfile(profileData)
        }
      } catch (error) {
        console.error('Failed to fetch profile:', error)
      }
    }
    fetchProfileData()
  }, [conversation.partnerPubkey])

  const displayName = profile?.username || conversation.partnerPubkey.slice(0, 8) + '...'
  const formattedTime = formatTimestamp(conversation.lastMessageAt)

  return (
    <div ref={navRef} className="scroll-mt-[6.5rem]">
      <button
        ref={buttonRef}
        onClick={onClick}
        className={cn(
          'w-full flex items-start gap-3 p-3 hover:bg-accent/50 transition-colors text-left',
          isActive && 'bg-accent',
          isSelected && 'ring-2 ring-primary ring-inset'
        )}
      >
      <UserAvatar userId={conversation.partnerPubkey} className="size-10 flex-shrink-0" />

      <div className="flex-1 min-w-0">
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-1.5 min-w-0">
            <span className="font-medium text-sm truncate">{displayName}</span>
            {isFollowing && (
              <span className="text-xs text-primary flex-shrink-0" title="Following">
                <Users className="size-3" />
              </span>
            )}
          </div>
          <div className="flex items-center gap-1 flex-shrink-0">
            <span className="text-xs text-muted-foreground">{formattedTime}</span>
            {isActive && onClose && (
              <button
                onClick={(e) => {
                  e.stopPropagation()
                  onClose()
                }}
                className="p-0.5 rounded hover:bg-muted-foreground/20 transition-colors"
                title="Close conversation"
              >
                <X className="size-4 text-muted-foreground" />
              </button>
            )}
          </div>
        </div>

        <div className="flex items-center gap-1.5 mt-0.5">
          {conversation.preferredEncryption === 'nip17' && (
            <span title="NIP-17 encrypted">
              <Lock className="size-3 text-green-500 flex-shrink-0" />
            </span>
          )}
          <p className="text-sm text-muted-foreground truncate">{conversation.lastMessagePreview}</p>
        </div>

        {conversation.unreadCount > 0 && (
          <span className="inline-flex items-center justify-center size-5 text-xs rounded-full bg-primary text-primary-foreground mt-1">
            {conversation.unreadCount}
          </span>
        )}
      </div>
      </button>
    </div>
  )
}
