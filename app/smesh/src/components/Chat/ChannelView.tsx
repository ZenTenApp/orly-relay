import { useChat } from '@/providers/ChatProvider'
import { useNostr } from '@/providers/NostrProvider'
import { useFetchProfile } from '@/hooks/useFetchProfile'
import { cn, isTouchDevice } from '@/lib/utils'
import { Pubkey } from '@/domain'
import {
  Hash,
  Loader2,
  Send,
  ChevronUp,
  Bell,
  BellOff,
  Shield,
  EyeOff,
  Ban,
  Lock
} from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import { Button } from '../ui/button'
import { Textarea } from '../ui/textarea'
import dayjs from 'dayjs'
import ChannelSettingsPanel from './ChannelSettingsPanel'

export default function ChannelView() {
  const {
    currentChannel,
    messages,
    isLoadingMessages,
    sendMessage,
    loadMoreMessages,
    mutedChannels,
    toggleMuteChannel,
    isOwnerOrMod
  } = useChat()
  const { pubkey } = useNostr()
  const [input, setInput] = useState('')
  const [isSending, setIsSending] = useState(false)
  const [isLoadingMore, setIsLoadingMore] = useState(false)
  const [showSettings, setShowSettings] = useState(false)
  const scrollRef = useRef<HTMLDivElement>(null)
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const shouldAutoScroll = useRef(true)

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    if (shouldAutoScroll.current && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [messages])

  // Scroll to bottom when channel changes
  useEffect(() => {
    shouldAutoScroll.current = true
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
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
    if (!input.trim() || isSending || !pubkey) return
    setIsSending(true)
    try {
      await sendMessage(input.trim())
      setInput('')
      shouldAutoScroll.current = true
      if (textareaRef.current) {
        textareaRef.current.style.height = 'auto'
        textareaRef.current.focus()
      }
    } finally {
      setIsSending(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault()
      handleSend()
    }
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

  const isMuted = mutedChannels.has(currentChannel.id)

  return (
    <div className="flex flex-col h-full">
      {/* Channel header */}
      <div className="flex items-center gap-2 px-3 py-2 border-b">
        <Hash className="size-4 text-muted-foreground" />
        <span className="font-semibold text-sm">{currentChannel.name}</span>
        {currentChannel.inviteOnly && (
          <span title="Invite only">
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
          onClick={() => toggleMuteChannel(currentChannel.id)}
          title={isMuted ? 'Unmute notifications' : 'Mute notifications'}
        >
          {isMuted ? (
            <BellOff className="size-3.5 text-muted-foreground" />
          ) : (
            <Bell className="size-3.5" />
          )}
        </Button>
        {isOwnerOrMod && (
          <Button
            variant="ghost"
            size="icon"
            className="size-7"
            onClick={() => setShowSettings(!showSettings)}
            title="Channel settings"
          >
            <Shield className="size-3.5" />
          </Button>
        )}
      </div>

      {/* Settings panel */}
      {showSettings && isOwnerOrMod && (
        <ChannelSettingsPanel onClose={() => setShowSettings(false)} />
      )}

      {/* Messages */}
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        className="flex-1 overflow-y-auto px-3 py-2 space-y-1"
      >
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
            />
          ))
        )}
      </div>

      {/* Composer */}
      {pubkey && (
        <div className="border-t p-2 flex items-end gap-2">
          <Textarea
            ref={textareaRef}
            value={input}
            onChange={(e) => {
              setInput(e.target.value)
              resizeTextarea()
            }}
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
    </div>
  )
}

function ChatMessage({
  msg,
  isOwn,
  showModActions
}: {
  msg: { id: string; pubkey: string; content: string; createdAt: number }
  isOwn: boolean
  showModActions: boolean
}) {
  const { hideMessage, blockUser } = useChat()
  const { profile } = useFetchProfile(msg.pubkey)
  const pk = Pubkey.tryFromString(msg.pubkey)
  const displayName = profile?.username || pk?.formatNpub(8) || msg.pubkey.slice(0, 12)
  const time = dayjs.unix(msg.createdAt).format('HH:mm')

  return (
    <div className={cn('group flex gap-2 py-0.5 text-sm', isOwn && 'flex-row-reverse')}>
      <div className={cn('max-w-[80%]', isOwn ? 'text-right' : 'text-left')}>
        <div className={cn('flex items-baseline gap-2', isOwn && 'justify-end')}>
          {!isOwn && (
            <span className="text-xs font-medium text-primary">{displayName}</span>
          )}
          <span className="text-[10px] text-muted-foreground opacity-0 group-hover:opacity-100 transition-opacity">
            {time}
          </span>
          {showModActions && (
            <span className="opacity-0 group-hover:opacity-100 transition-opacity flex gap-0.5">
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
        <div className="break-words whitespace-pre-wrap">{msg.content}</div>
      </div>
    </div>
  )
}
