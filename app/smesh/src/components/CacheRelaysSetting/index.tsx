/**
 * Cache Relays Setting Component
 *
 * Configure NRC connections as cache relays for faster event loading.
 * Cache relays are queried first (400ms timeout) before regular relays,
 * and loaded events are pushed to them in background.
 */

import { useState, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { useNostr } from '@/providers/NostrProvider'
import nrcCacheRelayService, { NRCCacheRelayConfig } from '@/services/nrc/nrc-cache-relay.service'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger
} from '@/components/ui/alert-dialog'
import { Plus, Trash2, Zap, Database } from 'lucide-react'

export default function CacheRelaysSetting() {
  const { t } = useTranslation()
  const { pubkey } = useNostr()

  // Cache relay state
  const [cacheRelays, setCacheRelays] = useState<NRCCacheRelayConfig[]>(nrcCacheRelayService.getAll())
  const [isDialogOpen, setIsDialogOpen] = useState(false)
  const [uri, setUri] = useState('')
  const [label, setLabel] = useState('')
  const [queryFirst, setQueryFirst] = useState(true)
  const [pushEvents, setPushEvents] = useState(true)
  const [isLoading, setIsLoading] = useState(false)
  const [isTesting, setIsTesting] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  const handleAdd = useCallback(async () => {
    if (!uri.trim() || !label.trim()) return

    setIsLoading(true)
    setError(null)
    try {
      // Test connection first
      await nrcCacheRelayService.testConnection(uri.trim())

      // Add the cache relay
      nrcCacheRelayService.add({
        uri: uri.trim(),
        label: label.trim(),
        enabled: true,
        queryFirst,
        pushEvents
      })

      setCacheRelays(nrcCacheRelayService.getAll())
      setIsDialogOpen(false)
      setUri('')
      setLabel('')
      setQueryFirst(true)
      setPushEvents(true)
    } catch (err) {
      console.error('Failed to add cache relay:', err)
      setError(err instanceof Error ? err.message : 'Connection failed')
    } finally {
      setIsLoading(false)
    }
  }, [uri, label, queryFirst, pushEvents])

  const handleRemove = useCallback((id: string) => {
    nrcCacheRelayService.remove(id)
    setCacheRelays(nrcCacheRelayService.getAll())
  }, [])

  const handleToggleEnabled = useCallback((id: string, enabled: boolean) => {
    nrcCacheRelayService.update(id, { enabled })
    setCacheRelays(nrcCacheRelayService.getAll())
  }, [])

  const handleToggleQueryFirst = useCallback((id: string, queryFirst: boolean) => {
    nrcCacheRelayService.update(id, { queryFirst })
    setCacheRelays(nrcCacheRelayService.getAll())
  }, [])

  const handleTogglePushEvents = useCallback((id: string, pushEvents: boolean) => {
    nrcCacheRelayService.update(id, { pushEvents })
    setCacheRelays(nrcCacheRelayService.getAll())
  }, [])

  const handleTest = useCallback(async (id: string, relayUri: string) => {
    setIsTesting(id)
    setError(null)
    try {
      await nrcCacheRelayService.testConnection(relayUri)
      nrcCacheRelayService.update(id, { lastConnected: Date.now(), lastError: undefined })
      setCacheRelays(nrcCacheRelayService.getAll())
    } catch (err) {
      const errorMsg = err instanceof Error ? err.message : 'Connection failed'
      nrcCacheRelayService.update(id, { lastError: errorMsg })
      setCacheRelays(nrcCacheRelayService.getAll())
    } finally {
      setIsTesting(null)
    }
  }, [])

  if (!pubkey) {
    return (
      <div className="text-muted-foreground text-sm p-4 text-center">
        {t('Login required to configure cache relays')}
      </div>
    )
  }

  return (
    <div className="space-y-4">
      <div className="p-3 bg-muted/30 rounded-lg">
        <p className="text-sm text-muted-foreground">
          {t('Cache relays store events via NRC for faster loading. They are queried first (400ms timeout) before regular relays.')}
        </p>
      </div>

      {/* Header with Add button */}
      <div className="flex items-center justify-between">
        <Label className="flex items-center gap-2">
          <Database className="w-4 h-4" />
          {t('Cache Relays')}
        </Label>
        <Button
          variant="outline"
          size="sm"
          onClick={() => setIsDialogOpen(true)}
          className="gap-1"
        >
          <Plus className="w-4 h-4" />
          {t('Add')}
        </Button>
      </div>

      {/* Cache Relays List */}
      {cacheRelays.length === 0 ? (
        <div className="text-sm text-muted-foreground p-4 text-center border border-dashed rounded-lg">
          {t('No cache relays configured')}
        </div>
      ) : (
        <div className="space-y-2">
          {cacheRelays.map((relay) => (
            <div
              key={relay.id}
              className="p-3 bg-muted/30 rounded-lg space-y-3"
            >
              <div className="flex items-center justify-between">
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="font-medium truncate">{relay.label}</span>
                    {relay.lastError && (
                      <span className="text-xs text-destructive">({t('Error')})</span>
                    )}
                    {relay.lastConnected && !relay.lastError && (
                      <span className="text-xs text-green-500">({t('Connected')})</span>
                    )}
                  </div>
                  <div className="text-xs text-muted-foreground truncate">
                    {relay.uri.length > 60 ? relay.uri.substring(0, 60) + '...' : relay.uri}
                  </div>
                </div>
                <div className="flex items-center gap-1">
                  <Switch
                    checked={relay.enabled}
                    onCheckedChange={(checked) => handleToggleEnabled(relay.id, checked)}
                    title={t('Enable/Disable')}
                  />
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => handleTest(relay.id, relay.uri)}
                    disabled={isTesting === relay.id}
                    title={t('Test Connection')}
                  >
                    <Zap className={`w-4 h-4 ${isTesting === relay.id ? 'animate-pulse' : ''}`} />
                  </Button>
                  <AlertDialog>
                    <AlertDialogTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="text-destructive hover:text-destructive"
                        title={t('Remove')}
                      >
                        <Trash2 className="w-4 h-4" />
                      </Button>
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>{t('Remove Cache Relay?')}</AlertDialogTitle>
                        <AlertDialogDescription>
                          {t('This will remove "{{label}}" from your cache relays.', {
                            label: relay.label
                          })}
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
                        <AlertDialogAction
                          onClick={() => handleRemove(relay.id)}
                          className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                        >
                          {t('Remove')}
                        </AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                </div>
              </div>

              {/* Options */}
              <div className="flex items-center gap-4 text-sm">
                <label className="flex items-center gap-2">
                  <Switch
                    checked={relay.queryFirst}
                    onCheckedChange={(checked) => handleToggleQueryFirst(relay.id, checked)}
                    disabled={!relay.enabled}
                  />
                  <span className="text-muted-foreground">{t('Query first')}</span>
                </label>
                <label className="flex items-center gap-2">
                  <Switch
                    checked={relay.pushEvents}
                    onCheckedChange={(checked) => handleTogglePushEvents(relay.id, checked)}
                    disabled={!relay.enabled}
                  />
                  <span className="text-muted-foreground">{t('Push events')}</span>
                </label>
              </div>

              {relay.lastError && (
                <div className="text-xs text-destructive">
                  {relay.lastError}
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Add Cache Relay Dialog */}
      <Dialog open={isDialogOpen} onOpenChange={setIsDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Add Cache Relay')}</DialogTitle>
            <DialogDescription>
              {t('Add an NRC connection as a cache relay for faster event loading')}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="cache-relay-uri">{t('Connection URI')}</Label>
              <Input
                id="cache-relay-uri"
                value={uri}
                onChange={(e) => setUri(e.target.value)}
                placeholder="nostr+relayconnect://..."
                className="font-mono text-xs"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="cache-relay-label">{t('Label')}</Label>
              <Input
                id="cache-relay-label"
                value={label}
                onChange={(e) => setLabel(e.target.value)}
                placeholder={t('e.g., Home Relay, Personal Cache')}
              />
            </div>
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <div>
                  <Label>{t('Query first')}</Label>
                  <p className="text-xs text-muted-foreground">
                    {t('Check cache relay before regular relays (400ms timeout)')}
                  </p>
                </div>
                <Switch
                  checked={queryFirst}
                  onCheckedChange={setQueryFirst}
                />
              </div>
              <div className="flex items-center justify-between">
                <div>
                  <Label>{t('Push events')}</Label>
                  <p className="text-xs text-muted-foreground">
                    {t('Store loaded events in cache relay in background')}
                  </p>
                </div>
                <Switch
                  checked={pushEvents}
                  onCheckedChange={setPushEvents}
                />
              </div>
            </div>
            {error && (
              <div className="text-sm text-destructive">{error}</div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => {
              setIsDialogOpen(false)
              setError(null)
            }}>
              {t('Cancel')}
            </Button>
            <Button
              onClick={handleAdd}
              disabled={!uri.trim() || !label.trim() || isLoading}
            >
              {isLoading ? t('Testing...') : t('Add')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
