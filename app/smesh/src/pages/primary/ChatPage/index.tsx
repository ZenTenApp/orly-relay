import ChannelList from '@/components/Chat/ChannelList'
import ChannelView from '@/components/Chat/ChannelView'
import PrimaryPageLayout from '@/layouts/PrimaryPageLayout'
import { useNostr } from '@/providers/NostrProvider'
import { TPageRef } from '@/types'
import { Hash, LogIn } from 'lucide-react'
import { forwardRef, useImperativeHandle, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { usePrimaryPage } from '@/PageManager'
import { Button } from '@/components/ui/button'

const ChatPage = forwardRef<TPageRef>((_, ref) => {
  const { t } = useTranslation()
  const layoutRef = useRef<TPageRef>(null)
  const { pubkey } = useNostr()
  const { navigate } = usePrimaryPage()

  useImperativeHandle(ref, () => layoutRef.current as TPageRef)

  return (
    <PrimaryPageLayout
      pageName="chat"
      ref={layoutRef}
      titlebar={<ChatTitlebar />}
    >
      {pubkey ? (
        <div className="flex h-[calc(100vh-3rem)] overflow-hidden">
          <div className="w-56 shrink-0 border-r overflow-hidden">
            <ChannelList />
          </div>
          <div className="flex-1 min-w-0 overflow-hidden">
            <ChannelView />
          </div>
        </div>
      ) : (
        <div className="flex flex-col items-center justify-center h-64 gap-4 text-muted-foreground">
          <Hash className="size-12" />
          <div className="text-center">
            <p className="font-medium">{t('Sign in to use NIRC')}</p>
            <p className="text-sm">{t('Public chat channels on this relay')}</p>
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

function ChatTitlebar() {
  const { t } = useTranslation()
  return (
    <div className="flex gap-2 items-center h-full pl-3">
      <Hash className="size-5" />
      <div className="text-lg font-semibold">{t('NIRC')}</div>
    </div>
  )
}
