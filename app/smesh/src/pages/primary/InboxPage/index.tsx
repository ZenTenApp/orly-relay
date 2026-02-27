import InboxContent from '@/components/Inbox/InboxContent'
import PrimaryPageLayout from '@/layouts/PrimaryPageLayout'
import { useDM } from '@/providers/DMProvider'
import { useNostr } from '@/providers/NostrProvider'
import { TPageRef } from '@/types'
import { MessageSquare, LogIn } from 'lucide-react'
import { forwardRef, useEffect, useImperativeHandle, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { usePrimaryPage } from '@/PageManager'
import { Button } from '@/components/ui/button'

const InboxPage = forwardRef<TPageRef>((_, ref) => {
  const { t } = useTranslation()
  const layoutRef = useRef<TPageRef>(null)
  const { pubkey } = useNostr()
  const { navigate } = usePrimaryPage()
  const { markInboxAsSeen } = useDM()

  useImperativeHandle(ref, () => layoutRef.current as TPageRef)

  // Mark inbox as seen when page is viewed
  useEffect(() => {
    if (pubkey) {
      markInboxAsSeen()
    }
  }, [pubkey, markInboxAsSeen])

  return (
    <PrimaryPageLayout
      pageName="inbox"
      ref={layoutRef}
      titlebar={<InboxTitlebar />}
      displayScrollToTopButton
    >
      {pubkey ? (
        <InboxContent />
      ) : (
        <div className="flex flex-col items-center justify-center h-64 gap-4 text-muted-foreground">
          <MessageSquare className="size-12" />
          <div className="text-center">
            <p className="font-medium">{t('Sign in to view your messages')}</p>
            <p className="text-sm">{t('Your encrypted conversations will appear here')}</p>
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
InboxPage.displayName = 'InboxPage'
export default InboxPage

function InboxTitlebar() {
  const { t } = useTranslation()
  return (
    <div className="flex gap-2 items-center h-full pl-3">
      <MessageSquare className="size-5" />
      <div className="text-lg font-semibold">{t('Inbox')}</div>
    </div>
  )
}
