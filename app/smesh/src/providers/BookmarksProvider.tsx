import { BookmarkList, tryToBookmarkList, Pubkey, eventDispatcher, EventBookmarked, EventUnbookmarked, BookmarkListPublished } from '@/domain'
import client from '@/services/client.service'
import { Event } from 'nostr-tools'
import { createContext, useContext } from 'react'
import { useNostr } from './NostrProvider'

type TBookmarksContext = {
  addBookmark: (event: Event) => Promise<void>
  removeBookmark: (event: Event) => Promise<void>
}

const BookmarksContext = createContext<TBookmarksContext | undefined>(undefined)

export const useBookmarks = () => {
  const context = useContext(BookmarksContext)
  if (!context) {
    throw new Error('useBookmarks must be used within a BookmarksProvider')
  }
  return context
}

export function BookmarksProvider({ children }: { children: React.ReactNode }) {
  const { pubkey: accountPubkey, publish, updateBookmarkListEvent } = useNostr()

  const addBookmark = async (event: Event) => {
    if (!accountPubkey) return

    const bookmarkListEvent = await client.fetchBookmarkListEvent(accountPubkey)
    const ownerPubkey = Pubkey.fromHex(accountPubkey)

    // Use domain aggregate
    const bookmarkList = tryToBookmarkList(bookmarkListEvent) ?? BookmarkList.empty(ownerPubkey)

    // Add bookmark using domain method
    const change = bookmarkList.addFromEvent(event)
    if (change.type === 'no_change') return

    // Publish the updated bookmark list
    const draftEvent = bookmarkList.toDraftEvent()
    const newBookmarkEvent = await publish(draftEvent)
    await updateBookmarkListEvent(newBookmarkEvent)

    // Dispatch domain events
    if (change.type === 'added') {
      await eventDispatcher.dispatch(
        new EventBookmarked(ownerPubkey, change.entry.id, change.entry.type)
      )
      await eventDispatcher.dispatch(
        new BookmarkListPublished(ownerPubkey, bookmarkList.count)
      )
    }
  }

  const removeBookmark = async (event: Event) => {
    if (!accountPubkey) return

    const bookmarkListEvent = await client.fetchBookmarkListEvent(accountPubkey)
    if (!bookmarkListEvent) return

    const bookmarkList = tryToBookmarkList(bookmarkListEvent)
    if (!bookmarkList) return

    const ownerPubkey = bookmarkList.owner

    // Remove bookmark using domain method
    const change = bookmarkList.removeFromEvent(event)
    if (change.type === 'no_change') return

    // Publish the updated bookmark list
    const draftEvent = bookmarkList.toDraftEvent()
    const newBookmarkEvent = await publish(draftEvent)
    await updateBookmarkListEvent(newBookmarkEvent)

    // Dispatch domain events
    if (change.type === 'removed') {
      await eventDispatcher.dispatch(
        new EventUnbookmarked(ownerPubkey, change.id)
      )
      await eventDispatcher.dispatch(
        new BookmarkListPublished(ownerPubkey, bookmarkList.count)
      )
    }
  }

  return (
    <BookmarksContext.Provider
      value={{
        addBookmark,
        removeBookmark
      }}
    >
      {children}
    </BookmarksContext.Provider>
  )
}
