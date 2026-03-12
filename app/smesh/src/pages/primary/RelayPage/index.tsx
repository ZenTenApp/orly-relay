import Relay from '@/components/Relay'
import PrimaryPageLayout from '@/layouts/PrimaryPageLayout'
import { normalizeUrl, simplifyUrl } from '@/lib/url'
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
        <div className="flex items-center justify-center py-12 text-muted-foreground">
          Select a relay to view its information.
        </div>
      )}
    </PrimaryPageLayout>
  )
})
RelayPage.displayName = 'RelayPage'
export default RelayPage

function RelayPageTitlebar({ url, showAdmin }: { url?: string; showAdmin?: boolean }) {
  return (
    <div className="flex items-center gap-2 px-3 h-full">
      {showAdmin ? <Shield /> : <Server />}
      <div className="text-lg font-semibold truncate">
        {showAdmin ? 'Relay Admin' : simplifyUrl(url ?? '')}
      </div>
    </div>
  )
}
