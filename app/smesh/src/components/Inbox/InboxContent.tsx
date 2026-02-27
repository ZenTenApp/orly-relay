import { useDM } from '@/providers/DMProvider'
import { useNostr } from '@/providers/NostrProvider'
import { Loader2, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import ConversationList from './ConversationList'
import { Button } from '../ui/button'
import RelayConfigurationRequired from '../RelayConfigurationRequired'

export default function InboxContent() {
  const { t } = useTranslation()
  const { relayList } = useNostr()
  const { isLoading, error, refreshConversations } = useDM()

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

  // Conversations list - clicking opens in secondary panel (or overlay on mobile)
  return (
    <div className="h-[calc(100vh-8rem)]">
      <ConversationList />
    </div>
  )
}
