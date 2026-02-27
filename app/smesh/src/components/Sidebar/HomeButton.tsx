import { usePrimaryPage } from '@/PageManager'
import { useKeyboardNavigation } from '@/providers/KeyboardNavigationProvider'
import { Home } from 'lucide-react'
import SidebarItem from './SidebarItem'

export default function HomeButton({ collapse, navIndex }: { collapse: boolean; navIndex?: number }) {
  const { navigate, current, display } = usePrimaryPage()
  const { resetPrimarySelection, clearColumn } = useKeyboardNavigation()

  const handleClick = () => {
    navigate('home')
    clearColumn(1)
    resetPrimarySelection()
  }

  return (
    <SidebarItem
      title="Home"
      onClick={handleClick}
      active={display && current === 'home'}
      collapse={collapse}
      navIndex={navIndex}
    >
      <Home />
    </SidebarItem>
  )
}
