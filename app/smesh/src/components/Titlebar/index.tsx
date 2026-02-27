import iconDark from '@/assets/smeshicondark.png'
import iconLight from '@/assets/smeshiconlight.png'
import { cn } from '@/lib/utils'
import { useSidebarDrawer } from '@/PageManager'
import { useScreenSize } from '@/providers/ScreenSizeProvider'
import { useTheme } from '@/providers/ThemeProvider'

export function Titlebar({
  children,
  className,
  hideBottomBorder = false,
  hideMenuButton = false
}: {
  children?: React.ReactNode
  className?: string
  hideBottomBorder?: boolean
  hideMenuButton?: boolean
}) {
  const { isSmallScreen } = useScreenSize()

  return (
    <div
      className={cn(
        'sticky top-0 w-full h-12 z-40 [&_svg]:size-5 [&_svg]:shrink-0 select-none bg-background',
        !hideBottomBorder && 'border-b',
        className
      )}
    >
      <div className="flex items-center h-full w-full">
        {isSmallScreen && !hideMenuButton && <MenuButton />}
        <div className="flex-1 h-full">{children}</div>
      </div>
    </div>
  )
}

function MenuButton() {
  const { toggle } = useSidebarDrawer()
  const { theme } = useTheme()
  const iconSrc = theme === 'light' ? iconLight : iconDark

  return (
    <button
      onClick={toggle}
      className="flex items-center justify-center w-10 h-full hover:bg-accent transition-colors"
      aria-label="Open menu"
    >
      <img src={iconSrc} alt="Menu" className="w-6 h-6" />
    </button>
  )
}
