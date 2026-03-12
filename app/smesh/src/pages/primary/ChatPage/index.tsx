import ChannelList from '@/components/Chat/ChannelList'
import ChannelView from '@/components/Chat/ChannelView'
import InboxContent from '@/components/Inbox/InboxContent'
import PrimaryPageLayout from '@/layouts/PrimaryPageLayout'
import { cn } from '@/lib/utils'
import { useDM } from '@/providers/DMProvider'
import { useNostr } from '@/providers/NostrProvider'
import { TPageRef } from '@/types'
import { Hash, LogIn, MessageCircle, MessageSquare } from 'lucide-react'
import { forwardRef, useEffect, useImperativeHandle, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePrimaryPage } from '@/PageManager'
import { Button } from '@/components/ui/button'
import { useChat } from '@/providers/ChatProvider'

type ChatTab = 'dms' | 'channels'

const ChatPage = forwardRef<TPageRef>((_, ref) => {
  const { t } = useTranslation()
  const layoutRef = useRef<TPageRef>(null)
  const { pubkey } = useNostr()
  const { navigate } = usePrimaryPage()
  const { markInboxAsSeen } = useDM()
  const [activeTab, setActiveTab] = useState<ChatTab>('dms')

  useImperativeHandle(ref, () => layoutRef.current as TPageRef)

  useEffect(() => {
    if (pubkey && activeTab === 'dms') {
      markInboxAsSeen()
    }
  }, [pubkey, activeTab, markInboxAsSeen])

  const { refreshChannels } = useChat()

  useEffect(() => {
    if (activeTab === 'channels') {
      refreshChannels()
    }
  }, [activeTab, refreshChannels])

  return (
    <PrimaryPageLayout
      pageName="chat"
      ref={layoutRef}
      titlebar={<ChatTitlebar activeTab={activeTab} onTabChange={setActiveTab} />}
    >
      {pubkey ? (
        activeTab === 'dms' ? (
          <InboxContent />
        ) : (
          <div className="flex h-[calc(100vh-3rem)] overflow-hidden">
            <div className="w-56 shrink-0 border-r overflow-hidden">
              <ChannelList />
            </div>
            <div className="flex-1 min-w-0 overflow-hidden">
              <ChannelView />
            </div>
          </div>
        )
      ) : (
        <div className="flex flex-col items-center justify-center h-64 gap-4 text-muted-foreground">
          <MessageCircle className="size-12" />
          <div className="text-center">
            <p className="font-medium">{t('Sign in to chat')}</p>
            <p className="text-sm">{t('Direct messages and public channels')}</p>
          </div>
          <Button onClick={() => navigate('settings')} className="gap-2">
            <LogIn className="size-4" />
            {t('Sign In')}
          </Button>
        </div>
      )}
    </PrimaryPageLayout>
  )
})
ChatPage.displayName = 'ChatPage'
export default ChatPage

function ChatTitlebar({
  activeTab,
  onTabChange
}: {
  activeTab: ChatTab
  onTabChange: (tab: ChatTab) => void
}) {
  const { t } = useTranslation()
  const { hasNewMessages } = useDM()
  const { hasUnreadChannels } = useChat()

  return (
    <div className="flex items-center h-full px-3 gap-1">
      <MessageCircle className="size-5 shrink-0" />
      <div className="text-lg font-semibold mr-3">{t('Chat')}</div>
      <button
        onClick={() => onTabChange('dms')}
        className={cn(
          'flex items-center gap-1.5 px-3 py-1 rounded-md text-sm font-medium transition-colors',
          activeTab === 'dms'
            ? 'bg-accent text-accent-foreground'
            : 'text-muted-foreground hover:text-foreground hover:bg-accent/50'
        )}
      >
        <MessageSquare className="size-3.5" />
        {t('DMs')}
        {hasNewMessages && (
          <div className="w-2 h-2 bg-primary rounded-full" />
        )}
      </button>
      <button
        onClick={() => onTabChange('channels')}
        className={cn(
          'flex items-center gap-1.5 px-3 py-1 rounded-md text-sm font-medium transition-colors',
          activeTab === 'channels'
            ? 'bg-accent text-accent-foreground'
            : 'text-muted-foreground hover:text-foreground hover:bg-accent/50'
        )}
      >
        <Hash className="size-3.5" />
        {t('Channels')}
        {hasUnreadChannels && (
          <div className="w-2 h-2 bg-primary rounded-full" />
        )}
      </button>
    </div>
  )
}
