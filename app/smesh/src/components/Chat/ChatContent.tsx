import {
  EmbeddedEventParser,
  EmbeddedMentionParser,
  EmbeddedUrlParser,
  EmbeddedHashtagParser,
  EmbeddedEmojiParser,
  parseContent
} from '@/lib/content-parser'
import { EmbeddedMention, EmbeddedHashtag } from '../Embedded'
import { SecondaryPageLink } from '@/PageManager'
import { toNote } from '@/lib/link'
import { truncateUrl } from '@/lib/url'
import { getEmojiInfosFromEmojiTags } from '@/lib/tag'
import Emoji from '../Emoji'
import { useMemo } from 'react'
import { Event } from 'nostr-tools'

/**
 * Lightweight inline content renderer for NIRC chat messages.
 * Reuses the same parseContent pipeline as the full Content component
 * but renders everything inline to fit the IRC-style monospace layout.
 */
export default function ChatContent({ content, event }: { content: string; event?: Event }) {
  const { nodes, emojiInfos } = useMemo(() => {
    if (!content) return { nodes: [], emojiInfos: [] }
    const nodes = parseContent(content, [
      EmbeddedEventParser,
      EmbeddedMentionParser,
      EmbeddedUrlParser,
      EmbeddedHashtagParser,
      EmbeddedEmojiParser
    ])
    const emojiInfos = getEmojiInfosFromEmojiTags(event?.tags)
    return { nodes, emojiInfos }
  }, [content, event])

  if (!nodes || nodes.length === 0) return null

  return (
    <span className="break-words whitespace-pre-wrap min-w-0">
      {nodes.map((node, i) => {
        if (node.type === 'text') {
          return <span key={i}>{node.data}</span>
        }
        if (node.type === 'mention') {
          const userId = (node.data as string).split(':')[1]
          if (!userId) return <span key={i}>{node.data as string}</span>
          return <EmbeddedMention key={i} userId={userId} className="inline" />
        }
        if (node.type === 'url') {
          return (
            <a
              key={i}
              href={node.data as string}
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary hover:underline"
              onClick={(e) => e.stopPropagation()}
            >
              {truncateUrl(node.data as string)}
            </a>
          )
        }
        if (node.type === 'image' || node.type === 'images') {
          const url = Array.isArray(node.data) ? node.data[0] : node.data
          return (
            <a
              key={i}
              href={url}
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary hover:underline"
              onClick={(e) => e.stopPropagation()}
            >
              [image]
            </a>
          )
        }
        if (node.type === 'media') {
          return (
            <a
              key={i}
              href={node.data as string}
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary hover:underline"
              onClick={(e) => e.stopPropagation()}
            >
              [media]
            </a>
          )
        }
        if (node.type === 'event') {
          const id = (node.data as string).split(':')[1]
          if (!id) return <span key={i}>{node.data as string}</span>
          return (
            <SecondaryPageLink
              key={i}
              to={toNote(id)}
              className="text-primary hover:underline"
              onClick={(e) => e.stopPropagation()}
            >
              [note]
            </SecondaryPageLink>
          )
        }
        if (node.type === 'hashtag') {
          return <EmbeddedHashtag key={i} hashtag={node.data as string} />
        }
        if (node.type === 'emoji') {
          const shortcode = (node.data as string).split(':')[1]
          const emoji = emojiInfos.find((e) => e.shortcode === shortcode)
          if (!emoji) return <span key={i}>{node.data as string}</span>
          return <Emoji key={i} emoji={emoji} classNames={{ img: 'size-4 inline' }} />
        }
        if (node.type === 'youtube' || node.type === 'x-post') {
          return (
            <a
              key={i}
              href={node.data as string}
              target="_blank"
              rel="noopener noreferrer"
              className="text-primary hover:underline"
              onClick={(e) => e.stopPropagation()}
            >
              {truncateUrl(node.data as string)}
            </a>
          )
        }
        if (node.type === 'invoice') {
          return (
            <span key={i} className="text-primary">
              [ln-invoice]
            </span>
          )
        }
        return null
      })}
    </span>
  )
}
