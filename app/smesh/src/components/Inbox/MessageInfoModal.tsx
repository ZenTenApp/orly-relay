import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import dmService from '@/services/dm.service'
import { TDirectMessage } from '@/types'
import { Loader2, RefreshCw, Server } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

interface MessageInfoModalProps {
  message: TDirectMessage | null
  open: boolean
  onOpenChange: (open: boolean) => void
  onRelaysUpdated?: (relays: string[]) => void
}

export default function MessageInfoModal({
  message,
  open,
  onOpenChange,
  onRelaysUpdated
}: MessageInfoModalProps) {
  const { t } = useTranslation()
  const [isChecking, setIsChecking] = useState(false)
  const [additionalRelays, setAdditionalRelays] = useState<string[]>([])

  if (!message) return null

  const allRelays = [...(message.seenOnRelays || []), ...additionalRelays]
  const uniqueRelays = [...new Set(allRelays)]

  const handleCheckOtherRelays = async () => {
    setIsChecking(true)
    try {
      const foundRelays = await dmService.checkOtherRelaysForEvent(
        message.id,
        uniqueRelays
      )
      if (foundRelays.length > 0) {
        const newRelays = [...additionalRelays, ...foundRelays]
        setAdditionalRelays(newRelays)
        onRelaysUpdated?.([...(message.seenOnRelays || []), ...newRelays])
      }
    } catch (error) {
      console.error('Failed to check other relays:', error)
    } finally {
      setIsChecking(false)
    }
  }

  const formatRelayUrl = (url: string) => {
    try {
      const parsed = new URL(url)
      return parsed.hostname + (parsed.pathname !== '/' ? parsed.pathname : '')
    } catch {
      return url
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-sm">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <Server className="size-4" />
            {t('Message Info')}
          </DialogTitle>
        </DialogHeader>

        <div className="space-y-4">
          {/* Protocol */}
          <div>
            <span className="text-sm font-medium text-muted-foreground">
              {t('Encryption')}
            </span>
            <p className="text-sm mt-1">
              {message.encryptionType === 'nip17' ? 'NIP-44 (Gift Wrap)' : 'NIP-04 (Legacy)'}
            </p>
          </div>

          {/* Relays */}
          <div>
            <span className="text-sm font-medium text-muted-foreground">
              {t('Seen on relays')}
            </span>
            {uniqueRelays.length > 0 ? (
              <ul className="mt-1 space-y-1">
                {uniqueRelays.map((relay) => (
                  <li
                    key={relay}
                    className="text-sm font-mono bg-muted px-2 py-1 rounded truncate"
                    title={relay}
                  >
                    {formatRelayUrl(relay)}
                  </li>
                ))}
              </ul>
            ) : (
              <p className="text-sm text-muted-foreground mt-1">
                {t('No relay information available')}
              </p>
            )}
          </div>

          {/* Check other relays button */}
          <Button
            variant="outline"
            size="sm"
            className="w-full"
            onClick={handleCheckOtherRelays}
            disabled={isChecking}
          >
            {isChecking ? (
              <>
                <Loader2 className="size-4 mr-2 animate-spin" />
                {t('Checking...')}
              </>
            ) : (
              <>
                <RefreshCw className="size-4 mr-2" />
                {t('Check for other relays')}
              </>
            )}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
