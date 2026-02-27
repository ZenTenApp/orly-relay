import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { parseEmojiPickerUnified } from '@/lib/utils'
import { TEmoji } from '@/types'
import { getSuggested } from 'emoji-picker-react/src/dataUtils/suggested'
import { MoreHorizontal } from 'lucide-react'
import { useCallback, useEffect, useRef, useState } from 'react'
import Emoji from '../Emoji'

const DEFAULT_SUGGESTED_EMOJIS = ['👍', '❤️', '😂', '🥲', '👀', '🫡', '🫂']

export default function SuggestedEmojis({
  onEmojiClick,
  onMoreButtonClick,
  onClose
}: {
  onEmojiClick: (emoji: string | TEmoji) => void
  onMoreButtonClick: () => void
  onClose?: () => void
}) {
  const [suggestedEmojis, setSuggestedEmojis] =
    useState<(string | TEmoji)[]>(DEFAULT_SUGGESTED_EMOJIS)
  const [selectedIndex, setSelectedIndex] = useState(0)
  const containerRef = useRef<HTMLDivElement>(null)

  // Total items: 1 (plus) + suggestedEmojis.length + 1 (more button)
  const totalItems = 1 + suggestedEmojis.length + 1

  useEffect(() => {
    try {
      const suggested = getSuggested()
      const emojiSet = new Set<string>()
      const suggestEmojis = (
        suggested
          .sort((a, b) => b.count - a.count)
          .map((item) => parseEmojiPickerUnified(item.unified))
          .filter(Boolean) as (string | TEmoji)[]
      )
        .concat(DEFAULT_SUGGESTED_EMOJIS)
        .filter((emoji) => {
          if (typeof emoji !== 'string') return true
          if (emojiSet.has(emoji)) return false
          emojiSet.add(emoji)
          return true
        })
      setSuggestedEmojis(suggestEmojis.slice(0, 9))
    } catch {
      // ignore
    }
  }, [])

  // Focus container on mount for keyboard events
  useEffect(() => {
    containerRef.current?.focus()
  }, [])

  const handleSelect = useCallback(() => {
    if (selectedIndex === 0) {
      // Plus button
      onEmojiClick('+')
    } else if (selectedIndex <= suggestedEmojis.length) {
      // Emoji
      onEmojiClick(suggestedEmojis[selectedIndex - 1])
    } else {
      // More button
      onMoreButtonClick()
    }
  }, [selectedIndex, suggestedEmojis, onEmojiClick, onMoreButtonClick])

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      switch (e.key) {
        case 'ArrowLeft':
          e.preventDefault()
          setSelectedIndex((prev) => (prev > 0 ? prev - 1 : totalItems - 1))
          break
        case 'ArrowRight':
          e.preventDefault()
          setSelectedIndex((prev) => (prev < totalItems - 1 ? prev + 1 : 0))
          break
        case 'ArrowUp':
          e.preventDefault()
          // Jump to first item
          setSelectedIndex(0)
          break
        case 'ArrowDown':
          e.preventDefault()
          // Jump to last item (more button)
          setSelectedIndex(totalItems - 1)
          break
        case 'Enter':
        case ' ':
          e.preventDefault()
          handleSelect()
          break
        case 'Escape':
          e.preventDefault()
          onClose?.()
          break
      }
    },
    [totalItems, handleSelect, onClose]
  )

  return (
    <div
      ref={containerRef}
      className="flex gap-1 p-1 outline-none"
      onClick={(e) => e.stopPropagation()}
      onKeyDown={handleKeyDown}
      tabIndex={0}
    >
      <div
        className={cn(
          'w-8 h-8 rounded-lg clickable flex justify-center items-center text-xl',
          selectedIndex === 0 && 'ring-2 ring-primary'
        )}
        onClick={() => onEmojiClick('+')}
      >
        <Emoji emoji="+" />
      </div>
      {suggestedEmojis.map((emoji, index) =>
        typeof emoji === 'string' ? (
          <div
            key={index}
            className={cn(
              'w-8 h-8 rounded-lg clickable flex justify-center items-center text-xl',
              selectedIndex === index + 1 && 'ring-2 ring-primary'
            )}
            onClick={() => onEmojiClick(emoji)}
          >
            {emoji}
          </div>
        ) : (
          <div
            className={cn(
              'flex flex-col items-center justify-center p-1 rounded-lg clickable',
              selectedIndex === index + 1 && 'ring-2 ring-primary'
            )}
            key={index}
            onClick={() => onEmojiClick(emoji)}
          >
            <Emoji emoji={emoji} classNames={{ img: 'size-6 rounded-md' }} />
          </div>
        )
      )}
      <Button
        variant="ghost"
        className={cn(
          'w-8 h-8 text-muted-foreground',
          selectedIndex === totalItems - 1 && 'ring-2 ring-primary'
        )}
        onClick={onMoreButtonClick}
      >
        <MoreHorizontal size={24} />
      </Button>
    </div>
  )
}
