import { Button } from '@/components/ui/button'
import { Eye } from 'lucide-react'
import { useTranslation } from 'react-i18next'

export default function NsfwNote({ show }: { show: () => void }) {
  const { t } = useTranslation()

  return (
    <div className="flex flex-col gap-2 items-center text-muted-foreground font-medium my-4">
      <div>{t('🔞 NSFW 🔞')}</div>
      <Button
        onClick={(e) => {
          e.stopPropagation()
          show()
        }}
        variant="outline"
      >
        <Eye />
        {t('Temporarily display this note')}
      </Button>
    </div>
  )
}
