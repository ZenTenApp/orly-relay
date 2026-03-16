import Relay from '@/components/Relay'
import RelayIcon from '@/components/RelayIcon'
import PrimaryPageLayout from '@/layouts/PrimaryPageLayout'
import { normalizeUrl, simplifyUrl } from '@/lib/url'
import { usePrimaryPage } from '@/PageManager'
import { useFavoriteRelays } from '@/providers/FavoriteRelaysProvider'
import { useRelayAdmin } from '@/providers/RelayAdminProvider'
import { TPageRef } from '@/types'
import { Server, Shield } from 'lucide-react'
import { forwardRef, lazy, Suspense, useMemo } from 'react'

const RelayAdminPanel = lazy(() => import('./admin'))

const RelayPage = forwardRef<TPageRef>(({ url }: { url?: string }, ref) => {
  const normalizedUrl = useMemo(() => (url ? normalizeUrl(url) : undefined), [url])
  const { isEmbedded, isAdmin, isLoading } = useRelayAdmin()

  const showAdmin = isEmbedded && isAdmin && !normalizedUrl

  return (
    <PrimaryPageLayout
      pageName="relay"
      titlebar={<RelayPageTitlebar url={normalizedUrl} showAdmin={showAdmin} />}
      displayScrollToTopButton
      ref={ref}
    >
      {normalizedUrl ? (
        <Relay url={normalizedUrl} />
      ) : showAdmin ? (
        <Suspense
          fallback={
            <div className="flex items-center justify-center py-12 text-muted-foreground">
              Loading admin...
            </div>
          }
        >
          <RelayAdminPanel />
        </Suspense>
      ) : isEmbedded && !isLoading ? (
        <div className="flex items-center justify-center py-12 text-muted-foreground">
          Log in as admin to access relay management.
        </div>
      ) : (
        <RelayList />
      )}
    </PrimaryPageLayout>
  )
})
RelayPage.displayName = 'RelayPage'
export default RelayPage

function RelayList() {
  const { favoriteRelays } = useFavoriteRelays()
  const { navigate } = usePrimaryPage()

  if (favoriteRelays.length === 0) {
    return (
      <div className="flex items-center justify-center py-12 text-muted-foreground">
        No relays configured.
      </div>
    )
  }

  return (
    <div className="p-3 space-y-1">
      {favoriteRelays.map((relay) => (
        <button
          key={relay}
          className="w-full flex items-center gap-3 p-3 rounded-lg clickable hover:bg-muted/50 transition-colors text-left"
          onClick={() => navigate('relay', { url: relay })}
        >
          <RelayIcon url={relay} />
          <div className="flex-1 truncate font-medium">{simplifyUrl(relay)}</div>
        </button>
      ))}
    </div>
  )
}

function RelayPageTitlebar({ url, showAdmin }: { url?: string; showAdmin?: boolean }) {
  return (
    <div className="flex items-center gap-2 px-3 h-full">
      {showAdmin ? <Shield /> : <Server />}
      <div className="text-lg font-semibold truncate">
        {showAdmin ? 'Relay Admin' : url ? simplifyUrl(url) : 'Relays'}
      </div>
    </div>
  )
}
