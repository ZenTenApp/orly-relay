import { usePrimaryPage } from '@/PageManager'
import { useChat } from '@/providers/ChatProvider'
import { useDM } from '@/providers/DMProvider'
import { useKeyboardNavigation } from '@/providers/KeyboardNavigationProvider'
import { MessageCircle } from 'lucide-react'
import SidebarItem from './SidebarItem'

export default function ChatButton({ collapse, navIndex }: { collapse: boolean; navIndex?: number }) {
  const { navigate, current, display } = usePrimaryPage()
  const { hasUnreadChannels } = useChat()
  const { hasNewMessages } = useDM()
  const { clearColumn } = useKeyboardNavigation()

  const hasUnread = hasUnreadChannels || hasNewMessages

  const handleClick = () => {
    navigate('chat')
    clearColumn(1)
  }

  return (
    <SidebarItem
      title="Chat"
      onClick={handleClick}
      active={display && current === 'chat'}
      collapse={collapse}
      navIndex={navIndex}
    >
      <div className="relative">
        <MessageCircle />
        {hasUnread && (
          <div className="absolute -top-1 right-0 w-2 h-2 ring-2 ring-background bg-primary rounded-full" />
        )}
      </div>
    </SidebarItem>
  )
}
