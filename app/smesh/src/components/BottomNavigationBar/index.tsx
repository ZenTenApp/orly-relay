import { cn } from '@/lib/utils'
import { useScreenSize } from '@/providers/ScreenSizeProvider'
import BackgroundAudio from '../BackgroundAudio'
import AccountButton from './AccountButton'
import BookmarkButton from './BookmarkButton'
import HomeButton from './HomeButton'
import NotificationsButton from './NotificationsButton'
import PostButton from './PostButton'
import SearchButton from './SearchButton'
import SettingsButton from './SettingsButton'

export default function BottomNavigationBar() {
  const { isTabletScreen } = useScreenSize()

  return (
    <div
      className={cn('fixed bottom-0 w-full z-40 bg-chrome-background border-t')}
      style={{
        paddingBottom: 'env(safe-area-inset-bottom)'
      }}
    >
      <BackgroundAudio className="rounded-none border-x-0 border-t-0 border-b bg-background" />
      <div className="w-full flex justify-around items-center [&_svg]:size-4 [&_svg]:shrink-0">
        <HomeButton />
        <NotificationsButton />
        {isTabletScreen && (
          <>
            <SearchButton />
            <BookmarkButton />
            <PostButton />
            <SettingsButton />
          </>
        )}
        <AccountButton />
      </div>
    </div>
  )
}
