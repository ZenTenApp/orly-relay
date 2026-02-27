import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useKeyboardNavigation } from '@/providers/KeyboardNavigationProvider'
import { Keyboard } from 'lucide-react'
import { useTranslation } from 'react-i18next'

export default function KeyboardModeButton({ collapse }: { collapse: boolean }) {
  const { t } = useTranslation()
  const { isEnabled, toggleKeyboardMode } = useKeyboardNavigation()

  return (
    <Button
      className={cn(
        'flex shadow-none items-center transition-colors duration-500 bg-transparent m-0 rounded-lg gap-2 text-sm font-semibold',
        collapse
          ? 'w-12 h-12 p-3 [&_svg]:size-full'
          : 'justify-start w-full h-auto py-2 px-3 [&_svg]:size-5',
        isEnabled && 'text-primary hover:text-primary bg-primary/10 hover:bg-primary/10'
      )}
      variant="ghost"
      title={t('Toggle keyboard navigation (⇧K)')}
      onClick={toggleKeyboardMode}
    >
      <Keyboard />
      {!collapse && (
        <div className="flex items-center gap-2">
          <span>{t('Keyboard')}</span>
          <kbd className="text-xs px-1.5 py-0.5 rounded bg-muted text-muted-foreground border">⇧K</kbd>
        </div>
      )}
      {collapse && (
        <span className="sr-only">{t('Toggle keyboard navigation')}</span>
      )}
    </Button>
  )
}
