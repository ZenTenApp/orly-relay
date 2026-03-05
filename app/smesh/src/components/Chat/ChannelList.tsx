import { useChat } from '@/providers/ChatProvider'
import { useSecondaryPage } from '@/PageManager'
import { toChatChannel } from '@/lib/link'
import { cn } from '@/lib/utils'
import { Hash, Plus, Loader2, RefreshCw, Lock, BellOff } from 'lucide-react'
import { useState } from 'react'
import { Button } from '../ui/button'
import CreateChannelDialog from './CreateChannelDialog'

export default function ChannelList() {
  const {
    channels,
    currentChannel,
    isLoadingChannels,
    refreshChannels,
    unreadCounts,
    mutedChannels
  } = useChat()
  const { push, pop } = useSecondaryPage()
  const [showCreate, setShowCreate] = useState(false)

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-between px-3 py-2 border-b">
        <span className="text-sm font-semibold">Channels</span>
        <div className="flex gap-1">
          <Button
            variant="ghost"
            size="icon"
            className="size-7"
            onClick={() => refreshChannels()}
            title="Refresh"
          >
            <RefreshCw className="size-3.5" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="size-7"
            onClick={() => setShowCreate(true)}
            title="Create channel"
          >
            <Plus className="size-3.5" />
          </Button>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto">
        {isLoadingChannels ? (
          <div className="flex justify-center py-8">
            <Loader2 className="size-5 animate-spin text-muted-foreground" />
          </div>
        ) : channels.length === 0 ? (
          <div className="px-3 py-8 text-center text-sm text-muted-foreground">
            No channels yet
          </div>
        ) : (
          channels.map((ch) => {
            const unread = unreadCounts[ch.id] || 0
            const isMuted = mutedChannels.has(ch.id)
            return (
              <button
                key={ch.id}
                onClick={() => {
                  if (currentChannel && currentChannel.id !== ch.id) {
                    pop()
                  }
                  push(toChatChannel(ch.id))
                }}
                className={cn(
                  'flex items-center gap-2 w-full px-3 py-2 text-left text-sm transition-colors hover:bg-accent',
                  currentChannel?.id === ch.id && 'bg-accent text-accent-foreground'
                )}
              >
                <Hash className="size-4 flex-shrink-0 text-muted-foreground" />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-1">
                    <span className={cn('truncate font-medium', unread > 0 && !isMuted && 'font-bold')}>
                      {ch.name}
                    </span>
                    {ch.inviteOnly && <Lock className="size-3 text-muted-foreground flex-shrink-0" />}
                    {isMuted && <BellOff className="size-3 text-muted-foreground flex-shrink-0" />}
                  </div>
                  {ch.about && (
                    <div className="truncate text-xs text-muted-foreground">{ch.about}</div>
                  )}
                </div>
                {unread > 0 && !isMuted && (
                  <span className="inline-flex items-center justify-center size-5 text-xs rounded-full bg-primary text-primary-foreground flex-shrink-0">
                    {unread > 99 ? '99+' : unread}
                  </span>
                )}
              </button>
            )
          })
        )}
      </div>

      <CreateChannelDialog open={showCreate} onOpenChange={setShowCreate} />
    </div>
  )
}
