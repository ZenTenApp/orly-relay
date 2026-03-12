import { useCallback, useEffect, useMemo, useState } from 'react'
import relayAdmin from '@/services/relay-admin.service'
import client from '@/services/client.service'
import { Button } from '@/components/ui/button'
import { toast } from 'sonner'
import { nip19 } from 'nostr-tools'

const EXAMPLE_POLICY = `{
  "kind": {
    "whitelist": [0, 1, 3, 6, 7, 10002],
    "blacklist": []
  },
  "global": {
    "description": "Global rules applied to all events",
    "size_limit": 65536,
    "max_age_of_event": 86400,
    "max_age_event_in_future": 300
  },
  "rules": {
    "1": {
      "description": "Kind 1 (short text notes)",
      "content_limit": 8192,
      "write_allow_follows": true
    },
    "30023": {
      "description": "Long-form articles",
      "content_limit": 100000,
      "tag_validation": {
        "d": "^[a-z0-9-]{1,64}$",
        "t": "^[a-z0-9-]{1,32}$"
      }
    }
  },
  "default_policy": "allow",
  "policy_admins": ["<your-hex-pubkey>"],
  "policy_follow_whitelist_enabled": true
}`

function npubToHex(input: string): string | null {
  if (!input) return null
  if (/^[0-9a-fA-F]{64}$/.test(input)) return input.toLowerCase()
  if (input.startsWith('npub1')) {
    try {
      const { type, data } = nip19.decode(input)
      if (type === 'npub' && typeof data === 'string') return data
    } catch {
      return null
    }
  }
  return null
}

function truncatePubkey(pk: string): string {
  return `${pk.substring(0, 16)}...${pk.substring(pk.length - 8)}`
}

