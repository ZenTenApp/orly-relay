import { usePrimaryPage } from '@/PageManager'
import { useRelayAdmin } from '@/providers/RelayAdminProvider'
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
  const { isEmbedded, isAdmin } = useRelayAdmin()
  const { clearColumn } = useKeyboardNavigation()

  if (!isEmbedded || !isAdmin) return null

  const handleClick = () => {
    navigate('relay')
    clearColumn(1)
  }

  return (
    <SidebarItem
      title="Relay Admin"
      onClick={handleClick}
      collapse={collapse}
      active={display && current === 'relay'}
      navIndex={navIndex}
    >
      <Server />
    </SidebarItem>
  )
}
