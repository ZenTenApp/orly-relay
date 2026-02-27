import Help from '@/components/Help'
import PrimaryPageLayout from '@/layouts/PrimaryPageLayout'
import { TPageRef } from '@/types'
import { HelpCircle } from 'lucide-react'
import { forwardRef } from 'react'
import { useTranslation } from 'react-i18next'

const HelpPage = forwardRef<TPageRef>((_, ref) => (
  <PrimaryPageLayout
    pageName="help"
    ref={ref}
    titlebar={<HelpPageTitlebar />}
    displayScrollToTopButton
  >
    <Help />
  </PrimaryPageLayout>
))
HelpPage.displayName = 'HelpPage'
export default HelpPage

function HelpPageTitlebar() {
  const { t } = useTranslation()

  return (
    <div className="flex gap-2 items-center h-full pl-3">
      <HelpCircle />
      <div className="text-lg font-semibold">{t('Help')}</div>
    </div>
  )
}
