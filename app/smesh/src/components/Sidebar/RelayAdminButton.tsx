import { usePrimaryPage } from '@/PageManager'
import { useKeyboardNavigation } from '@/providers/KeyboardNavigationProvider'
import { Server } from 'lucide-react'
import SidebarItem from './SidebarItem'

export default function RelayAdminButton({
  collapse,
  navIndex
}: {
  collapse: boolean
  navIndex?: number
}) {
  const { current, navigate, display } = usePrimaryPage()
  const { clearColumn } = useKeyboardNavigation()

  const handleClick = () => {
    navigate('relay')
    clearColumn(1)
  }

  return (
    <SidebarItem
      title="Relay"
      onClick={handleClick}
      collapse={collapse}
      active={display && current === 'relay'}
      navIndex={navIndex}
    >
      <Server />
    </SidebarItem>
  )
}
