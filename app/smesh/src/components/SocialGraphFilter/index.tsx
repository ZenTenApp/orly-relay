import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { useSocialGraphFilter } from '@/providers/SocialGraphFilterProvider'
import { Loader2, Minus, Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'

const DEPTH_LABELS: Record<number, string> = {
  1: 'Direct follows',
  2: 'Follows of follows'
}

interface SocialGraphFilterProps {
  temporaryProximity: number | null
  temporaryIncludeMode: boolean
  onTemporaryProximityChange: (level: number | null) => void
  onTemporaryIncludeModeChange: (include: boolean) => void
}

export default function SocialGraphFilter({
  temporaryProximity,
  temporaryIncludeMode,
  onTemporaryProximityChange,
  onTemporaryIncludeModeChange
}: SocialGraphFilterProps) {
  const { t } = useTranslation()
  const { graphPubkeyCount, isLoading } = useSocialGraphFilter()

  const isEnabled = temporaryProximity !== null
  const depth = temporaryProximity ?? 1

  const handleToggle = (enabled: boolean) => {
    onTemporaryProximityChange(enabled ? 1 : null)
  }

  const handleIncrease = () => {
    if (depth < 2) {
      onTemporaryProximityChange(depth + 1)
    }
  }

  const handleDecrease = () => {
    if (depth > 1) {
      onTemporaryProximityChange(depth - 1)
    }
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <Label htmlFor="social-graph-filter" className="font-medium">
          {t('Social graph filter')}
        </Label>
        <Switch id="social-graph-filter" checked={isEnabled} onCheckedChange={handleToggle} />
      </div>

      {isEnabled && (
        <>
          {/* Include/Exclude toggle */}
          <div className="flex items-center gap-2">
            <Button
              variant={temporaryIncludeMode ? 'default' : 'outline'}
              size="sm"
              className="flex-1"
              onClick={() => onTemporaryIncludeModeChange(true)}
            >
              {t('Include')}
            </Button>
            <Button
              variant={!temporaryIncludeMode ? 'default' : 'outline'}
              size="sm"
              className="flex-1"
              onClick={() => onTemporaryIncludeModeChange(false)}
            >
              {t('Exclude')}
            </Button>
          </div>

          {/* Depth stepper */}
          <div className="flex items-center justify-between rounded-lg border px-3 py-2">
            <div className="flex-1">
              <p className="text-sm font-medium">{t(DEPTH_LABELS[depth])}</p>
              <p className="text-xs text-muted-foreground">
                {isLoading ? (
                  <span className="flex items-center gap-1">
                    <Loader2 className="h-3 w-3 animate-spin" />
                    {t('Loading...')}
                  </span>
                ) : (
                  t('{{count}} users', { count: graphPubkeyCount })
                )}
              </p>
            </div>
            <div className="flex items-center gap-1">
              <Button
                variant="outline"
                size="icon"
                className="h-8 w-8"
                onClick={handleDecrease}
                disabled={depth <= 1}
              >
                <Minus className="h-4 w-4" />
              </Button>
              <span className="w-6 text-center text-sm font-medium">{depth}</span>
              <Button
                variant="outline"
                size="icon"
                className="h-8 w-8"
                onClick={handleIncrease}
                disabled={depth >= 2}
              >
                <Plus className="h-4 w-4" />
              </Button>
            </div>
          </div>

          {/* Mode description */}
          <p className="text-xs text-muted-foreground">
            {temporaryIncludeMode
              ? t('Only show notes from users in your social graph')
              : t('Hide notes from users in your social graph')}
          </p>
        </>
      )}
    </div>
  )
}
