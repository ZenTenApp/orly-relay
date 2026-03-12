import { useCallback, useEffect, useRef, useState } from 'react'
import relayAdmin from '@/services/relay-admin.service'
import { useRelayAdmin } from '@/providers/RelayAdminProvider'
import { useNostr } from '@/providers/NostrProvider'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { toast } from 'sonner'

interface SprocketStatus {
  is_running: boolean
  pid?: number
  script_exists: boolean
}

interface SprocketVersion {
  name: string
  modified: string
  is_current: boolean
}

export default function SprocketTab() {
  const { pubkey } = useNostr()
  const { isOwner, userRole } = useRelayAdmin()

  const [script, setScript] = useState('')
  const [status, setStatus] = useState<SprocketStatus | null>(null)
  const [versions, setVersions] = useState<SprocketVersion[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [uploadFile, setUploadFile] = useState<File | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const loadStatus = useCallback(async () => {
    try {
      const data = (await relayAdmin.loadSprocketStatus()) as unknown as SprocketStatus
      setStatus(data)
    } catch (e) {
      console.error('Failed to load sprocket status:', e)
    }
  }, [])

  const loadScript = useCallback(async () => {
    setIsLoading(true)
    try {
      const text = await relayAdmin.loadSprocketScript()
      setScript(text)
      toast.success('Script loaded')
    } catch (e) {
      toast.error(`Failed to load script: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setIsLoading(false)
    }
  }, [])

  const loadVersions = useCallback(async () => {
    try {
      const data = (await relayAdmin.loadSprocketVersions()) as unknown as SprocketVersion[]
      setVersions(data)
    } catch (e) {
      toast.error(`Failed to load versions: ${e instanceof Error ? e.message : String(e)}`)
    }
  }, [])

  useEffect(() => {
    if (!isOwner) return
    loadStatus()
    loadScript()
    loadVersions()
  }, [isOwner, loadStatus, loadScript, loadVersions])

  const handleSave = async () => {
    setIsLoading(true)
    try {
      await relayAdmin.saveSprocketScript(script)
      toast.success('Script saved and updated')
      await loadStatus()
      await loadVersions()
    } catch (e) {
      toast.error(`Save failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setIsLoading(false)
    }
  }

  const handleRestart = async () => {
    setIsLoading(true)
    try {
      await relayAdmin.restartSprocket()
      toast.success('Sprocket restarted')
      await loadStatus()
    } catch (e) {
      toast.error(`Restart failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setIsLoading(false)
    }
  }

  const handleDelete = async () => {
    if (!confirm('Delete the sprocket script? This cannot be undone.')) return
    setIsLoading(true)
    try {
      await relayAdmin.deleteSprocket()
      setScript('')
      toast.success('Script deleted')
      await loadStatus()
      await loadVersions()
    } catch (e) {
      toast.error(`Delete failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setIsLoading(false)
    }
  }

  const handleFileSelect = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0] || null
    setUploadFile(file)
  }

  const handleUpload = async () => {
    if (!uploadFile) return
    setIsLoading(true)
    try {
      const text = await uploadFile.text()
      setScript(text)
      await relayAdmin.saveSprocketScript(text)
      toast.success('Script uploaded and updated')
      setUploadFile(null)
      if (fileInputRef.current) fileInputRef.current.value = ''
      await loadStatus()
      await loadVersions()
    } catch (e) {
      toast.error(`Upload failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setIsLoading(false)
    }
  }

  const handleLoadVersion = async (version: SprocketVersion) => {
    setIsLoading(true)
    try {
      const text = await relayAdmin.loadSprocketVersion(version.name)
      setScript(text)
      toast.success(`Loaded version: ${version.name}`)
    } catch (e) {
      toast.error(`Failed to load version: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setIsLoading(false)
    }
  }

  const handleDeleteVersion = async (versionName: string) => {
    if (!confirm(`Delete version "${versionName}"?`)) return
    setIsLoading(true)
    try {
      await relayAdmin.deleteSprocketVersion(versionName)
      toast.success(`Version "${versionName}" deleted`)
      await loadVersions()
    } catch (e) {
      toast.error(`Failed to delete version: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setIsLoading(false)
    }
  }

  if (!pubkey) {
    return (
      <div className="p-8 text-center">
        <p className="text-muted-foreground mb-4">Please log in to access sprocket management.</p>
      </div>
    )
  }

  if (!isOwner) {
    return (
      <div className="p-8 text-center space-y-2">
        <p className="text-muted-foreground">Owner permission required for sprocket management.</p>
        <p className="text-sm text-muted-foreground">
          Set the <code className="rounded bg-muted px-1.5 py-0.5 font-mono text-xs">ORLY_OWNERS</code> environment
          variable with your npub when starting the relay.
        </p>
        <p className="text-sm text-muted-foreground">
          Current role: <span className="font-semibold">{userRole || 'none'}</span>
        </p>
      </div>
    )
  }

  return (
    <div className="p-4 space-y-4 w-full max-w-2xl">
      <h3 className="text-lg font-semibold">Sprocket Script Management</h3>

      {/* Script Editor Section */}
      <div className="rounded-lg border bg-card p-4 space-y-4">
        <div className="flex items-center justify-between">
          <h4 className="font-semibold">Script Editor</h4>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={handleRestart}
              disabled={isLoading}
            >
              Restart
            </Button>
            <Button
              variant="destructive"
              size="sm"
              onClick={handleDelete}
              disabled={isLoading || !status?.script_exists}
            >
              Delete Script
            </Button>
          </div>
        </div>

        {/* Upload */}
        <div className="space-y-2">
          <label className="text-sm font-medium">Upload Script</label>
          <div className="flex items-center gap-2">
            <input
              ref={fileInputRef}
              type="file"
              accept=".sh,.bash"
              onChange={handleFileSelect}
              disabled={isLoading}
              className="flex-1 text-sm file:mr-2 file:rounded-md file:border-0 file:bg-primary file:px-3 file:py-1.5 file:text-xs file:font-medium file:text-primary-foreground hover:file:bg-primary-hover"
            />
            <Button size="sm" onClick={handleUpload} disabled={isLoading || !uploadFile}>
              Upload
            </Button>
          </div>
        </div>

        {/* Status */}
        <div className="rounded-md border bg-background p-3 space-y-1 text-sm">
          <div className="flex justify-between">
            <span className="font-medium">Status</span>
            <span className={cn(status?.is_running ? 'text-green-500' : 'text-red-500')}>
              {status?.is_running ? 'Running' : 'Stopped'}
            </span>
          </div>
          {status?.pid != null && (
            <div className="flex justify-between">
              <span className="font-medium">PID</span>
              <span>{status.pid}</span>
            </div>
          )}
          <div className="flex justify-between">
            <span className="font-medium">Script</span>
            <span>{status?.script_exists ? 'Exists' : 'Not found'}</span>
          </div>
        </div>

        {/* Editor */}
        <textarea
          value={script}
          onChange={(e) => setScript(e.target.value)}
          placeholder={'#!/bin/bash\n# Enter your sprocket script here...'}
          disabled={isLoading}
          className="w-full h-72 rounded-md border bg-background p-3 font-mono text-sm resize-y focus:outline-none focus:ring-2 focus:ring-ring disabled:opacity-50 disabled:cursor-not-allowed"
        />

        {/* Actions */}
        <div className="flex gap-2">
          <Button size="sm" onClick={handleSave} disabled={isLoading}>
            Save & Update
          </Button>
          <Button variant="outline" size="sm" onClick={loadScript} disabled={isLoading}>
            Load Current
          </Button>
        </div>
      </div>

      {/* Versions Section */}
      <div className="rounded-lg border bg-card p-4 space-y-3">
        <h4 className="font-semibold">Script Versions</h4>

        <div className="space-y-2">
          {versions.length === 0 ? (
            <p className="text-sm text-muted-foreground text-center py-4">No versions found.</p>
          ) : (
            versions.map((v) => (
              <div
                key={v.name}
                className={cn(
                  'flex items-center justify-between rounded-md border p-3',
                  v.is_current ? 'border-primary bg-primary/5' : 'bg-background'
                )}
              >
                <div className="min-w-0 flex-1">
                  <div className="font-medium text-sm truncate">{v.name}</div>
                  <div className="text-xs text-muted-foreground flex items-center gap-2">
                    {new Date(v.modified).toLocaleString()}
                    {v.is_current && (
                      <span className="rounded bg-primary px-1.5 py-0.5 text-[10px] font-semibold text-primary-foreground">
                        Current
                      </span>
                    )}
                  </div>
                </div>
                <div className="flex gap-1 ml-2 shrink-0">
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => handleLoadVersion(v)}
                    disabled={isLoading}
                  >
                    Load
                  </Button>
                  {!v.is_current && (
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={() => handleDeleteVersion(v.name)}
                      disabled={isLoading}
                    >
                      Delete
                    </Button>
                  )}
                </div>
              </div>
            ))
          )}
        </div>

        <Button variant="outline" size="sm" onClick={loadVersions} disabled={isLoading}>
          Refresh Versions
        </Button>
      </div>
    </div>
  )
}
