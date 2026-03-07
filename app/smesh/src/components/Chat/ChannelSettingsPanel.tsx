import { useChat } from '@/providers/ChatProvider'
import { useNostr } from '@/providers/NostrProvider'
import { useFetchProfile } from '@/hooks/useFetchProfile'
import { Pubkey } from '@/domain'
import { TAccessMode, EXPIRY_OPTIONS, DEFAULT_MESSAGE_EXPIRY } from '@/services/chat.service'
import {
  Lock,
  LockOpen,
  Globe,
  UserPlus,
  UserMinus,
  ShieldPlus,
  ShieldMinus,
  UserCheck,
  UserX,
  Mail,
  MailX,
  Undo2,
  X,
  Bell,
  BellOff
} from 'lucide-react'
import { useState } from 'react'
import { Button } from '../ui/button'

type TSubmitKey = 'enter' | 'ctrl+enter'

function loadSubmitKey(): TSubmitKey {
  const v = localStorage.getItem('nirc:submitKey')
  return v === 'enter' ? 'enter' : 'ctrl+enter'
}

export default function ChannelSettingsPanel({ onClose }: { onClose: () => void }) {
  const {
    currentChannel,
    channelMods,
    channelMembers,
    channelBlocked,
    channelInvited,
    channelRequested,
    channelRejected,
    channelAccessMode,
    isOwnerOrMod,
    addMod,
    removeMod,
    approveMember,
    removeMember,
    unblockUser,
    updateAccessMode,
    updateMessageExpiry,
    sendInvite,
    revokeInvite,
    acceptRequest,
    rejectRequest,
    revokeRejection,
    mutedChannels,
    toggleMuteChannel
  } = useChat()
  const { pubkey } = useNostr()
  const [addInput, setAddInput] = useState('')
  const [addMode, setAddMode] = useState<'member' | 'mod' | 'invite'>('member')
  const [submitKey, setSubmitKey] = useState<TSubmitKey>(loadSubmitKey)

  if (!currentChannel) return null

  const isOwner = currentChannel.creator === pubkey
  const isMuted = mutedChannels.has(currentChannel.id)

  const handleAdd = async () => {
    const pk = addInput.trim()
    if (!pk) return
    let hexPk = pk
    const parsed = Pubkey.tryFromString(pk)
    if (parsed) hexPk = parsed.hex
    if (addMode === 'mod') {
      await addMod(hexPk)
    } else if (addMode === 'invite') {
      await sendInvite(hexPk)
    } else {
      await approveMember(hexPk)
    }
    setAddInput('')
  }

  const accessModes: { mode: TAccessMode; label: string; icon: React.ReactNode }[] = [
    { mode: 'open', label: 'Open', icon: <Globe className="size-3" /> },
    { mode: 'whitelist', label: 'Whitelist', icon: <Lock className="size-3" /> },
    { mode: 'blacklist', label: 'Blacklist', icon: <LockOpen className="size-3" /> }
  ]

  return (
    <div className="absolute inset-0 z-20 bg-background overflow-y-auto">
      <div className="max-w-lg mx-auto p-4 space-y-5">
        {/* Header */}
        <div className="flex items-center justify-between">
          <span className="font-semibold text-sm">Settings — #{currentChannel.name}</span>
          <Button variant="ghost" size="icon" className="size-7" onClick={onClose}>
            <X className="size-4" />
          </Button>
        </div>

        {/* --- Chat Settings (all users) --- */}
        <section className="space-y-2">
          <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Chat</h3>
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-xs text-muted-foreground">Send message with</span>
              <div className="flex gap-1">
                {(['enter', 'ctrl+enter'] as const).map((key) => (
                  <button
                    key={key}
                    className={`text-xs px-2 py-1 rounded border ${submitKey === key ? 'bg-primary text-primary-foreground border-primary' : 'border-border'}`}
                    onClick={() => {
                      setSubmitKey(key)
                      localStorage.setItem('nirc:submitKey', key)
                    }}
                  >
                    {key === 'enter' ? 'Enter' : 'Ctrl+Enter'}
                  </button>
                ))}
              </div>
            </div>
            <div className="text-[10px] text-muted-foreground">
              {submitKey === 'enter' ? 'Shift+Enter for newline' : 'Enter for newline'}
            </div>
            <div className="flex items-center justify-between">
              <span className="text-xs text-muted-foreground">Notifications</span>
              <Button
                variant="outline"
                size="sm"
                className="h-7 text-xs gap-1"
                onClick={() => toggleMuteChannel(currentChannel.id)}
              >
                {isMuted ? <BellOff className="size-3" /> : <Bell className="size-3" />}
                {isMuted ? 'Muted' : 'Active'}
              </Button>
            </div>
          </div>
        </section>

        {/* --- Access Mode (owner only) --- */}
        {isOwner && (
          <section className="space-y-2">
            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Access Mode</h3>
            <div className="flex gap-1">
              {accessModes.map(({ mode, label, icon }) => (
                <button
                  key={mode}
                  className={`text-xs px-3 py-1.5 rounded border flex items-center gap-1.5 ${channelAccessMode === mode ? 'bg-primary text-primary-foreground border-primary' : 'border-border'}`}
                  onClick={() => updateAccessMode(mode)}
                >
                  {icon} {label}
                </button>
              ))}
            </div>
            <div className="text-[10px] text-muted-foreground">
              {channelAccessMode === 'open' && 'Anyone authenticated can read and write.'}
              {channelAccessMode === 'whitelist' && 'Only listed members, mods, and invitees can access.'}
              {channelAccessMode === 'blacklist' && 'Everyone except excluded users can access.'}
            </div>
          </section>
        )}

        {/* --- Message Expiry (owner only) --- */}
        {isOwner && (
          <section className="space-y-2">
            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Message Expiry</h3>
            <div className="flex flex-wrap gap-1">
              {EXPIRY_OPTIONS.map(({ label, value }) => (
                <button
                  key={value}
                  className={`text-xs px-3 py-1.5 rounded border ${
                    (currentChannel.messageExpiry ?? DEFAULT_MESSAGE_EXPIRY) === value
                      ? 'bg-primary text-primary-foreground border-primary'
                      : 'border-border'
                  }`}
                  onClick={() => updateMessageExpiry(value)}
                >
                  {label}
                </button>
              ))}
            </div>
            <div className="text-[10px] text-muted-foreground">
              Messages will include a NIP-40 expiration tag set to this duration from send time.
            </div>
          </section>
        )}

        {/* --- Add member/mod/invite (owner + mods) --- */}
        {isOwnerOrMod && (
          <section className="space-y-2">
            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">
              Add User
            </h3>
            <div className="flex gap-1">
              <button
                className={`text-xs px-2 py-0.5 rounded ${addMode === 'member' ? 'bg-primary text-primary-foreground' : 'bg-muted'}`}
                onClick={() => setAddMode('member')}
              >
                Member
              </button>
              <button
                className={`text-xs px-2 py-0.5 rounded ${addMode === 'invite' ? 'bg-primary text-primary-foreground' : 'bg-muted'}`}
                onClick={() => setAddMode('invite')}
              >
                Invite
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
                {addMode === 'mod' ? <ShieldPlus className="size-3" /> : addMode === 'invite' ? <Mail className="size-3" /> : <UserPlus className="size-3" />}
              </Button>
            </div>
          </section>
        )}

        {/* --- Moderators --- */}
        {channelMods.length > 0 && (
          <section className="space-y-1.5">
            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Moderators</h3>
            {channelMods.map((pk) => (
              <div key={pk} className="flex items-center justify-between text-xs py-0.5">
                <span className="font-mono">
                  <PubkeyName hex={pk} />
                  {pk === currentChannel.creator && (
                    <span className="text-muted-foreground ml-1">(owner)</span>
                  )}
                </span>
                {isOwner && pk !== currentChannel.creator && (
                  <button
                    onClick={() => removeMod(pk)}
                    className="text-muted-foreground hover:text-destructive"
                    title="Remove mod (cascades invites/blocks)"
                  >
                    <ShieldMinus className="size-3" />
                  </button>
                )}
              </div>
            ))}
          </section>
        )}

        {/* --- Members / Excluded (depends on mode) --- */}
        {channelAccessMode === 'whitelist' && channelMembers.length > 0 && (
          <section className="space-y-1.5">
            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Allowed Members</h3>
            {channelMembers.map((entry) => (
              <div key={entry.pubkey} className="flex items-center justify-between text-xs py-0.5">
                <span className="font-mono">
                  <PubkeyName hex={entry.pubkey} />
                  {entry.addedBy && (
                    <span className="text-muted-foreground ml-1">
                      via <PubkeyName hex={entry.addedBy} />
                    </span>
                  )}
                </span>
                {isOwnerOrMod && (
                  <button
                    onClick={() => removeMember(entry.pubkey)}
                    className="text-muted-foreground hover:text-destructive"
                    title="Remove member"
                  >
                    <UserMinus className="size-3" />
                  </button>
                )}
              </div>
            ))}
          </section>
        )}

        {channelAccessMode === 'blacklist' && channelBlocked.length > 0 && (
          <section className="space-y-1.5">
            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Excluded Users</h3>
            {channelBlocked.map((entry) => (
              <div key={entry.pubkey} className="flex items-center justify-between text-xs py-0.5">
                <span className="font-mono text-destructive">
                  <PubkeyName hex={entry.pubkey} />
                  {entry.addedBy && (
                    <span className="text-muted-foreground ml-1">
                      by <PubkeyName hex={entry.addedBy} />
                    </span>
                  )}
                </span>
                {isOwnerOrMod && (
                  <button
                    onClick={() => unblockUser(entry.pubkey)}
                    className="text-muted-foreground hover:text-foreground"
                    title="Remove from excluded"
                  >
                    <UserCheck className="size-3" />
                  </button>
                )}
              </div>
            ))}
          </section>
        )}

        {/* --- Invites (owner + mods) --- */}
        {isOwnerOrMod && channelInvited.length > 0 && (
          <section className="space-y-1.5">
            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Pending Invites</h3>
            {channelInvited.map((entry) => (
              <div key={entry.pubkey} className="flex items-center justify-between text-xs py-0.5">
                <span className="font-mono">
                  <PubkeyName hex={entry.pubkey} />
                  {entry.addedBy && (
                    <span className="text-muted-foreground ml-1">
                      by <PubkeyName hex={entry.addedBy} />
                    </span>
                  )}
                </span>
                <button
                  onClick={() => revokeInvite(entry.pubkey)}
                  className="text-muted-foreground hover:text-destructive"
                  title="Revoke invite"
                >
                  <MailX className="size-3" />
                </button>
              </div>
            ))}
          </section>
        )}

        {/* --- Requests (owner + mods) --- */}
        {isOwnerOrMod && channelRequested.length > 0 && (
          <section className="space-y-1.5">
            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Join Requests</h3>
            {channelRequested.map((pk) => (
              <div key={pk} className="flex items-center justify-between text-xs py-0.5">
                <span className="font-mono"><PubkeyName hex={pk} /></span>
                <div className="flex gap-1">
                  <button
                    onClick={() => acceptRequest(pk)}
                    className="text-muted-foreground hover:text-foreground"
                    title="Accept"
                  >
                    <UserCheck className="size-3" />
                  </button>
                  <button
                    onClick={() => rejectRequest(pk)}
                    className="text-muted-foreground hover:text-destructive"
                    title="Reject"
                  >
                    <UserX className="size-3" />
                  </button>
                </div>
              </div>
            ))}
          </section>
        )}

        {/* --- Rejected (owner only can revoke) --- */}
        {channelRejected.length > 0 && (
          <section className="space-y-1.5">
            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Rejected</h3>
            {channelRejected.map((pk) => (
              <div key={pk} className="flex items-center justify-between text-xs py-0.5">
                <span className="font-mono text-destructive"><PubkeyName hex={pk} /></span>
                {isOwner && (
                  <button
                    onClick={() => revokeRejection(pk)}
                    className="text-muted-foreground hover:text-foreground"
                    title="Revoke rejection"
                  >
                    <Undo2 className="size-3" />
                  </button>
                )}
              </div>
            ))}
          </section>
        )}

        {/* --- Blocked users (from kind 44 mod actions, always shown) --- */}
        {channelAccessMode !== 'blacklist' && channelBlocked.length > 0 && (
          <section className="space-y-1.5">
            <h3 className="text-xs font-semibold text-muted-foreground uppercase tracking-wide">Blocked</h3>
            {channelBlocked.map((entry) => (
              <div key={entry.pubkey} className="flex items-center justify-between text-xs py-0.5">
                <span className="font-mono text-destructive"><PubkeyName hex={entry.pubkey} /></span>
                {isOwnerOrMod && (
                  <button
                    onClick={() => unblockUser(entry.pubkey)}
                    className="text-muted-foreground hover:text-foreground"
                    title="Unblock"
                  >
                    <UserCheck className="size-3" />
                  </button>
                )}
              </div>
            ))}
          </section>
        )}
      </div>
    </div>
  )
}

function PubkeyName({ hex }: { hex: string }) {
  const { profile } = useFetchProfile(hex)
  const pk = Pubkey.tryFromString(hex)
  return <>{profile?.username || pk?.formatNpub(8) || hex.slice(0, 12)}</>
}