export default function PolicyTab() {
  const [policyJson, setPolicyJson] = useState('')
  const [isLoading, setIsLoading] = useState(false)
  const [policyEnabled, setPolicyEnabled] = useState(false)
  const [isPolicyAdmin, setIsPolicyAdmin] = useState(false)
  const [userRole, setUserRole] = useState('')
  const [validationErrors, setValidationErrors] = useState<string[]>([])
  const [policyFollows, setPolicyFollows] = useState<string[]>([])
  const [newAdminInput, setNewAdminInput] = useState('')

  const isLoggedIn = !!client.pubkey

  const policyAdmins = useMemo(() => {
    try {
      if (policyJson) {
        const parsed = JSON.parse(policyJson)
        return (parsed.policy_admins || []) as string[]
      }
    } catch {
      // ignore
    }
    return [] as string[]
  }, [policyJson])

  useEffect(() => {
    relayAdmin.loadPolicyConfig().then((config) => {
      setPolicyEnabled(!!(config as { enabled?: boolean }).enabled)
    }).catch(() => setPolicyEnabled(false))

    if (isLoggedIn) {
      relayAdmin.fetchUserRole().then((role) => {
        setUserRole(role)
        // Check if current user is a policy admin by loading policy
        loadPolicySilent()
      }).catch(() => setUserRole(''))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isLoggedIn])

  // After policy loads, check if current user is a policy admin
  useEffect(() => {
    if (client.pubkey && policyAdmins.length > 0) {
      setIsPolicyAdmin(policyAdmins.includes(client.pubkey))
    }
  }, [policyAdmins])

  const loadPolicySilent = useCallback(async () => {
    try {
      const data = await relayAdmin.loadPolicy()
      if (data && Object.keys(data).length > 0) {
        setPolicyJson(JSON.stringify(data, null, 2))
      }
    } catch {
      // silent
    }
  }, [])

  const handleLoadPolicy = useCallback(async () => {
    setIsLoading(true)
    setValidationErrors([])
    try {
      // Try loading via kind 12345 events from relay first
      const events = await client.fetchEvents(client.currentRelays, {
        kinds: [12345],
        limit: 1
      })
      if (events && events.length > 0) {
        let content = events[0].content
        try {
          content = JSON.stringify(JSON.parse(content), null, 2)
        } catch {
          // keep as-is
        }
        setPolicyJson(content)
        toast.success('Policy loaded from relay event')
      } else {
        // Fall back to API
        const data = await relayAdmin.loadPolicy()
        if (data && Object.keys(data).length > 0) {
          setPolicyJson(JSON.stringify(data, null, 2))
          toast.success('Policy loaded from file')
        } else {
          setPolicyJson('')
          toast.info('No policy configuration found')
        }
      }
    } catch (e) {
      toast.error(`Error loading policy: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setIsLoading(false)
    }
  }, [])

  const handleValidatePolicy = useCallback((): boolean => {
    const errors: string[] = []

    if (!policyJson.trim()) {
      setValidationErrors(['Policy JSON is empty'])
      toast.error('Validation failed')
      return false
    }

    let parsed: Record<string, unknown>
    try {
      parsed = JSON.parse(policyJson)
    } catch (e) {
      setValidationErrors([`JSON parse error: ${e instanceof Error ? e.message : String(e)}`])
      toast.error('Invalid JSON syntax')
      return false
    }

    if (typeof parsed !== 'object' || parsed === null) {
      setValidationErrors(['Policy must be a JSON object'])
      toast.error('Validation failed')
      return false
    }

    // Validate policy_admins
    if (parsed.policy_admins) {
      if (!Array.isArray(parsed.policy_admins)) {
        errors.push('policy_admins must be an array')
      } else {
        for (const admin of parsed.policy_admins) {
          if (typeof admin !== 'string' || !/^[0-9a-fA-F]{64}$/.test(admin)) {
            errors.push(`Invalid policy_admin pubkey: ${admin}`)
          }
        }
      }
    }

    // Validate rules
    if (parsed.rules) {
      if (typeof parsed.rules !== 'object') {
        errors.push('rules must be an object')
      } else {
        for (const [kindStr, rule] of Object.entries(parsed.rules as Record<string, Record<string, unknown>>)) {
          if (!/^\d+$/.test(kindStr)) {
            errors.push(`Invalid kind number: ${kindStr}`)
          }
          if (rule.tag_validation && typeof rule.tag_validation === 'object') {
            for (const [tag, pattern] of Object.entries(rule.tag_validation as Record<string, string>)) {
              try {
                new RegExp(pattern)
              } catch {
                errors.push(`Invalid regex for tag '${tag}': ${pattern}`)
              }
            }
          }
        }
      }
    }

    // Validate default_policy
    if (parsed.default_policy && !['allow', 'deny'].includes(parsed.default_policy as string)) {
      errors.push("default_policy must be 'allow' or 'deny'")
    }

    setValidationErrors(errors)
    if (errors.length > 0) {
      toast.error('Validation failed - see errors below')
      return false
    }

    toast.success('Validation passed')
    return true
  }, [policyJson])

  const handleSavePolicy = useCallback(async () => {
    const isValid = handleValidatePolicy()
    if (!isValid) return

    setIsLoading(true)
    try {
      if (!client.signer) {
        toast.error('No signer available. Please log in.')
        return
      }

      const event = await client.signer.signEvent({
        kind: 12345,
        created_at: Math.floor(Date.now() / 1000),
        tags: [],
        content: policyJson
      })

      await client.publishEvent(client.currentRelays, event)
      toast.success('Policy updated and published')
    } catch (e) {
      toast.error(`Error saving policy: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setIsLoading(false)
    }
  }, [policyJson, handleValidatePolicy])

  const handleFormatJson = useCallback(() => {
    try {
      const parsed = JSON.parse(policyJson)
      setPolicyJson(JSON.stringify(parsed, null, 2))
      toast.success('JSON formatted')
    } catch (e) {
      toast.error(`Cannot format: ${e instanceof Error ? e.message : String(e)}`)
    }
  }, [policyJson])

  const handleRefreshFollows = useCallback(async () => {
    setIsLoading(true)
    setPolicyFollows([])
    try {
      let admins: string[] = []
      try {
        const config = JSON.parse(policyJson || '{}')
        admins = config.policy_admins || []
      } catch {
        toast.error('Cannot parse policy JSON to get admins')
        setIsLoading(false)
        return
      }

      if (admins.length === 0) {
        toast.warning('No policy admins configured')
        setIsLoading(false)
        return
      }

      const events = await client.fetchEvents(client.currentRelays, {
        kinds: [3],
        authors: admins,
        limit: admins.length
      })

      const followsSet = new Set<string>()
      for (const event of events) {
        if (event.tags) {
          for (const tag of event.tags) {
            if (tag[0] === 'p' && tag[1] && tag[1].length === 64) {
              followsSet.add(tag[1])
            }
          }
        }
      }

      const follows = Array.from(followsSet)
      setPolicyFollows(follows)
      toast.success(`Loaded ${follows.length} follows from ${events.length} admin(s)`)
    } catch (e) {
      toast.error(`Error loading follows: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setIsLoading(false)
    }
  }, [policyJson])

  const handleAddAdmin = useCallback(() => {
    const input = newAdminInput.trim()
    if (!input) {
      toast.error('Please enter a pubkey')
      return
    }

    const hexPubkey = npubToHex(input)
    if (!hexPubkey || hexPubkey.length !== 64) {
      toast.error('Invalid pubkey format. Use hex (64 chars) or npub')
      return
    }

    try {
      const config = JSON.parse(policyJson || '{}')
      if (!config.policy_admins) config.policy_admins = []
      if (config.policy_admins.includes(hexPubkey)) {
        toast.warning('Admin already in list')
        return
      }
      config.policy_admins.push(hexPubkey)
      setPolicyJson(JSON.stringify(config, null, 2))
      setNewAdminInput('')
      toast.info("Admin added - click 'Save & Publish' to apply")
    } catch (e) {
      toast.error(`Error adding admin: ${e instanceof Error ? e.message : String(e)}`)
    }
  }, [newAdminInput, policyJson])

  const handleRemoveAdmin = useCallback(
    (pubkey: string) => {
      try {
        const config = JSON.parse(policyJson || '{}')
        if (config.policy_admins) {
          config.policy_admins = config.policy_admins.filter((p: string) => p !== pubkey)
          setPolicyJson(JSON.stringify(config, null, 2))
          toast.info("Admin removed - click 'Save & Publish' to apply")
        }
      } catch (e) {
        toast.error(`Error removing admin: ${e instanceof Error ? e.message : String(e)}`)
      }
    },
    [policyJson]
  )

  // Not logged in
  if (!isLoggedIn) {
    return (
      <div className="p-4 w-full">
        <h2 className="text-2xl font-semibold mb-4">Policy Configuration</h2>
        <div className="text-center py-8 rounded-lg border bg-card">
          <p className="text-muted-foreground">Please log in to access policy configuration.</p>
        </div>
      </div>
    )
  }

  // Logged in but no permission
  if (userRole !== 'owner' && !isPolicyAdmin) {
    return (
      <div className="p-4 w-full">
        <h2 className="text-2xl font-semibold mb-4">Policy Configuration</h2>
        <div className="text-center py-8 rounded-lg border bg-card space-y-2">
          <p className="text-muted-foreground">
            Policy configuration requires owner or policy admin permissions.
          </p>
          <p className="text-muted-foreground">
            To become a policy admin, ask an existing policy admin to add your pubkey to the{' '}
            <code className="bg-muted px-1.5 py-0.5 rounded text-xs font-mono">policy_admins</code>{' '}
            list.
          </p>
          <p className="text-sm text-muted-foreground">
            Current user role: <span className="font-semibold">{userRole || 'none'}</span>
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="p-4 space-y-4 w-full">
      <h2 className="text-2xl font-semibold">Policy Configuration</h2>

      {/* Policy Editor Section */}
      <div className="rounded-lg border bg-card p-4 space-y-3">
        <div className="flex items-center justify-between flex-wrap gap-2">
          <h3 className="text-lg font-semibold">Policy Editor</h3>
          <div className="flex items-center gap-2">
            <span
              className={`px-3 py-1 rounded-full text-xs font-semibold ${
                policyEnabled
                  ? 'bg-green-600 text-white'
                  : 'bg-destructive text-destructive-foreground'
              }`}
            >
              {policyEnabled ? 'Policy Enabled' : 'Policy Disabled'}
            </span>
            {isPolicyAdmin && (
              <span className="px-3 py-1 rounded-full text-xs font-semibold bg-primary text-primary-foreground">
                Policy Admin
              </span>
            )}
          </div>
        </div>

        <div className="rounded-md bg-muted/50 border p-3 text-sm space-y-1">
          <p>
            Edit the policy JSON below and click "Save & Publish" to update the relay's policy
            configuration. Changes are applied immediately after validation.
          </p>
          <p className="text-muted-foreground text-xs">
            Policy updates are published as kind 12345 events and require policy admin permissions.
          </p>
        </div>

        <textarea
          className="w-full h-96 rounded-md border bg-background p-3 font-mono text-sm leading-relaxed resize-y focus:outline-none focus:ring-2 focus:ring-ring disabled:opacity-50 disabled:cursor-not-allowed"
          value={policyJson}
          onChange={(e) => setPolicyJson(e.target.value)}
          placeholder="Loading policy configuration..."
          disabled={isLoading}
          spellCheck={false}
        />

        {validationErrors.length > 0 && (
          <div className="rounded-md bg-destructive/10 border border-destructive p-3">
            <h4 className="text-sm font-semibold text-destructive mb-1">Validation Errors:</h4>
            <ul className="list-disc pl-5 text-sm text-destructive space-y-0.5">
              {validationErrors.map((err, i) => (
                <li key={i}>{err}</li>
              ))}
            </ul>
          </div>
        )}

        <div className="flex flex-wrap gap-2">
          <Button variant="outline" size="sm" onClick={handleLoadPolicy} disabled={isLoading}>
            Load Current
          </Button>
          <Button variant="secondary" size="sm" onClick={handleFormatJson} disabled={isLoading}>
            Format JSON
          </Button>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              handleValidatePolicy()
            }}
            disabled={isLoading}
            className="border-yellow-500/50 text-yellow-500 hover:bg-yellow-500/10"
          >
            Validate
          </Button>
          <Button size="sm" onClick={handleSavePolicy} disabled={isLoading}>
            Save & Publish
          </Button>
        </div>
      </div>

      {/* Policy Administrators Section */}
      <div className="rounded-lg border bg-card p-4 space-y-3">
        <h3 className="text-lg font-semibold">Policy Administrators</h3>

        <div className="rounded-md bg-muted/50 border p-3 text-sm space-y-1">
          <p>
            Policy admins can update the relay's policy configuration via kind 12345 events. Their
            follows get whitelisted if{' '}
            <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">
              policy_follow_whitelist_enabled
            </code>{' '}
            is true in the policy.
          </p>
          <p className="text-muted-foreground text-xs">
            Note: Policy admins are separate from relay admins (ORLY_ADMINS). Changes here update
            the JSON editor - click "Save & Publish" to apply.
          </p>
        </div>

        <div className="space-y-2">
          {policyAdmins.length === 0 ? (
            <p className="text-center py-3 text-muted-foreground italic text-sm">
              No policy admins configured
            </p>
          ) : (
            policyAdmins.map((admin) => (
              <div
                key={admin}
                className="flex items-center justify-between rounded-md border bg-background px-3 py-2"
              >
                <span className="font-mono text-sm" title={admin}>
                  {truncatePubkey(admin)}
                </span>
                <button
                  onClick={() => handleRemoveAdmin(admin)}
                  disabled={isLoading}
                  title="Remove admin"
                  className="w-6 h-6 rounded-full bg-destructive text-destructive-foreground text-xs flex items-center justify-center hover:brightness-90 disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  X
                </button>
              </div>
            ))
          )}
        </div>

        <div className="flex gap-2">
          <input
            type="text"
            placeholder="npub or hex pubkey"
            value={newAdminInput}
            onChange={(e) => setNewAdminInput(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleAddAdmin()}
            disabled={isLoading}
            className="flex-1 rounded-md border bg-background px-3 py-2 font-mono text-sm focus:outline-none focus:ring-2 focus:ring-ring disabled:opacity-50"
          />
          <Button
            size="sm"
            onClick={handleAddAdmin}
            disabled={isLoading || !newAdminInput.trim()}
          >
            + Add Admin
          </Button>
        </div>
      </div>

      {/* Policy Follow Whitelist Section */}
      <div className="rounded-lg border bg-card p-4 space-y-3">
        <h3 className="text-lg font-semibold">Policy Follow Whitelist</h3>

        <div className="rounded-md bg-muted/50 border p-3 text-sm">
          <p>
            Pubkeys followed by policy admins (kind 3 events). These get automatic read+write access
            when rules have{' '}
            <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">
              write_allow_follows: true
            </code>
            .
          </p>
        </div>

        <div className="flex items-center justify-between">
          <span className="text-sm font-semibold">
            {policyFollows.length} pubkey(s) in whitelist
          </span>
          <Button variant="outline" size="sm" onClick={handleRefreshFollows} disabled={isLoading}>
            Refresh Follows
          </Button>
        </div>

        <div className="max-h-72 overflow-y-auto rounded-md border bg-background">
          {policyFollows.length === 0 ? (
            <p className="text-center py-4 text-sm text-muted-foreground italic">
              No follows loaded. Click "Refresh Follows" to load from database.
            </p>
          ) : (
            <div className="grid grid-cols-[repeat(auto-fill,minmax(200px,1fr))] gap-2 p-3">
              {policyFollows.map((follow) => (
                <div
                  key={follow}
                  title={follow}
                  className="px-2 py-1.5 rounded-md border bg-card font-mono text-xs truncate"
                >
                  {follow.substring(0, 12)}...{follow.substring(follow.length - 6)}
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {/* Policy Reference Section */}
      <div className="rounded-lg border bg-card p-4 space-y-3">
        <h3 className="text-lg font-semibold">Policy Reference</h3>

        <div className="space-y-3 text-sm">
          <div>
            <h4 className="font-semibold mb-1">Structure Overview</h4>
            <ul className="list-disc pl-5 space-y-0.5 text-muted-foreground">
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">kind.whitelist</code>{' '}
                - Only allow these event kinds (takes precedence)
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">kind.blacklist</code>{' '}
                - Deny these event kinds (if no whitelist)
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">global</code> -
                Rules applied to all events
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">rules</code> -
                Per-kind rules (keyed by kind number as string)
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">default_policy</code>{' '}
                - "allow" or "deny" when no rules match
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">policy_admins</code>{' '}
                - Hex pubkeys that can update policy
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">
                  policy_follow_whitelist_enabled
                </code>{' '}
                - Enable follow-based access
              </li>
            </ul>
          </div>

          <div>
            <h4 className="font-semibold mb-1">Rule Fields</h4>
            <ul className="list-disc pl-5 space-y-0.5 text-muted-foreground">
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">description</code>{' '}
                - Human-readable rule description
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">write_allow</code>{' '}
                /{' '}
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">write_deny</code>{' '}
                - Pubkey lists for write access
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">read_allow</code>{' '}
                /{' '}
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">read_deny</code>{' '}
                - Pubkey lists for read access
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">
                  write_allow_follows
                </code>{' '}
                - Grant access to policy admin follows
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">size_limit</code>{' '}
                - Max total event size in bytes
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">content_limit</code>{' '}
                - Max content field size in bytes
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">max_expiry</code>{' '}
                - Max expiry offset in seconds
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">
                  max_age_of_event
                </code>{' '}
                - Max age of created_at in seconds
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">
                  max_age_event_in_future
                </code>{' '}
                - Max future offset in seconds
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">
                  must_have_tags
                </code>{' '}
                - Required tag letters (e.g., ["d", "t"])
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">
                  tag_validation
                </code>{' '}
                - Regex patterns for tag values
              </li>
              <li>
                <code className="bg-muted px-1 py-0.5 rounded text-xs font-mono">script</code> -
                Path to external validation script
              </li>
            </ul>
          </div>

          <div>
            <h4 className="font-semibold mb-1">Example Policy</h4>
            <pre className="rounded-md border bg-background p-3 font-mono text-xs leading-snug overflow-x-auto whitespace-pre">
              {EXAMPLE_POLICY}
            </pre>
          </div>
        </div>
      </div>
    </div>
  )
}
