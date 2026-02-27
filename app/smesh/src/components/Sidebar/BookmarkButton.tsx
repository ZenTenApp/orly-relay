import { usePrimaryPage } from '@/PageManager'
import { useKeyboardNavigation } from '@/providers/KeyboardNavigationProvider'
import { useNostr } from '@/providers/NostrProvider'
import { Bookmark } from 'lucide-react'
import SidebarItem from './SidebarItem'

export default function BookmarkButton({ collapse, navIndex }: { collapse: boolean; navIndex?: number }) {
  const { navigate, current, display } = usePrimaryPage()
  const { checkLogin } = useNostr()
  const { clearColumn } = useKeyboardNavigation()

  const handleClick = () => {
    checkLogin(() => {
      navigate('bookmark')
      clearColumn(1)
    })
  }

  return (
    <SidebarItem
      title="Bookmarks"
      onClick={handleClick}
      active={display && current === 'bookmark'}
      collapse={collapse}
      navIndex={navIndex}
    >
      <Bookmark />
    </SidebarItem>
  )
}
