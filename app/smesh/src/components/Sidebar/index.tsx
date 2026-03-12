import Icon from '@/assets/Icon'
import Logo from '@/assets/Logo'
import { cn } from '@/lib/utils'
import { usePrimaryPage } from '@/PageManager'
import { useNostr } from '@/providers/NostrProvider'
import { useScreenSize } from '@/providers/ScreenSizeProvider'
import { useTheme } from '@/providers/ThemeProvider'
import { useUserPreferences } from '@/providers/UserPreferencesProvider'
import { ChevronsLeft, ChevronsRight } from 'lucide-react'
import AccountButton from './AccountButton'
import BookmarkButton from './BookmarkButton'
import ChatButton from './ChatButton'
import HelpButton from './HelpButton'
import HomeButton from './HomeButton'
import KeyboardModeButton from './KeyboardModeButton'
import LayoutSwitcher from './LayoutSwitcher'
import NotificationsButton from './NotificationButton'
import PostButton from './PostButton'
import ProfileButton from './ProfileButton'
import RelayAdminButton from './RelayAdminButton'
import SearchButton from './SearchButton'
import SettingsButton from './SettingsButton'

export default function PrimaryPageSidebar() {
  const { isSmallScreen, isNarrowDesktop } = useScreenSize()
  const { themeSetting } = useTheme()
  const { sidebarCollapse, updateSidebarCollapse, enableSingleColumnLayout } = useUserPreferences()
  const { pubkey } = useNostr()
  const { navigate } = usePrimaryPage()

  if (isSmallScreen) return null

  // Force collapsed mode in narrow desktop (768-1024px)
  const isCollapsed = isNarrowDesktop || sidebarCollapse

  return (
    <div
      className={cn(
        'relative z-40 flex flex-col pb-2 pt-3 justify-between h-full shrink-0 bg-chrome-background',
        isCollapsed ? 'px-2 w-16' : 'px-4 w-52'
      )}
    >
      <div className="space-y-2">
        {isCollapsed ? (
          <button
            className="px-3 py-1 mb-4 w-full cursor-pointer hover:opacity-80 transition-opacity"
            onClick={() => navigate('home')}
            aria-label="Go to home"
          >
            <Icon />
          </button>
        ) : (
          <button
            className="px-4 mb-4 w-full cursor-pointer hover:opacity-80 transition-opacity"
            onClick={() => navigate('home')}
            aria-label="Go to home"
          >
            <Logo />
          </button>
        )}
        <HomeButton collapse={isCollapsed} navIndex={0} />
        <NotificationsButton collapse={isCollapsed} navIndex={1} />
        <SearchButton collapse={isCollapsed} navIndex={2} />
        {pubkey && <ChatButton collapse={isCollapsed} navIndex={3} />}
        <ProfileButton collapse={isCollapsed} navIndex={pubkey ? 4 : 3} />
        {pubkey && <BookmarkButton collapse={isCollapsed} navIndex={5} />}
        <RelayAdminButton collapse={isCollapsed} navIndex={pubkey ? 6 : 4} />
        <SettingsButton collapse={isCollapsed} navIndex={pubkey ? 7 : 5} />
        <PostButton collapse={isCollapsed} navIndex={pubkey ? 8 : 5} />
      </div>
      <div className="space-y-4">
        <HelpButton collapse={isCollapsed} navIndex={pubkey ? 9 : 6} />
        <KeyboardModeButton collapse={isCollapsed} />
        <LayoutSwitcher collapse={isCollapsed} />
        <AccountButton collapse={isCollapsed} />
      </div>
      {!isNarrowDesktop && (
        <button
          className={cn(
            'absolute flex flex-col justify-center items-center right-0 w-5 h-6 p-0 rounded-l-md hover:shadow-md text-muted-foreground hover:text-foreground hover:bg-background transition-colors [&_svg]:size-4',
            themeSetting === 'dark' || enableSingleColumnLayout ? 'top-3' : 'top-5'
          )}
          onClick={(e) => {
            e.stopPropagation()
            updateSidebarCollapse(!sidebarCollapse)
          }}
        >
          {sidebarCollapse ? <ChevronsRight /> : <ChevronsLeft />}
        </button>
      )}
    </div>
  )
}
