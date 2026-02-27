import { Button } from '@/components/ui/button'
import { X } from 'lucide-react'
import QrScanner from 'qr-scanner'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

export default function QrScannerModal({
  onScan,
  onClose
}: {
  onScan: (result: string) => void
  onClose: () => void
}) {
  const { t } = useTranslation()
  const videoRef = useRef<HTMLVideoElement>(null)
  const scannerRef = useRef<QrScanner | null>(null)
  const [error, setError] = useState<string | null>(null)

  const handleScan = useCallback(
    (result: QrScanner.ScanResult) => {
      onScan(result.data)
      onClose()
    },
    [onScan, onClose]
  )

  useEffect(() => {
    if (!videoRef.current) return

    const scanner = new QrScanner(videoRef.current, handleScan, {
      preferredCamera: 'environment',
      highlightScanRegion: true,
      highlightCodeOutline: true
    })

    scannerRef.current = scanner

    scanner.start().catch(() => {
      setError(t('Failed to access camera'))
    })

    return () => {
      scanner.destroy()
    }
  }, [handleScan, t])

  return (
    <div className="fixed inset-0 z-50 bg-black/80 flex items-center justify-center">
      <div className="relative w-full max-w-sm mx-4">
        <Button
          variant="ghost"
          size="icon"
          className="absolute -top-12 right-0 text-white hover:bg-white/20"
          onClick={onClose}
        >
          <X className="h-6 w-6" />
        </Button>
        <div className="rounded-lg overflow-hidden bg-black">
          {error ? (
            <div className="p-8 text-center text-destructive">{error}</div>
          ) : (
            <video ref={videoRef} className="w-full" />
          )}
        </div>
        <p className="text-center text-white/70 text-sm mt-4">
          {t('Point camera at QR code')}
        </p>
      </div>
    </div>
  )
}
