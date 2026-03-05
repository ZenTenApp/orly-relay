import { useChat } from '@/providers/ChatProvider'
import { useNostr } from '@/providers/NostrProvider'
import { Pubkey } from '@/domain'
import {
  Lock,
  LockOpen,
  UserPlus,
  UserMinus,
  ShieldPlus,
  ShieldMinus,
  UserCheck,
  X
} from 'lucide-react'
import { useState } from 'react'
import { Button } from '../ui/button'

export default function ChannelSettingsPanel({ onClose }: { onClose: () => void }) {
  const {
    currentChannel,
    channelMods,
    channelMembers,
    channelBlocked,
    addMod,
    removeMod,
    approveMember,
    removeMember,
    unblockUser,
    updateChannelSettings
  } = useChat()
  const { pubkey } = useNostr()
  const [addInput, setAddInput] = useState('')
  const [addMode, setAddMode] = useState<'member' | 'mod'>('member')

  if (!currentChannel) return null

  const isOwner = currentChannel.creator === pubkey

  const handleAdd = async () => {
    const pk = addInput.trim()
    if (!pk) return
    // Accept hex pubkey or npub
    let hexPk = pk
    const parsed = Pubkey.tryFromString(pk)
    if (parsed) {
      hexPk = parsed.hex
    }
    if (addMode === 'mod') {
      await addMod(hexPk)
    } else {
      await approveMember(hexPk)
    }
    setAddInput('')
  }

  const formatPk = (hex: string) => {
    const pk = Pubkey.tryFromString(hex)
    return pk?.formatNpub(8) ?? hex.slice(0, 12)
  }

  return (
    <div className="border-b p-3 space-y-3 bg-muted/30 text-sm">
      <div className="flex items-center justify-between">
        <span className="font-semibold">Channel Settings</span>
        <Button variant="ghost" size="icon" className="size-6" onClick={onClose}>
          <X className="size-3.5" />
        </Button>
      </div>

      {/* Invite-only toggle */}
      <div className="flex items-center justify-between">
        <span className="text-xs text-muted-foreground">Access mode</span>
        {isOwner ? (
          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs gap-1"
            onClick={() => updateChannelSettings(!currentChannel.inviteOnly)}
          >
            {currentChannel.inviteOnly ? (
              <>
                <Lock className="size-3" />
                Invite only
              </>
            ) : (
              <>
                <LockOpen className="size-3" />
                Open
              </>
            )}
          </Button>
        ) : (
          <span className="text-xs flex items-center gap-1">
            {currentChannel.inviteOnly ? (
              <>
                <Lock className="size-3" />
                Invite only
              </>
            ) : (
              <>
                <LockOpen className="size-3" />
                Open
              </>
            )}
          </span>
        )}
      </div>

      {/* Add member/mod */}
      <div className="space-y-1.5">
        <div className="flex gap-1">
          <button
            className={`text-xs px-2 py-0.5 rounded ${addMode === 'member' ? 'bg-primary text-primary-foreground' : 'bg-muted'}`}
            onClick={() => setAddMode('member')}
          >
            Member
          </button>
          {isOwner && (
            <button
              className={`text-xs px-2 py-0.5 rounded ${addMode === 'mod' ? 'bg-primary text-primary-foreground' : 'bg-muted'}`}
              onClick={() => setAddMode('mod')}
            >
              Mod
            </button>
          )}
        </div>
        <div className="flex gap-1">
          <input
            type="text"
            placeholder={`Add ${addMode} (npub or hex)`}
            value={addInput}
            onChange={(e) => setAddInput(e.target.value)}
            className="flex-1 px-2 py-1 text-xs border rounded bg-background"
            onKeyDown={(e) => e.key === 'Enter' && handleAdd()}
          />
          <Button variant="outline" size="sm" className="h-7" onClick={handleAdd} disabled={!addInput.trim()}>
            {addMode === 'mod' ? <ShieldPlus className="size-3" /> : <UserPlus className="size-3" />}
          </Button>
        </div>
      </div>

      {/* Mods list */}
      {channelMods.length > 0 && (
        <div className="space-y-1">
          <span className="text-xs text-muted-foreground font-medium">Mods</span>
          {channelMods.map((pk) => (
            <div key={pk} className="flex items-center justify-between text-xs py-0.5">
              <span className="font-mono">
                {formatPk(pk)}
                {pk === currentChannel.creator && (
                  <span className="text-muted-foreground ml-1">(owner)</span>
                )}
              </span>
              {isOwner && pk !== currentChannel.creator && (
                <button
                  onClick={() => removeMod(pk)}
                  className="text-muted-foreground hover:text-destructive"
                  title="Remove mod"
                >
                  <ShieldMinus className="size-3" />
                </button>
              )}
            </div>
          ))}
        </div>
      )}

      {/* Members list */}
      {channelMembers.length > 0 && (
        <div className="space-y-1">
          <span className="text-xs text-muted-foreground font-medium">Members</span>
          {channelMembers.map((pk) => (
            <div key={pk} className="flex items-center justify-between text-xs py-0.5">
              <span className="font-mono">{formatPk(pk)}</span>
              <button
                onClick={() => removeMember(pk)}
                className="text-muted-foreground hover:text-destructive"
                title="Remove member"
              >
                <UserMinus className="size-3" />
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Blocked users */}
      {channelBlocked.length > 0 && (
        <div className="space-y-1">
          <span className="text-xs text-muted-foreground font-medium">Blocked</span>
          {channelBlocked.map((pk) => (
            <div key={pk} className="flex items-center justify-between text-xs py-0.5">
              <span className="font-mono text-destructive">{formatPk(pk)}</span>
              <button
                onClick={() => unblockUser(pk)}
                className="text-muted-foreground hover:text-foreground"
                title="Unblock"
              >
                <UserCheck className="size-3" />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
