import { useRegisterSW } from 'virtual:pwa-register/react'

export default function UpdateNotification() {
  useRegisterSW({
    onRegisteredSW(_swUrl, r) {
      // Check for updates every 60 seconds
      if (r) {
        setInterval(() => {
          r.update()
        }, 60 * 1000)
      }
    },
    onRegisterError(error) {
      console.error('SW registration error:', error)
    }
  })

  // With autoUpdate + skipWaiting + clientsClaim, the new SW activates immediately.
  // Listen for controller change and reload to pick up new assets.
  if (typeof window !== 'undefined' && navigator.serviceWorker) {
    navigator.serviceWorker.addEventListener('controllerchange', () => {
      window.location.reload()
    })
  }

  return null
}
