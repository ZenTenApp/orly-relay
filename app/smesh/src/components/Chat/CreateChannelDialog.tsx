import { useChat } from '@/providers/ChatProvider'
import { TAccessMode } from '@/services/chat.service'
import { Globe, Loader2, Lock, LockOpen } from 'lucide-react'
import { useState } from 'react'
import { Button } from '../ui/button'

const accessModes: { mode: TAccessMode; label: string; icon: React.ReactNode; desc: string }[] = [
  { mode: 'open', label: 'Open', icon: <Globe className="size-3" />, desc: 'Anyone authenticated can read and write' },
  { mode: 'whitelist', label: 'Whitelist', icon: <Lock className="size-3" />, desc: 'Only listed members can access' },
  { mode: 'blacklist', label: 'Blacklist', icon: <LockOpen className="size-3" />, desc: 'Everyone except excluded users can access' }
]

export default function CreateChannelDialog({
  open,
  onOpenChange
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { createChannel } = useChat()
  const [name, setName] = useState('')
  const [about, setAbout] = useState('')
  const [accessMode, setAccessMode] = useState<TAccessMode>('open')
  const [isCreating, setIsCreating] = useState(false)

  if (!open) return null

  const handleCreate = async () => {
    if (!name.trim()) return
    setIsCreating(true)
    try {
      await createChannel(name.trim(), about.trim(), accessMode)
      setName('')
      setAbout('')
      setAccessMode('open')
      onOpenChange(false)
    } finally {
      setIsCreating(false)
    }
  }

  const current = accessModes.find((m) => m.mode === accessMode)!

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => onOpenChange(false)}>
      <div
        className="bg-background border rounded-lg p-4 w-80 space-y-3"
        onClick={(e) => e.stopPropagation()}
      >
        <h3 className="font-semibold">Create Channel</h3>
        <input
          type="text"
          placeholder="Channel name"
          value={name}
          onChange={(e) => setName(e.target.value)}
          className="w-full px-3 py-2 text-sm border rounded-md bg-background"
          autoFocus
          onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
        />
        <input
          type="text"
          placeholder="Description (optional)"
          value={about}
          onChange={(e) => setAbout(e.target.value)}
          className="w-full px-3 py-2 text-sm border rounded-md bg-background"
          onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
        />
        <div className="space-y-1.5">
          <span className="text-xs text-muted-foreground">Access mode</span>
          <div className="flex gap-1">
            {accessModes.map(({ mode, label, icon }) => (
              <button
                key={mode}
                className={`text-xs px-3 py-1.5 rounded border flex items-center gap-1.5 ${accessMode === mode ? 'bg-primary text-primary-foreground border-primary' : 'border-border'}`}
                onClick={() => setAccessMode(mode)}
              >
                {icon} {label}
              </button>
            ))}
          </div>
          <div className="text-[10px] text-muted-foreground">{current.desc}</div>
        </div>
        <div className="flex justify-end gap-2">
          <Button variant="ghost" size="sm" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button size="sm" onClick={handleCreate} disabled={!name.trim() || isCreating}>
            {isCreating ? <Loader2 className="size-4 animate-spin" /> : 'Create'}
          </Button>
        </div>
      </div>
    </div>
  )
}
