import { useTheme } from '@/providers/ThemeProvider'
import logoLight from './smeshlight.png'
import logoDark from './smeshdark.png'

export default function Logo({ className }: { className?: string }) {
  const { theme } = useTheme()
  const logoSrc = theme === 'light' ? logoLight : logoDark

  return <img src={logoSrc} alt="Smesh" className={className} />
}
