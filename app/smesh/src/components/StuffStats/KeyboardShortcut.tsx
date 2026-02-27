import { useKeyboardNavigation } from '@/providers/KeyboardNavigationProvider'

export default function KeyboardShortcut({ shortcut }: { shortcut: string }) {
  const { isEnabled } = useKeyboardNavigation()

  if (!isEnabled) return null

  return (
    <kbd className="absolute -top-1.5 -right-2 text-[9px] font-mono opacity-50 group-hover:opacity-100">
      {shortcut}
    </kbd>
  )
}
