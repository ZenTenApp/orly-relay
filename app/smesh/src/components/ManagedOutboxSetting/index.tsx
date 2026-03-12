import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import managedOutboxService from '@/services/managed-outbox.service'
import relayStatsService from '@/services/relay-stats.service'
import storage, { dispatchSettingsChanged } from '@/services/local-storage.service'
import type { TRelayEntry } from '@/types/relay-management'
import type { TOutboxMode } from '@/types/relay-management'
import { Check, ChevronDown, ChevronUp, Shield, ShieldAlert, ShieldOff, X } from 'lucide-react'
import { useCallback, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

type RelayTab = 'pending' | 'approved' | 'rejected'

function failureRateColor(rate: number): string {
  if (rate >= 0.99) return 'text-red-900 dark:text-red-300'
  if (rate > 0.5) return 'text-red-600 dark:text-red-400'
  if (rate > 0.1) return 'text-yellow-600 dark:text-yellow-400'
  return 'text-green-600 dark:text-green-400'
}

function RelayRow({ entry, onAction }: { entry: TRelayEntry; onAction: () => void }) {
  const { t } = useTranslation()
  const [expanded, setExpanded] = useState(false)
  const failureRate = relayStatsService.getFailureRate(entry.url)
  const autoDisabled = relayStatsService.isAutoDisabled(entry.url)

  return (
    <div className="border rounded-lg p-3 space-y-2">
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 min-w-0 flex-1">
          <button
            type="button"
            onClick={() => setExpanded(!expanded)}
            className="text-muted-foreground hover:text-foreground shrink-0"
          >
            {expanded ? <ChevronUp className="size-4" /> : <ChevronDown className="size-4" />}
          </button>
          <span className="truncate text-sm font-mono">{entry.url}</span>
          {autoDisabled && (
            <span title={t('Auto-disabled')}><ShieldAlert className="size-4 text-red-500 shrink-0" /></span>
          )}
          {entry.manualExclude && (
            <span title={t('Manually excluded')}><ShieldOff className="size-4 text-orange-500 shrink-0" /></span>
          )}
        </div>
        <div className="flex items-center gap-1 shrink-0">
          <span className="text-xs text-muted-foreground capitalize px-1.5 py-0.5 bg-muted rounded">
            {entry.direction}
          </span>
          <span className={`text-xs font-mono ${failureRateColor(failureRate)}`}>
            {(failureRate * 100).toFixed(0)}%
          </span>
        </div>
      </div>
      {expanded && (
        <div className="pl-6 space-y-2">
          {entry.reason && (
            <div className="text-xs text-muted-foreground">{entry.reason}</div>
          )}
          {entry.relayIp && (
            <div className="text-xs text-muted-foreground">IP: {entry.relayIp}</div>
          )}
          <div className="flex gap-2 flex-wrap">
            {entry.status !== 'approved' && (
              <Button
                size="sm"
                variant="outline"
                onClick={() => { managedOutboxService.approve(entry.url); onAction() }}
              >
                <Check className="size-3 mr-1" /> {t('Approve')}
              </Button>
            )}
            {entry.status !== 'rejected' && (
              <Button
                size="sm"
                variant="outline"
                onClick={() => { managedOutboxService.reject(entry.url); onAction() }}
              >
                <X className="size-3 mr-1" /> {t('Reject')}
              </Button>
            )}
            {entry.status !== 'pending' && (
              <Button
                size="sm"
                variant="outline"
                onClick={() => { managedOutboxService.resetStatus(entry.url); onAction() }}
              >
                {t('Reset')}
              </Button>
            )}
            <div className="flex items-center gap-1.5">
              <Switch
                checked={entry.manualExclude}
                onCheckedChange={(checked) => {
                  managedOutboxService.setManualExclude(entry.url, checked)
                  onAction()
                }}
              />
              <span className="text-xs text-muted-foreground">{t('Exclude')}</span>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default function ManagedOutboxSetting() {
  const { t } = useTranslation()
  const [outboxMode, setOutboxMode] = useState<TOutboxMode>(
    storage.getOutboxMode() as TOutboxMode
  )
  const [tab, setTab] = useState<RelayTab>('pending')
  const [refreshKey, setRefreshKey] = useState(0)

  const refresh = useCallback(() => setRefreshKey((k) => k + 1), [])

  const pending = useMemo(() => managedOutboxService.getPendingRelays(), [refreshKey])
  const approved = useMemo(() => managedOutboxService.getApprovedRelays(), [refreshKey])
  const rejected = useMemo(() => managedOutboxService.getRejectedRelays(), [refreshKey])
  const excluded = useMemo(() => managedOutboxService.getExcludedRelays(), [refreshKey])
  const autoDisabled = useMemo(() => managedOutboxService.getAutoDisabledRelays(), [refreshKey])

  const currentList = tab === 'pending' ? pending : tab === 'approved' ? approved : rejected

  const handleModeChange = (mode: string) => {
    storage.setOutboxMode(mode)
    setOutboxMode(mode as TOutboxMode)
    dispatchSettingsChanged()
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <Label className="text-base font-normal">{t('Outbox mode')}</Label>
        <Select value={outboxMode} onValueChange={handleModeChange}>
          <SelectTrigger className="w-40">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="automatic">{t('Automatic')}</SelectItem>
            <SelectItem value="managed">{t('Managed')}</SelectItem>
          </SelectContent>
        </Select>
      </div>

      <div className="flex flex-wrap gap-2 text-xs text-muted-foreground">
        <span>{pending.length} {t('pending')}</span>
        <span>·</span>
        <span>{approved.length} {t('approved')}</span>
        <span>·</span>
        <span>{rejected.length} {t('rejected')}</span>
        <span>·</span>
        <span>{excluded.length} {t('excluded')}</span>
        <span>·</span>
        <span>{autoDisabled.length} {t('auto-disabled')}</span>
      </div>

      <div className="flex gap-1 border-b">
        {(['pending', 'approved', 'rejected'] as const).map((t_) => (
          <button
            key={t_}
            type="button"
            onClick={() => setTab(t_)}
            className={`px-3 py-1.5 text-sm capitalize border-b-2 transition-colors ${
              tab === t_
                ? 'border-primary text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground'
            }`}
          >
            {t(t_)} ({t_ === 'pending' ? pending.length : t_ === 'approved' ? approved.length : rejected.length})
          </button>
        ))}
      </div>

      {tab === 'pending' && pending.length > 1 && (
        <div className="flex gap-2">
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              managedOutboxService.bulkApprove(pending.map((e) => e.url))
              refresh()
            }}
          >
            <Shield className="size-3 mr-1" /> {t('Approve all')}
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={() => {
              managedOutboxService.bulkReject(pending.map((e) => e.url))
              refresh()
            }}
          >
            <ShieldOff className="size-3 mr-1" /> {t('Reject all')}
          </Button>
        </div>
      )}

      <div className="space-y-2 max-h-96 overflow-y-auto">
        {currentList.length === 0 ? (
          <div className="text-sm text-muted-foreground py-4 text-center">
            {t('No relays')}
          </div>
        ) : (
          currentList.map((entry) => (
            <RelayRow key={entry.url} entry={entry} onAction={refresh} />
          ))
        )}
      </div>
    </div>
  )
}
