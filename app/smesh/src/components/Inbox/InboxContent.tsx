import { toDMConversation } from '@/lib/link'
import { useSecondaryPage } from '@/PageManager'
import { useDM } from '@/providers/DMProvider'
import { useNostr } from '@/providers/NostrProvider'
import { nip19 } from 'nostr-tools'
import { Loader2, Plus, RefreshCw, X } from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import ConversationList from './ConversationList'
import { Button } from '../ui/button'
import RelayConfigurationRequired from '../RelayConfigurationRequired'

export default function InboxContent() {
  const { t } = useTranslation()
  const { relayList } = useNostr()
  const { isLoading, error, refreshConversations, startConversation } = useDM()
  const { push } = useSecondaryPage()
  const [showNewDM, setShowNewDM] = useState(false)
  const [newDMInput, setNewDMInput] = useState('')
  const [newDMError, setNewDMError] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  // Check if user has relay list configured for DMs
  const hasRelayList = relayList && (relayList.read.length > 0 || relayList.write.length > 0)

  if (!hasRelayList) {
    return (
      <div className="p-4">
        <RelayConfigurationRequired type="dm" />
      </div>
    )
  }

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="flex flex-col items-center gap-2 text-muted-foreground">
          <Loader2 className="size-8 animate-spin" />
          <span className="text-sm">{t('Loading messages...')}</span>
        </div>
      </div>
    )
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-64 gap-4 text-muted-foreground">
        <p>{error}</p>
        <Button onClick={refreshConversations} variant="outline" size="sm" className="gap-2">
          <RefreshCw className="size-4" />
          {t('Retry')}
        </Button>
      </div>
    )
  }

  const handleNewDM = () => {
    setNewDMError('')
    const input = newDMInput.trim()
    if (!input) return

    let hexPubkey: string
    try {
      if (input.startsWith('npub1')) {
        const decoded = nip19.decode(input)
        if (decoded.type !== 'npub') {
          setNewDMError(t('Invalid npub'))
          return
        }
        hexPubkey = decoded.data
      } else if (/^[0-9a-f]{64}$/i.test(input)) {
        hexPubkey = input.toLowerCase()
      } else {
        setNewDMError(t('Enter an npub or 64-char hex pubkey'))
        return
      }
    } catch {
      setNewDMError(t('Invalid npub'))
      return
    }

    startConversation(hexPubkey)
    push(toDMConversation(hexPubkey))
    setShowNewDM(false)
    setNewDMInput('')
  }

  // Conversations list - clicking opens in secondary panel (or overlay on mobile)
  return (
    <div className="h-[calc(100vh-8rem)]">
      <div className="px-3 py-2 border-b flex items-center gap-2">
        {showNewDM ? (
          <div className="flex-1 flex items-center gap-2">
            <input
              ref={inputRef}
              type="text"
              value={newDMInput}
              onChange={(e) => { setNewDMInput(e.target.value); setNewDMError('') }}
              onKeyDown={(e) => e.key === 'Enter' && handleNewDM()}
              placeholder="npub1..."
              className="flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
              autoFocus
            />
            <Button size="sm" variant="ghost" onClick={handleNewDM}>
              {t('Go')}
            </Button>
            <Button size="sm" variant="ghost" onClick={() => { setShowNewDM(false); setNewDMInput(''); setNewDMError('') }}>
              <X className="size-4" />
            </Button>
          </div>
        ) : (
          <Button
            size="sm"
            variant="ghost"
            className="gap-1.5 text-muted-foreground hover:text-foreground"
            onClick={() => setShowNewDM(true)}
          >
            <Plus className="size-4" />
            {t('New DM')}
          </Button>
        )}
      </div>
      {newDMError && (
        <div className="px-3 py-1 text-xs text-destructive">{newDMError}</div>
      )}
      <ConversationList />
    </div>
  )
}
