import { useChat } from '@/providers/ChatProvider'
import { useNostr } from '@/providers/NostrProvider'
import { useFetchProfile } from '@/hooks/useFetchProfile'
import { isTouchDevice } from '@/lib/utils'
import { Pubkey } from '@/domain'
import {
  Hash,
  Loader2,
  Send,
  ChevronUp,
  Shield,
  EyeOff,
  Ban,
  Lock,
  Settings2,
  Users,
  LogIn
} from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { Button } from '../ui/button'
import { Textarea } from '../ui/textarea'
import dayjs from 'dayjs'
import ChannelSettingsPanel from './ChannelSettingsPanel'
import ChatContent from './ChatContent'
import UserProfileModal from './UserProfileModal'
import MentionPopup from './MentionPopup'
import MemberListPanel from './MemberListPanel'

type TSubmitKey = 'enter' | 'ctrl+enter'

function loadSubmitKey(): TSubmitKey {
  const v = localStorage.getItem('nirc:submitKey')
  return v === 'enter' ? 'enter' : 'ctrl+enter'
}

export default function ChannelView() {
  const {
    currentChannel,
    messages,
    isLoadingMessages,
    sendMessage,
    loadMoreMessages,
    isOwnerOrMod,
    isMember,
    channelAccessMode,
    channelParticipants
  } = useChat()
  const { pubkey } = useNostr()
  const [input, setInput] = useState('')
  const [isSending, setIsSending] = useState(false)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [showSettings, setShowSettings] = useState(false)
  const [showMemberList, setShowMemberList] = useState(false)
  const [profilePubkey, setProfilePubkey] = useState<string | null>(null)
  const scrollRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const shouldAutoScroll = useRef(true)
  const composerRef = useRef<HTMLDivElement>(null)

  // @ mention state
  const [mentionQuery, setMentionQuery] = useState<string | null>(null)
  const [mentionStart, setMentionStart] = useState(0)

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    if (shouldAutoScroll.current && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [messages])

  // Scroll to bottom and restore draft when channel changes
  useEffect(() => {
    shouldAutoScroll.current = true
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
    if (currentChannel) {
      const draft = localStorage.getItem(`nirc:draft:${currentChannel.id}`) || ''
      setInput(draft)
    } else {
      setInput('')
    }
    setShowSettings(false)
    setShowMemberList(false)
    setMentionQuery(null)
  }, [currentChannel?.id])

  // Focus input on channel select (desktop only)
  useEffect(() => {
    if (currentChannel && !isTouchDevice()) {
      setTimeout(() => textareaRef.current?.focus(), 100)
    }
  }, [currentChannel?.id])

  const handleScroll = useCallback(() => {
    if (!scrollRef.current) return
    const { scrollTop, scrollHeight, clientHeight } = scrollRef.current
    shouldAutoScroll.current = scrollHeight - scrollTop - clientHeight < 100
  }, [])

  const handleLoadMore = async () => {
    setIsLoadingMore(true)
    try {
      await loadMoreMessages()
    } finally {
      setIsLoadingMore(false)
    }
  }

  const handleSend = async () => {
    if (!input.trim() || isSending || !pubkey || !currentChannel) return
    setIsSending(true)
    try {
      await sendMessage(input.trim())
      setInput('')
      localStorage.removeItem(`nirc:draft:${currentChannel.id}`)
      shouldAutoScroll.current = true
      if (textareaRef.current) {
        textareaRef.current.style.height = 'auto'
      }
    } finally {
      setIsSending(false)
      setTimeout(() => textareaRef.current?.focus(), 0)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    // Close mention popup on Escape
    if (e.key === 'Escape' && mentionQuery !== null) {
      setMentionQuery(null)
      return
    }
    if (e.key === 'Enter') {
      const sk = loadSubmitKey()
      if (sk === 'enter' && !e.shiftKey && !e.ctrlKey && !e.metaKey) {
        e.preventDefault()
        handleSend()
      } else if (sk === 'ctrl+enter' && (e.ctrlKey || e.metaKey)) {
        e.preventDefault()
        handleSend()
      }
    }
  }

  const handleInputChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value
    setInput(val)
    if (currentChannel) {
      localStorage.setItem(`nirc:draft:${currentChannel.id}`, val)
    }
    resizeTextarea()

    // Detect @ mention
    const cursorPos = e.target.selectionStart || 0
    const textBefore = val.slice(0, cursorPos)
    const atMatch = textBefore.match(/@(\w*)$/)
    if (atMatch) {
      setMentionQuery(atMatch[1].toLowerCase())
      setMentionStart(cursorPos - atMatch[0].length)
    } else {
      setMentionQuery(null)
    }
  }

  const handleMentionSelect = (selectedPubkey: string, _displayName: string) => {
    const npub = Pubkey.tryFromString(selectedPubkey)?.npub || selectedPubkey
    const before = input.slice(0, mentionStart)
    const after = input.slice(textareaRef.current?.selectionStart || input.length)
    const newInput = `${before}nostr:${npub} ${after}`
    setInput(newInput)
    setMentionQuery(null)
    if (currentChannel) {
      localStorage.setItem(`nirc:draft:${currentChannel.id}`, newInput)
    }
    setTimeout(() => textareaRef.current?.focus(), 0)
  }

  const resizeTextarea = useCallback(() => {
    const ta = textareaRef.current
    if (!ta) return
    ta.style.height = 'auto'
    const max = window.innerHeight * 0.3
    ta.style.height = `${Math.min(ta.scrollHeight, max)}px`
    ta.style.overflowY = ta.scrollHeight > max ? 'auto' : 'hidden'
  }, [])

  if (!currentChannel) {
    return (
      <div className="flex flex-col items-center justify-center h-full text-muted-foreground gap-2">
        <Hash className="size-10" />
        <span className="text-sm">Select a channel</span>
      </div>
    )
  }

  const isRestricted = channelAccessMode !== 'open' && !isMember

  return (
    <div className="flex flex-col h-full relative">
      {/* Channel header */}
      <div className="flex items-center gap-2 px-3 py-2 border-b">
        <button
          className="flex items-center gap-0.5 hover:text-primary"
          onClick={() => setShowMemberList(!showMemberList)}
          title="Member list"
        >
          <Hash className="size-4 text-muted-foreground" />
        </button>
        <span className="font-semibold text-sm">{currentChannel.name}</span>
        {channelAccessMode !== 'open' && (
          <span title={channelAccessMode === 'whitelist' ? 'Whitelist' : 'Blacklist'}>
            <Lock className="size-3 text-muted-foreground" />
          </span>
        )}
        {currentChannel.about && (
          <span className="text-xs text-muted-foreground truncate">
            — {currentChannel.about}
          </span>
        )}
        <div className="flex-1" />
        <Button
          variant="ghost"
          size="icon"
          className="size-7"
          onClick={() => setShowMemberList(!showMemberList)}
          title="Member list"
        >
          <Users className="size-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="size-7"
          onClick={() => setShowSettings(!showSettings)}
          title="Channel settings"
        >
          {isOwnerOrMod ? <Shield className="size-3.5" /> : <Settings2 className="size-3.5" />}
        </Button>
      </div>

      {/* Settings overlay */}
      {showSettings && (
        <ChannelSettingsPanel onClose={() => setShowSettings(false)} />
      )}

      {/* Member list panel */}
      {showMemberList && !showSettings && (
        <MemberListPanel onClose={() => setShowMemberList(false)} />
      )}

      {/* Messages */}
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto px-3 py-2 space-y-1"
      >
        {isRestricted ? (
          <div className="flex flex-col items-center justify-center h-full gap-3 text-muted-foreground">
            <Lock className="size-8" />
            <span className="text-sm">This channel requires access</span>
            <RequestAccessButton />
          </div>
        ) : (
          <>
            {messages.length > 0 && (
              <div className="flex justify-center py-2">
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleLoadMore}
                  disabled={isLoadingMore}
                  className="text-xs"
                >
                  {isLoadingMore ? (
                    <Loader2 className="size-3 animate-spin mr-1" />
                  ) : (
                    <ChevronUp className="size-3 mr-1" />
                  )}
                  Load older
                </Button>
              </div>
            )}

            {isLoadingMessages ? (
              <div className="flex justify-center py-8">
                <Loader2 className="size-5 animate-spin text-muted-foreground" />
              </div>
            ) : messages.length === 0 ? (
              <div className="text-center py-8 text-sm text-muted-foreground">
                No messages yet. Say something.
              </div>
            ) : (
              messages.map((msg) => (
                <ChatMessage
                  key={msg.id}
                  msg={msg}
                  isOwn={msg.pubkey === pubkey}
                  showModActions={isOwnerOrMod && msg.pubkey !== pubkey}
                  onUsernameClick={setProfilePubkey}
                />
              ))
            )}
          </>
        )}
      </div>

      {/* Composer */}
      {pubkey && isMember && !isRestricted && (
        <div ref={composerRef} className="border-t p-2 flex items-end gap-2 relative">
          {/* Mention popup */}
          {mentionQuery !== null && (
            <MentionPopup
              participants={channelParticipants}
              query={mentionQuery}
              onSelect={handleMentionSelect}
              position={{ bottom: composerRef.current?.clientHeight || 48, left: 8 }}
            />
          )}
          <Textarea
            ref={textareaRef}
            value={input}
            onChange={handleInputChange}
            onKeyDown={handleKeyDown}
            placeholder={`Message #${currentChannel.name}`}
            className="min-h-[36px] resize-none overflow-hidden text-sm"
            disabled={isSending}
          />
          <Button
            onClick={handleSend}
            disabled={!input.trim() || isSending}
            size="icon"
            className="flex-shrink-0 size-9"
          >
            {isSending ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <Send className="size-4" />
            )}
          </Button>
        </div>
      )}

      {/* User profile modal */}
      {profilePubkey && (
        <UserProfileModal
          pubkeyHex={profilePubkey}
          onClose={() => setProfilePubkey(null)}
        />
      )}
    </div>
  )
}

