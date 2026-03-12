import { useRef, useState } from 'react'
import relayAdmin from '@/services/relay-admin.service'
import { Button } from '@/components/ui/button'
import { toast } from 'sonner'

export default function ImportTab() {
  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [isImporting, setIsImporting] = useState(false)
  const fileRef = useRef<HTMLInputElement>(null)

  const handleImport = async () => {
    if (!selectedFile) return
    setIsImporting(true)
    try {
      const result = await relayAdmin.importEvents(selectedFile)
      toast.success(`Import complete: ${JSON.stringify(result)}`)
      setSelectedFile(null)
      if (fileRef.current) fileRef.current.value = ''
    } catch (e) {
      toast.error(`Import failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setIsImporting(false)
    }
  }

  return (
    <div className="p-4 max-w-lg space-y-3">
      <div className="rounded-lg bg-card p-4 space-y-4">
        <h3 className="text-lg font-semibold">Import Events</h3>
        <p className="text-sm text-muted-foreground">
          Upload a JSONL file to import events into the database.
        </p>
        <input
          ref={fileRef}
          type="file"
          accept=".jsonl,.txt"
          onChange={(e) => setSelectedFile(e.target.files?.[0] || null)}
          className="block w-full text-sm file:mr-4 file:py-2 file:px-4 file:rounded-md file:border-0 file:text-sm file:font-semibold file:bg-primary file:text-primary-foreground hover:file:bg-primary/80"
        />
        <Button onClick={handleImport} disabled={!selectedFile || isImporting}>
          {isImporting ? 'Importing...' : 'Import Events'}
        </Button>
      </div>
    </div>
  )
}
