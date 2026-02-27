import { useMemo } from 'react'
import ExternalLink from '../ExternalLink'
import Player from './Player'

export default function YoutubeEmbeddedPlayer({
  url,
  className
}: {
  url: string
  className?: string
  mustLoad?: boolean // kept for API compatibility, now ignored
}) {
  const { videoId, isShort } = useMemo(() => parseYoutubeUrl(url), [url])

  if (!videoId) {
    return <ExternalLink url={url} />
  }

  return <Player videoId={videoId} isShort={isShort} className={className} />
}

function parseYoutubeUrl(url: string) {
  const patterns = [
    /(?:youtube\.com\/watch\?v=|youtu\.be\/|youtube\.com\/embed\/)([^&\n?#]+)/,
    /youtube\.com\/watch\?.*v=([^&\n?#]+)/,
    /youtube\.com\/shorts\/([^&\n?#]+)/,
    /youtube\.com\/live\/([^&\n?#]+)/
  ]

  let videoId: string | null = null
  let isShort = false
  for (const [index, pattern] of patterns.entries()) {
    const match = url.match(pattern)
    if (match) {
      videoId = match[1].trim()
      isShort = index === 2 // Check if it's a short video
      break
    }
  }
  return { videoId, isShort }
}
