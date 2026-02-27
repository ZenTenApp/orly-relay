import {
  EmbeddedEmojiParser,
  EmbeddedHashtagParser,
  EmbeddedMentionParser,
  EmbeddedUrlParser,
  EmbeddedWebsocketUrlParser,
  parseContent
} from '@/lib/content-parser'
import { TEmoji } from '@/types'
import { useMemo } from 'react'
import { EmbeddedHashtag, EmbeddedMention, EmbeddedWebsocketUrl } from '../Embedded'
import Emoji from '../Emoji'
import ExternalLink from '../ExternalLink'

export default function ProfileAbout({
  about,
  emojis,
  className
}: {
  about?: string
  emojis?: TEmoji[]
  className?: string
}) {
  const aboutNodes = useMemo(() => {
    if (!about) return null

    const nodes = parseContent(about, [
      EmbeddedMentionParser,
      EmbeddedWebsocketUrlParser,
      EmbeddedUrlParser,
      EmbeddedHashtagParser,
      EmbeddedEmojiParser
    ])

    // Create emoji map for quick lookup
    const emojiMap = new Map<string, TEmoji>()
    emojis?.forEach((emoji) => {
      emojiMap.set(emoji.shortcode, emoji)
    })

    return nodes.map((node, index) => {
      if (node.type === 'url') {
        return <ExternalLink key={index} url={node.data} />
      }
      if (node.type === 'websocket-url') {
        return <EmbeddedWebsocketUrl key={index} url={node.data} />
      }
      if (node.type === 'hashtag') {
        return <EmbeddedHashtag key={index} hashtag={node.data} />
      }
      if (node.type === 'mention') {
        return <EmbeddedMention key={index} userId={node.data.split(':')[1]} />
      }
      if (node.type === 'emoji') {
        const shortcode = node.data.split(':')[1]
        const emoji = emojiMap.get(shortcode)
        if (!emoji) return node.data
        return <Emoji classNames={{ img: 'mb-1' }} emoji={emoji} key={index} />
      }
      return node.data
    })
  }, [about, emojis])

  return <div className={className}>{aboutNodes}</div>
}
