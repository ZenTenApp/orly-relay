import { toHelp } from '@/lib/link'
import { usePrimaryPage, useSecondaryPage } from '@/PageManager'
import { useKeyboardNavigation } from '@/providers/KeyboardNavigationProvider'
import { useUserPreferences } from '@/providers/UserPreferencesProvider'
import { HelpCircle } from 'lucide-react'
import SidebarItem from './SidebarItem'

export default function HelpButton({ collapse, navIndex }: { collapse: boolean; navIndex?: number }) {
  const { current, navigate, display } = usePrimaryPage()
  const { push } = useSecondaryPage()
  const { enableSingleColumnLayout } = useUserPreferences()
  const { clearColumn } = useKeyboardNavigation()

  const handleClick = () => {
    if (enableSingleColumnLayout) {
      navigate('help')
      clearColumn(1)
    } else {
      push(toHelp())
    }
  }

  return (
    <SidebarItem
      title="Help"
      onClick={handleClick}
      collapse={collapse}
      active={display && current === 'help'}
      navIndex={navIndex}
    >
      <HelpCircle />
    </SidebarItem>
  )
}
