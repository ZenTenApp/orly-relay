import { toSettings } from '@/lib/link'
import { usePrimaryPage, useSecondaryPage } from '@/PageManager'
import { useUserPreferences } from '@/providers/UserPreferencesProvider'
import { Settings } from 'lucide-react'
import BottomNavigationBarItem from './BottomNavigationBarItem'

export default function SettingsButton() {
  const { current, navigate, display } = usePrimaryPage()
  const { push } = useSecondaryPage()
  const { enableSingleColumnLayout } = useUserPreferences()

  return (
    <BottomNavigationBarItem
      active={current === 'settings' && display}
      onClick={() => (enableSingleColumnLayout ? navigate('settings') : push(toSettings()))}
    >
      <Settings />
    </BottomNavigationBarItem>
  )
}
