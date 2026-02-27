import { useTheme } from '@/providers/ThemeProvider'
import iconLight from './smeshiconlight.png'
import iconDark from './smeshicondark.png'

export default function Icon({ className }: { className?: string }) {
  const { theme } = useTheme()
  const iconSrc = theme === 'light' ? iconLight : iconDark

  return <img src={iconSrc} alt="Smesh" className={className} />
}
