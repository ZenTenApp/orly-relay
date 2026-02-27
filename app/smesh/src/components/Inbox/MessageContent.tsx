import { useSecondaryPage } from '@/PageManager'
import {
  EmbeddedEventParser,
  EmbeddedMentionParser,
  EmbeddedUrlParser,
  parseContent
} from '@/lib/content-parser'
import { toNote, toProfile } from '@/lib/link'
import { truncateUrl } from '@/lib/url'
import { cn } from '@/lib/utils'
import { useMemo } from 'react'

interface MessageContentProps {
  content: string
  className?: string
  /** If true, links will be styled for dark background (primary-foreground color) */
  isOwnMessage?: boolean
}

/**
 * Renders DM message content with linkified URLs and nostr entities.
 * - URLs open in new tab
 * - nostr:npub/nprofile opens user profile in secondary pane
 * - nostr:note1/nevent opens note in secondary pane
 */
export default function MessageContent({ content, className, isOwnMessage }: MessageContentProps) {
  const { push } = useSecondaryPage()

  const nodes = useMemo(() => {
    return parseContent(content, [EmbeddedEventParser, EmbeddedMentionParser, EmbeddedUrlParser])
  }, [content])

  const linkClass = cn(
    'underline cursor-pointer hover:opacity-80',
    isOwnMessage ? 'text-primary-foreground' : 'text-primary'
  )

  return (
    <span className={cn('whitespace-pre-wrap break-words', className)}>
      {nodes.map((node, index) => {
        if (node.type === 'text') {
          return node.data
        }

        // URLs - open in new tab
        if (node.type === 'url' || node.type === 'image' || node.type === 'media') {
          const url = node.data as string
          return (
            <a
              key={index}
              href={url}
              target="_blank"
              rel="noreferrer"
              className={linkClass}
              onClick={(e) => e.stopPropagation()}
            >
              {truncateUrl(url)}
            </a>
          )
        }

        // YouTube and X posts - open in new tab
        if (node.type === 'youtube' || node.type === 'x-post') {
          const url = node.data as string
          return (
            <a
              key={index}
              href={url}
              target="_blank"
              rel="noreferrer"
              className={linkClass}
              onClick={(e) => e.stopPropagation()}
            >
              {truncateUrl(url)}
            </a>
          )
        }

        // nostr: mention (npub/nprofile) - open profile in secondary pane
        if (node.type === 'mention') {
          const bech32 = (node.data as string).replace('nostr:', '')
          return (
            <button
              key={index}
              className={linkClass}
              onClick={(e) => {
                e.stopPropagation()
                push(toProfile(bech32))
              }}
            >
              @{bech32.slice(0, 12)}...
            </button>
          )
        }

        // nostr: event (note1/nevent/naddr) - open note in secondary pane
        if (node.type === 'event') {
          const bech32 = (node.data as string).replace('nostr:', '')
          // Determine display based on prefix
          const isNote = bech32.startsWith('note1')
          const prefix = isNote ? 'note' : bech32.startsWith('nevent') ? 'nevent' : 'naddr'
          return (
            <button
              key={index}
              className={linkClass}
              onClick={(e) => {
                e.stopPropagation()
                push(toNote(bech32))
              }}
            >
              {prefix}:{bech32.slice(prefix.length, prefix.length + 8)}...
            </button>
          )
        }

        return null
      })}
    </span>
  )
}
