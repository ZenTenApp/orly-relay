import { usePrimaryPage } from '@/PageManager'
import { useKeyboardNavigation } from '@/providers/KeyboardNavigationProvider'
import { useNostr } from '@/providers/NostrProvider'
import { useNotification } from '@/providers/NotificationProvider'
import { Bell } from 'lucide-react'
import SidebarItem from './SidebarItem'

export default function NotificationsButton({ collapse, navIndex }: { collapse: boolean; navIndex?: number }) {
  const { checkLogin } = useNostr()
  const { navigate, current, display } = usePrimaryPage()
  const { hasNewNotification } = useNotification()
  const { clearColumn } = useKeyboardNavigation()

  const handleClick = () => {
    checkLogin(() => {
      navigate('notifications')
      clearColumn(1)
    })
  }

  return (
    <SidebarItem
      title="Notifications"
      onClick={handleClick}
      active={display && current === 'notifications'}
      collapse={collapse}
      navIndex={navIndex}
    >
      <div className="relative">
        <Bell />
        {hasNewNotification && (
          <div className="absolute -top-1 right-0 w-2 h-2 ring-2 ring-background bg-primary rounded-full" />
        )}
      </div>
    </SidebarItem>
  )
}
