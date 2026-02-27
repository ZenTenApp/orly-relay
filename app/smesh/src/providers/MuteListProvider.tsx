import {
  MuteList,
  fromMuteListToHexSet,
  Pubkey,
  CannotMuteSelfError,
  MuteVisibility
} from '@/domain'
import { MuteListRepositoryImpl } from '@/infrastructure/persistence'
import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useNostr } from './NostrProvider'

type TMuteListContext = {
  mutePubkeySet: Set<string>
  muteList: MuteList | null
  isLoading: boolean
  changing: boolean
  getMutePubkeys: () => string[]
  getMuteType: (pubkey: string) => MuteVisibility | null
  mutePubkeyPublicly: (pubkey: string) => Promise<void>
  mutePubkeyPrivately: (pubkey: string) => Promise<void>
  unmutePubkey: (pubkey: string) => Promise<void>
  switchToPublicMute: (pubkey: string) => Promise<void>
  switchToPrivateMute: (pubkey: string) => Promise<void>
}

const MuteListContext = createContext<TMuteListContext | undefined>(undefined)

export const useMuteList = () => {
  const context = useContext(MuteListContext)
  if (!context) {
    throw new Error('useMuteList must be used within a MuteListProvider')
  }
  return context
}

export function MuteListProvider({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation()
  const { pubkey: accountPubkey, publish, nip04Decrypt, nip04Encrypt } = useNostr()

  // State managed by this provider
  const [muteList, setMuteList] = useState<MuteList | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [changing, setChanging] = useState(false)

  // Use refs for unstable NostrProvider functions to prevent constant repository recreation.
  // publish, nip04Decrypt, nip04Encrypt are not memoized in NostrProvider, so they get new
  // references on every render. Using refs keeps the repository stable across renders.
  const publishRef = useRef(publish)
  const nip04DecryptRef = useRef(nip04Decrypt)
  const nip04EncryptRef = useRef(nip04Encrypt)
  publishRef.current = publish
  nip04DecryptRef.current = nip04Decrypt
  nip04EncryptRef.current = nip04Encrypt

  // Create repository instance - only recreated when accountPubkey changes
  const repository = useMemo(() => {
    if (!accountPubkey) return null
    return new MuteListRepositoryImpl({
      publish: (draftEvent) => publishRef.current(draftEvent),
      currentUserPubkey: accountPubkey,
      decrypt: async (ciphertext, pk) => nip04DecryptRef.current(pk, ciphertext),
      encrypt: async (plaintext, pk) => nip04EncryptRef.current(pk, plaintext)
    })
  }, [accountPubkey])

  // Legacy compatibility: expose as Set<string> for existing consumers
  const mutePubkeySet = useMemo(
    () => (muteList ? fromMuteListToHexSet(muteList) : new Set<string>()),
    [muteList]
  )

  // Load mute list when account changes
  useEffect(() => {
    let cancelled = false

    const loadMuteList = async () => {
      if (!accountPubkey || !repository) {
        if (!cancelled) setMuteList(null)
        return
      }

      if (!cancelled) setIsLoading(true)
      try {
        const ownerPubkey = Pubkey.tryFromString(accountPubkey)
        if (!ownerPubkey) {
          if (!cancelled) setMuteList(null)
          return
        }

        const list = await repository.findByOwner(ownerPubkey)
        if (!cancelled) setMuteList(list)
      } catch (error) {
        console.error('Failed to load mute list:', error)
        if (!cancelled) setMuteList(null)
      } finally {
        if (!cancelled) setIsLoading(false)
      }
    }

    loadMuteList()
    return () => { cancelled = true }
  }, [accountPubkey, repository])

  const getMutePubkeys = useCallback(() => {
    return Array.from(mutePubkeySet)
  }, [mutePubkeySet])

  const getMuteType = useCallback(
    (pubkey: string): MuteVisibility | null => {
      if (!muteList) return null
      const pk = Pubkey.tryFromString(pubkey)
      return pk ? muteList.getMuteVisibility(pk) : null
    },
    [muteList]
  )

  const mutePubkeyPublicly = useCallback(
    async (pubkey: string) => {
      if (!accountPubkey || !repository || changing) return

      setChanging(true)
      try {
        const ownerPubkey = Pubkey.fromHex(accountPubkey)
        const targetPubkey = Pubkey.tryFromString(pubkey)
        if (!targetPubkey) return

        // Fetch latest to avoid conflicts
        const currentMuteList = await repository.findByOwner(ownerPubkey)

        if (!currentMuteList) {
          const result = confirm(t('MuteListNotFoundConfirmation'))
          if (!result) return
        }

        const list = currentMuteList ?? MuteList.empty(ownerPubkey)

        try {
          const change = list.mutePublicly(targetPubkey)
          if (change.type === 'no_change') return

          await repository.save(list)
          setMuteList(list)
          toast.success(t('Successfully updated mute list'))
        } catch (error) {
          if (error instanceof CannotMuteSelfError) return
          throw error
        }
      } catch (error) {
        toast.error(t('Failed to mute user publicly') + ': ' + (error as Error).message)
      } finally {
        setChanging(false)
      }
    },
    [accountPubkey, repository, changing, t]
  )

  const mutePubkeyPrivately = useCallback(
    async (pubkey: string) => {
      if (!accountPubkey || !repository || changing) return

      setChanging(true)
      try {
        const ownerPubkey = Pubkey.fromHex(accountPubkey)
        const targetPubkey = Pubkey.tryFromString(pubkey)
        if (!targetPubkey) return

        const currentMuteList = await repository.findByOwner(ownerPubkey)

        if (!currentMuteList) {
          const result = confirm(t('MuteListNotFoundConfirmation'))
          if (!result) return
        }

        const list = currentMuteList ?? MuteList.empty(ownerPubkey)

        try {
          const change = list.mutePrivately(targetPubkey)
          if (change.type === 'no_change') return

          await repository.save(list)
          setMuteList(list)
          toast.success(t('Successfully updated mute list'))
        } catch (error) {
          if (error instanceof CannotMuteSelfError) return
          throw error
        }
      } catch (error) {
        toast.error(t('Failed to mute user privately') + ': ' + (error as Error).message)
      } finally {
        setChanging(false)
      }
    },
    [accountPubkey, repository, changing, t]
  )

  const unmutePubkey = useCallback(
    async (pubkey: string) => {
      if (!accountPubkey || !repository || changing) return

      setChanging(true)
      try {
        const ownerPubkey = Pubkey.fromHex(accountPubkey)
        const targetPubkey = Pubkey.tryFromString(pubkey)
        if (!targetPubkey) return

        const currentMuteList = await repository.findByOwner(ownerPubkey)
        if (!currentMuteList) return

        const change = currentMuteList.unmute(targetPubkey)
        if (change.type === 'no_change') return

        await repository.save(currentMuteList)
        setMuteList(currentMuteList)
        toast.success(t('Successfully updated mute list'))
      } catch (error) {
        toast.error(t('Failed to unmute user') + ': ' + (error as Error).message)
      } finally {
        setChanging(false)
      }
    },
    [accountPubkey, repository, changing, t]
  )

  const switchToPublicMute = useCallback(
    async (pubkey: string) => {
      if (!accountPubkey || !repository || changing) return

      setChanging(true)
      try {
        const ownerPubkey = Pubkey.fromHex(accountPubkey)
        const targetPubkey = Pubkey.tryFromString(pubkey)
        if (!targetPubkey) return

        const currentMuteList = await repository.findByOwner(ownerPubkey)
        if (!currentMuteList) return

        const change = currentMuteList.switchToPublic(targetPubkey)
        if (change.type === 'no_change') return

        await repository.save(currentMuteList)
        setMuteList(currentMuteList)
        toast.success(t('Successfully updated mute list'))
      } catch (error) {
        toast.error(t('Failed to switch mute visibility') + ': ' + (error as Error).message)
      } finally {
        setChanging(false)
      }
    },
    [accountPubkey, repository, changing, t]
  )

  const switchToPrivateMute = useCallback(
    async (pubkey: string) => {
      if (!accountPubkey || !repository || changing) return

      setChanging(true)
      try {
        const ownerPubkey = Pubkey.fromHex(accountPubkey)
        const targetPubkey = Pubkey.tryFromString(pubkey)
        if (!targetPubkey) return

        const currentMuteList = await repository.findByOwner(ownerPubkey)
        if (!currentMuteList) return

        const change = currentMuteList.switchToPrivate(targetPubkey)
        if (change.type === 'no_change') return

        await repository.save(currentMuteList)
        setMuteList(currentMuteList)
        toast.success(t('Successfully updated mute list'))
      } catch (error) {
        toast.error(t('Failed to switch mute visibility') + ': ' + (error as Error).message)
      } finally {
        setChanging(false)
      }
    },
    [accountPubkey, repository, changing, t]
  )

  return (
    <MuteListContext.Provider
      value={{
        mutePubkeySet,
        muteList,
        isLoading,
        changing,
        getMutePubkeys,
        getMuteType,
        mutePubkeyPublicly,
        mutePubkeyPrivately,
        unmutePubkey,
        switchToPublicMute,
        switchToPrivateMute
      }}
    >
      {children}
    </MuteListContext.Provider>
  )
}
