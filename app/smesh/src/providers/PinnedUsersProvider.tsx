import { Pubkey, PinnedUsersList, fromPinnedUsersListToHexSet } from '@/domain'
import { PinnedUsersListRepositoryImpl } from '@/infrastructure/persistence'
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { useNostr } from './NostrProvider'

type TPinnedUsersContext = {
  pinnedPubkeySet: Set<string>
  isLoading: boolean
  isPinned: (pubkey: string) => boolean
  pinUser: (pubkey: string) => Promise<void>
  unpinUser: (pubkey: string) => Promise<void>
  togglePin: (pubkey: string) => Promise<void>
}

const PinnedUsersContext = createContext<TPinnedUsersContext | undefined>(undefined)

export const usePinnedUsers = () => {
  const context = useContext(PinnedUsersContext)
  if (!context) {
    throw new Error('usePinnedUsers must be used within a PinnedUsersProvider')
  }
  return context
}

export function PinnedUsersProvider({ children }: { children: React.ReactNode }) {
  const { pubkey: accountPubkey, publish, nip04Decrypt, nip04Encrypt } = useNostr()

  // State managed by this provider
  const [pinnedUsersList, setPinnedUsersList] = useState<PinnedUsersList | null>(null)
  const [isLoading, setIsLoading] = useState(false)

  // Create repository instance
  const repository = useMemo(() => {
    if (!publish || !accountPubkey) return null
    return new PinnedUsersListRepositoryImpl({
      publish,
      currentUserPubkey: accountPubkey,
      decrypt: async (ciphertext, pk) => nip04Decrypt(pk, ciphertext),
      encrypt: async (plaintext, pk) => nip04Encrypt(pk, plaintext)
    })
  }, [publish, accountPubkey, nip04Decrypt, nip04Encrypt])

  // Convert to legacy hex set for backwards compatibility
  const pinnedPubkeySet = useMemo(() => {
    if (!pinnedUsersList) return new Set<string>()
    return fromPinnedUsersListToHexSet(pinnedUsersList)
  }, [pinnedUsersList])

  // Load pinned users list when account changes
  useEffect(() => {
    const loadPinnedUsersList = async () => {
      if (!accountPubkey || !repository) {
        setPinnedUsersList(null)
        return
      }

      setIsLoading(true)
      try {
        const ownerPubkey = Pubkey.tryFromString(accountPubkey)
        if (!ownerPubkey) {
          setPinnedUsersList(null)
          return
        }

        const list = await repository.findByOwner(ownerPubkey)
        setPinnedUsersList(list)
      } catch (error) {
        console.error('Failed to load pinned users list:', error)
        setPinnedUsersList(null)
      } finally {
        setIsLoading(false)
      }
    }

    loadPinnedUsersList()
  }, [accountPubkey, repository])

  const isPinned = useCallback(
    (pubkey: string) => {
      if (!pinnedUsersList) return false
      const pk = Pubkey.tryFromString(pubkey)
      return pk ? pinnedUsersList.isPinned(pk) : false
    },
    [pinnedUsersList]
  )

  const pinUser = useCallback(
    async (pubkey: string) => {
      if (!accountPubkey || !repository || isPinned(pubkey)) return

      try {
        const pk = Pubkey.tryFromString(pubkey)
        if (!pk) return

        const ownerPk = Pubkey.tryFromString(accountPubkey)
        if (!ownerPk) return

        // Fetch latest to avoid conflicts
        const currentList = await repository.findByOwner(ownerPk)
        const list = currentList ?? PinnedUsersList.empty(ownerPk)

        const change = list.pin(pk)
        if (change.type === 'no_change') return

        await repository.save(list)
        setPinnedUsersList(list)
      } catch (error) {
        console.error('Failed to pin user:', error)
      }
    },
    [accountPubkey, repository, isPinned]
  )

  const unpinUser = useCallback(
    async (pubkey: string) => {
      if (!accountPubkey || !repository || !isPinned(pubkey)) return

      try {
        const pk = Pubkey.tryFromString(pubkey)
        if (!pk) return

        const ownerPk = Pubkey.tryFromString(accountPubkey)
        if (!ownerPk) return

        const currentList = await repository.findByOwner(ownerPk)
        if (!currentList) return

        const change = currentList.unpin(pk)
        if (change.type === 'no_change') return

        await repository.save(currentList)
        setPinnedUsersList(currentList)
      } catch (error) {
        console.error('Failed to unpin user:', error)
      }
    },
    [accountPubkey, repository, isPinned]
  )

  const togglePin = useCallback(
    async (pubkey: string) => {
      if (isPinned(pubkey)) {
        await unpinUser(pubkey)
      } else {
        await pinUser(pubkey)
      }
    },
    [isPinned, pinUser, unpinUser]
  )

  return (
    <PinnedUsersContext.Provider
      value={{
        pinnedPubkeySet,
        isLoading,
        isPinned,
        pinUser,
        unpinUser,
        togglePin
      }}
    >
      {children}
    </PinnedUsersContext.Provider>
  )
}
