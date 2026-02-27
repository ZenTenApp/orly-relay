import UserAvatar from '@/components/UserAvatar'
import { formatTimestamp } from '@/lib/timestamp'
import { cn } from '@/lib/utils'
import { useDM } from '@/providers/DMProvider'
import { useNostr } from '@/providers/NostrProvider'
import client from '@/services/client.service'
import indexedDb from '@/services/indexed-db.service'
import { TDirectMessage, TProfile } from '@/types'
import { ChevronDown, ChevronUp, Loader2, MoreVertical, RefreshCw, Settings, Trash2, Undo2, Users, X } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '../ui/button'
import { ScrollArea } from '../ui/scroll-area'
import { Checkbox } from '../ui/checkbox'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from '../ui/dropdown-menu'
import MessageComposer from './MessageComposer'
import MessageContent from './MessageContent'
import MessageInfoModal from './MessageInfoModal'
import ConversationSettingsModal from './ConversationSettingsModal'
import { useFollowList } from '@/providers/FollowListProvider'

interface MessageViewProps {
  onBack?: () => void
  hideHeader?: boolean
}

export default function MessageView({ onBack, hideHeader }: MessageViewProps) {
  const { t } = useTranslation()
  const { pubkey } = useNostr()
  const {
    currentConversation,
    messages,
    isLoadingConversation,
    isNewConversation,
    clearNewConversationFlag,
    reloadConversation,
    // Selection mode
    selectedMessages,
    isSelectionMode,
    toggleMessageSelection,
    clearSelection,
    deleteSelectedMessages,
    deleteAllInConversation,
    undeleteAllInConversation
  } = useDM()
  const { followingSet } = useFollowList()
  const [profile, setProfile] = useState<TProfile | null>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const [selectedMessage, setSelectedMessage] = useState<TDirectMessage | null>(null)
  const [messageInfoOpen, setMessageInfoOpen] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(false)
  const [selectedRelays, setSelectedRelays] = useState<string[]>([])
  const [showPulse, setShowPulse] = useState(false)
  const [showJumpButton, setShowJumpButton] = useState(false)
  const [newMessageCount, setNewMessageCount] = useState(0)
  const lastMessageCountRef = useRef(0)
  const isAtBottomRef = useRef(true)
  // Progressive loading: start with 20 messages, load more on demand
  const [visibleLimit, setVisibleLimit] = useState(20)
  const LOAD_MORE_INCREMENT = 20

  const isFollowing = currentConversation ? followingSet.has(currentConversation) : false

  // Calculate visible messages (show most recent, allow loading older)
  const hasMoreMessages = messages.length > visibleLimit
  const visibleMessages = hasMoreMessages
    ? messages.slice(-visibleLimit) // Show last N messages (most recent)
    : messages

  // Load more older messages
  const loadMoreMessages = () => {
    setVisibleLimit((prev) => prev + LOAD_MORE_INCREMENT)
  }

  // Reset visible limit when conversation changes
  useEffect(() => {
    setVisibleLimit(20)
  }, [currentConversation])

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

  useEffect(() => {
    if (!currentConversation) return

    const fetchProfileData = async () => {
      try {
        const profileData = await client.fetchProfile(currentConversation)
        if (profileData) {
          setProfile(profileData)
        }
      } catch (error) {
        console.error('Failed to fetch profile:', error)
      }
    }
    fetchProfileData()
  }, [currentConversation])

  // Load saved relay settings when conversation changes
  useEffect(() => {
    if (!currentConversation || !pubkey) return

    const loadRelaySettings = async () => {
      const saved = await indexedDb.getConversationRelaySettings(pubkey, currentConversation)
      setSelectedRelays(saved || [])
    }
    loadRelaySettings()
  }, [currentConversation, pubkey])

  // Save relay settings when they change
  const handleRelaysChange = async (relays: string[]) => {
    setSelectedRelays(relays)
    if (pubkey && currentConversation) {
      await indexedDb.putConversationRelaySettings(pubkey, currentConversation, relays)
    }
  }

  // Handle scroll position tracking
  const handleScroll = () => {
    if (!scrollRef.current) return
    const { scrollTop, scrollHeight, clientHeight } = scrollRef.current
    const distanceFromBottom = scrollHeight - scrollTop - clientHeight
    const atBottom = distanceFromBottom < 100 // 100px threshold

    isAtBottomRef.current = atBottom
    setShowJumpButton(!atBottom)

    // Reset new message count when user scrolls to bottom
    if (atBottom) {
      setNewMessageCount(0)
      lastMessageCountRef.current = messages.length
    }
  }

  // Track new messages when scrolled up
  useEffect(() => {
    if (!isAtBottomRef.current && messages.length > lastMessageCountRef.current) {
      setNewMessageCount(messages.length - lastMessageCountRef.current)
    } else if (isAtBottomRef.current) {
      lastMessageCountRef.current = messages.length
    }
  }, [messages.length])

  // Scroll to bottom when messages change (only if already at bottom)
  useEffect(() => {
    if (scrollRef.current && isAtBottomRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
      lastMessageCountRef.current = messages.length
    }
  }, [messages])

  // Scroll to bottom function
  const scrollToBottom = () => {
    if (scrollRef.current) {
      scrollRef.current.scrollTo({
        top: scrollRef.current.scrollHeight,
        behavior: 'smooth'
      })
      setNewMessageCount(0)
      lastMessageCountRef.current = messages.length
      isAtBottomRef.current = true
      setShowJumpButton(false)
    }
  }

  // Reset scroll state when conversation changes
  useEffect(() => {
    isAtBottomRef.current = true
    setShowJumpButton(false)
    setNewMessageCount(0)
    lastMessageCountRef.current = 0
  }, [currentConversation])

  // Scroll to bottom when conversation opens and messages are loaded
  const hasMessages = messages.length > 0
  useEffect(() => {
    if (currentConversation && hasMessages && scrollRef.current) {
      // Use requestAnimationFrame to ensure DOM is ready
      requestAnimationFrame(() => {
        if (scrollRef.current) {
          scrollRef.current.scrollTop = scrollRef.current.scrollHeight
          lastMessageCountRef.current = messages.length
        }
      })
    }
  }, [currentConversation, hasMessages])

  if (!currentConversation || !pubkey) {
    return null
  }

  const displayName = profile?.username || currentConversation.slice(0, 8) + '...'

  return (
    <div className="flex flex-col h-full">
      {/* Header - show when not hidden, or when in selection mode */}
      {(!hideHeader || isSelectionMode) && (
        <div className="flex items-center gap-3 p-3 border-b">
          {isSelectionMode ? (
            // Selection mode header
            <>
              <Button
                variant="ghost"
                size="icon"
                onClick={clearSelection}
                className="size-8"
                title={t('Cancel')}
              >
                <X className="size-4" />
              </Button>
              <div className="flex items-center gap-2">
                <Trash2 className="size-4 text-destructive" />
                <span className="font-medium text-sm">{t('Delete')}</span>
              </div>
              <div className="flex-1" />
              <Button
                variant="outline"
                size="sm"
                onClick={deleteSelectedMessages}
                disabled={selectedMessages.size === 0}
                className="text-xs"
              >
                {t('Selected')} ({selectedMessages.size})
              </Button>
              <Button
                variant="destructive"
                size="sm"
                onClick={deleteAllInConversation}
                className="text-xs"
              >
                {t('All')}
              </Button>
            </>
          ) : (
            // Normal header
            <>
              <UserAvatar userId={currentConversation} className="size-8" />
              <div className="flex-1 min-w-0">
                <div className="flex items-center gap-1.5">
                  <span className="font-medium text-sm truncate">{displayName}</span>
                  {isFollowing && (
                    <span title="Following">
                      <Users className="size-3 text-primary" />
                    </span>
                  )}
                </div>
                {profile?.nip05 && (
                  <span className="text-xs text-muted-foreground truncate">{profile.nip05}</span>
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
              {onBack && (
                <Button
                  variant="ghost"
                  size="icon"
                  className="size-8"
                  title={t('Close conversation')}
                  onClick={onBack}
                >
                  <X className="size-4" />
                </Button>
              )}
            </>
          )}
        </div>
      )}

      {/* Messages */}
      <div className="flex-1 relative overflow-hidden">
        <ScrollArea ref={scrollRef} className="h-full p-3" onScrollCapture={handleScroll}>
        {isLoadingConversation && messages.length === 0 ? (
          <div className="flex items-center justify-center h-full">
            <Loader2 className="size-6 animate-spin text-muted-foreground" />
          </div>
        ) : messages.length === 0 ? (
          <div className="flex items-center justify-center h-full text-muted-foreground">
            <p className="text-sm">{t('No messages yet. Send one to start the conversation!')}</p>
          </div>
        ) : (
          <div className="space-y-3">
            {/* Load more button at top */}
            {hasMoreMessages && (
              <div className="flex justify-center py-2">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={loadMoreMessages}
                  className="text-xs text-muted-foreground"
                >
                  <ChevronUp className="size-4 mr-1" />
                  {t('Load older messages')} ({messages.length - visibleLimit} more)
                </Button>
              </div>
            )}
            {isLoadingConversation && (
              <div className="flex justify-center py-2">
                <Loader2 className="size-4 animate-spin text-muted-foreground" />
              </div>
            )}
            {visibleMessages.map((message) => {
              const isOwn = message.senderPubkey === pubkey
              const isSelected = selectedMessages.has(message.id)
              return (
                <div
                  key={message.id}
                  className={cn(
                    'flex items-start gap-2 group',
                    isOwn ? 'flex-row-reverse' : 'flex-row'
                  )}
                >
                  {/* Checkbox - shows on hover or when in selection mode */}
                  <div
                    className={cn(
                      'flex-shrink-0 transition-opacity',
                      isSelectionMode || isSelected
                        ? 'opacity-100'
                        : 'opacity-0 group-hover:opacity-100'
                    )}
                  >
                    <Checkbox
                      checked={isSelected}
                      onCheckedChange={() => toggleMessageSelection(message.id)}
                      className="mt-2"
                    />
                  </div>
                  <div
                    className={cn(
                      'max-w-[80%] rounded-lg px-3 py-2',
                      isOwn
                        ? 'bg-primary text-primary-foreground'
                        : 'bg-muted',
                      isSelected && 'ring-2 ring-primary ring-offset-2'
                    )}
                  >
                    <MessageContent
                      content={message.content}
                      className="text-sm"
                      isOwnMessage={isOwn}
                    />
                    <div
                      className={cn(
                        'flex items-center justify-between gap-2 mt-1 text-xs',
                        isOwn ? 'text-primary-foreground/70' : 'text-muted-foreground'
                      )}
                    >
                      <span>{formatTimestamp(message.createdAt)}</span>
                      <button
                        onClick={() => {
                          setSelectedMessage(message)
                          setMessageInfoOpen(true)
                        }}
                        className={cn(
                          'font-mono opacity-60 hover:opacity-100 transition-opacity',
                          isOwn ? 'hover:text-primary-foreground' : 'hover:text-foreground'
                        )}
                        title={t('Message info')}
                      >
                        {message.encryptionType === 'nip17' ? '44' : '4'}
                      </button>
                    </div>
                  </div>
                </div>
              )
            })}
          </div>
        )}
        </ScrollArea>

        {/* Jump to newest button */}
        {showJumpButton && (
          <Button
            onClick={scrollToBottom}
            className="absolute bottom-4 right-4 rounded-full shadow-lg size-10 p-0"
            size="icon"
          >
            <ChevronDown className="size-5" />
            {newMessageCount > 0 && (
              <span className="absolute -top-1 -right-1 bg-destructive text-destructive-foreground rounded-full min-w-5 h-5 flex items-center justify-center text-xs font-medium px-1">
                {newMessageCount > 99 ? '99+' : newMessageCount}
              </span>
            )}
          </Button>
        )}
      </div>

      {/* Composer */}
      <div className="border-t">
        <MessageComposer />
      </div>

      {/* Message Info Modal */}
      <MessageInfoModal
        message={selectedMessage}
        open={messageInfoOpen}
        onOpenChange={setMessageInfoOpen}
      />

      {/* Conversation Settings Modal */}
      <ConversationSettingsModal
        partnerPubkey={currentConversation}
        open={settingsOpen}
        onOpenChange={setSettingsOpen}
        selectedRelays={selectedRelays}
        onSelectedRelaysChange={handleRelaysChange}
      />
    </div>
  )
}
