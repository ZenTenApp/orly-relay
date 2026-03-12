/**
 * RelayAdminProvider
 *
 * Detects whether smesh is embedded in an ORLY relay (same-origin) and provides
 * admin context: user role, ACL mode, relay info. Admin features are only shown
 * when embedded AND the user has admin/owner privileges.
 */
import relayAdmin from '@/services/relay-admin.service'
import { useNostr } from '@/providers/NostrProvider'
import { createContext, useContext, useEffect, useState, useCallback } from 'react'

type TRelayAdminContext = {
  isEmbedded: boolean
  isLoading: boolean
  userRole: string
  aclMode: string
  relayInfo: Record<string, unknown> | null
  isAdmin: boolean
  isOwner: boolean
  refreshRole: () => Promise<void>
  refreshACLMode: () => Promise<void>
}

const RelayAdminContext = createContext<TRelayAdminContext | undefined>(undefined)

export const useRelayAdmin = () => {
  const context = useContext(RelayAdminContext)
  if (!context) {
    throw new Error('useRelayAdmin must be used within a RelayAdminProvider')
  }
  return context
}

export function RelayAdminProvider({ children }: { children: React.ReactNode }) {
  const { pubkey } = useNostr()
  const [isEmbedded, setIsEmbedded] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [userRole, setUserRole] = useState('')
  const [aclMode, setAclMode] = useState('')
  const [relayInfo, setRelayInfo] = useState<Record<string, unknown> | null>(null)

  // Detect embedded mode on mount
  useEffect(() => {
    let cancelled = false
    const detect = async () => {
      const embedded = await relayAdmin.isEmbeddedInRelay()
      if (cancelled) return
      setIsEmbedded(embedded)
      if (embedded) {
        const [info, mode] = await Promise.all([
          relayAdmin.fetchRelayInfo(),
          relayAdmin.fetchACLMode()
        ])
        if (!cancelled) {
          setRelayInfo(info)
          setAclMode(mode)
        }
      }
      if (!cancelled) setIsLoading(false)
    }
    detect()
    return () => {
      cancelled = true
    }
  }, [])

  // Fetch user role when pubkey changes and we're embedded
  useEffect(() => {
    if (!isEmbedded || !pubkey) {
      setUserRole('')
      return
    }
    let cancelled = false
    const fetchRole = async () => {
      try {
        const role = await relayAdmin.fetchUserRole()
        if (!cancelled) setUserRole(role)
      } catch {
        if (!cancelled) setUserRole('')
      }
    }
    fetchRole()
    return () => {
      cancelled = true
    }
  }, [isEmbedded, pubkey])

  const refreshRole = useCallback(async () => {
    if (!isEmbedded || !pubkey) return
    const role = await relayAdmin.fetchUserRole()
    setUserRole(role)
  }, [isEmbedded, pubkey])

  const refreshACLMode = useCallback(async () => {
    if (!isEmbedded) return
    const mode = await relayAdmin.fetchACLMode()
    setAclMode(mode)
  }, [isEmbedded])

  const isAdmin = userRole === 'admin' || userRole === 'owner'
  const isOwner = userRole === 'owner'

  return (
    <RelayAdminContext.Provider
      value={{
        isEmbedded,
        isLoading,
        userRole,
        aclMode,
        relayInfo,
        isAdmin,
        isOwner,
        refreshRole,
        refreshACLMode
      }}
    >
      {children}
    </RelayAdminContext.Provider>
  )
}
