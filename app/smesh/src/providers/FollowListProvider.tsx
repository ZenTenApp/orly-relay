import {
  FollowList,
  fromFollowListToHexSet,
  Pubkey,
  CannotFollowSelfError
} from '@/domain'
import { FollowListRepositoryImpl } from '@/infrastructure/persistence'
import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useNostr } from './NostrProvider'

type TFollowListContext = {
  followingSet: Set<string>
  followList: FollowList | null
  isLoading: boolean
  follow: (pubkey: string) => Promise<void>
  unfollow: (pubkey: string) => Promise<void>
}

const FollowListContext = createContext<TFollowListContext | undefined>(undefined)

export const useFollowList = () => {
  const context = useContext(FollowListContext)
  if (!context) {
    throw new Error('useFollowList must be used within a FollowListProvider')
  }
  return context
}

export function FollowListProvider({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation()
  const { pubkey: accountPubkey, publish } = useNostr()

  // State managed by this provider
  const [followList, setFollowList] = useState<FollowList | null>(null)
  const [isLoading, setIsLoading] = useState(false)

  // Create repository instance
  const repository = useMemo(() => {
    if (!publish) return null
    return new FollowListRepositoryImpl({ publish })
  }, [publish])

  // Legacy compatibility: expose as Set<string> for existing consumers
  const followingSet = useMemo(
    () => (followList ? fromFollowListToHexSet(followList) : new Set<string>()),
    [followList]
  )

  // Load follow list when account changes
  useEffect(() => {
    const loadFollowList = async () => {
      if (!accountPubkey || !repository) {
        setFollowList(null)
        return
      }

      setIsLoading(true)
      try {
        const ownerPubkey = Pubkey.tryFromString(accountPubkey)
        if (!ownerPubkey) {
          setFollowList(null)
          return
        }

        const list = await repository.findByOwner(ownerPubkey)
        setFollowList(list)
      } catch (error) {
        console.error('Failed to load follow list:', error)
        setFollowList(null)
      } finally {
        setIsLoading(false)
      }
    }

    loadFollowList()
  }, [accountPubkey, repository])

  const follow = useCallback(
    async (pubkey: string) => {
      if (!accountPubkey || !repository) return

      const ownerPubkey = Pubkey.tryFromString(accountPubkey)
      const targetPubkey = Pubkey.tryFromString(pubkey)
      if (!ownerPubkey || !targetPubkey) return

      try {
        // Fetch latest to avoid conflicts
        const currentFollowList = await repository.findByOwner(ownerPubkey)

        if (!currentFollowList) {
          const result = confirm(t('FollowListNotFoundConfirmation'))
          if (!result) return
        }

        // Create or update using domain logic
        const list = currentFollowList ?? FollowList.empty(ownerPubkey)

        const change = list.follow(targetPubkey)
        if (change.type === 'no_change') return

        // Save via repository (handles publish and caching)
        await repository.save(list)

        // Update local state
        setFollowList(list)
      } catch (error) {
        if (error instanceof CannotFollowSelfError) {
          return
        }
        console.error('Failed to follow:', error)
        throw error
      }
    },
    [accountPubkey, repository, t]
  )

  const unfollow = useCallback(
    async (pubkey: string) => {
      if (!accountPubkey || !repository) return

      const ownerPubkey = Pubkey.tryFromString(accountPubkey)
      const targetPubkey = Pubkey.tryFromString(pubkey)
      if (!ownerPubkey || !targetPubkey) return

      try {
        // Fetch latest to avoid conflicts
        const currentFollowList = await repository.findByOwner(ownerPubkey)
        if (!currentFollowList) return

        const change = currentFollowList.unfollow(targetPubkey)
        if (change.type === 'no_change') return

        // Save via repository
        await repository.save(currentFollowList)

        // Update local state
        setFollowList(currentFollowList)
      } catch (error) {
        console.error('Failed to unfollow:', error)
        throw error
      }
    },
    [accountPubkey, repository]
  )

  return (
    <FollowListContext.Provider
      value={{
        followingSet,
        followList,
        isLoading,
        follow,
        unfollow
      }}
    >
      {children}
    </FollowListContext.Provider>
  )
}
