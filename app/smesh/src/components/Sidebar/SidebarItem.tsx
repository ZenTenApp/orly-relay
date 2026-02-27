import { Button, ButtonProps } from '@/components/ui/button'
import { useKeyboardNavigable } from '@/hooks/useKeyboardNavigable'
import { cn } from '@/lib/utils'
import { forwardRef, useCallback, useRef } from 'react'
import { useTranslation } from 'react-i18next'

const SidebarItem = forwardRef<
  HTMLButtonElement,
  ButtonProps & {
    title: string
    collapse: boolean
    description?: string
    active?: boolean
    navIndex?: number
  }
>(({ children, title, description, className, active, collapse, navIndex, onClick, ...props }, _ref) => {
  const { t } = useTranslation()
  const buttonRef = useRef<HTMLButtonElement>(null)

  const handleActivate = useCallback(() => {
    buttonRef.current?.click()
  }, [])

  const { ref: navRef, isSelected } = useKeyboardNavigable(0, navIndex ?? 0, {
    meta: { type: 'sidebar', onActivate: handleActivate }
  })

  return (
    <div ref={navRef}>
      <Button
        className={cn(
          'flex shadow-none items-center transition-colors duration-500 bg-transparent m-0 rounded-lg gap-4 text-lg font-semibold',
          collapse
            ? 'w-12 h-12 p-3 [&_svg]:size-full'
            : 'justify-start w-full h-auto py-2 px-3 [&_svg]:size-5',
          active && 'text-primary hover:text-primary bg-primary/10 hover:bg-primary/10',
          isSelected && 'ring-2 ring-primary ring-offset-2 ring-offset-background',
          className
        )}
        variant="ghost"
        title={t(title)}
        ref={buttonRef}
        onClick={onClick}
        {...props}
      >
        {children}
        {!collapse && <div>{t(description ?? title)}</div>}
      </Button>
    </div>
  )
})
SidebarItem.displayName = 'SidebarItem'
export default SidebarItem
