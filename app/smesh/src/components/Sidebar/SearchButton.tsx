import { usePrimaryPage } from '@/PageManager'
import { useKeyboardNavigation } from '@/providers/KeyboardNavigationProvider'
import { Search } from 'lucide-react'
import SidebarItem from './SidebarItem'

export default function SearchButton({ collapse, navIndex }: { collapse: boolean; navIndex?: number }) {
  const { navigate, current, display } = usePrimaryPage()
  const { clearColumn } = useKeyboardNavigation()

  const handleClick = () => {
    navigate('search')
    clearColumn(1)
  }

  return (
    <SidebarItem
      title="Search"
      onClick={handleClick}
      active={current === 'search' && display}
      collapse={collapse}
      navIndex={navIndex}
    >
      <Search />
    </SidebarItem>
  )
}
