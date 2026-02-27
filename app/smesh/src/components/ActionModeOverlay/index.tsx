import { cn } from '@/lib/utils'
import { TActionType, useKeyboardNavigation } from '@/providers/KeyboardNavigationProvider'
import { MessageSquare, Repeat2, Quote, Heart, Zap } from 'lucide-react'

const ACTIONS: { type: TActionType; icon: typeof MessageSquare; label: string }[] = [
  { type: 'reply', icon: MessageSquare, label: 'Reply' },
  { type: 'repost', icon: Repeat2, label: 'Repost' },
  { type: 'quote', icon: Quote, label: 'Quote' },
  { type: 'react', icon: Heart, label: 'React' },
  { type: 'zap', icon: Zap, label: 'Zap' }
]

export default function ActionModeOverlay() {
  const { actionMode, isEnabled } = useKeyboardNavigation()

  if (!isEnabled || !actionMode.active) return null

  return (
    <div className="fixed bottom-20 left-1/2 -translate-x-1/2 z-50 pointer-events-none">
      <div className="flex gap-1 bg-background/95 backdrop-blur-sm border rounded-full px-3 py-2 shadow-lg">
        {ACTIONS.map(({ type, icon: Icon, label }) => (
          <div
            key={type}
            className={cn(
              'flex flex-col items-center gap-1 p-2 rounded-full transition-all duration-150',
              actionMode.selectedAction === type
                ? 'bg-primary text-primary-foreground scale-110'
                : 'text-muted-foreground'
            )}
            title={label}
          >
            <Icon className="size-5" />
          </div>
        ))}
      </div>
      <div className="text-center text-xs text-muted-foreground mt-2">
        Tab to cycle, Enter to activate, Esc to cancel
      </div>
    </div>
  )
}
