import MessageView from '@/components/Inbox/MessageView'
import UserAvatar from '@/components/UserAvatar'
import { Button } from '@/components/ui/button'
import { Titlebar } from '@/components/Titlebar'
import { useSecondaryPage } from '@/PageManager'
import { useDM } from '@/providers/DMProvider'
import { useFollowList } from '@/providers/FollowListProvider'
import client from '@/services/client.service'
import { TPageRef, TProfile } from '@/types'
import { ChevronLeft, MoreVertical, RefreshCw, Settings, Trash2, Undo2, Users, X } from 'lucide-react'
import { nip19 } from 'nostr-tools'
import { forwardRef, useEffect, useImperativeHandle, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from '@/components/ui/dropdown-menu'
import ConversationSettingsModal from '@/components/Inbox/ConversationSettingsModal'
import indexedDb from '@/services/indexed-db.service'
import { useNostr } from '@/providers/NostrProvider'

interface DMConversationPageProps {
  pubkey?: string
}

const DMConversationPage = forwardRef<TPageRef, DMConversationPageProps>(({ pubkey }, ref) => {
  const { t } = useTranslation()
  const layoutRef = useRef<TPageRef>(null)
  const { pubkey: userPubkey } = useNostr()
  const {
    selectConversation,
    currentConversation,
    isLoadingConversation,
    isNewConversation,
    clearNewConversationFlag,
    reloadConversation,
    deleteAllInConversation,
    undeleteAllInConversation
  } = useDM()
  const { pop } = useSecondaryPage()
  const { followingSet } = useFollowList()
  const [profile, setProfile] = useState<TProfile | null>(null)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [selectedRelays, setSelectedRelays] = useState<string[]>([])
  const [showPulse, setShowPulse] = useState(false)

  // Decode npub to hex if needed
  const hexPubkey = useMemo(() => {
    if (!pubkey) return null
    if (pubkey.startsWith('npub')) {
      try {
        const decoded = nip19.decode(pubkey)
        return decoded.type === 'npub' ? decoded.data : null
      } catch {
        return null
      }
    }
    return pubkey
  }, [pubkey])

  const isFollowing = hexPubkey ? followingSet.has(hexPubkey) : false

  useImperativeHandle(ref, () => layoutRef.current as TPageRef)

  // Select the conversation when this page mounts
  useEffect(() => {
    if (hexPubkey && hexPubkey !== currentConversation) {
      selectConversation(hexPubkey)
    }
  }, [hexPubkey, selectConversation, currentConversation])

  // Clear conversation when page unmounts
  useEffect(() => {
    return () => {
      selectConversation(null)
    }
  }, [])

  // Fetch profile
  useEffect(() => {
    if (!hexPubkey) return

    const fetchProfileData = async () => {
      try {
        const profileData = await client.fetchProfile(hexPubkey)
        if (profileData) {
          setProfile(profileData)
        }
      } catch (error) {
        console.error('Failed to fetch profile:', error)
      }
    }
    fetchProfileData()
  }, [hexPubkey])

  // Handle pulsing animation for new conversations
  useEffect(() => {
    if (isNewConversation) {
      setShowPulse(true)
      const timer = setTimeout(() => {
        setShowPulse(false)
        clearNewConversationFlag()
      }, 10000)
      return () => clearTimeout(timer)
    }
  }, [isNewConversation, clearNewConversationFlag])

  // Load saved relay settings when conversation changes
  useEffect(() => {
    if (!hexPubkey || !userPubkey) return

    const loadRelaySettings = async () => {
      const saved = await indexedDb.getConversationRelaySettings(userPubkey, hexPubkey)
      setSelectedRelays(saved || [])
    }
    loadRelaySettings()
  }, [hexPubkey, userPubkey])

  // Save relay settings when they change
  const handleRelaysChange = async (relays: string[]) => {
    setSelectedRelays(relays)
    if (userPubkey && hexPubkey) {
      await indexedDb.putConversationRelaySettings(userPubkey, hexPubkey, relays)
    }
  }

  const handleBack = () => {
    selectConversation(null)
    pop()
  }

  const displayName = profile?.username || (hexPubkey ? hexPubkey.slice(0, 8) + '...' : '')

  // Custom titlebar with user info
  const titlebar = (
    <div className="flex items-center gap-2 w-full px-1">
      <Button
        className="flex gap-1 items-center justify-start pl-2 pr-1"
        variant="ghost"
        size="titlebar-icon"
        title={t('back')}
        onClick={handleBack}
      >
        <ChevronLeft />
      </Button>
      {hexPubkey && (
        <>
          <UserAvatar userId={hexPubkey} className="size-7" />
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-1.5">
              <span className="font-semibold text-sm truncate">{displayName}</span>
              {isFollowing && (
                <span title="Following">
                  <Users className="size-3 text-primary" />
                </span>
              )}
            </div>
            {profile?.nip05 && (
              <span className="text-xs text-muted-foreground truncate block">{profile.nip05}</span>
            )}
          </div>
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            title={t('Reload messages')}
            onClick={reloadConversation}
            disabled={isLoadingConversation}
          >
            <RefreshCw className={cn('size-4', isLoadingConversation && 'animate-spin')} />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className={cn('size-8', showPulse && 'animate-pulse ring-2 ring-primary ring-offset-2')}
            title={t('Conversation settings')}
            onClick={() => {
              setShowPulse(false)
              clearNewConversationFlag()
              setSettingsOpen(true)
            }}
          >
            <Settings className="size-4" />
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="size-8">
                <MoreVertical className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={deleteAllInConversation} className="text-destructive focus:text-destructive">
                <Trash2 className="size-4 mr-2" />
                {t('Delete All')}
              </DropdownMenuItem>
              <DropdownMenuItem onClick={undeleteAllInConversation}>
                <Undo2 className="size-4 mr-2" />
                {t('Undelete All')}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            title={t('Close conversation')}
            onClick={handleBack}
          >
            <X className="size-4" />
          </Button>
        </>
      )}
    </div>
  )

  return (
    <div className="flex flex-col h-[var(--vh)]">
      <Titlebar className="p-1 shrink-0" hideBottomBorder={false}>
        {titlebar}
      </Titlebar>
      <div className="flex-1 min-h-0">
        <MessageView hideHeader />
      </div>
      {hexPubkey && (
        <ConversationSettingsModal
          partnerPubkey={hexPubkey}
          open={settingsOpen}
          onOpenChange={setSettingsOpen}
          selectedRelays={selectedRelays}
          onSelectedRelaysChange={handleRelaysChange}
        />
      )}
    </div>
  )
})

DMConversationPage.displayName = 'DMConversationPage'
export default DMConversationPage
