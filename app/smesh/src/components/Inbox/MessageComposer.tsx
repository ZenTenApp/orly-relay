import { cn, isTouchDevice } from '@/lib/utils'
import { useDM } from '@/providers/DMProvider'
import { useNostr } from '@/providers/NostrProvider'
import { AlertCircle, ChevronDown, ChevronUp, Loader2, Send } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '../ui/button'
import { Textarea } from '../ui/textarea'

export default function MessageComposer() {
  const { t } = useTranslation()
  const { sendMessage, currentConversation } = useDM()
  const { relayList } = useNostr()
  const [message, setMessage] = useState('')
  const [isSending, setIsSending] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [showRelays, setShowRelays] = useState(false)
  const [selectedRelays, setSelectedRelays] = useState<Set<string>>(new Set())
  const textareaRef = useRef<HTMLTextAreaElement>(null)

  // Get user's write relays
  const writeRelays = useMemo(() => relayList?.write || [], [relayList])

  // Initialize selected relays when write relays change
  useEffect(() => {
    if (writeRelays.length > 0 && selectedRelays.size === 0) {
      setSelectedRelays(new Set(writeRelays))
    }
  }, [writeRelays])

  // Auto-focus input when conversation changes (desktop only to avoid triggering mobile keyboard)
  useEffect(() => {
    if (currentConversation && !isTouchDevice()) {
      const timer = setTimeout(() => textareaRef.current?.focus(), 100)
      return () => clearTimeout(timer)
    }
  }, [currentConversation])

  // Auto-resize textarea to fit content, capped at 50% viewport height
  const resizeTextarea = useCallback(() => {
    const textarea = textareaRef.current
    if (!textarea) return
    textarea.style.height = 'auto'
    const maxHeight = window.innerHeight * 0.5
    textarea.style.height = `${Math.min(textarea.scrollHeight, maxHeight)}px`
    textarea.style.overflowY = textarea.scrollHeight > maxHeight ? 'auto' : 'hidden'
  }, [])

  const toggleRelay = (url: string) => {
    setSelectedRelays((prev) => {
      const next = new Set(prev)
      if (next.has(url)) {
        // Don't allow deselecting all relays
        if (next.size > 1) {
          next.delete(url)
        }
      } else {
        next.add(url)
      }
      return next
    })
  }

  const handleSend = async () => {
    if (!message.trim() || !currentConversation || isSending) return

    setIsSending(true)
    setError(null)
    try {
      const relaysToUse = Array.from(selectedRelays)
      await sendMessage(message.trim(), relaysToUse.length > 0 ? relaysToUse : undefined)
      setMessage('')
      // Reset textarea height and return focus after sending
      if (textareaRef.current) {
        textareaRef.current.style.height = 'auto'
        textareaRef.current.style.overflowY = 'hidden'
        textareaRef.current.focus()
      }
    } catch (err) {
      console.error('Failed to send message:', err)
      setError(err instanceof Error ? err.message : t('Failed to send message'))
    } finally {
      setIsSending(false)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault()
      handleSend()
    }
  }

  // Format relay URL for display
  const formatRelayUrl = (url: string) => {
    return url.replace(/^wss?:\/\//, '').replace(/\/$/, '')
  }

  return (
    <div className="p-3 space-y-2">
      {error && (
        <div className="flex items-center gap-2 text-destructive text-xs">
          <AlertCircle className="size-3 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Relay selector */}
      {writeRelays.length > 0 && (
        <div className="space-y-1">
          <button
            onClick={() => setShowRelays(!showRelays)}
            className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition-colors"
          >
            {showRelays ? <ChevronUp className="size-3" /> : <ChevronDown className="size-3" />}
            <span>
              {t('Relays')} ({selectedRelays.size}/{writeRelays.length})
            </span>
          </button>
          {showRelays && (
            <div className="flex flex-wrap gap-1">
              {writeRelays.map((url) => (
                <button
                  key={url}
                  onClick={() => toggleRelay(url)}
                  className={cn(
                    'text-xs px-2 py-0.5 rounded-full border transition-colors',
                    selectedRelays.has(url)
                      ? 'bg-primary text-primary-foreground border-primary'
                      : 'bg-muted text-muted-foreground border-muted hover:border-primary/50'
                  )}
                  title={url}
                >
                  {formatRelayUrl(url)}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      <div className="flex items-end gap-2">
        <Textarea
          ref={textareaRef}
          value={message}
          onChange={(e) => {
            setMessage(e.target.value)
            if (error) setError(null)
            resizeTextarea()
          }}
          onKeyDown={handleKeyDown}
          placeholder={t('Type a message...')}
          className="min-h-[40px] resize-none overflow-hidden"
          disabled={isSending || !currentConversation}
        />
        <Button
          onClick={handleSend}
          disabled={!message.trim() || isSending || !currentConversation}
          size="icon"
          className="flex-shrink-0"
        >
          {isSending ? (
            <Loader2 className="size-4 animate-spin" />
          ) : (
            <Send className="size-4" />
          )}
        </Button>
      </div>
    </div>
  )
}
