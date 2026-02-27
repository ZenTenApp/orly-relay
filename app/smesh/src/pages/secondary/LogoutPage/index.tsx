import { Button } from '@/components/ui/button'
import SecondaryPageLayout from '@/layouts/SecondaryPageLayout'
import { useSecondaryPage } from '@/PageManager'
import { useNostr } from '@/providers/NostrProvider'
import { forwardRef } from 'react'
import { useTranslation } from 'react-i18next'

const LogoutPage = forwardRef(({ index }: { index?: number }, ref) => {
  const { t } = useTranslation()
  const { pop } = useSecondaryPage()
  const { account, removeAccount } = useNostr()

  const handleLogout = () => {
    if (account) {
      removeAccount(account)
      pop()
    }
  }

  return (
    <SecondaryPageLayout ref={ref} index={index} title={t('Logout')}>
      <div className="p-4 space-y-6">
        <p className="text-muted-foreground">{t('Are you sure you want to logout?')}</p>
        <div className="flex flex-col gap-3">
          <Button variant="outline" onClick={() => pop()} className="w-full">
            {t('Cancel')}
          </Button>
          <Button variant="destructive" onClick={handleLogout} className="w-full">
            {t('Logout')}
          </Button>
        </div>
      </div>
    </SecondaryPageLayout>
  )
})
LogoutPage.displayName = 'LogoutPage'
export default LogoutPage
