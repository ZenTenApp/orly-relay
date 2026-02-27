import { cn } from '@/lib/utils'
import { ExternalLink } from 'lucide-react'
import { memo } from 'react'

interface PostProps {
  tweetId: string
  url: string
  className?: string
  embedded?: boolean // kept for API compatibility, now ignored
}

/**
 * Privacy-preserving X/Twitter post link card.
 *
 * Does NOT load Twitter's widgets.js script. Shows a styled card
 * with the post URL and opens in a new tab when clicked.
 *
 * This eliminates all tracking that Twitter's embedded widget performs:
 * - No platform.twitter.com/widgets.js loading
 * - No cookies set
 * - No browser fingerprinting
 * - No behavioral tracking
 */
const Post = memo(({ tweetId, url, className }: PostProps) => {
  // Extract username from URL if possible
  const usernameMatch = url.match(/(?:twitter\.com|x\.com)\/([^/]+)\/status/i)
  const username = usernameMatch ? usernameMatch[1] : null

  return (
    <a
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      onClick={(e) => e.stopPropagation()}
      className={cn(
        'block rounded-xl border overflow-hidden cursor-pointer group',
        'bg-card hover:bg-accent/50 transition-colors',
        'p-4 flex items-center gap-4',
        className
      )}
      style={{ maxWidth: '550px' }}
    >
      {/* X logo */}
      <div className="flex-shrink-0 bg-black dark:bg-white rounded-full p-3">
        <svg
          viewBox="0 0 24 24"
          className="size-6 text-white dark:text-black"
          fill="currentColor"
        >
          <path d="M18.244 2.25h3.308l-7.227 8.26 8.502 11.24H16.17l-5.214-6.817L4.99 21.75H1.68l7.73-8.835L1.254 2.25H8.08l4.713 6.231zm-1.161 17.52h1.833L7.084 4.126H5.117z" />
        </svg>
      </div>

      {/* Post info */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <span className="font-semibold text-foreground">
            {username ? `@${username}` : 'X Post'}
          </span>
        </div>
        <div className="text-sm text-muted-foreground truncate">
          Post ID: {tweetId}
        </div>
      </div>

      {/* External link indicator */}
      <div className="flex-shrink-0 opacity-60 group-hover:opacity-100 transition-opacity">
        <ExternalLink className="size-5" />
      </div>
    </a>
  )
})

Post.displayName = 'XPost'

export default Post
