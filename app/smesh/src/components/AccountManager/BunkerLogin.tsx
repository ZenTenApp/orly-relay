import QrScannerModal from '@/components/QrScannerModal'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useNostr } from '@/providers/NostrProvider'
import { BunkerSigner } from '@/providers/NostrProvider/bunker.signer'
import { ArrowLeft, Loader2, QrCode, Server, Copy, Check, ScanLine } from 'lucide-react'
import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import QRCode from 'qrcode'

// No default bunker relay - user must configure to protect privacy
// Popular options: wss://relay.nsec.app, wss://relay.damus.io, or user's own relay
const DEFAULT_BUNKER_RELAY = ''

export default function BunkerLogin({
  back,
  onLoginSuccess
}: {
  back: () => void
  onLoginSuccess: () => void
}) {
  const { t } = useTranslation()
  const { bunkerLoginWithSigner, bunkerLogin } = useNostr()
  const [mode, setMode] = useState<'choose' | 'scan' | 'paste'>('choose')
  const [bunkerUrl, setBunkerUrl] = useState('')
  const [relayUrl, setRelayUrl] = useState(DEFAULT_BUNKER_RELAY)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [connectUrl, setConnectUrl] = useState<string | null>(null)
  const [qrDataUrl, setQrDataUrl] = useState<string | null>(null)
  const [copied, setCopied] = useState(false)
  const [showScanner, setShowScanner] = useState(false)

  // Generate QR code when in scan mode
  useEffect(() => {
    if (mode !== 'scan') return

    let cancelled = false

    const startConnection = async () => {
      setLoading(true)
      setError(null)

      // Validate relay URL is configured
      if (!relayUrl.trim()) {
        setError(t('Relay URL is required - enter a relay to use for bunker connection'))
        setLoading(false)
        return
      }

      try {
        const { connectUrl, signer: signerPromise } = await BunkerSigner.awaitSignerConnection(
          relayUrl,
          undefined,
          120000 // 2 minute timeout
        )

        if (cancelled) return

        setConnectUrl(connectUrl)

        // Generate QR code
        const qr = await QRCode.toDataURL(connectUrl, {
          width: 256,
          margin: 2,
          color: { dark: '#000000', light: '#ffffff' }
        })
        setQrDataUrl(qr)
        setLoading(false)

        // Wait for signer to connect
        const signer = await signerPromise

        if (cancelled) {
          signer.disconnect()
          return
        }

        // Get the user's pubkey from the signer
        const pubkey = await signer.getPublicKey()

        // Complete login
        await bunkerLoginWithSigner(signer, pubkey)
        onLoginSuccess()
      } catch (err) {
        if (!cancelled) {
          setError((err as Error).message)
          setLoading(false)
        }
      }
    }

    startConnection()

    return () => {
      cancelled = true
    }
  }, [mode, relayUrl, bunkerLoginWithSigner, onLoginSuccess])

  const handleScan = (result: string) => {
    setBunkerUrl(result)
    setError(null)
  }

  const handlePasteSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!bunkerUrl.trim()) {
      setError(t('Please enter a bunker URL'))
      return
    }

    if (!bunkerUrl.startsWith('bunker://')) {
      setError(t('Invalid bunker URL format. Must start with bunker://'))
      return
    }

    setLoading(true)
    setError(null)

    try {
      // Use the existing bunkerLogin flow for bunker:// URLs
      await bunkerLogin(bunkerUrl.trim())
      onLoginSuccess()
    } catch (err) {
      setError((err as Error).message)
    } finally {
      setLoading(false)
    }
  }

  const copyToClipboard = async () => {
    if (connectUrl) {
      await navigator.clipboard.writeText(connectUrl)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  if (mode === 'choose') {
    return (
      <div className="flex flex-col gap-4" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center gap-2">
          <Button size="icon" variant="ghost" className="rounded-full" onClick={back}>
            <ArrowLeft className="size-4" />
          </Button>
          <div className="flex items-center gap-2">
            <Server className="size-5" />
            <span className="font-semibold">{t('Login with Bunker')}</span>
          </div>
        </div>

        <div className="space-y-3">
          <Button
            variant="outline"
            className="w-full justify-start gap-3 h-auto py-4"
            onClick={() => setMode('scan')}
          >
            <QrCode className="size-6" />
            <div className="text-left">
              <div className="font-medium">{t('Show QR Code')}</div>
              <div className="text-xs text-muted-foreground">
                {t('Scan with Amber or another NIP-46 signer')}
              </div>
            </div>
          </Button>

          <Button
            variant="outline"
            className="w-full justify-start gap-3 h-auto py-4"
            onClick={() => setMode('paste')}
          >
            <Server className="size-6" />
            <div className="text-left">
              <div className="font-medium">{t('Paste Bunker URL')}</div>
              <div className="text-xs text-muted-foreground">
                {t('Enter a bunker:// URL from your signer')}
              </div>
            </div>
          </Button>
        </div>

        <div className="text-xs text-muted-foreground space-y-2 pt-2">
          <p>
            <strong>{t('What is a bunker?')}</strong>
          </p>
          <p>
            {t(
              'A bunker (NIP-46) is a remote signing service that keeps your private key secure while allowing you to sign Nostr events. Your key never leaves the bunker.'
            )}
          </p>
        </div>
      </div>
    )
  }

  if (mode === 'scan') {
    return (
      <div className="flex flex-col gap-4" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center gap-2">
          <Button size="icon" variant="ghost" className="rounded-full" onClick={() => setMode('choose')}>
            <ArrowLeft className="size-4" />
          </Button>
          <div className="flex items-center gap-2">
            <QrCode className="size-5" />
            <span className="font-semibold">{t('Scan with Signer')}</span>
          </div>
        </div>

        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="relayUrl">{t('Relay URL')}</Label>
            <Input
              id="relayUrl"
              type="text"
              placeholder="wss://relay.nsec.app"
              value={relayUrl}
              onChange={(e) => setRelayUrl(e.target.value)}
              disabled={loading || !!qrDataUrl}
              className="font-mono text-sm"
            />
            <p className="text-xs text-muted-foreground">
              {t('Enter a relay URL for bunker connection (e.g., wss://relay.nsec.app)')}
            </p>
          </div>

          {loading && !qrDataUrl && (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
            </div>
          )}

          {qrDataUrl && (
            <div className="flex flex-col items-center gap-4">
              <div
                className="relative cursor-pointer rounded-lg overflow-hidden"
                onClick={copyToClipboard}
                title={t('Click to copy URL')}
              >
                <img src={qrDataUrl} alt="Bunker QR Code" className="w-64 h-64" />
                <div className="absolute inset-0 flex items-center justify-center bg-black/50 opacity-0 hover:opacity-100 transition-opacity">
                  {copied ? (
                    <Check className="size-8 text-white" />
                  ) : (
                    <Copy className="size-8 text-white" />
                  )}
                </div>
              </div>

              <p className="text-sm text-muted-foreground text-center">
                {t('Scan this QR code with Amber or your NIP-46 signer')}
              </p>

              <div className="flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span className="text-sm text-muted-foreground">{t('Waiting for connection...')}</span>
              </div>

              {connectUrl && (
                <div className="w-full">
                  <Label className="text-xs text-muted-foreground">{t('Connection URL')}</Label>
                  <div className="flex gap-2 mt-1">
                    <Input
                      value={connectUrl}
                      readOnly
                      className="font-mono text-xs"
                    />
                    <Button size="icon" variant="outline" onClick={copyToClipboard}>
                      {copied ? <Check className="size-4" /> : <Copy className="size-4" />}
                    </Button>
                  </div>
                </div>
              )}
            </div>
          )}

          {error && <div className="text-sm text-destructive text-center">{error}</div>}
        </div>
      </div>
    )
  }

  // Paste mode
  return (
    <>
      {showScanner && (
        <QrScannerModal onScan={handleScan} onClose={() => setShowScanner(false)} />
      )}
      <div className="flex flex-col gap-4" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center gap-2">
          <Button size="icon" variant="ghost" className="rounded-full" onClick={() => setMode('choose')}>
            <ArrowLeft className="size-4" />
          </Button>
          <div className="flex items-center gap-2">
            <Server className="size-5" />
            <span className="font-semibold">{t('Paste Bunker URL')}</span>
          </div>
        </div>

        <form onSubmit={handlePasteSubmit} className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="bunkerUrl">{t('Bunker URL')}</Label>
            <div className="flex gap-2">
              <Input
                id="bunkerUrl"
                type="text"
                placeholder="bunker://pubkey?relay=wss://..."
                value={bunkerUrl}
                onChange={(e) => setBunkerUrl(e.target.value)}
                disabled={loading}
                className="font-mono text-sm"
              />
              <Button
                type="button"
                variant="outline"
                size="icon"
                onClick={() => setShowScanner(true)}
                disabled={loading}
                title={t('Scan QR code')}
              >
                <ScanLine className="h-4 w-4" />
              </Button>
            </div>
            <p className="text-xs text-muted-foreground">
              {t(
                'Enter the bunker connection URL. This is typically provided by your signing device or service.'
              )}
            </p>
          </div>

          {error && <div className="text-sm text-destructive">{error}</div>}

          <Button type="submit" className="w-full" disabled={loading || !bunkerUrl.trim()}>
            {loading ? (
              <>
                <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                {t('Connecting...')}
              </>
            ) : (
              t('Connect to Bunker')
            )}
          </Button>
        </form>
      </div>
    </>
  )
}
