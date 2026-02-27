import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { SecondaryPageLink } from '@/PageManager'
import { Settings, Radio } from 'lucide-react'
import { useTranslation } from 'react-i18next'

export type RelayConfigType = 'search' | 'nrc' | 'dm' | 'bunker'

interface RelayConfigurationRequiredProps {
  type: RelayConfigType
  className?: string
}

interface ConfigInfo {
  titleKey: string
  descriptionKey: string
  settingsPath: string
}

const CONFIG_INFO: Record<RelayConfigType, ConfigInfo> = {
  search: {
    titleKey: 'Search Relays Not Configured',
    descriptionKey: 'Configure search relays to enable searching for users and notes. Search queries will only be sent to relays you explicitly configure.',
    settingsPath: '/settings' // Search relay settings in main settings
  },
  nrc: {
    titleKey: 'NRC Relay Not Configured',
    descriptionKey: 'Configure a rendezvous relay to enable Nostr Relay Connect (NRC) for device pairing. Your connection requests will only be sent to the relay you configure.',
    settingsPath: '/settings' // NRC settings in main settings
  },
  dm: {
    titleKey: 'Relay Configuration Required',
    descriptionKey: 'Direct messages require relay configuration. Configure your relay list to enable private messaging.',
    settingsPath: '/settings/relays#mailbox'
  },
  bunker: {
    titleKey: 'Bunker Relay Not Configured',
    descriptionKey: 'Enter a relay URL to use for bunker authentication. Your connection will only use the relay you specify.',
    settingsPath: '' // Bunker relay is configured inline during login
  }
}

export default function RelayConfigurationRequired({ type, className }: RelayConfigurationRequiredProps) {
  const { t } = useTranslation()
  const config = CONFIG_INFO[type]

  return (
    <Card className={className}>
      <CardHeader className="pb-3">
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-full bg-muted">
            <Radio className="size-5 text-muted-foreground" />
          </div>
          <div>
            <CardTitle className="text-base">{t(config.titleKey)}</CardTitle>
          </div>
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <CardDescription className="text-sm">
          {t(config.descriptionKey)}
        </CardDescription>
        {config.settingsPath && (
          <SecondaryPageLink to={config.settingsPath}>
            <Button variant="outline" size="sm" className="gap-2">
              <Settings className="size-4" />
              {t('Configure Relays')}
            </Button>
          </SecondaryPageLink>
        )}
      </CardContent>
    </Card>
  )
}
