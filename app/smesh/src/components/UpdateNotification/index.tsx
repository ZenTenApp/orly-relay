import { Button } from '@/components/ui/button'
import { RefreshCw, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useRegisterSW } from 'virtual:pwa-register/react'

export default function UpdateNotification() {
  const { t } = useTranslation()
  const [dismissed, setDismissed] = useState(false)
  const [updating, setUpdating] = useState(false)

  const {
    needRefresh: [needRefresh],
    updateServiceWorker
  } = useRegisterSW({
    onRegisteredSW(_swUrl, r) {
      // Check for updates every 5 minutes
      if (r) {
        setInterval(() => {
          r.update()
        }, 5 * 60 * 1000)
      }
    },
    onRegisterError(error) {
      console.error('SW registration error:', error)
    }
  })

  const handleUpdate = async () => {
    setUpdating(true)
    try {
      // Clear all caches so stale assets can't be served after reload
      const cacheNames = await caches.keys()
      await Promise.all(cacheNames.map((name) => caches.delete(name)))

      // Wait for the new service worker to take control before reloading.
      // controllerchange fires when the new SW calls clients.claim().
      const controllerChanged = new Promise<void>((resolve) => {
        const sw = navigator.serviceWorker
        if (!sw) {
          resolve()
          return
        }
        const onControllerChange = () => {
          sw.removeEventListener('controllerchange', onControllerChange)
          resolve()
        }
        sw.addEventListener('controllerchange', onControllerChange)
        // Timeout: reload anyway after 3s if controllerchange doesn't fire
        setTimeout(resolve, 3000)
      })

      // Tell the waiting SW to skipWaiting and activate
      await updateServiceWorker(false)
      await controllerChanged

      window.location.reload()
    } catch {
      // Last resort: hard reload
      window.location.reload()
    }
  }

  const handleDismiss = () => {
    setDismissed(true)
  }

  if (!needRefresh || dismissed) {
    return null
  }

  return (
    <div className="fixed top-0 left-0 right-0 z-[100] bg-primary text-primary-foreground px-4 py-2 flex items-center justify-center gap-3 shadow-lg">
      <span className="text-sm font-medium">
        {t('A new version is available')}
      </span>
      <Button
        size="sm"
        variant="secondary"
        onClick={handleUpdate}
        disabled={updating}
        className="h-7 px-3 text-xs"
      >
        <RefreshCw className={`size-3 mr-1 ${updating ? 'animate-spin' : ''}`} />
        {updating ? t('Updating...') : t('Refresh')}
      </Button>
      <button
        onClick={handleDismiss}
        className="p-1 hover:bg-primary-foreground/20 rounded"
        aria-label={t('Dismiss')}
      >
        <X className="size-4" />
      </button>
    </div>
  )
}
