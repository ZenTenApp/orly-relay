import { usePrimaryPage } from '@/PageManager'
import { useDM } from '@/providers/DMProvider'
import { useKeyboardNavigation } from '@/providers/KeyboardNavigationProvider'
import { MessageSquare } from 'lucide-react'
import SidebarItem from './SidebarItem'

export default function InboxButton({ collapse, navIndex }: { collapse: boolean; navIndex?: number }) {
  const { navigate, current, display } = usePrimaryPage()
  const { hasNewMessages } = useDM()
  const { clearColumn } = useKeyboardNavigation()

  const handleClick = () => {
    navigate('inbox')
    clearColumn(1)
  }

  return (
    <SidebarItem
      title="Inbox"
      onClick={handleClick}
      active={display && current === 'inbox'}
      collapse={collapse}
      navIndex={navIndex}
    >
      <div className="relative">
        <MessageSquare />
        {hasNewMessages && (
          <div className="absolute -top-1 right-0 w-2 h-2 ring-2 ring-background bg-primary rounded-full" />
        )}
      </div>
    </SidebarItem>
  )
}
