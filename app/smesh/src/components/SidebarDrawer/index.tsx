import GiteaIcon from '@/assets/GiteaIcon'
import Logo from '@/assets/Logo'
import { Sheet, SheetContent, SheetDescription, SheetTitle } from '@/components/ui/sheet'
import { usePrimaryPage } from '@/PageManager'
import { useNostr } from '@/providers/NostrProvider'
import { VisuallyHidden } from '@radix-ui/react-visually-hidden'
import AccountButton from '../Sidebar/AccountButton'
import BookmarkButton from '../Sidebar/BookmarkButton'
import ChatButton from '../Sidebar/ChatButton'
import HomeButton from '../Sidebar/HomeButton'
import LogoutButton from '../Sidebar/LogoutButton'
import NotificationsButton from '../Sidebar/NotificationButton'
import ProfileButton from '../Sidebar/ProfileButton'
import SettingsButton from '../Sidebar/SettingsButton'

type SidebarDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export default function SidebarDrawer({ open, onOpenChange }: SidebarDrawerProps) {
  const { pubkey } = useNostr()
  const { navigate } = usePrimaryPage()

  const handleItemClick = () => {
    onOpenChange(false)
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        side="left"
        hideClose
        className="w-64 p-0 bg-chrome-background border-r-0 rounded-r-none"
      >
        <VisuallyHidden>
          <SheetTitle>Navigation Menu</SheetTitle>
          <SheetDescription>App navigation and account menu</SheetDescription>
        </VisuallyHidden>
        <div className="flex flex-col h-full pb-4 pt-3 px-4 justify-between">
          {/* Account at top */}
          <div className="space-y-4">
            <div onClick={handleItemClick}>
              <AccountButton collapse={false} />
            </div>
          </div>

          {/* Navigation items in the middle */}
          <div className="space-y-2 flex-1 py-4">
            <div onClick={handleItemClick}>
              <HomeButton collapse={false} />
            </div>
            <div onClick={handleItemClick}>
              <NotificationsButton collapse={false} />
            </div>
            {pubkey && (
              <div onClick={handleItemClick}>
                <ChatButton collapse={false} />
              </div>
            )}
            <div onClick={handleItemClick}>
              <ProfileButton collapse={false} />
            </div>
            {pubkey && (
              <div onClick={handleItemClick}>
                <BookmarkButton collapse={false} />
              </div>
            )}
            <div onClick={handleItemClick}>
              <SettingsButton collapse={false} />
            </div>
            {pubkey && <LogoutButton collapse={false} />}
          </div>

          {/* Logo and version at bottom */}
          <div className="space-y-2">
            <button
              className="px-4 w-full cursor-pointer hover:opacity-80 transition-opacity"
              onClick={() => {
                navigate('home')
                handleItemClick()
              }}
              aria-label="Go to home"
            >
              <Logo />
            </button>
            <a
              href="https://git.smesh.lol/orly"
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center justify-center gap-2 text-xs text-muted-foreground hover:text-foreground transition-colors"
            >
              <GiteaIcon className="w-4 h-4" />
              <span>v{import.meta.env.APP_VERSION}</span>
            </a>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  )
}
