import LoginDialog from '@/components/LoginDialog'
import Profile from '@/components/Profile'
import { Button } from '@/components/ui/button'
import PrimaryPageLayout from '@/layouts/PrimaryPageLayout'
import { useNostr } from '@/providers/NostrProvider'
import { TPageRef } from '@/types'
import { ArrowDownUp, UserRound } from 'lucide-react'
import { forwardRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

const ProfilePage = forwardRef<TPageRef>((_, ref) => {
  const { pubkey } = useNostr()

  return (
    <PrimaryPageLayout
      pageName="profile"
      titlebar={<ProfilePageTitlebar />}
      displayScrollToTopButton
      ref={ref}
    >
      <Profile id={pubkey ?? undefined} />
    </PrimaryPageLayout>
  )
})
ProfilePage.displayName = 'ProfilePage'
export default ProfilePage

function ProfilePageTitlebar() {
  const { t } = useTranslation()
  const [loginDialogOpen, setLoginDialogOpen] = useState(false)

  return (
    <>
      <div className="flex justify-between items-center h-full w-full pl-3 pr-1">
        <div className="flex gap-2 items-center">
          <UserRound />
          <div className="text-lg font-semibold">{t('Profile')}</div>
        </div>
        <Button variant="ghost" size="titlebar-icon" onClick={() => setLoginDialogOpen(true)}>
          <ArrowDownUp />
        </Button>
      </div>
      <LoginDialog open={loginDialogOpen} setOpen={setLoginDialogOpen} />
    </>
  )
}
