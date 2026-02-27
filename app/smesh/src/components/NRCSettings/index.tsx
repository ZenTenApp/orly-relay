/**
 * NRC Settings Component
 *
 * UI for managing Nostr Relay Connect (NRC) connections and listener settings.
 * Includes both:
 * - Listener mode: Allow other devices to connect to this one
 * - Client mode: Connect to and sync from other devices
 */

import { useState, useCallback, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { useNRC } from '@/providers/NRCProvider'
import { useNostr } from '@/providers/NostrProvider'
import storage, { dispatchSettingsChanged } from '@/services/local-storage.service'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
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
import {
  Link2,
  Plus,
  Trash2,
  Copy,
  Check,
  QrCode,
  Wifi,
  WifiOff,
  Users,
  Server,
  RefreshCw,
  Smartphone,
  Download,
  Camera,
  Zap
} from 'lucide-react'
import { NRCConnection, RemoteConnection } from '@/services/nrc'
import QRCode from 'qrcode'
import { Html5Qrcode } from 'html5-qrcode'

export default function NRCSettings() {
  const { t } = useTranslation()
  const { pubkey } = useNostr()
  const {
    // Listener state
    isEnabled,
    isConnected,
    connections,
    activeSessions,
    rendezvousUrl,
    enable,
    disable,
    addConnection,
    removeConnection,
    getConnectionURI,
    setRendezvousUrl,
    // Client state
    remoteConnections,
    isSyncing,
    syncProgress,
    addRemoteConnection,
    removeRemoteConnection,
    testRemoteConnection,
    syncFromDevice,
    syncAllRemotes
  } = useNRC()

  // Listener state
  const [newConnectionLabel, setNewConnectionLabel] = useState('')
  const [newConnectionRendezvousUrl, setNewConnectionRendezvousUrl] = useState('')
  const [isAddDialogOpen, setIsAddDialogOpen] = useState(false)
  const [isQRDialogOpen, setIsQRDialogOpen] = useState(false)
  const [currentQRConnection, setCurrentQRConnection] = useState<NRCConnection | null>(null)
  const [currentQRUri, setCurrentQRUri] = useState('')
  const [qrDataUrl, setQrDataUrl] = useState('')
  const [copiedUri, setCopiedUri] = useState(false)
  const [isLoading, setIsLoading] = useState(false)
  const [enableError, setEnableError] = useState<string | null>(null)
  const [addConnectionError, setAddConnectionError] = useState<string | null>(null)

  // Client state
  const [connectionUri, setConnectionUri] = useState('')
  const [newRemoteLabel, setNewRemoteLabel] = useState('')
  const [isConnectDialogOpen, setIsConnectDialogOpen] = useState(false)
  const [isScannerOpen, setIsScannerOpen] = useState(false)
  const [scannerError, setScannerError] = useState('')
  const scannerRef = useRef<Html5Qrcode | null>(null)
  const scannerContainerRef = useRef<HTMLDivElement>(null)

  // Private config sync setting
  const [nrcOnlyConfigSync, setNrcOnlyConfigSync] = useState(storage.getNrcOnlyConfigSync())

  const handleToggleNrcOnlyConfig = useCallback((checked: boolean) => {
    storage.setNrcOnlyConfigSync(checked)
    setNrcOnlyConfigSync(checked)
    dispatchSettingsChanged()
  }, [])

  // Generate QR code when URI changes
  const generateQRCode = useCallback(async (uri: string) => {
    try {
      const dataUrl = await QRCode.toDataURL(uri, {
        width: 256,
        margin: 2,
        color: { dark: '#000000', light: '#ffffff' }
      })
      setQrDataUrl(dataUrl)
    } catch (error) {
      console.error('Failed to generate QR code:', error)
    }
  }, [])

  const handleToggleEnabled = useCallback(async () => {
    if (isEnabled) {
      disable()
      setEnableError(null)
    } else {
      setIsLoading(true)
      setEnableError(null)
      try {
        await enable()
      } catch (error) {
        const message = error instanceof Error ? error.message : 'Failed to enable NRC'
        setEnableError(message)
        console.error('Failed to enable NRC:', error)
      } finally {
        setIsLoading(false)
      }
    }
  }, [isEnabled, enable, disable])

  const handleAddConnection = useCallback(async () => {
    if (!newConnectionLabel.trim()) return

    setIsLoading(true)
    setAddConnectionError(null)
    try {
      // Use connection-specific URL if provided, otherwise uses global default
      const connectionRendezvousUrl = newConnectionRendezvousUrl.trim() || undefined
      const { uri, connection } = await addConnection(newConnectionLabel.trim(), connectionRendezvousUrl)
      setIsAddDialogOpen(false)
      setNewConnectionLabel('')
      setNewConnectionRendezvousUrl('')

      // Show QR code
      setCurrentQRConnection(connection)
      setCurrentQRUri(uri)
      await generateQRCode(uri)
      setIsQRDialogOpen(true)
    } catch (error) {
      const message = error instanceof Error ? error.message : 'Failed to add connection'
      setAddConnectionError(message)
      console.error('Failed to add connection:', error)
    } finally {
      setIsLoading(false)
    }
  }, [newConnectionLabel, newConnectionRendezvousUrl, addConnection])

  const handleShowQR = useCallback(
    async (connection: NRCConnection) => {
      try {
        const uri = getConnectionURI(connection)
        setCurrentQRConnection(connection)
        setCurrentQRUri(uri)
        await generateQRCode(uri)
        setIsQRDialogOpen(true)
      } catch (error) {
        console.error('Failed to get connection URI:', error)
      }
    },
    [getConnectionURI, generateQRCode]
  )

  const handleCopyUri = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(currentQRUri)
      setCopiedUri(true)
      setTimeout(() => setCopiedUri(false), 2000)
    } catch (error) {
      console.error('Failed to copy URI:', error)
    }
  }, [currentQRUri])

  const handleRemoveConnection = useCallback(
    async (id: string) => {
      try {
        await removeConnection(id)
      } catch (error) {
        console.error('Failed to remove connection:', error)
      }
    },
    [removeConnection]
  )

  // ===== Client Handlers =====
  const handleAddRemoteConnection = useCallback(async () => {
    if (!connectionUri.trim() || !newRemoteLabel.trim()) return

    setIsLoading(true)
    try {
      await addRemoteConnection(connectionUri.trim(), newRemoteLabel.trim())
      setIsConnectDialogOpen(false)
      setConnectionUri('')
      setNewRemoteLabel('')
    } catch (error) {
      console.error('Failed to add remote connection:', error)
    } finally {
      setIsLoading(false)
    }
  }, [connectionUri, newRemoteLabel, addRemoteConnection])

  const handleRemoveRemoteConnection = useCallback(
    async (id: string) => {
      try {
        await removeRemoteConnection(id)
      } catch (error) {
        console.error('Failed to remove remote connection:', error)
      }
    },
    [removeRemoteConnection]
  )

  const handleSyncDevice = useCallback(
    async (id: string) => {
      try {
        await syncFromDevice(id)
      } catch (error) {
        console.error('Failed to sync from device:', error)
      }
    },
    [syncFromDevice]
  )

  const handleTestConnection = useCallback(
    async (id: string) => {
      try {
        await testRemoteConnection(id)
      } catch (error) {
        console.error('Failed to test connection:', error)
      }
    },
    [testRemoteConnection]
  )

  const handleSyncAll = useCallback(async () => {
    try {
      await syncAllRemotes()
    } catch (error) {
      console.error('Failed to sync all remotes:', error)
    }
  }, [syncAllRemotes])

  const startScanner = useCallback(async () => {
    if (!scannerContainerRef.current) return

    setScannerError('')
    try {
      const scanner = new Html5Qrcode('qr-scanner-container')
      scannerRef.current = scanner

      await scanner.start(
        { facingMode: 'environment' },
        {
          fps: 10,
          qrbox: { width: 250, height: 250 }
        },
        (decodedText) => {
          // Found a QR code
          if (decodedText.startsWith('nostr+relayconnect://')) {
            setConnectionUri(decodedText)
            stopScanner()
            setIsScannerOpen(false)
            setIsConnectDialogOpen(true)
          }
        },
        () => {
          // Ignore errors while scanning
        }
      )
    } catch (error) {
      console.error('Failed to start scanner:', error)
      setScannerError(error instanceof Error ? error.message : 'Failed to start camera')
    }
  }, [])

  const stopScanner = useCallback(() => {
    if (scannerRef.current) {
      scannerRef.current.stop().catch(() => {
        // Ignore errors when stopping
      })
      scannerRef.current = null
    }
  }, [])

  const handleOpenScanner = useCallback(() => {
    setIsScannerOpen(true)
    // Start scanner after dialog renders
    setTimeout(startScanner, 100)
  }, [startScanner])

  const handleCloseScanner = useCallback(() => {
    stopScanner()
    setIsScannerOpen(false)
    setScannerError('')
  }, [stopScanner])

  if (!pubkey) {
    return (
      <div className="text-muted-foreground text-sm">
        {t('Login required to use NRC')}
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Private Configuration Sync Toggle */}
      <div className="flex items-center justify-between p-3 bg-muted/30 rounded-lg">
        <div className="space-y-1">
          <Label htmlFor="nrc-only-config" className="text-base font-medium">
            {t('Private Configuration Sync')}
          </Label>
          <p className="text-sm text-muted-foreground">
            {t('Only sync configurations between paired devices, not to public relays')}
          </p>
        </div>
        <Switch
          id="nrc-only-config"
          checked={nrcOnlyConfigSync}
          onCheckedChange={handleToggleNrcOnlyConfig}
        />
      </div>

      <Tabs defaultValue="listener" className="w-full">
        <TabsList className="grid w-full grid-cols-2">
          <TabsTrigger value="listener" className="gap-2">
            <Server className="w-4 h-4" />
            {t('Share')}
          </TabsTrigger>
          <TabsTrigger value="client" className="gap-2">
            <Smartphone className="w-4 h-4" />
            {t('Connect')}
          </TabsTrigger>
        </TabsList>

        {/* ===== LISTENER TAB ===== */}
        <TabsContent value="listener" className="space-y-6 mt-4">
          {/* Enable/Disable Toggle */}
          <div className="flex items-center justify-between">
            <div className="space-y-1">
              <Label htmlFor="nrc-enabled" className="text-base font-medium">
                {t('Enable Relay Connect')}
              </Label>
              <p className="text-sm text-muted-foreground">
                {t('Allow other devices to sync with this client')}
              </p>
            </div>
            <Switch
              id="nrc-enabled"
              checked={isEnabled}
              onCheckedChange={handleToggleEnabled}
              disabled={isLoading}
            />
          </div>

          {/* Enable Error */}
          {enableError && (
            <div className="p-3 bg-destructive/10 border border-destructive/20 rounded-lg text-sm text-destructive">
              {enableError}
            </div>
          )}

          {/* Status Indicator */}
          {isEnabled && (
            <div className="flex items-center gap-4 p-3 bg-muted/50 rounded-lg">
              <div className="flex items-center gap-2">
                {isConnected ? (
                  <Wifi className="w-4 h-4 text-green-500" />
                ) : (
                  <WifiOff className="w-4 h-4 text-yellow-500" />
                )}
                <span className="text-sm">
                  {isConnected ? t('Connected') : t('Connecting...')}
                </span>
              </div>
              {activeSessions > 0 && (
                <div className="flex items-center gap-2">
                  <Users className="w-4 h-4" />
                  <span className="text-sm">
                    {activeSessions} {t('active session(s)')}
                  </span>
                </div>
              )}
            </div>
          )}

          {/* Rendezvous Relay */}
          <div className="space-y-2">
            <Label htmlFor="rendezvous-url" className="flex items-center gap-2">
              <Server className="w-4 h-4" />
              {t('Rendezvous Relay')}
            </Label>
            <Input
              id="rendezvous-url"
              value={rendezvousUrl}
              onChange={(e) => setRendezvousUrl(e.target.value)}
              placeholder="wss://relay.example.com"
              disabled={isEnabled}
            />
            {isEnabled && (
              <p className="text-xs text-muted-foreground">
                {t('Disable NRC to change the relay')}
              </p>
            )}
          </div>

          {/* Connections List */}
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <Label className="flex items-center gap-2">
                <Link2 className="w-4 h-4" />
                {t('Authorized Devices')}
              </Label>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setIsAddDialogOpen(true)}
                className="gap-1"
              >
                <Plus className="w-4 h-4" />
                {t('Add')}
              </Button>
            </div>

            {connections.length === 0 ? (
              <div className="text-sm text-muted-foreground p-4 text-center border border-dashed rounded-lg">
                {t('No devices connected yet')}
              </div>
            ) : (
              <div className="space-y-2">
                {connections.map((connection) => (
                  <div
                    key={connection.id}
                    className="flex items-center justify-between p-3 bg-muted/30 rounded-lg"
                  >
                    <div className="flex-1 min-w-0">
                      <div className="font-medium truncate">{connection.label}</div>
                      <div className="text-xs text-muted-foreground">
                        {new Date(connection.createdAt).toLocaleDateString()}
                      </div>
                    </div>
                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => handleShowQR(connection)}
                        title={t('Show QR Code')}
                      >
                        <QrCode className="w-4 h-4" />
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
                            <AlertDialogTitle>{t('Remove Device?')}</AlertDialogTitle>
                            <AlertDialogDescription>
                              {t('This will revoke access for "{{label}}". The device will no longer be able to sync.', {
                                label: connection.label
                              })}
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
                            <AlertDialogAction
                              onClick={() => handleRemoveConnection(connection.id)}
                              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                            >
                              {t('Remove')}
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </TabsContent>

        {/* ===== CLIENT TAB ===== */}
        <TabsContent value="client" className="space-y-6 mt-4">
          {/* Sync Progress */}
          {isSyncing && syncProgress && (
            <div className="p-3 bg-muted/50 rounded-lg space-y-2">
              <div className="flex items-center gap-2">
                <RefreshCw className="w-4 h-4 animate-spin" />
                <span className="text-sm font-medium">
                  {syncProgress.phase === 'connecting' && t('Connecting...')}
                  {syncProgress.phase === 'requesting' && t('Requesting events...')}
                  {syncProgress.phase === 'receiving' && t('Receiving events...')}
                  {syncProgress.phase === 'complete' && t('Sync complete')}
                  {syncProgress.phase === 'error' && t('Error')}
                </span>
              </div>
              {syncProgress.eventsReceived > 0 && (
                <div className="text-xs text-muted-foreground">
                  {t('{{count}} events received', { count: syncProgress.eventsReceived })}
                </div>
              )}
              {syncProgress.message && syncProgress.phase === 'error' && (
                <div className="text-xs text-destructive">{syncProgress.message}</div>
              )}
            </div>
          )}

          {/* Connect to Device */}
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <Label className="flex items-center gap-2">
                <Download className="w-4 h-4" />
                {t('Remote Devices')}
              </Label>
              <div className="flex gap-2">
                <Button
                  variant="outline"
                  size="sm"
                  onClick={handleOpenScanner}
                  className="gap-1"
                >
                  <Camera className="w-4 h-4" />
                  {t('Scan')}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setIsConnectDialogOpen(true)}
                  className="gap-1"
                >
                  <Plus className="w-4 h-4" />
                  {t('Add')}
                </Button>
              </div>
            </div>

            {remoteConnections.length === 0 ? (
              <div className="text-sm text-muted-foreground p-4 text-center border border-dashed rounded-lg">
                {t('No remote devices configured')}
              </div>
            ) : (
              <div className="space-y-2">
                {/* Sync All Button */}
                {remoteConnections.length > 1 && (
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={handleSyncAll}
                    disabled={isSyncing}
                    className="w-full gap-2"
                  >
                    <RefreshCw className={`w-4 h-4 ${isSyncing ? 'animate-spin' : ''}`} />
                    {t('Sync All Devices')}
                  </Button>
                )}

                {remoteConnections.map((remote: RemoteConnection) => (
                  <div
                    key={remote.id}
                    className="flex items-center justify-between p-3 bg-muted/30 rounded-lg"
                  >
                    <div className="flex-1 min-w-0">
                      <div className="font-medium truncate">{remote.label}</div>
                      <div className="text-xs text-muted-foreground">
                        {remote.lastSync ? (
                          <>
                            {t('Last sync')}: {new Date(remote.lastSync).toLocaleString()}
                            {remote.eventCount !== undefined && (
                              <span className="ml-2">({remote.eventCount} {t('events')})</span>
                            )}
                          </>
                        ) : (
                          t('Never synced')
                        )}
                      </div>
                    </div>
                    <div className="flex items-center gap-1">
                      {/* Show Test button if never synced, Sync button otherwise */}
                      {!remote.lastSync ? (
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => handleTestConnection(remote.id)}
                          disabled={isSyncing}
                          title={t('Test Connection')}
                        >
                          <Zap className={`w-4 h-4 ${isSyncing ? 'animate-pulse' : ''}`} />
                        </Button>
                      ) : null}
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => handleSyncDevice(remote.id)}
                        disabled={isSyncing}
                        title={t('Sync')}
                      >
                        <RefreshCw className={`w-4 h-4 ${isSyncing ? 'animate-spin' : ''}`} />
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
                            <AlertDialogTitle>{t('Remove Remote Device?')}</AlertDialogTitle>
                            <AlertDialogDescription>
                              {t('This will remove "{{label}}" from your remote devices list.', {
                                label: remote.label
                              })}
                            </AlertDialogDescription>
                          </AlertDialogHeader>
                          <AlertDialogFooter>
                            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
                            <AlertDialogAction
                              onClick={() => handleRemoveRemoteConnection(remote.id)}
                              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                            >
                              {t('Remove')}
                            </AlertDialogAction>
                          </AlertDialogFooter>
                        </AlertDialogContent>
                      </AlertDialog>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </TabsContent>
      </Tabs>

      {/* ===== DIALOGS ===== */}

      {/* Add Connection Dialog (Listener) */}
      <Dialog open={isAddDialogOpen} onOpenChange={(open) => {
        setIsAddDialogOpen(open)
        if (open) {
          // Pre-populate with global rendezvous URL when opening
          setNewConnectionRendezvousUrl(rendezvousUrl)
          setAddConnectionError(null)
        } else {
          // Clear when closing
          setNewConnectionLabel('')
          setNewConnectionRendezvousUrl('')
          setAddConnectionError(null)
        }
      }}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Add Device')}</DialogTitle>
            <DialogDescription>
              {t('Create a connection URI to link another device')}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="device-label">{t('Device Name')}</Label>
              <Input
                id="device-label"
                value={newConnectionLabel}
                onChange={(e) => setNewConnectionLabel(e.target.value)}
                placeholder={t('e.g., Phone, Laptop')}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="device-rendezvous" className="flex items-center gap-2">
                <Server className="w-4 h-4" />
                {t('Rendezvous Relay')}
              </Label>
              <Input
                id="device-rendezvous"
                value={newConnectionRendezvousUrl}
                onChange={(e) => setNewConnectionRendezvousUrl(e.target.value)}
                placeholder="wss://relay.example.com"
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    handleAddConnection()
                  }
                }}
              />
              <p className="text-xs text-muted-foreground">
                {t('Relay used to establish the connection')}
              </p>
            </div>
            {addConnectionError && (
              <div className="p-3 bg-destructive/10 border border-destructive/20 rounded-lg text-sm text-destructive">
                {addConnectionError}
              </div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsAddDialogOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button
              onClick={handleAddConnection}
              disabled={!newConnectionLabel.trim() || isLoading}
            >
              {t('Create')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* QR Code Dialog */}
      <Dialog open={isQRDialogOpen} onOpenChange={setIsQRDialogOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t('Connection QR Code')}</DialogTitle>
            <DialogDescription>
              {currentQRConnection && (
                <>
                  {t('Scan this code with "{{label}}" to connect', {
                    label: currentQRConnection.label
                  })}
                </>
              )}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col items-center gap-4 py-4">
            {qrDataUrl && (
              <div className="p-4 bg-white rounded-lg">
                <img src={qrDataUrl} alt="Connection QR Code" className="w-64 h-64" />
              </div>
            )}
            <div className="w-full">
              <div className="flex items-center gap-2">
                <Input
                  value={currentQRUri}
                  readOnly
                  className="font-mono text-xs"
                />
                <Button
                  variant="outline"
                  size="icon"
                  onClick={handleCopyUri}
                  title={t('Copy')}
                >
                  {copiedUri ? (
                    <Check className="w-4 h-4 text-green-500" />
                  ) : (
                    <Copy className="w-4 h-4" />
                  )}
                </Button>
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button onClick={() => setIsQRDialogOpen(false)}>{t('Done')}</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Connect to Remote Dialog (Client) */}
      <Dialog open={isConnectDialogOpen} onOpenChange={setIsConnectDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t('Connect to Device')}</DialogTitle>
            <DialogDescription>
              {t('Enter a connection URI from another device to sync with it')}
            </DialogDescription>
          </DialogHeader>
          <div className="space-y-4 py-4">
            <div className="space-y-2">
              <Label htmlFor="connection-uri">{t('Connection URI')}</Label>
              <Input
                id="connection-uri"
                value={connectionUri}
                onChange={(e) => setConnectionUri(e.target.value)}
                placeholder="nostr+relayconnect://..."
                className="font-mono text-xs"
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="remote-label">{t('Device Name')}</Label>
              <Input
                id="remote-label"
                value={newRemoteLabel}
                onChange={(e) => setNewRemoteLabel(e.target.value)}
                placeholder={t('e.g., Desktop, Main Phone')}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    handleAddRemoteConnection()
                  }
                }}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setIsConnectDialogOpen(false)}>
              {t('Cancel')}
            </Button>
            <Button
              onClick={handleAddRemoteConnection}
              disabled={!connectionUri.trim() || !newRemoteLabel.trim() || isLoading}
            >
              {t('Connect')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* QR Scanner Dialog */}
      <Dialog open={isScannerOpen} onOpenChange={handleCloseScanner}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t('Scan QR Code')}</DialogTitle>
            <DialogDescription>
              {t('Point your camera at a connection QR code')}
            </DialogDescription>
          </DialogHeader>
          <div className="py-4">
            <div
              id="qr-scanner-container"
              ref={scannerContainerRef}
              className="w-full aspect-square bg-muted rounded-lg overflow-hidden"
            />
            {scannerError && (
              <div className="mt-2 text-sm text-destructive">{scannerError}</div>
            )}
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={handleCloseScanner}>
              {t('Cancel')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
