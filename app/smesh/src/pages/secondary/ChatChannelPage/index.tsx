import ChannelView from '@/components/Chat/ChannelView'
import { Button } from '@/components/ui/button'
import { Titlebar } from '@/components/Titlebar'
import { useSecondaryPage } from '@/PageManager'
import { useChat } from '@/providers/ChatProvider'
import { TPageRef } from '@/types'
import { ChevronLeft } from 'lucide-react'
import { forwardRef, useEffect, useImperativeHandle, useRef } from 'react'

interface ChatChannelPageProps {
  channelId?: string
}

const ChatChannelPage = forwardRef<TPageRef, ChatChannelPageProps>(
  ({ channelId }, ref) => {
    const layoutRef = useRef<TPageRef>(null)
    const { selectChannelById } = useChat()
    const { pop } = useSecondaryPage()

    useImperativeHandle(ref, () => layoutRef.current as TPageRef)

    // Select the channel when this page mounts
    useEffect(() => {
      if (channelId) {
        selectChannelById(channelId)
      }
    }, [channelId, selectChannelById])

    // Clear channel selection when page unmounts
    useEffect(() => {
      return () => {
        selectChannelById(null)
      }
    }, [])

    const handleBack = () => {
      selectChannelById(null)
      pop()
    }

    return (
      <div className="flex flex-col h-[var(--vh)]">
        <Titlebar className="p-1 shrink-0" hideBottomBorder>
          <div className="flex items-center w-full px-1">
            <Button
              className="flex gap-1 items-center justify-start pl-2 pr-1"
              variant="ghost"
              size="titlebar-icon"
              title="Back to channels"
              onClick={handleBack}
            >
              <ChevronLeft />
            </Button>
          </div>
        </Titlebar>
        <div className="flex-1 min-h-0">
          <ChannelView />
        </div>
      </div>
    )
  }
)

ChatChannelPage.displayName = 'ChatChannelPage'
export default ChatChannelPage
