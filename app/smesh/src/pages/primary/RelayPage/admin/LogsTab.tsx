import { useCallback, useEffect, useRef, useState } from 'react'
import relayAdmin from '@/services/relay-admin.service'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { toast } from 'sonner'

interface LogEntry {
  timestamp: string
  level: string
  message: string
  file?: string
  line?: number
}

const LOG_LEVELS = ['trace', 'debug', 'info', 'warn', 'error', 'fatal']

function levelColor(level: string): string {
  switch (level?.toUpperCase()) {
    case 'TRC':
    case 'TRACE':
      return 'bg-gray-500 text-white'
    case 'DBG':
    case 'DEBUG':
      return 'bg-cyan-600 text-white'
    case 'INF':
    case 'INFO':
      return 'bg-green-600 text-white'
    case 'WRN':
    case 'WARN':
      return 'bg-yellow-500 text-black'
    case 'ERR':
    case 'ERROR':
      return 'bg-red-600 text-white'
    case 'FTL':
    case 'FATAL':
      return 'bg-red-900 text-white'
    default:
      return 'bg-green-600 text-white'
  }
}

export default function LogsTab() {
  const [logs, setLogs] = useState<LogEntry[]>([])
  const [isLoading, setIsLoading] = useState(false)
  const [hasMore, setHasMore] = useState(true)
  const [totalLogs, setTotalLogs] = useState(0)
  const [currentLevel, setCurrentLevel] = useState('info')
  const [selectedLevel, setSelectedLevel] = useState('info')
  const [error, setError] = useState('')
  const offsetRef = useRef(0)
  const triggerRef = useRef<HTMLDivElement>(null)

  const loadLogs = useCallback(
    async (refresh = false) => {
      if (isLoading) return
      setIsLoading(true)
      setError('')

      if (refresh) {
        offsetRef.current = 0
        setLogs([])
      }

      try {
        const data = await relayAdmin.getLogs(
          String(offsetRef.current),
          100
        ) as { logs?: LogEntry[]; total?: number; has_more?: boolean }
        const newLogs = data.logs || []
        if (refresh) {
          setLogs(newLogs)
        } else {
          setLogs((prev) => [...prev, ...newLogs])
        }
        setTotalLogs(data.total || 0)
        setHasMore(data.has_more || false)
        offsetRef.current += newLogs.length
      } catch (e) {
        setError(e instanceof Error ? e.message : 'Failed to load logs')
      } finally {
        setIsLoading(false)
      }
    },
    [isLoading]
  )

  useEffect(() => {
    loadLogs(true)
    relayAdmin.getLogLevel().then((level) => {
      setCurrentLevel(level)
      setSelectedLevel(level)
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // IntersectionObserver for infinite scroll
  useEffect(() => {
    const el = triggerRef.current
    if (!el) return
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries[0].isIntersecting && hasMore && !isLoading) {
          loadLogs(false)
        }
      },
      { threshold: 0.1 }
    )
    observer.observe(el)
    return () => observer.disconnect()
  }, [hasMore, isLoading, loadLogs])

  const handleLevelChange = async (level: string) => {
    setSelectedLevel(level)
    if (level === currentLevel) return
    try {
      await relayAdmin.setLogLevel(level)
      setCurrentLevel(level)
      toast.success(`Log level set to ${level}`)
    } catch (e) {
      setSelectedLevel(currentLevel)
      toast.error(`Failed to set log level: ${e instanceof Error ? e.message : String(e)}`)
    }
  }

  const handleClear = async () => {
    if (!confirm('Clear all logs?')) return
    try {
      await relayAdmin.clearLogs()
      setLogs([])
      setTotalLogs(0)
      setHasMore(false)
      offsetRef.current = 0
      toast.success('Logs cleared')
    } catch (e) {
      toast.error(`Failed: ${e instanceof Error ? e.message : String(e)}`)
    }
  }

  return (
    <div className="p-4 space-y-3 w-full">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h3 className="text-lg font-semibold">Logs</h3>
        <div className="flex items-center gap-2 flex-wrap">
          <label className="text-sm text-muted-foreground">Level:</label>
          <select
            value={selectedLevel}
            onChange={(e) => handleLevelChange(e.target.value)}
            className="rounded-md border bg-card px-2 py-1 text-sm"
          >
            {LOG_LEVELS.map((l) => (
              <option key={l} value={l}>
                {l}
              </option>
            ))}
          </select>
          <Button variant="outline" size="sm" onClick={handleClear} disabled={isLoading || logs.length === 0}>
            Clear
          </Button>
          <Button size="sm" onClick={() => loadLogs(true)} disabled={isLoading}>
            {isLoading ? 'Loading...' : 'Refresh'}
          </Button>
        </div>
      </div>

      {error && <div className="rounded-md bg-destructive/10 text-destructive p-3 text-sm">{error}</div>}

      <div className="text-xs text-muted-foreground">
        Showing {logs.length} of {totalLogs} logs (Level: {currentLevel})
      </div>

      <div className="space-y-1">
        {logs.length === 0 && !isLoading ? (
          <div className="text-center py-8 text-muted-foreground">No logs available.</div>
        ) : (
          logs.map((log, i) => (
            <div
              key={i}
              className="flex items-start gap-2 rounded-md bg-card px-3 py-2 font-mono text-xs"
            >
              <span className="text-muted-foreground whitespace-nowrap shrink-0">
                {log.timestamp ? new Date(log.timestamp).toLocaleString() : ''}
              </span>
              <span
                className={cn(
                  'rounded px-1.5 py-0.5 text-center font-bold uppercase shrink-0 min-w-[3.5em]',
                  levelColor(log.level)
                )}
              >
                {log.level}
              </span>
              {log.file && (
                <span className="text-muted-foreground/50 shrink-0">
                  {log.file}:{log.line}
                </span>
              )}
              <span className="flex-1 break-all">{log.message}</span>
            </div>
          ))
        )}
        <div ref={triggerRef} className="text-center py-4 text-xs text-muted-foreground">
          {isLoading ? 'Loading more...' : hasMore ? 'Scroll for more' : logs.length > 0 ? 'End of logs' : ''}
        </div>
      </div>
    </div>
  )
}