function RequestAccessButton() {
  const { currentChannel } = useChat()
  const { pubkey, signEvent } = useNostr()
  const [sent, setSent] = useState(false)
  const [sending, setSending] = useState(false)

  const handleRequest = async () => {
    if (!currentChannel || !pubkey || sent) return
    setSending(true)
    try {
      // Send a DM to channel creator requesting access
      const { default: clientService } = await import('@/services/client.service')
      const content = `nirc:request:${currentChannel.id}:${currentChannel.name}`
      const draft = {
        kind: 4,
        created_at: Math.floor(Date.now() / 1000),
        tags: [['p', currentChannel.creator]],
        content
      }
      const signed = await signEvent(draft)
      await clientService.publishEvent(['wss://relay.orly.dev/'], signed)
      setSent(true)
    } catch {
      // ignore
    } finally {
      setSending(false)
    }
  }

  if (!pubkey) return null

  return (
    <Button
      variant="outline"
      size="sm"
      className="gap-1.5"
      onClick={handleRequest}
      disabled={sent || sending}
    >
      {sending ? (
        <Loader2 className="size-3 animate-spin" />
      ) : (
        <LogIn className="size-3" />
      )}
      {sent ? 'Request Sent' : 'Request Access'}
    </Button>
  )
}

function ChatMessage({
  msg,
  isOwn,
  showModActions,
  onUsernameClick
}: {
  msg: { id: string; pubkey: string; content: string; createdAt: number; event?: import('nostr-tools').Event }
  isOwn: boolean
  showModActions: boolean
  onUsernameClick: (pubkey: string) => void
}) {
  const { hideMessage, blockUser } = useChat()
  const { profile } = useFetchProfile(msg.pubkey)
  const pk = Pubkey.tryFromString(msg.pubkey)
  const displayName = profile?.username || pk?.formatNpub(8) || msg.pubkey.slice(0, 12)
  const time = dayjs.unix(msg.createdAt).format('HH:mm')
  const isAction = msg.content.startsWith('/me ')
  const actionText = isAction ? msg.content.slice(4) : null

  return (
    <div className="group flex items-start gap-0 py-0.5 text-sm font-mono leading-snug">
      <span className="text-[11px] text-muted-foreground shrink-0">[{time}]&nbsp;</span>
      {isAction ? (
        <span className="italic break-words min-w-0">
          <span className="text-muted-foreground">*</span>{' '}
          <button
            className={`${isOwn ? 'text-muted-foreground' : 'text-primary'} hover:underline cursor-pointer`}
            onClick={() => onUsernameClick(msg.pubkey)}
          >
            {displayName}
          </button>{' '}
          <ChatContent content={actionText!} event={msg.event} />
        </span>
      ) : (
        <>
          <span className="shrink-0">
            <span className="text-muted-foreground">&lt;</span>
            <button
              className={`${isOwn ? 'text-muted-foreground' : 'text-primary font-medium'} hover:underline cursor-pointer`}
              onClick={() => onUsernameClick(msg.pubkey)}
            >
              {displayName}
            </button>
            <span className="text-muted-foreground">&gt;</span>
          </span>
          <span className="min-w-0">&nbsp;<ChatContent content={msg.content} event={msg.event} /></span>
        </>
      )}
      {showModActions && (
        <span className="opacity-0 group-hover:opacity-100 transition-opacity flex gap-0.5 ml-1 shrink-0">
          <button
            onClick={() => hideMessage(msg.id)}
            className="text-muted-foreground hover:text-destructive p-0.5"
            title="Hide message"
          >
            <EyeOff className="size-3" />
          </button>
          <button
            onClick={() => blockUser(msg.pubkey)}
            className="text-muted-foreground hover:text-destructive p-0.5"
            title="Block user"
          >
            <Ban className="size-3" />
          </button>
        </span>
      )}
    </div>
  )
}
