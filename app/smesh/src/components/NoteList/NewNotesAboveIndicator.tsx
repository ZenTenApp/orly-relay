import { Button } from '@/components/ui/button'
import { ArrowUp } from 'lucide-react'
import { useTranslation } from 'react-i18next'

export default function NewNotesAboveIndicator({
  count,
  onClick
}: {
  count: number
  onClick: () => void
}) {
  const { t } = useTranslation()

  if (count <= 0) return null

  return (
    <div className="sticky top-[calc(6rem+1px)] z-40 flex justify-center pointer-events-none">
      <Button
        onClick={onClick}
        variant="secondary"
        className="rounded-full h-8 px-3 gap-1.5 shadow-md pointer-events-auto"
        size="sm"
      >
        <ArrowUp className="size-3.5" />
        <span className="text-sm">
          {t('n new notes above', { n: count > 99 ? '99+' : count })}
        </span>
      </Button>
    </div>
  )
}
