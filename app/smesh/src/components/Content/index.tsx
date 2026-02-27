import {
  EmbeddedEmojiParser,
  EmbeddedEventParser,
  EmbeddedHashtagParser,
  EmbeddedLNInvoiceParser,
  EmbeddedMentionParser,
  EmbeddedUrlParser,
  EmbeddedWebsocketUrlParser,
  parseContent
} from '@/lib/content-parser'
import { getImetaInfosFromEvent } from '@/lib/event'
import { getEmojiInfosFromEmojiTags, getImetaInfoFromImetaTag } from '@/lib/tag'
import { cn } from '@/lib/utils'
import { useContentPolicy } from '@/providers/ContentPolicyProvider'
import mediaUpload from '@/services/media-upload.service'
import { TImetaInfo } from '@/types'
import { Event } from 'nostr-tools'
import { useMemo, useRef, useState } from 'react'
import {
  EmbeddedHashtag,
  EmbeddedLNInvoice,
  EmbeddedMention,
  EmbeddedNote,
  EmbeddedWebsocketUrl
} from '../Embedded'
import Emoji from '../Emoji'
import ExternalLink from '../ExternalLink'
import HighlightButton from '../HighlightButton'
import ResponsiveImageGallery from '../ResponsiveImageGallery'
import MediaPlayer from '../MediaPlayer'
import PostEditor from '../PostEditor'
import WebPreview from '../WebPreview'
import XEmbeddedPost from '../XEmbeddedPost'
import YoutubeEmbeddedPlayer from '../YoutubeEmbeddedPlayer'
import MarkdownText from './MarkdownText'

