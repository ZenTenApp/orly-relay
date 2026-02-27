import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

export interface UseInfiniteScrollOptions<T> {
  /**
   * The initial data items
   */
  items: T[]
  /**
   * Whether to initially show all items or use pagination
   * @default false
   */
  showAllInitially?: boolean
  /**
   * Number of items to show initially and load per batch
   * @default 10
   */
  showCount?: number
  /**
   * Initial loading state, which can be used to prevent loading more data until initial load is complete
   */
  initialLoading?: boolean
  /**
   * The function to load more data
   * Returns true if there are more items to load, false otherwise
   */
  onLoadMore: () => Promise<boolean>
  /**
   * IntersectionObserver options
   */
  observerOptions?: IntersectionObserverInit
}

const DEFAULT_OBSERVER_OPTIONS: IntersectionObserverInit = {
  root: null,
  rootMargin: '100px',
  threshold: 0
}

export function useInfiniteScroll<T>({
  items,
  showAllInitially = false,
  showCount: initialShowCount = 10,
  onLoadMore,
  initialLoading = false,
  observerOptions = DEFAULT_OBSERVER_OPTIONS
}: UseInfiniteScrollOptions<T>) {
  const [hasMore, setHasMore] = useState(true)
  const [showCount, setShowCount] = useState(showAllInitially ? Infinity : initialShowCount)
  const [loading, setLoading] = useState(false)
  const bottomRef = useRef<HTMLDivElement | null>(null)

  // Store all mutable state in a ref so the loadMore callback never goes stale
  const stateRef = useRef({
    loading,
    hasMore,
    showCount,
    itemsLength: items.length,
    initialLoading,
    onLoadMore
  })

  stateRef.current = {
    loading,
    hasMore,
    showCount,
    itemsLength: items.length,
    initialLoading,
    onLoadMore
  }

  // Stable callback — never changes identity, reads everything from stateRef
  const loadMore = useCallback(async () => {
    const { loading, hasMore, showCount, itemsLength, initialLoading, onLoadMore } =
      stateRef.current

    if (initialLoading || loading) return

    // If there are more items to show, increase showCount first
    if (showCount < itemsLength) {
      setShowCount((prev) => prev + initialShowCount)
      // Only fetch more data when remaining items are running low
      if (itemsLength - showCount > initialShowCount * 2) {
        return
      }
    }

    if (!hasMore) return
    setLoading(true)
    const newHasMore = await onLoadMore()
    setHasMore(newHasMore)
    setLoading(false)
  }, [initialShowCount])

  // IntersectionObserver — created once, never torn down/rebuilt
  useEffect(() => {
    const currentBottomRef = bottomRef.current
    if (!currentBottomRef) return

    const observer = new IntersectionObserver((entries) => {
      if (entries[0].isIntersecting) {
        loadMore()
      }
    }, observerOptions)

    observer.observe(currentBottomRef)

    return () => {
      observer.disconnect()
    }
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const visibleItems = useMemo(() => {
    return showAllInitially ? items : items.slice(0, showCount)
  }, [items, showAllInitially, showCount])

  const shouldShowLoadingIndicator = hasMore || showCount < items.length || loading

  return {
    visibleItems,
    loading,
    hasMore,
    shouldShowLoadingIndicator,
    bottomRef,
    setHasMore,
    setLoading
  }
}
