import { usePrimaryPage } from '@/PageManager'
import { useChat } from '@/providers/ChatProvider'
import { useKeyboardNavigation } from '@/providers/KeyboardNavigationProvider'
import { Hash } from 'lucide-react'
import SidebarItem from './SidebarItem'

export default function ChatButton({ collapse, navIndex }: { collapse: boolean; navIndex?: number }) {
  const { navigate, current, display } = usePrimaryPage()
  const { hasUnreadChannels } = useChat()
  const { clearColumn } = useKeyboardNavigation()

  const handleClick = () => {
    navigate('chat')
    clearColumn(1)
  }

  return (
    <SidebarItem
      title="NIRC"
      onClick={handleClick}
      active={display && current === 'chat'}
      collapse={collapse}
      navIndex={navIndex}
    >
      <div className="relative">
        <Hash />
        {hasUnreadChannels && (
          <div className="absolute -top-1 right-0 w-2 h-2 ring-2 ring-background bg-primary rounded-full" />
        )}
      </div>
    </SidebarItem>
  )
}