export default function Content({
  event,
  content,
  className,
  mustLoadMedia,
  enableHighlight = false,
  enableMarkdown: enableMarkdownProp
}: {
  event?: Event
  content?: string
  className?: string
  mustLoadMedia?: boolean
  enableHighlight?: boolean
  enableMarkdown?: boolean
}) {
  const { enableMarkdown: globalEnableMarkdown } = useContentPolicy()
  const markdown = enableMarkdownProp ?? globalEnableMarkdown
  const contentRef = useRef<HTMLDivElement>(null)
  const [showHighlightEditor, setShowHighlightEditor] = useState(false)
  const [selectedText, setSelectedText] = useState('')
  const { nodes, allImages, lastNormalUrl, emojiInfos } = useMemo(() => {
    const _content = event?.content ?? content
    if (!_content) return {}

    const nodes = parseContent(_content, [
      EmbeddedEventParser,
      EmbeddedMentionParser,
      EmbeddedUrlParser,
      EmbeddedLNInvoiceParser,
      EmbeddedWebsocketUrlParser,
      EmbeddedHashtagParser,
      EmbeddedEmojiParser
    ])

    const imetaInfos = event ? getImetaInfosFromEvent(event) : []
    const allImages = nodes
      .map((node) => {
        if (node.type === 'image') {
          const imageInfo = imetaInfos.find((image) => image.url === node.data)
          if (imageInfo) {
            return imageInfo
          }
          const tag = mediaUpload.getImetaTagByUrl(node.data)
          return tag
            ? getImetaInfoFromImetaTag(tag, event?.pubkey)
            : { url: node.data, pubkey: event?.pubkey }
        }
        if (node.type === 'images') {
          const urls = Array.isArray(node.data) ? node.data : [node.data]
          return urls.map((url) => {
            const imageInfo = imetaInfos.find((image) => image.url === url)
            return imageInfo ?? { url, pubkey: event?.pubkey }
          })
        }
        return null
      })
      .filter(Boolean)
      .flat() as TImetaInfo[]

    const emojiInfos = getEmojiInfosFromEmojiTags(event?.tags)

    const lastNormalUrlNode = nodes.findLast((node) => node.type === 'url')
    const lastNormalUrl =
      typeof lastNormalUrlNode?.data === 'string' ? lastNormalUrlNode.data : undefined

    return { nodes, allImages, emojiInfos, lastNormalUrl }
  }, [event, content])

  if (!nodes || nodes.length === 0) {
    return null
  }

  const handleHighlight = (text: string) => {
    setSelectedText(text)
    setShowHighlightEditor(true)
  }

  let imageIndex = 0
  return (
    <>
      <div
        ref={contentRef}
        className={cn('text-wrap break-words prose prose-zinc dark:prose-invert max-w-none prose-p:mt-0 prose-p:mb-2 prose-headings:my-2 prose-ul:my-1 prose-ol:my-1 prose-pre:my-2 prose-blockquote:my-2', className)}
      >
        {nodes.map((node, index) => {
          if (node.type === 'text') {
            if (!markdown) {
              return <span key={index} className="whitespace-pre-wrap">{node.data}</span>
            }
            // Split on paragraph breaks so each fragment renders inline (flowing
            // with adjacent hashtags/links) while preserving visual paragraph gaps.
            const paragraphs = node.data.split(/\n\s*\n/)
            return (
              <span key={index}>
                {paragraphs.map((para, i) => {
                  const leading = para.match(/^(\s+)/)?.[1] ?? ''
                  const trailing = para.match(/(\s+)$/)?.[1] ?? ''
                  const trimmed = para.slice(leading.length, para.length - trailing.length)
                  return (
                    <span key={i}>
                      {i > 0 && <span className="block mb-2" />}
                      {leading}
                      {trimmed ? <MarkdownText text={trimmed} /> : null}
                      {trailing}
                    </span>
                  )
                })}
              </span>
            )
          }
          if (node.type === 'image' || node.type === 'images') {
            const start = imageIndex
            const end = imageIndex + (Array.isArray(node.data) ? node.data.length : 1)
            imageIndex = end
            return (
              <ResponsiveImageGallery
                className="mt-2"
                key={index}
                images={allImages}
                start={start}
                end={end}
                mustLoad={mustLoadMedia}
              />
            )
          }
          if (node.type === 'media') {
            return (
              <MediaPlayer className="mt-2" key={index} src={node.data} mustLoad={mustLoadMedia} />
            )
          }
          if (node.type === 'url') {
            return <ExternalLink url={node.data} key={index} />
          }
          if (node.type === 'invoice') {
            return <EmbeddedLNInvoice invoice={node.data} key={index} className="mt-2" />
          }
          if (node.type === 'websocket-url') {
            return <EmbeddedWebsocketUrl url={node.data} key={index} />
          }
          if (node.type === 'event') {
            const id = node.data.split(':')[1]
            if (!id) return <span key={index}>{node.data}</span>
            return <EmbeddedNote key={index} noteId={id} className="mt-2" />
          }
          if (node.type === 'mention') {
            const userId = node.data.split(':')[1]
            if (!userId) return <span key={index}>{node.data}</span>
            return <EmbeddedMention key={index} userId={userId} />
          }
          if (node.type === 'hashtag') {
            return <EmbeddedHashtag hashtag={node.data} key={index} />
          }
          if (node.type === 'emoji') {
            const shortcode = node.data.split(':')[1]
            const emoji = emojiInfos.find((e) => e.shortcode === shortcode)
            if (!emoji) return node.data
            return <Emoji classNames={{ img: 'mb-1' }} emoji={emoji} key={index} />
          }
          if (node.type === 'youtube') {
            return (
              <YoutubeEmbeddedPlayer
                key={index}
                url={node.data}
                className="mt-2"
                mustLoad={mustLoadMedia}
              />
            )
          }
          if (node.type === 'x-post') {
            return (
              <XEmbeddedPost
                key={index}
                url={node.data}
                className="mt-2"
                mustLoad={mustLoadMedia}
              />
            )
          }
          return null
        })}
        {lastNormalUrl && <WebPreview className="mt-2" url={lastNormalUrl} />}
      </div>
      {enableHighlight && (
        <HighlightButton onHighlight={handleHighlight} containerRef={contentRef} />
      )}
      {enableHighlight && (
        <PostEditor
          highlightedText={selectedText}
          parentStuff={event}
          open={showHighlightEditor}
          setOpen={setShowHighlightEditor}
        />
      )}
    </>
  )
}
