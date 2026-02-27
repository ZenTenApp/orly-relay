import { useInfiniteScroll } from '@/hooks/useInfiniteScroll'
import { useStuff } from '@/hooks/useStuff'
import { useAllDescendantThreads } from '@/hooks/useThread'
import { getEventKey, isMentioningMutedUsers } from '@/lib/event'
import { useContentPolicy } from '@/providers/ContentPolicyProvider'
import { useMuteList } from '@/providers/MuteListProvider'
import { useUserTrust } from '@/providers/UserTrustProvider'
import threadService from '@/services/thread.service'
import { Event as NEvent } from 'nostr-tools'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { LoadingBar } from '../LoadingBar'
import ReplyNote, { ReplyNoteSkeleton } from '../ReplyNote'
import SubReplies from './SubReplies'

const LIMIT = 100
const SHOW_COUNT = 10

export default function ReplyNoteList({ stuff, navIndexOffset = 0 }: { stuff: NEvent | string; navIndexOffset?: number }) {
  const { t } = useTranslation()
  const { hideUntrustedInteractions, isUserTrusted } = useUserTrust()
  const { mutePubkeySet } = useMuteList()
  const { hideContentMentioningMutedUsers } = useContentPolicy()
  const { stuffKey } = useStuff(stuff)
  const allThreads = useAllDescendantThreads(stuffKey)
  const [initialLoading, setInitialLoading] = useState(false)

  const replies = useMemo(() => {
    const replyKeySet = new Set<string>()
    const thread = allThreads.get(stuffKey) || []
    const replyEvents = thread.filter((evt) => {
      const key = getEventKey(evt)
      if (replyKeySet.has(key)) return false
      if (mutePubkeySet.has(evt.pubkey)) return false
      if (hideContentMentioningMutedUsers && isMentioningMutedUsers(evt, mutePubkeySet)) {
        return false
      }
      if (hideUntrustedInteractions && !isUserTrusted(evt.pubkey)) {
        const replyKey = getEventKey(evt)
        const repliesForThisReply = allThreads.get(replyKey)
        // If the reply is not trusted and there are no trusted replies for this reply, skip rendering
        if (
          !repliesForThisReply ||
          repliesForThisReply.every((evt) => !isUserTrusted(evt.pubkey))
        ) {
          return false
        }
      }

      replyKeySet.add(key)
      return true
    })
    return replyEvents.sort((a, b) => b.created_at - a.created_at)
  }, [
    stuffKey,
    allThreads,
    mutePubkeySet,
    hideContentMentioningMutedUsers,
    hideUntrustedInteractions,
    isUserTrusted
  ])

  // Initial subscription
  useEffect(() => {
    const loadInitial = async () => {
      setInitialLoading(true)
      await threadService.subscribe(stuff, LIMIT)
      setInitialLoading(false)
    }

    loadInitial()

    return () => {
      threadService.unsubscribe(stuff)
    }
  }, [stuff])

  const handleLoadMore = useCallback(async () => {
    return await threadService.loadMore(stuff, LIMIT)
  }, [stuff])

  const { visibleItems, loading, shouldShowLoadingIndicator, bottomRef } = useInfiniteScroll({
    items: replies,
    showCount: SHOW_COUNT,
    onLoadMore: handleLoadMore,
    initialLoading
  })

  return (
    <div className="min-h-[80vh]">
      {(loading || initialLoading) && <LoadingBar />}
      <div>
        {visibleItems.map((reply, index) => (
          <Item key={reply.id} reply={reply} navIndex={navIndexOffset + index} />
        ))}
      </div>
      <div ref={bottomRef} />
      {shouldShowLoadingIndicator ? (
        <ReplyNoteSkeleton />
      ) : (
        <div className="text-sm mt-2 mb-3 text-center text-muted-foreground">
          {replies.length > 0 ? t('no more replies') : t('no replies')}
        </div>
      )}
    </div>
  )
}

// Use larger gaps between items to leave room for sub-replies
const NAV_INDEX_MULTIPLIER = 100

function Item({ reply, navIndex }: { reply: NEvent; navIndex: number }) {
  const key = useMemo(() => getEventKey(reply), [reply])
  const baseNavIndex = navIndex * NAV_INDEX_MULTIPLIER

  return (
    <div className="relative border-b">
      <ReplyNote event={reply} navColumn={2} navIndex={baseNavIndex} />
      <SubReplies parentKey={key} revealerNavIndex={baseNavIndex + 1} subReplyNavIndexStart={baseNavIndex + 2} />
    </div>
  )
}
