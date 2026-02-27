import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { useNostr } from '@/providers/NostrProvider'
import client from '@/services/client.service'
import indexedDb from '@/services/indexed-db.service'
import { TRelayList } from '@/types'
import { Check, Loader2, Lock, LockOpen, User, Users, Zap } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

type EncryptionPreference = 'auto' | 'nip04' | 'nip17'

interface ConversationSettingsModalProps {
  partnerPubkey: string | null
  open: boolean
  onOpenChange: (open: boolean) => void
  selectedRelays: string[]
  onSelectedRelaysChange: (relays: string[]) => void
}

type RelayInfo = {
  url: string
  isYours: boolean
  isTheirs: boolean
  isShared: boolean
}

export default function ConversationSettingsModal({
  partnerPubkey,
  open,
  onOpenChange,
  selectedRelays,
  onSelectedRelaysChange
}: ConversationSettingsModalProps) {
  const { t } = useTranslation()
  const { pubkey, relayList: myRelayList, hasNip44Support } = useNostr()
  const [partnerRelayList, setPartnerRelayList] = useState<TRelayList | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [relays, setRelays] = useState<RelayInfo[]>([])
  const [encryptionPreference, setEncryptionPreference] = useState<EncryptionPreference>('auto')

  // Fetch partner's relay list when modal opens
  useEffect(() => {
    if (!open || !partnerPubkey) return

    const fetchPartnerRelays = async () => {
      setIsLoading(true)
      try {
        const relayList = await client.fetchRelayList(partnerPubkey)
        setPartnerRelayList(relayList)
      } catch (error) {
        console.error('Failed to fetch partner relay list:', error)
      } finally {
        setIsLoading(false)
      }
    }

    fetchPartnerRelays()
  }, [open, partnerPubkey])

  // Load encryption preference when modal opens
  useEffect(() => {
    if (!open || !partnerPubkey || !pubkey) return

    const loadEncryptionPreference = async () => {
      const saved = await indexedDb.getConversationEncryptionPreference(pubkey, partnerPubkey)
      setEncryptionPreference(saved || 'auto')
    }
    loadEncryptionPreference()
  }, [open, partnerPubkey, pubkey])

  // Save encryption preference when it changes
  const handleEncryptionChange = async (value: EncryptionPreference) => {
    setEncryptionPreference(value)
    if (pubkey && partnerPubkey) {
      await indexedDb.putConversationEncryptionPreference(pubkey, partnerPubkey, value)
    }
  }

  // Build relay list when data is available
  useEffect(() => {
    if (!myRelayList || !partnerRelayList) return

    const myWriteRelays = new Set(myRelayList.write.map((r) => r.replace(/\/$/, '')))
    const theirReadRelays = new Set(partnerRelayList.read.map((r) => r.replace(/\/$/, '')))

    // Combine all relays
    const allRelayUrls = new Set<string>()
    myRelayList.write.forEach((r) => allRelayUrls.add(r.replace(/\/$/, '')))
    partnerRelayList.read.forEach((r) => allRelayUrls.add(r.replace(/\/$/, '')))

    const relayInfos: RelayInfo[] = Array.from(allRelayUrls).map((url) => {
      const normalizedUrl = url.replace(/\/$/, '')
      const isYours = myWriteRelays.has(normalizedUrl)
      const isTheirs = theirReadRelays.has(normalizedUrl)
      return {
        url,
        isYours,
        isTheirs,
        isShared: isYours && isTheirs
      }
    })

    // Sort: shared first, then yours, then theirs
    relayInfos.sort((a, b) => {
      if (a.isShared && !b.isShared) return -1
      if (!a.isShared && b.isShared) return 1
      if (a.isYours && !b.isYours) return -1
      if (!a.isYours && b.isYours) return 1
      return a.url.localeCompare(b.url)
    })

    setRelays(relayInfos)

    // If no relays selected yet, default to shared relays
    if (selectedRelays.length === 0) {
      const sharedRelays = relayInfos.filter((r) => r.isShared).map((r) => r.url)
      if (sharedRelays.length > 0) {
        onSelectedRelaysChange(sharedRelays)
      }
    }
  }, [myRelayList, partnerRelayList])

  const toggleRelay = (url: string) => {
    if (selectedRelays.includes(url)) {
      onSelectedRelaysChange(selectedRelays.filter((r) => r !== url))
    } else {
      onSelectedRelaysChange([...selectedRelays, url])
    }
  }

  const selectAllShared = () => {
    const sharedUrls = relays.filter((r) => r.isShared).map((r) => r.url)
    onSelectedRelaysChange(sharedUrls)
  }

  const selectAll = () => {
    onSelectedRelaysChange(relays.map((r) => r.url))
  }

  const formatRelayUrl = (url: string) => {
    try {
      const parsed = new URL(url)
      return parsed.hostname + (parsed.pathname !== '/' ? parsed.pathname : '')
    } catch {
      return url
    }
  }

  if (!partnerPubkey || !pubkey) return null

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md max-h-[80vh] flex flex-col">
        <DialogHeader>
          <DialogTitle>{t('Conversation Settings')}</DialogTitle>
        </DialogHeader>

        <div className="flex-1 overflow-hidden flex flex-col gap-4">
          {/* Encryption Preference */}
          <div className="space-y-2">
            <Label className="text-sm font-medium">{t('Encryption')}</Label>
            <RadioGroup
              value={encryptionPreference}
              onValueChange={(value) => handleEncryptionChange(value as EncryptionPreference)}
              className="grid grid-cols-3 gap-2"
            >
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="auto" id="enc-auto" />
                <Label
                  htmlFor="enc-auto"
                  className="flex items-center gap-1 text-xs cursor-pointer"
                >
                  <Zap className="size-3" />
                  {t('Auto')}
                </Label>
              </div>
              <div className="flex items-center space-x-2">
                <RadioGroupItem value="nip04" id="enc-nip04" />
                <Label
                  htmlFor="enc-nip04"
                  className="flex items-center gap-1 text-xs cursor-pointer"
                >
                  <LockOpen className="size-3" />
                  NIP-04
                </Label>
              </div>
              <div className="flex items-center space-x-2">
                <RadioGroupItem
                  value="nip17"
                  id="enc-nip17"
                  disabled={!hasNip44Support}
                />
                <Label
                  htmlFor="enc-nip17"
                  className={`flex items-center gap-1 text-xs cursor-pointer ${!hasNip44Support ? 'opacity-50' : ''}`}
                >
                  <Lock className="size-3" />
                  NIP-17
                </Label>
              </div>
            </RadioGroup>
            <p className="text-xs text-muted-foreground">
              {encryptionPreference === 'auto'
                ? t('Matches existing conversation encryption, or sends both on first message')
                : encryptionPreference === 'nip04'
                  ? t('Classic encryption (NIP-04) - compatible with all clients')
                  : t('Modern encryption (NIP-17) - more private with metadata protection')}
            </p>
          </div>

          <div className="border-t pt-4">
            <Label className="text-sm font-medium">{t('Relays')}</Label>
          </div>

          {/* Legend */}
          <div className="flex flex-wrap gap-3 text-xs text-muted-foreground">
            <div className="flex items-center gap-1">
              <User className="size-3" />
              <span>{t('You')}</span>
            </div>
            <div className="flex items-center gap-1">
              <Users className="size-3" />
              <span>{t('Them')}</span>
            </div>
            <div className="flex items-center gap-1">
              <div className="size-3 rounded bg-green-500/20 border border-green-500/50" />
              <span>{t('Shared')}</span>
            </div>
            <div className="flex items-center gap-1">
              <Check className="size-3" />
              <span>{t('Selected for sending')}</span>
            </div>
          </div>

          {/* Quick actions */}
          <div className="flex gap-2">
            <Button variant="outline" size="sm" onClick={selectAllShared}>
              {t('Select shared')}
            </Button>
            <Button variant="outline" size="sm" onClick={selectAll}>
              {t('Select all')}
            </Button>
          </div>

          {/* Relay list */}
          <div className="flex-1 overflow-y-auto space-y-1 min-h-0">
            {isLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 className="size-6 animate-spin text-muted-foreground" />
              </div>
            ) : relays.length === 0 ? (
              <p className="text-sm text-muted-foreground text-center py-4">
                {t('No relay information available')}
              </p>
            ) : (
              relays.map((relay) => (
                <div
                  key={relay.url}
                  className={`flex items-center gap-2 p-2 rounded-lg cursor-pointer hover:bg-accent/50 transition-colors ${
                    relay.isShared ? 'bg-green-500/10 border border-green-500/30' : 'bg-muted/50'
                  }`}
                  onClick={() => toggleRelay(relay.url)}
                >
                  <Checkbox
                    checked={selectedRelays.includes(relay.url)}
                    onCheckedChange={() => toggleRelay(relay.url)}
                  />
                  <div className="flex-1 min-w-0">
                    <span className="text-sm font-mono truncate block" title={relay.url}>
                      {formatRelayUrl(relay.url)}
                    </span>
                  </div>
                  <div className="flex items-center gap-1 flex-shrink-0">
                    {relay.isYours && (
                      <span
                        className="text-xs px-1.5 py-0.5 rounded bg-blue-500/20 text-blue-600 dark:text-blue-400"
                        title={t('Your write relay')}
                      >
                        <User className="size-3" />
                      </span>
                    )}
                    {relay.isTheirs && (
                      <span
                        className="text-xs px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-600 dark:text-purple-400"
                        title={t('Their read relay')}
                      >
                        <Users className="size-3" />
                      </span>
                    )}
                  </div>
                </div>
              ))
            )}
          </div>

          {/* Info text */}
          <p className="text-xs text-muted-foreground">
            {t('Selected relays will be used when sending new messages in this conversation.')}
          </p>
        </div>
      </DialogContent>
    </Dialog>
  )
}
