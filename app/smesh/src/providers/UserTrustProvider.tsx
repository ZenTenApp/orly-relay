import client from '@/services/client.service'
import fayan from '@/services/fayan.service'
import storage, { dispatchSettingsChanged } from '@/services/local-storage.service'
import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { useNostr } from './NostrProvider'

type TUserTrustContext = {
  hideUntrustedInteractions: boolean
  hideUntrustedNotifications: boolean
  hideUntrustedNotes: boolean
  updateHideUntrustedInteractions: (hide: boolean) => void
  updateHideUntrustedNotifications: (hide: boolean) => void
  updateHideUntrustedNotes: (hide: boolean) => void
  isUserTrusted: (pubkey: string) => boolean
  isSpammer: (pubkey: string) => Promise<boolean>
}

const UserTrustContext = createContext<TUserTrustContext | undefined>(undefined)

export const useUserTrust = () => {
  const context = useContext(UserTrustContext)
  if (!context) {
    throw new Error('useUserTrust must be used within a UserTrustProvider')
  }
  return context
}

const wotSet = new Set<string>()

export function UserTrustProvider({ children }: { children: React.ReactNode }) {
  const { pubkey: currentPubkey } = useNostr()
  const [hideUntrustedInteractions, setHideUntrustedInteractions] = useState(() =>
    storage.getHideUntrustedInteractions()
  )
  const [hideUntrustedNotifications, setHideUntrustedNotifications] = useState(() =>
    storage.getHideUntrustedNotifications()
  )
  const [hideUntrustedNotes, setHideUntrustedNotes] = useState(() =>
    storage.getHideUntrustedNotes()
  )

  useEffect(() => {
    if (!currentPubkey) return

    const initWoT = async () => {
      const followings = await client.fetchFollowings(currentPubkey, false)
      followings.forEach((pubkey) => wotSet.add(pubkey))

      const batchSize = 20
      for (let i = 0; i < followings.length; i += batchSize) {
        const batch = followings.slice(i, i + batchSize)
        await Promise.allSettled(
          batch.map(async (pubkey) => {
            const _followings = await client.fetchFollowings(pubkey, false)
            _followings.forEach((following) => {
              wotSet.add(following)
            })
          })
        )
        await new Promise((resolve) => setTimeout(resolve, 200))
      }
    }
    initWoT()
  }, [currentPubkey])

  const isUserTrusted = useCallback(
    (pubkey: string) => {
      if (!currentPubkey || pubkey === currentPubkey) return true
      return wotSet.has(pubkey)
    },
    [currentPubkey]
  )

  const isSpammer = useCallback(
    async (pubkey: string) => {
      if (isUserTrusted(pubkey)) return false
      const percentile = await fayan.fetchUserPercentile(pubkey)
      if (percentile === null) return false
      return percentile < 60
    },
    [isUserTrusted]
  )

  const updateHideUntrustedInteractions = (hide: boolean) => {
    setHideUntrustedInteractions(hide)
    storage.setHideUntrustedInteractions(hide)
    dispatchSettingsChanged()
  }

  const updateHideUntrustedNotifications = (hide: boolean) => {
    setHideUntrustedNotifications(hide)
    storage.setHideUntrustedNotifications(hide)
    dispatchSettingsChanged()
  }

  const updateHideUntrustedNotes = (hide: boolean) => {
    setHideUntrustedNotes(hide)
    storage.setHideUntrustedNotes(hide)
    dispatchSettingsChanged()
  }

  return (
    <UserTrustContext.Provider
      value={{
        hideUntrustedInteractions,
        hideUntrustedNotifications,
        hideUntrustedNotes,
        updateHideUntrustedInteractions,
        updateHideUntrustedNotifications,
        updateHideUntrustedNotes,
        isUserTrusted,
        isSpammer
      }}
    >
      {children}
    </UserTrustContext.Provider>
  )
}
