import { toSettings } from '@/lib/link'
import { usePrimaryPage, useSecondaryPage } from '@/PageManager'
import { useKeyboardNavigation } from '@/providers/KeyboardNavigationProvider'
import { useUserPreferences } from '@/providers/UserPreferencesProvider'
import { Settings } from 'lucide-react'
import SidebarItem from './SidebarItem'

export default function SettingsButton({ collapse, navIndex }: { collapse: boolean; navIndex?: number }) {
  const { current, navigate, display } = usePrimaryPage()
  const { push } = useSecondaryPage()
  const { enableSingleColumnLayout } = useUserPreferences()
  const { clearColumn } = useKeyboardNavigation()

  const handleClick = () => {
    if (enableSingleColumnLayout) {
      navigate('settings')
      clearColumn(1)
    } else {
      push(toSettings())
    }
  }

  return (
    <SidebarItem
      title="Settings"
      onClick={handleClick}
      collapse={collapse}
      active={display && current === 'settings'}
      navIndex={navIndex}
    >
      <Settings />
    </SidebarItem>
  )
}
