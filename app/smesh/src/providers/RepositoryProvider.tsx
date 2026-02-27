import {
  FollowListRepository,
  MuteListRepository,
  PinnedUsersListRepository
} from '@/domain'
import {
  FollowListRepositoryImpl,
  MuteListRepositoryImpl,
  PinnedUsersListRepositoryImpl
} from '@/infrastructure/persistence'
import { createContext, useContext, useMemo } from 'react'
import { useNostr } from './NostrProvider'

/**
 * Repository context providing access to all domain repositories
 */
type TRepositoryContext = {
  followListRepository: FollowListRepository
  muteListRepository: MuteListRepository
  pinnedUsersListRepository: PinnedUsersListRepository
}

const RepositoryContext = createContext<TRepositoryContext | undefined>(undefined)

/**
 * Hook to access repositories
 * @throws Error if used outside RepositoryProvider
 */
export const useRepositories = () => {
  const context = useContext(RepositoryContext)
  if (!context) {
    throw new Error('useRepositories must be used within a RepositoryProvider')
  }
  return context
}

/**
 * Provider that creates and provides repository instances with injected dependencies.
 * Must be nested within NostrProvider to access publish and encryption functions.
 */
export function RepositoryProvider({ children }: { children: React.ReactNode }) {
  const { pubkey, publish, nip04Encrypt, nip04Decrypt } = useNostr()

  const repositories = useMemo<TRepositoryContext | null>(() => {
    if (!pubkey) return null

    const followListRepository = new FollowListRepositoryImpl({ publish })

    const muteListRepository = new MuteListRepositoryImpl({
      publish,
      currentUserPubkey: pubkey,
      decrypt: async (ciphertext, pk) => nip04Decrypt(pk, ciphertext),
      encrypt: async (plaintext, pk) => nip04Encrypt(pk, plaintext)
    })

    const pinnedUsersListRepository = new PinnedUsersListRepositoryImpl({
      publish,
      currentUserPubkey: pubkey,
      decrypt: async (ciphertext, pk) => nip04Decrypt(pk, ciphertext),
      encrypt: async (plaintext, pk) => nip04Encrypt(pk, plaintext)
    })

    return {
      followListRepository,
      muteListRepository,
      pinnedUsersListRepository
    }
  }, [pubkey, publish, nip04Encrypt, nip04Decrypt])

  // If not logged in, still render children but context will throw if accessed
  if (!repositories) {
    return <>{children}</>
  }

  return (
    <RepositoryContext.Provider value={repositories}>
      {children}
    </RepositoryContext.Provider>
  )
}
