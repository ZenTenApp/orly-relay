import { cn } from '@/lib/utils'
import { ExternalLink, Play } from 'lucide-react'
import { memo, useState } from 'react'

interface PlayerProps {
  videoId: string
  isShort: boolean
  className?: string
}

/**
 * Privacy-preserving YouTube thumbnail display.
 *
 * Does NOT load any YouTube scripts or iframes. Shows a static thumbnail
 * from YouTube's image CDN and opens the video in a new tab when clicked.
 *
 * This eliminates all tracking that YouTube's embedded player performs:
 * - No /youtubei/v1/log_event telemetry
 * - No cookies set
 * - No browser fingerprinting
 * - No behavioral tracking on scroll/visibility
 */
const Player = memo(({ videoId, isShort, className }: PlayerProps) => {
  const [imgError, setImgError] = useState(false)

  // YouTube thumbnail URLs - try maxres first, fallback to hqdefault
  // maxresdefault.jpg may not exist for all videos
  const thumbnailUrl = imgError
    ? `https://i.ytimg.com/vi/${videoId}/hqdefault.jpg`
    : `https://i.ytimg.com/vi/${videoId}/maxresdefault.jpg`

  // Construct the direct YouTube URL
  const youtubeUrl = isShort
    ? `https://www.youtube.com/shorts/${videoId}`
    : `https://www.youtube.com/watch?v=${videoId}`

  return (
    <a
      href={youtubeUrl}
      target="_blank"
      rel="noopener noreferrer"
      onClick={(e) => e.stopPropagation()}
      className={cn(
        'block rounded-xl border overflow-hidden cursor-pointer relative group',
        isShort ? 'aspect-[9/16] max-h-[80vh] sm:max-h-[60vh]' : 'aspect-video max-h-[60vh]',
        className
      )}
    >
      {/* Thumbnail image */}
      <img
        src={thumbnailUrl}
        alt="YouTube video thumbnail"
        className="w-full h-full object-cover"
        loading="lazy"
        onError={() => !imgError && setImgError(true)}
      />

      {/* Play button overlay */}
      <div className="absolute inset-0 flex items-center justify-center bg-black/20 group-hover:bg-black/40 transition-colors">
        <div className="bg-red-600 rounded-full p-4 group-hover:scale-110 transition-transform shadow-lg">
          <Play className="size-8 text-white fill-white" />
        </div>
      </div>

      {/* External link indicator */}
      <div className="absolute top-2 right-2 bg-black/60 rounded-md px-2 py-1 flex items-center gap-1 text-white text-xs opacity-0 group-hover:opacity-100 transition-opacity">
        <ExternalLink className="size-3" />
        <span>YouTube</span>
      </div>
    </a>
  )
})

Player.displayName = 'YoutubePlayer'

export default Player
