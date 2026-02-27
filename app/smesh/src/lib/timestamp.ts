import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'

dayjs.extend(relativeTime)

/**
 * Format a unix timestamp (seconds) to a relative time string.
 * e.g., "5 minutes ago", "2 hours ago", "3 days ago"
 */
export function formatTimestamp(timestamp: number): string {
  return dayjs.unix(timestamp).fromNow()
}

/**
 * Format a unix timestamp to a short time string.
 * Shows time for today, date for older messages.
 */
export function formatMessageTime(timestamp: number): string {
  const date = dayjs.unix(timestamp)
  const now = dayjs()

  if (date.isSame(now, 'day')) {
    return date.format('HH:mm')
  } else if (date.isSame(now, 'year')) {
    return date.format('MMM D')
  } else {
    return date.format('MMM D, YYYY')
  }
}
