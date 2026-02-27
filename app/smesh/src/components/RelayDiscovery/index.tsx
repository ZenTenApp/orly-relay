/**
 * Relay Discovery Tool
 *
 * Discovers all known relays on the Nostr network and displays them
 * sorted by frequency of occurrence in NIP-65 relay lists.
 */

import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Slider } from '@/components/ui/slider'
import relayDiscoveryService, {
  DiscoveryProgress,
  DiscoveryResult,
  RelayFrequency
} from '@/services/relay-discovery.service'
import storage from '@/services/local-storage.service'
import { Copy, Download, Loader2, Play, RefreshCw, Square } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

export default function RelayDiscovery() {
  const { t } = useTranslation()
  const [isRunning, setIsRunning] = useState(false)
  const [progress, setProgress] = useState<DiscoveryProgress | null>(null)
  const [result, setResult] = useState<DiscoveryResult | null>(null)
  const [copied, setCopied] = useState(false)
  const [fallbackCount, setFallbackCount] = useState(storage.getFallbackRelayCount())

  // Load cached result on mount
  useEffect(() => {
    const cached = relayDiscoveryService.getCachedResult()
    if (cached) {
      setResult(cached)
    }
  }, [])

  const handleStart = useCallback(async () => {
    setIsRunning(true)
    setProgress({
      phase: 'phase1',
      relaysQueried: 0,
      totalRelays: 0,
      eventsFound: 0,
      uniqueRelaysFound: 0
    })

    try {
      const discoveryResult = await relayDiscoveryService.discover((prog) => {
        setProgress(prog)
      })
      setResult(discoveryResult)
      toast.success(t('Discovery complete'), {
        description: `${discoveryResult.relays.length} relays found`
      })
    } catch (err) {
      console.error('[RelayDiscovery] Error:', err)
      toast.error(t('Discovery failed'))
    } finally {
      setIsRunning(false)
      setProgress(null)
    }
  }, [t])

  const handleStop = useCallback(() => {
    relayDiscoveryService.abort()
    setIsRunning(false)
    setProgress(null)
  }, [])

  const handleRefresh = useCallback(() => {
    relayDiscoveryService.clearCache()
    setResult(null)
    handleStart()
  }, [handleStart])

  const handleCopy = useCallback(() => {
    if (!result) return
    const text = relayDiscoveryService.exportAsPlaintext(result.relays)
    navigator.clipboard.writeText(text)
    setCopied(true)
    setTimeout(() => setCopied(false), 2000)
    toast.success(t('Copied to clipboard'))
  }, [result, t])

  const handleDownload = useCallback(() => {
    if (!result) return
    relayDiscoveryService.downloadAsFile(result.relays)
    toast.success(t('Downloaded'))
  }, [result, t])

  const getPhaseLabel = (phase: string): string => {
    switch (phase) {
      case 'phase1':
        return t('Phase 1: Querying bootstrap relays')
      case 'phase2':
        return t('Phase 2: Querying discovered relays')
      case 'complete':
        return t('Complete')
      default:
        return ''
    }
  }

  const getProgressPercent = (): number => {
    if (!progress) return 0
    if (progress.totalRelays === 0) return 0

    const basePercent = progress.phase === 'phase1' ? 0 : 50
    const phasePercent = (progress.relaysQueried / progress.totalRelays) * 50
    return Math.round(basePercent + phasePercent)
  }

  return (
    <div className="space-y-4">
      <div className="text-sm text-muted-foreground">
        {t('Discover all known relays on the Nostr network by querying NIP-65 relay lists.')}
      </div>

      {/* Fallback Relay Configuration */}
      <div className="space-y-2">
        <Label>
          {t('Fallback relay count')}: {fallbackCount}
        </Label>
        <div className="text-xs text-muted-foreground">
          {t('Number of top discovered relays to search when notes aren\'t found.')}
        </div>
        <Slider
          value={[fallbackCount]}
          onValueChange={([value]) => {
            setFallbackCount(value)
            storage.setFallbackRelayCount(value)
          }}
          min={3}
          max={50}
          step={1}
          disabled={!result}
        />
        <div className="flex justify-between text-xs text-muted-foreground">
          <span>3</span>
          <span>50</span>
        </div>
      </div>

      {/* Controls */}
      <div className="flex gap-2 flex-wrap">
        {!isRunning ? (
          <>
            {!result ? (
              <Button onClick={handleStart} size="sm">
                <Play className="h-4 w-4 mr-2" />
                {t('Start Discovery')}
              </Button>
            ) : (
              <Button onClick={handleRefresh} size="sm" variant="outline">
                <RefreshCw className="h-4 w-4 mr-2" />
                {t('Refresh')}
              </Button>
            )}
          </>
        ) : (
          <Button onClick={handleStop} size="sm" variant="destructive">
            <Square className="h-4 w-4 mr-2" />
            {t('Stop')}
          </Button>
        )}

        {result && !isRunning && (
          <>
            <Button onClick={handleCopy} size="sm" variant="outline">
              <Copy className="h-4 w-4 mr-2" />
              {copied ? t('Copied!') : t('Copy')}
            </Button>
            <Button onClick={handleDownload} size="sm" variant="outline">
              <Download className="h-4 w-4 mr-2" />
              {t('Download')}
            </Button>
          </>
        )}
      </div>

      {/* Progress */}
      {isRunning && progress && (
        <div className="space-y-2">
          <div className="flex items-center gap-2 text-sm">
            <Loader2 className="h-4 w-4 animate-spin" />
            <span>{getPhaseLabel(progress.phase)}</span>
          </div>
          <div className="h-2 w-full rounded-full bg-muted overflow-hidden">
            <div
              className="h-full bg-primary transition-all duration-300"
              style={{ width: `${getProgressPercent()}%` }}
            />
          </div>
          <div className="text-xs text-muted-foreground">
            {t('Relays queried')}: {progress.relaysQueried}/{progress.totalRelays} |{' '}
            {t('Events found')}: {progress.eventsFound} |{' '}
            {t('Unique relays')}: {progress.uniqueRelaysFound}
          </div>
        </div>
      )}

      {/* Results */}
      {result && !isRunning && (
        <div className="space-y-2">
          <div className="text-sm font-medium">
            {t('Found {{count}} relays from {{events}} relay list events', {
              count: result.relays.length,
              events: result.totalEvents
            })}
          </div>
          <div className="text-xs text-muted-foreground">
            {t('Last updated')}: {new Date(result.timestamp).toLocaleString()}
          </div>

          <ScrollArea className="h-[300px] rounded-md border">
            <div className="p-2">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b">
                    <th className="text-left py-2 px-2">#</th>
                    <th className="text-left py-2 px-2">{t('Relay URL')}</th>
                    <th className="text-right py-2 px-2">{t('Count')}</th>
                    <th className="text-right py-2 px-2">%</th>
                  </tr>
                </thead>
                <tbody>
                  {result.relays.map((relay, index) => (
                    <RelayRow key={relay.url} relay={relay} index={index + 1} />
                  ))}
                </tbody>
              </table>
            </div>
          </ScrollArea>
        </div>
      )}
    </div>
  )
}

function RelayRow({ relay, index }: { relay: RelayFrequency; index: number }) {
  return (
    <tr className="border-b border-border/50 hover:bg-muted/50">
      <td className="py-1.5 px-2 text-muted-foreground">{index}</td>
      <td className="py-1.5 px-2 font-mono text-xs break-all">{relay.url}</td>
      <td className="py-1.5 px-2 text-right tabular-nums">{relay.count}</td>
      <td className="py-1.5 px-2 text-right tabular-nums text-muted-foreground">
        {relay.percentage}%
      </td>
    </tr>
  )
}
