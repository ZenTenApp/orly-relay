import { useChat } from '@/providers/ChatProvider'
import { Loader2, Lock } from 'lucide-react'
import { useState } from 'react'
import { Button } from '../ui/button'

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
  const [isCreating, setIsCreating] = useState(false)

  if (!open) return null

  const handleCreate = async () => {
    if (!name.trim()) return
    setIsCreating(true)
    try {
      await createChannel(name.trim(), about.trim())
      setName('')
      setAbout('')
      onOpenChange(false)
    } finally {
      setIsCreating(false)
    }
  }

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
        <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <Lock className="size-3" />
          <span>Channels are invite-only by default</span>
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
