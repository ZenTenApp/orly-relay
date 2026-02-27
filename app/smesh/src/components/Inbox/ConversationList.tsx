import { toDMConversation } from '@/lib/link'
import { useSecondaryPage } from '@/PageManager'
import { useDM } from '@/providers/DMProvider'
import { useFollowList } from '@/providers/FollowListProvider'
import { useMuteList } from '@/providers/MuteListProvider'
import storage from '@/services/local-storage.service'
import { Check, Loader2, MessageSquare, MoreVertical, RefreshCw } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '../ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger
} from '../ui/dropdown-menu'
import { ScrollArea } from '../ui/scroll-area'
import ConversationItem from './ConversationItem'

export default function ConversationList() {
  const { t } = useTranslation()
  const { push, pop } = useSecondaryPage()
  const {
    conversations,
    currentConversation,
    selectConversation,
    refreshConversations,
    loadMoreConversations,
    hasMoreConversations,
    isLoading
  } = useDM()
  const { followingSet } = useFollowList()
  const { mutePubkeySet } = useMuteList()
  const loadMoreRef = useRef<HTMLDivElement>(null)
  const [filterMode, setFilterMode] = useState<'all' | 'follows'>(() =>
    storage.getDMConversationFilter()
  )

  // Filter and sort conversations
  const sortedConversations = useMemo(() => {
    let filtered = [...conversations]

    if (filterMode === 'follows') {
      // Only show conversations from follows, and hide muted users
      filtered = filtered.filter(
        (c) => followingSet.has(c.partnerPubkey) && !mutePubkeySet.has(c.partnerPubkey)
      )
    }

    return filtered.sort((a, b) => b.lastMessageAt - a.lastMessageAt)
  }, [conversations, filterMode, followingSet, mutePubkeySet])

  const handleFilterChange = (mode: 'all' | 'follows') => {
    setFilterMode(mode)
    storage.setDMConversationFilter(mode)
  }

  // Infinite scroll: load more when sentinel is visible
  const handleIntersection = useCallback(
    (entries: IntersectionObserverEntry[]) => {
      const [entry] = entries
      if (entry.isIntersecting && hasMoreConversations && !isLoading) {
        loadMoreConversations()
      }
    },
    [hasMoreConversations, isLoading, loadMoreConversations]
  )

  useEffect(() => {
    const observer = new IntersectionObserver(handleIntersection, {
      root: null,
      rootMargin: '100px',
      threshold: 0
    })

    if (loadMoreRef.current) {
      observer.observe(loadMoreRef.current)
    }

    return () => observer.disconnect()
  }, [handleIntersection])

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-between p-3 border-b">
        <span className="font-medium text-sm">{t('Conversations')}</span>
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="icon"
            className="size-8"
            onClick={refreshConversations}
            disabled={isLoading}
          >
            <RefreshCw className={`size-4 ${isLoading ? 'animate-spin' : ''}`} />
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" className="size-8">
                <MoreVertical className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onClick={() => handleFilterChange('follows')}>
                {filterMode === 'follows' && <Check className="size-4 mr-2" />}
                <span className={filterMode !== 'follows' ? 'ml-6' : ''}>
                  {t('Only show follows')}
                </span>
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => handleFilterChange('all')}>
                {filterMode === 'all' && <Check className="size-4 mr-2" />}
                <span className={filterMode !== 'all' ? 'ml-6' : ''}>{t('Show all')}</span>
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </div>

      <ScrollArea className="flex-1">
        {sortedConversations.length === 0 && !isLoading ? (
          <div className="flex flex-col items-center justify-center h-48 gap-2 text-muted-foreground px-4">
            <MessageSquare className="size-8" />
            <p className="text-sm text-center">{t('No conversations yet')}</p>
            <p className="text-xs text-center">{t('Start a conversation by visiting a profile')}</p>
          </div>
        ) : (
          <div className="divide-y">
            {sortedConversations.map((conversation, index) => (
              <ConversationItem
                key={conversation.partnerPubkey}
                conversation={conversation}
                isActive={currentConversation === conversation.partnerPubkey}
                isFollowing={followingSet.has(conversation.partnerPubkey)}
                navIndex={index}
                onClick={() => {
                  // If already viewing a different conversation, pop first to replace
                  if (currentConversation && currentConversation !== conversation.partnerPubkey) {
                    pop()
                  }
                  push(toDMConversation(conversation.partnerPubkey))
                }}
                onClose={() => {
                  selectConversation(null)
                  pop()
                }}
              />
            ))}
            {/* Sentinel element for infinite scroll */}
            {hasMoreConversations && (
              <div ref={loadMoreRef} className="flex justify-center py-4">
                <Loader2 className="size-5 animate-spin text-muted-foreground" />
              </div>
            )}
          </div>
        )}
      </ScrollArea>
    </div>
  )
}
