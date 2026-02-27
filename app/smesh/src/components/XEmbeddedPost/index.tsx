import { useMemo } from 'react'
import ExternalLink from '../ExternalLink'
import Post from './Post'

export default function XEmbeddedPost({
  url,
  className,
  embedded = true
}: {
  url: string
  className?: string
  mustLoad?: boolean // kept for API compatibility, now ignored
  embedded?: boolean
}) {
  const { tweetId } = useMemo(() => parseXUrl(url), [url])

  if (!tweetId) {
    return <ExternalLink url={url} />
  }

  return <Post tweetId={tweetId} url={url} className={className} embedded={embedded} />
}

function parseXUrl(url: string): { tweetId: string | null } {
  const pattern = /(?:twitter\.com|x\.com)\/(?:#!\/)?(?:\w+)\/status(?:es)?\/(\d+)/i
  const match = url.match(pattern)
  return {
    tweetId: match ? match[1] : null
  }
}
