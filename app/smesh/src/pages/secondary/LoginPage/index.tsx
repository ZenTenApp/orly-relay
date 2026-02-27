import AccountManager from '@/components/AccountManager'
import SecondaryPageLayout from '@/layouts/SecondaryPageLayout'
import { useSecondaryPage } from '@/PageManager'
import { forwardRef } from 'react'
import { useTranslation } from 'react-i18next'

const LoginPage = forwardRef(({ index }: { index?: number }, ref) => {
  const { t } = useTranslation()
  const { pop } = useSecondaryPage()

  return (
    <SecondaryPageLayout ref={ref} index={index} title={t('Login')}>
      <div className="p-4">
        <AccountManager close={() => pop()} />
      </div>
    </SecondaryPageLayout>
  )
})
LoginPage.displayName = 'LoginPage'
export default LoginPage
