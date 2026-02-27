import Help from '@/components/Help'
import SecondaryPageLayout from '@/layouts/SecondaryPageLayout'
import { forwardRef } from 'react'
import { useTranslation } from 'react-i18next'

const HelpPage = forwardRef(({ index }: { index?: number }, ref) => {
  const { t } = useTranslation()

  return (
    <SecondaryPageLayout ref={ref} index={index} title={t('Help')}>
      <Help />
    </SecondaryPageLayout>
  )
})
HelpPage.displayName = 'HelpPage'
export default HelpPage
