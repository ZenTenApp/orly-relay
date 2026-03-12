import { useState } from 'react'
import { useNostr } from '@/providers/NostrProvider'
import { useRelayAdmin } from '@/providers/RelayAdminProvider'
import relayAdmin from '@/services/relay-admin.service'
import { Button } from '@/components/ui/button'
import { toast } from 'sonner'

export default function ExportTab() {
  const { pubkey } = useNostr()
  const { isAdmin, isOwner } = useRelayAdmin()
  const [isExporting, setIsExporting] = useState(false)

  const downloadBlob = (blob: Blob, filename: string) => {
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    URL.revokeObjectURL(url)
  }

  const exportMyEvents = async () => {
    if (!pubkey) return
    setIsExporting(true)
    try {
      const blob = await relayAdmin.exportEvents([pubkey])
      downloadBlob(blob, `my-events-${Date.now()}.jsonl`)
      toast.success('Export complete')
    } catch (e) {
      toast.error(`Export failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setIsExporting(false)
    }
  }

  const exportAllEvents = async () => {
    setIsExporting(true)
    try {
      const blob = await relayAdmin.exportEvents()
      downloadBlob(blob, `all-events-${Date.now()}.jsonl`)
      toast.success('Export complete')
    } catch (e) {
      toast.error(`Export failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setIsExporting(false)
    }
  }

  return (
    <div className="space-y-4 p-4 max-w-lg">
      {pubkey && (
        <div className="rounded-lg bg-card p-4 space-y-3">
          <h3 className="text-lg font-semibold">Export My Events</h3>
          <p className="text-sm text-muted-foreground">
            Download your personal events as a JSONL file.
          </p>
          <Button onClick={exportMyEvents} disabled={isExporting}>
            {isExporting ? 'Exporting...' : 'Export My Events'}
          </Button>
        </div>
      )}
      {(isAdmin || isOwner) && (
        <div className="rounded-lg bg-card p-4 space-y-3">
          <h3 className="text-lg font-semibold">Export All Events</h3>
          <p className="text-sm text-muted-foreground">
            Download the complete database as a JSONL file. This includes all events from all
            users.
          </p>
          <Button onClick={exportAllEvents} disabled={isExporting}>
            {isExporting ? 'Exporting...' : 'Export All Events'}
          </Button>
        </div>
      )}
    </div>
  )
}
