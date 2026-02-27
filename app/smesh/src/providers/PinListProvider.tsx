import {
  PinList,
  tryToPinList,
  Pubkey,
  CannotPinOthersContentError,
  CanOnlyPinNotesError,
  eventDispatcher,
  NotePinned,
  NoteUnpinned,
  PinsLimitExceeded,
  PinListPublished
} from '@/domain'
import client from '@/services/client.service'
import { Event } from 'nostr-tools'
import { createContext, useContext, useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { useNostr } from './NostrProvider'

type TPinListContext = {
  pinnedEventHexIdSet: Set<string>
  pin: (event: Event) => Promise<void>
  unpin: (event: Event) => Promise<void>
}

const PinListContext = createContext<TPinListContext | undefined>(undefined)

export const usePinList = () => {
  const context = useContext(PinListContext)
  if (!context) {
    throw new Error('usePinList must be used within a PinListProvider')
  }
  return context
}

export function PinListProvider({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation()
  const { pubkey: accountPubkey, pinListEvent, publish, updatePinListEvent } = useNostr()

  // Use domain aggregate for pinned event IDs
  const pinnedEventHexIdSet = useMemo(() => {
    const pinList = tryToPinList(pinListEvent)
    return pinList?.getEventIdSet() ?? new Set<string>()
  }, [pinListEvent])

  const pin = async (event: Event) => {
    if (!accountPubkey) return

    const _pin = async () => {
      const pinListEvent = await client.fetchPinListEvent(accountPubkey)
      const ownerPubkey = Pubkey.fromHex(accountPubkey)

      // Use domain aggregate
      const pinList = tryToPinList(pinListEvent) ?? PinList.empty(ownerPubkey)

      // Pin using domain method - throws if invalid
      const change = pinList.pin(event)
      if (change.type === 'no_change') return

      // Publish the updated pin list
      const draftEvent = pinList.toDraftEvent()
      const newPinListEvent = await publish(draftEvent)
      await updatePinListEvent(newPinListEvent)

      // Dispatch domain events
      if (change.type === 'pinned') {
        await eventDispatcher.dispatch(
          new NotePinned(ownerPubkey, change.entry.eventId)
        )
      } else if (change.type === 'limit_exceeded') {
        const removedIds = change.removed.map((e) => e.eventId.hex)
        await eventDispatcher.dispatch(
          new PinsLimitExceeded(ownerPubkey, removedIds)
        )
        // Also dispatch the pinned event for the new pin
        const newPinEntry = pinList.getEntries()[pinList.count - 1]
        if (newPinEntry) {
          await eventDispatcher.dispatch(
            new NotePinned(ownerPubkey, newPinEntry.eventId)
          )
        }
      }
      await eventDispatcher.dispatch(
        new PinListPublished(ownerPubkey, pinList.count)
      )
    }

    const { unwrap } = toast.promise(_pin, {
      loading: t('Pinning...'),
      success: t('Pinned!'),
      error: (err) => {
        if (err instanceof CannotPinOthersContentError) {
          return t('Can only pin your own notes')
        }
        if (err instanceof CanOnlyPinNotesError) {
          return t('Can only pin short text notes')
        }
        return t('Failed to pin: {{error}}', { error: err.message })
      }
    })
    await unwrap()
  }

  const unpin = async (event: Event) => {
    if (!accountPubkey) return

    const _unpin = async () => {
      const pinListEvent = await client.fetchPinListEvent(accountPubkey)
      if (!pinListEvent) return

      const pinList = tryToPinList(pinListEvent)
      if (!pinList) return

      const ownerPubkey = pinList.owner

      // Unpin using domain method
      const change = pinList.unpinEvent(event)
      if (change.type === 'no_change') return

      // Publish the updated pin list
      const draftEvent = pinList.toDraftEvent()
      const newPinListEvent = await publish(draftEvent)
      await updatePinListEvent(newPinListEvent)

      // Dispatch domain events
      if (change.type === 'unpinned') {
        await eventDispatcher.dispatch(
          new NoteUnpinned(ownerPubkey, change.eventId)
        )
        await eventDispatcher.dispatch(
          new PinListPublished(ownerPubkey, pinList.count)
        )
      }
    }

    const { unwrap } = toast.promise(_unpin, {
      loading: t('Unpinning...'),
      success: t('Unpinned!'),
      error: (err) => t('Failed to unpin: {{error}}', { error: err.message })
    })
    await unwrap()
  }

  return (
    <PinListContext.Provider
      value={{
        pinnedEventHexIdSet,
        pin,
        unpin
      }}
    >
      {children}
    </PinListContext.Provider>
  )
}
