import { ExtendedKind } from '@/constants'
import { tagNameEquals } from '@/lib/tag'
import { TDMDeletedState, TRelayInfo } from '@/types'
import { Event, Filter, kinds, matchFilters } from 'nostr-tools'

type TValue<T = any> = {
  key: string
  value: T | null
  addedAt: number
}

const StoreNames = {
  PROFILE_EVENTS: 'profileEvents',
  RELAY_LIST_EVENTS: 'relayListEvents',
  FOLLOW_LIST_EVENTS: 'followListEvents',
  MUTE_LIST_EVENTS: 'muteListEvents',
  BOOKMARK_LIST_EVENTS: 'bookmarkListEvents',
  BLOSSOM_SERVER_LIST_EVENTS: 'blossomServerListEvents',
  USER_EMOJI_LIST_EVENTS: 'userEmojiListEvents',
  EMOJI_SET_EVENTS: 'emojiSetEvents',
  PIN_LIST_EVENTS: 'pinListEvents',
  FAVORITE_RELAYS: 'favoriteRelays',
  RELAY_SETS: 'relaySets',
  FOLLOWING_FAVORITE_RELAYS: 'followingFavoriteRelays',
  RELAY_INFOS: 'relayInfos',
  DECRYPTED_CONTENTS: 'decryptedContents',
  PINNED_USERS_EVENTS: 'pinnedUsersEvents',
  DM_EVENTS: 'dmEvents',
  DM_CONVERSATIONS: 'dmConversations',
  DM_MESSAGES: 'dmMessages',
  UNWRAPPED_GIFT_WRAPS: 'unwrappedGiftWraps',
  DM_DELETED_STATE: 'dmDeletedState',
  CACHED_EVENTS: 'cachedEvents', // General event cache for NRC cache relays
  RELAY_STATS: 'relayStats', // Per-relay per-network failure stats
  MANAGED_RELAYS: 'managedRelays', // Outbox relay approval state
  MUTE_DECRYPTED_TAGS: 'muteDecryptedTags', // deprecated
  RELAY_INFO_EVENTS: 'relayInfoEvents' // deprecated
}

class IndexedDbService {
  static instance: IndexedDbService
  static getInstance(): IndexedDbService {
    if (!IndexedDbService.instance) {
      IndexedDbService.instance = new IndexedDbService()
      IndexedDbService.instance.init()
    }
    return IndexedDbService.instance
  }

  private db: IDBDatabase | null = null
  private initPromise: Promise<void> | null = null

  init(): Promise<void> {
    if (!this.initPromise) {
      this.initPromise = new Promise((resolve, reject) => {
        const request = window.indexedDB.open('smesh', 16)

        request.onerror = (event) => {
          reject(event)
        }

        request.onsuccess = () => {
          this.db = request.result
          resolve()
        }

        request.onupgradeneeded = () => {
          const db = request.result
          if (!db.objectStoreNames.contains(StoreNames.PROFILE_EVENTS)) {
            db.createObjectStore(StoreNames.PROFILE_EVENTS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.RELAY_LIST_EVENTS)) {
            db.createObjectStore(StoreNames.RELAY_LIST_EVENTS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.FOLLOW_LIST_EVENTS)) {
            db.createObjectStore(StoreNames.FOLLOW_LIST_EVENTS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.MUTE_LIST_EVENTS)) {
            db.createObjectStore(StoreNames.MUTE_LIST_EVENTS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.BOOKMARK_LIST_EVENTS)) {
            db.createObjectStore(StoreNames.BOOKMARK_LIST_EVENTS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.DECRYPTED_CONTENTS)) {
            db.createObjectStore(StoreNames.DECRYPTED_CONTENTS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.FAVORITE_RELAYS)) {
            db.createObjectStore(StoreNames.FAVORITE_RELAYS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.RELAY_SETS)) {
            db.createObjectStore(StoreNames.RELAY_SETS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.FOLLOWING_FAVORITE_RELAYS)) {
            db.createObjectStore(StoreNames.FOLLOWING_FAVORITE_RELAYS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.BLOSSOM_SERVER_LIST_EVENTS)) {
            db.createObjectStore(StoreNames.BLOSSOM_SERVER_LIST_EVENTS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.USER_EMOJI_LIST_EVENTS)) {
            db.createObjectStore(StoreNames.USER_EMOJI_LIST_EVENTS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.EMOJI_SET_EVENTS)) {
            db.createObjectStore(StoreNames.EMOJI_SET_EVENTS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.RELAY_INFOS)) {
            db.createObjectStore(StoreNames.RELAY_INFOS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.PIN_LIST_EVENTS)) {
            db.createObjectStore(StoreNames.PIN_LIST_EVENTS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.PINNED_USERS_EVENTS)) {
            db.createObjectStore(StoreNames.PINNED_USERS_EVENTS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.DM_EVENTS)) {
            db.createObjectStore(StoreNames.DM_EVENTS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.DM_CONVERSATIONS)) {
            db.createObjectStore(StoreNames.DM_CONVERSATIONS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.DM_MESSAGES)) {
            db.createObjectStore(StoreNames.DM_MESSAGES, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.UNWRAPPED_GIFT_WRAPS)) {
            db.createObjectStore(StoreNames.UNWRAPPED_GIFT_WRAPS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.DM_DELETED_STATE)) {
            db.createObjectStore(StoreNames.DM_DELETED_STATE, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.CACHED_EVENTS)) {
            const store = db.createObjectStore(StoreNames.CACHED_EVENTS, { keyPath: 'id' })
            store.createIndex('kind', 'kind', { unique: false })
            store.createIndex('pubkey', 'pubkey', { unique: false })
            store.createIndex('created_at', 'created_at', { unique: false })
          }

          if (!db.objectStoreNames.contains(StoreNames.RELAY_STATS)) {
            db.createObjectStore(StoreNames.RELAY_STATS, { keyPath: 'key' })
          }
          if (!db.objectStoreNames.contains(StoreNames.MANAGED_RELAYS)) {
            db.createObjectStore(StoreNames.MANAGED_RELAYS, { keyPath: 'key' })
          }

          if (db.objectStoreNames.contains(StoreNames.RELAY_INFO_EVENTS)) {
            db.deleteObjectStore(StoreNames.RELAY_INFO_EVENTS)
          }
          if (db.objectStoreNames.contains(StoreNames.MUTE_DECRYPTED_TAGS)) {
            db.deleteObjectStore(StoreNames.MUTE_DECRYPTED_TAGS)
          }
          this.db = db
        }
      })
      setTimeout(() => this.cleanUp(), 1000 * 60) // 1 minute
    }
    return this.initPromise
  }

  async putNullReplaceableEvent(pubkey: string, kind: number, d?: string) {
    const storeName = this.getStoreNameByKind(kind)
    if (!storeName) {
      return Promise.reject('store name not found')
    }
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(storeName, 'readwrite')
      const store = transaction.objectStore(storeName)

      const key = this.getReplaceableEventKey(pubkey, d)
      const getRequest = store.get(key)
      getRequest.onsuccess = () => {
        const oldValue = getRequest.result as TValue<Event> | undefined
        if (oldValue) {
          transaction.commit()
          return resolve(oldValue.value)
        }
        const putRequest = store.put(this.formatValue(key, null))
        putRequest.onsuccess = () => {
          transaction.commit()
          resolve(null)
        }

        putRequest.onerror = (event) => {
          transaction.commit()
          reject(event)
        }
      }

      getRequest.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async putReplaceableEvent(event: Event): Promise<Event> {
    const storeName = this.getStoreNameByKind(event.kind)
    if (!storeName) {
      return Promise.reject('store name not found')
    }
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(storeName, 'readwrite')
      const store = transaction.objectStore(storeName)

      const key = this.getReplaceableEventKeyFromEvent(event)
      const getRequest = store.get(key)
      getRequest.onsuccess = () => {
        const oldValue = getRequest.result as TValue<Event> | undefined
        if (oldValue?.value && oldValue.value.created_at >= event.created_at) {
          transaction.commit()
          return resolve(oldValue.value)
        }
        const putRequest = store.put(this.formatValue(key, event))
        putRequest.onsuccess = () => {
          transaction.commit()
          resolve(event)
        }

        putRequest.onerror = (event) => {
          transaction.commit()
          reject(event)
        }
      }

      getRequest.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async getReplaceableEventByCoordinate(coordinate: string): Promise<Event | undefined | null> {
    const [kind, pubkey, ...rest] = coordinate.split(':')
    const d = rest.length > 0 ? rest.join(':') : undefined
    return this.getReplaceableEvent(pubkey, parseInt(kind), d)
  }

  async getReplaceableEvent(
    pubkey: string,
    kind: number,
    d?: string
  ): Promise<Event | undefined | null> {
    const storeName = this.getStoreNameByKind(kind)
    if (!storeName) {
      return undefined
    }
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(storeName, 'readonly')
      const store = transaction.objectStore(storeName)
      const key = this.getReplaceableEventKey(pubkey, d)
      const request = store.get(key)

      request.onsuccess = () => {
        transaction.commit()
        resolve((request.result as TValue<Event>)?.value)
      }

      request.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async getManyReplaceableEvents(
    pubkeys: readonly string[],
    kind: number
  ): Promise<(Event | undefined | null)[]> {
    const storeName = this.getStoreNameByKind(kind)
    if (!storeName) {
      return Promise.reject('store name not found')
    }
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(storeName, 'readonly')
      const store = transaction.objectStore(storeName)
      const events: (Event | null)[] = new Array(pubkeys.length).fill(undefined)
      let count = 0
      pubkeys.forEach((pubkey, i) => {
        const request = store.get(this.getReplaceableEventKey(pubkey))

        request.onsuccess = () => {
          const event = (request.result as TValue<Event | null>)?.value
          if (event || event === null) {
            events[i] = event
          }

          if (++count === pubkeys.length) {
            transaction.commit()
            resolve(events)
          }
        }

        request.onerror = () => {
          if (++count === pubkeys.length) {
            transaction.commit()
            resolve(events)
          }
        }
      })
    })
  }

  async getDecryptedContent(key: string): Promise<string | null> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.DECRYPTED_CONTENTS, 'readonly')
      const store = transaction.objectStore(StoreNames.DECRYPTED_CONTENTS)
      const request = store.get(key)

      request.onsuccess = () => {
        transaction.commit()
        resolve((request.result as TValue<string>)?.value)
      }

      request.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async putDecryptedContent(key: string, content: string): Promise<void> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.DECRYPTED_CONTENTS, 'readwrite')
      const store = transaction.objectStore(StoreNames.DECRYPTED_CONTENTS)

      const putRequest = store.put(this.formatValue(key, content))
      putRequest.onsuccess = () => {
        transaction.commit()
        resolve()
      }

      putRequest.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async iterateProfileEvents(callback: (event: Event) => Promise<void>): Promise<void> {
    await this.initPromise
    if (!this.db) {
      return
    }

    return new Promise<void>((resolve, reject) => {
      const transaction = this.db!.transaction(StoreNames.PROFILE_EVENTS, 'readwrite')
      const store = transaction.objectStore(StoreNames.PROFILE_EVENTS)
      const request = store.openCursor()
      request.onsuccess = (event) => {
        const cursor = (event.target as IDBRequest).result
        if (cursor) {
          const value = (cursor.value as TValue<Event>).value
          if (value) {
            callback(value)
          }
          cursor.continue()
        } else {
          transaction.commit()
          resolve()
        }
      }

      request.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async putFollowingFavoriteRelays(pubkey: string, relays: [string, string[]][]): Promise<void> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.FOLLOWING_FAVORITE_RELAYS, 'readwrite')
      const store = transaction.objectStore(StoreNames.FOLLOWING_FAVORITE_RELAYS)

      const putRequest = store.put(this.formatValue(pubkey, relays))
      putRequest.onsuccess = () => {
        transaction.commit()
        resolve()
      }

      putRequest.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async getFollowingFavoriteRelays(pubkey: string): Promise<[string, string[]][] | null> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.FOLLOWING_FAVORITE_RELAYS, 'readonly')
      const store = transaction.objectStore(StoreNames.FOLLOWING_FAVORITE_RELAYS)
      const request = store.get(pubkey)

      request.onsuccess = () => {
        transaction.commit()
        resolve((request.result as TValue<[string, string[]][]>)?.value)
      }

      request.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async putRelayInfo(relayInfo: TRelayInfo): Promise<void> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.RELAY_INFOS, 'readwrite')
      const store = transaction.objectStore(StoreNames.RELAY_INFOS)

      const putRequest = store.put(this.formatValue(relayInfo.url, relayInfo))
      putRequest.onsuccess = () => {
        transaction.commit()
        resolve()
      }

      putRequest.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async getRelayInfo(url: string): Promise<TRelayInfo | null> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.RELAY_INFOS, 'readonly')
      const store = transaction.objectStore(StoreNames.RELAY_INFOS)
      const request = store.get(url)

      request.onsuccess = () => {
        transaction.commit()
        resolve((request.result as TValue<TRelayInfo>)?.value)
      }

      request.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  // DM-related methods
  async putDMEvent(event: Event): Promise<void> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.DM_EVENTS, 'readwrite')
      const store = transaction.objectStore(StoreNames.DM_EVENTS)

      const putRequest = store.put(this.formatValue(event.id, event))
      putRequest.onsuccess = () => {
        transaction.commit()
        resolve()
      }

      putRequest.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async getDMEvent(eventId: string): Promise<Event | null> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.DM_EVENTS, 'readonly')
      const store = transaction.objectStore(StoreNames.DM_EVENTS)
      const request = store.get(eventId)

      request.onsuccess = () => {
        transaction.commit()
        resolve((request.result as TValue<Event>)?.value ?? null)
      }

      request.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async getAllDMEvents(userPubkey: string): Promise<Event[]> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.DM_EVENTS, 'readonly')
      const store = transaction.objectStore(StoreNames.DM_EVENTS)
      const request = store.openCursor()
      const events: Event[] = []

      request.onsuccess = (event) => {
        const cursor = (event.target as IDBRequest).result
        if (cursor) {
          const dmEvent = (cursor.value as TValue<Event>).value
          if (dmEvent) {
            // Include events where user is sender or recipient
            const isUserEvent =
              dmEvent.pubkey === userPubkey ||
              dmEvent.tags.some((tag) => tag[0] === 'p' && tag[1] === userPubkey)
            if (isUserEvent) {
              events.push(dmEvent)
            }
          }
          cursor.continue()
        } else {
          transaction.commit()
          resolve(events)
        }
      }

      request.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async putDMConversation(
    userPubkey: string,
    partnerPubkey: string,
    lastMessageAt: number,
    lastMessagePreview: string,
    encryptionType: 'nip04' | 'nip17' | null
  ): Promise<void> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.DM_CONVERSATIONS, 'readwrite')
      const store = transaction.objectStore(StoreNames.DM_CONVERSATIONS)
      const key = `${userPubkey}:${partnerPubkey}`

      const putRequest = store.put(
        this.formatValue(key, {
          partnerPubkey,
          lastMessageAt,
          lastMessagePreview,
          encryptionType
        })
      )
      putRequest.onsuccess = () => {
        transaction.commit()
        resolve()
      }

      putRequest.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async getDMConversations(
    userPubkey: string
  ): Promise<
    Array<{
      partnerPubkey: string
      lastMessageAt: number
      lastMessagePreview: string
      encryptionType: 'nip04' | 'nip17' | null
    }>
  > {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.DM_CONVERSATIONS, 'readonly')
      const store = transaction.objectStore(StoreNames.DM_CONVERSATIONS)
      const request = store.openCursor()
      const conversations: Array<{
        partnerPubkey: string
        lastMessageAt: number
        lastMessagePreview: string
        encryptionType: 'nip04' | 'nip17' | null
      }> = []

      request.onsuccess = (event) => {
        const cursor = (event.target as IDBRequest).result
        if (cursor) {
          const key = cursor.key as string
          if (key.startsWith(`${userPubkey}:`)) {
            const value = (cursor.value as TValue).value
            if (value) {
              conversations.push(value)
            }
          }
          cursor.continue()
        } else {
          transaction.commit()
          // Sort by lastMessageAt descending
          conversations.sort((a, b) => b.lastMessageAt - a.lastMessageAt)
          resolve(conversations)
        }
      }

      request.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async putConversationRelaySettings(
    userPubkey: string,
    partnerPubkey: string,
    selectedRelays: string[]
  ): Promise<void> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.DM_CONVERSATIONS, 'readwrite')
      const store = transaction.objectStore(StoreNames.DM_CONVERSATIONS)
      const key = `${userPubkey}:${partnerPubkey}:relays`

      const putRequest = store.put(this.formatValue(key, { selectedRelays }))
      putRequest.onsuccess = () => {
        transaction.commit()
        resolve()
      }

      putRequest.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async getConversationRelaySettings(
    userPubkey: string,
    partnerPubkey: string
  ): Promise<string[] | null> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.DM_CONVERSATIONS, 'readonly')
      const store = transaction.objectStore(StoreNames.DM_CONVERSATIONS)
      const key = `${userPubkey}:${partnerPubkey}:relays`
      const request = store.get(key)

      request.onsuccess = () => {
        transaction.commit()
        const result = (request.result as TValue)?.value
        resolve(result?.selectedRelays ?? null)
      }

      request.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async putConversationEncryptionPreference(
    userPubkey: string,
    partnerPubkey: string,
    preference: 'nip04' | 'nip17' | 'auto'
  ): Promise<void> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.DM_CONVERSATIONS, 'readwrite')
      const store = transaction.objectStore(StoreNames.DM_CONVERSATIONS)
      const key = `${userPubkey}:${partnerPubkey}:encryption`

      const putRequest = store.put(this.formatValue(key, { preference }))
      putRequest.onsuccess = () => {
        transaction.commit()
        resolve()
      }

      putRequest.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async getConversationEncryptionPreference(
    userPubkey: string,
    partnerPubkey: string
  ): Promise<'nip04' | 'nip17' | 'auto' | null> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.DM_CONVERSATIONS, 'readonly')
      const store = transaction.objectStore(StoreNames.DM_CONVERSATIONS)
      const key = `${userPubkey}:${partnerPubkey}:encryption`
      const request = store.get(key)

      request.onsuccess = () => {
        transaction.commit()
        const result = (request.result as TValue)?.value
        resolve(result?.preference ?? null)
      }

      request.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async putConversationMessages(
    userPubkey: string,
    partnerPubkey: string,
    messages: Array<{
      id: string
      senderPubkey: string
      recipientPubkey: string
      content: string
      createdAt: number
      encryptionType: 'nip04' | 'nip17'
      seenOnRelays?: string[]
    }>
  ): Promise<void> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.DM_MESSAGES, 'readwrite')
      const store = transaction.objectStore(StoreNames.DM_MESSAGES)
      const key = `${userPubkey}:${partnerPubkey}`

      const putRequest = store.put(this.formatValue(key, messages))
      putRequest.onsuccess = () => {
        transaction.commit()
        resolve()
      }

      putRequest.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  async getConversationMessages(
    userPubkey: string,
    partnerPubkey: string
  ): Promise<
    Array<{
      id: string
      senderPubkey: string
      recipientPubkey: string
      content: string
      createdAt: number
      encryptionType: 'nip04' | 'nip17'
      seenOnRelays?: string[]
    }> | null
  > {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.DM_MESSAGES, 'readonly')
      const store = transaction.objectStore(StoreNames.DM_MESSAGES)
      const key = `${userPubkey}:${partnerPubkey}`
      const request = store.get(key)

      request.onsuccess = () => {
        transaction.commit()
        resolve((request.result as TValue)?.value ?? null)
      }

      request.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  /**
   * Cache an unwrapped NIP-17 gift wrap inner event
   * This avoids repeated decryption just to identify the sender
   */
  async putUnwrappedGiftWrap(
    giftWrapId: string,
    innerEvent: {
      pubkey: string // actual sender
      recipientPubkey: string
      content: string
      createdAt: number
    }
  ): Promise<void> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.UNWRAPPED_GIFT_WRAPS, 'readwrite')
      const store = transaction.objectStore(StoreNames.UNWRAPPED_GIFT_WRAPS)

      const putRequest = store.put(this.formatValue(giftWrapId, innerEvent))
      putRequest.onsuccess = () => {
        transaction.commit()
        resolve()
      }

      putRequest.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  /**
   * Get a cached unwrapped NIP-17 gift wrap inner event
   */
  async getUnwrappedGiftWrap(
    giftWrapId: string
  ): Promise<{
    pubkey: string
    recipientPubkey: string
    content: string
    createdAt: number
  } | null> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.UNWRAPPED_GIFT_WRAPS, 'readonly')
      const store = transaction.objectStore(StoreNames.UNWRAPPED_GIFT_WRAPS)
      const request = store.get(giftWrapId)

      request.onsuccess = () => {
        transaction.commit()
        resolve((request.result as TValue)?.value ?? null)
      }

      request.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  /**
   * Clear all DM-related caches (for full refresh)
   */
  async clearAllDMCaches(): Promise<void> {
    await this.initPromise
    if (!this.db) {
      return
    }

    const storeNames = [
      StoreNames.DM_EVENTS,
      StoreNames.DM_CONVERSATIONS,
      StoreNames.DM_MESSAGES,
      StoreNames.UNWRAPPED_GIFT_WRAPS,
      StoreNames.DECRYPTED_CONTENTS
    ]

    const transaction = this.db.transaction(storeNames, 'readwrite')

    await Promise.all(
      storeNames.map(
        (storeName) =>
          new Promise<void>((resolve, reject) => {
            const store = transaction.objectStore(storeName)
            const request = store.clear()
            request.onsuccess = () => resolve()
            request.onerror = (event) => reject(event)
          })
      )
    )

    transaction.commit()
  }

  /**
   * Get the deleted messages state for a user (local cache only)
   */
  async getDeletedMessagesState(pubkey: string): Promise<TDMDeletedState | null> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.DM_DELETED_STATE, 'readonly')
      const store = transaction.objectStore(StoreNames.DM_DELETED_STATE)
      const request = store.get(pubkey)

      request.onsuccess = () => {
        transaction.commit()
        resolve((request.result as TValue<TDMDeletedState>)?.value ?? null)
      }

      request.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  /**
   * Store the deleted messages state for a user (local cache)
   */
  async putDeletedMessagesState(pubkey: string, state: TDMDeletedState): Promise<void> {
    await this.initPromise
    return new Promise((resolve, reject) => {
      if (!this.db) {
        return reject('database not initialized')
      }
      const transaction = this.db.transaction(StoreNames.DM_DELETED_STATE, 'readwrite')
      const store = transaction.objectStore(StoreNames.DM_DELETED_STATE)

      const putRequest = store.put(this.formatValue(pubkey, state))
      putRequest.onsuccess = () => {
        transaction.commit()
        resolve()
      }

      putRequest.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  private getReplaceableEventKeyFromEvent(event: Event): string {
    if (
      [kinds.Metadata, kinds.Contacts].includes(event.kind) ||
      (event.kind >= 10000 && event.kind < 20000)
    ) {
      return this.getReplaceableEventKey(event.pubkey)
    }

    const [, d] = event.tags.find(tagNameEquals('d')) ?? []
    return this.getReplaceableEventKey(event.pubkey, d)
  }

  private getReplaceableEventKey(pubkey: string, d?: string): string {
    return d === undefined ? pubkey : `${pubkey}:${d}`
  }

  private getStoreNameByKind(kind: number): string | undefined {
    switch (kind) {
      case kinds.Metadata:
        return StoreNames.PROFILE_EVENTS
      case kinds.RelayList:
        return StoreNames.RELAY_LIST_EVENTS
      case kinds.Contacts:
        return StoreNames.FOLLOW_LIST_EVENTS
      case kinds.Mutelist:
        return StoreNames.MUTE_LIST_EVENTS
      case ExtendedKind.BLOSSOM_SERVER_LIST:
        return StoreNames.BLOSSOM_SERVER_LIST_EVENTS
      case kinds.Relaysets:
        return StoreNames.RELAY_SETS
      case ExtendedKind.FAVORITE_RELAYS:
        return StoreNames.FAVORITE_RELAYS
      case kinds.BookmarkList:
        return StoreNames.BOOKMARK_LIST_EVENTS
      case kinds.UserEmojiList:
        return StoreNames.USER_EMOJI_LIST_EVENTS
      case kinds.Emojisets:
        return StoreNames.EMOJI_SET_EVENTS
      case kinds.Pinlist:
        return StoreNames.PIN_LIST_EVENTS
      case ExtendedKind.PINNED_USERS:
        return StoreNames.PINNED_USERS_EVENTS
      default:
        return undefined
    }
  }

  private formatValue<T>(key: string, value: T): TValue<T> {
    return {
      key,
      value,
      addedAt: Date.now()
    }
  }

  /**
   * Query all events across all stores for NRC sync.
   * Returns events matching the provided filters.
   *
   * Note: This method queries all event-containing stores and filters
   * client-side using matchFilters. Device-specific event filtering
   * should be done by the caller.
   */
  async queryEventsForNRC(filters: Filter[]): Promise<Event[]> {
    await this.initPromise
    if (!this.db) {
      return []
    }

    // List of stores that contain Event objects
    const eventStores = [
      StoreNames.PROFILE_EVENTS,
      StoreNames.RELAY_LIST_EVENTS,
      StoreNames.FOLLOW_LIST_EVENTS,
      StoreNames.MUTE_LIST_EVENTS,
      StoreNames.BOOKMARK_LIST_EVENTS,
      StoreNames.BLOSSOM_SERVER_LIST_EVENTS,
      StoreNames.USER_EMOJI_LIST_EVENTS,
      StoreNames.EMOJI_SET_EVENTS,
      StoreNames.PIN_LIST_EVENTS,
      StoreNames.PINNED_USERS_EVENTS,
      StoreNames.FAVORITE_RELAYS,
      StoreNames.RELAY_SETS,
      StoreNames.DM_EVENTS
    ]

    const allEvents: Event[] = []

    // Query each store
    const transaction = this.db.transaction(eventStores, 'readonly')

    await Promise.all(
      eventStores.map(
        (storeName) =>
          new Promise<void>((resolve) => {
            const store = transaction.objectStore(storeName)
            const request = store.openCursor()

            request.onsuccess = (event) => {
              const cursor = (event.target as IDBRequest).result
              if (cursor) {
                const value = cursor.value as TValue<Event | null>
                if (value.value) {
                  // Check if event matches any of the filters
                  if (matchFilters(filters, value.value)) {
                    allEvents.push(value.value)
                  }
                }
                cursor.continue()
              } else {
                resolve()
              }
            }

            request.onerror = () => {
              resolve() // Continue even if one store fails
            }
          })
      )
    )

    // Sort by created_at descending (newest first)
    allEvents.sort((a, b) => b.created_at - a.created_at)

    // Apply limit from filters if specified
    const limit = Math.min(...filters.map((f) => f.limit ?? Infinity))
    if (limit !== Infinity && limit > 0) {
      return allEvents.slice(0, limit)
    }

    return allEvents
  }

  /**
   * Store an event in the general cache.
   * Used by NRC cache relays to cache events fetched from regular relays.
   */
  async putCachedEvent(event: Event): Promise<void> {
    await this.initPromise
    if (!this.db) {
      return
    }

    return new Promise((resolve, reject) => {
      const transaction = this.db!.transaction(StoreNames.CACHED_EVENTS, 'readwrite')
      const store = transaction.objectStore(StoreNames.CACHED_EVENTS)

      // Store the event directly (it already has an 'id' field)
      const putRequest = store.put(event)
      putRequest.onsuccess = () => {
        transaction.commit()
        resolve()
      }

      putRequest.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  /**
   * Store multiple events in the general cache.
   */
  async putCachedEvents(events: Event[]): Promise<void> {
    if (events.length === 0) return

    await this.initPromise
    if (!this.db) {
      return
    }

    return new Promise((resolve) => {
      const transaction = this.db!.transaction(StoreNames.CACHED_EVENTS, 'readwrite')
      const store = transaction.objectStore(StoreNames.CACHED_EVENTS)

      let completed = 0
      for (const event of events) {
        const putRequest = store.put(event)
        putRequest.onsuccess = () => {
          completed++
          if (completed === events.length) {
            transaction.commit()
            resolve()
          }
        }
        putRequest.onerror = () => {
          completed++
          if (completed === events.length) {
            transaction.commit()
            resolve()
          }
        }
      }
    })
  }

  /**
   * Get a cached event by ID.
   */
  async getCachedEvent(id: string): Promise<Event | null> {
    await this.initPromise
    if (!this.db) {
      return null
    }

    return new Promise((resolve, reject) => {
      const transaction = this.db!.transaction(StoreNames.CACHED_EVENTS, 'readonly')
      const store = transaction.objectStore(StoreNames.CACHED_EVENTS)
      const request = store.get(id)

      request.onsuccess = () => {
        transaction.commit()
        resolve(request.result ?? null)
      }

      request.onerror = (event) => {
        transaction.commit()
        reject(event)
      }
    })
  }

  /**
   * Query cached events matching the provided filters.
   * Returns events sorted by created_at descending.
   */
  async queryCachedEvents(filters: Filter[]): Promise<Event[]> {
    await this.initPromise
    if (!this.db) {
      return []
    }

    return new Promise((resolve) => {
      const transaction = this.db!.transaction(StoreNames.CACHED_EVENTS, 'readonly')
      const store = transaction.objectStore(StoreNames.CACHED_EVENTS)
      const request = store.openCursor()
      const events: Event[] = []

      request.onsuccess = (event) => {
        const cursor = (event.target as IDBRequest).result
        if (cursor) {
          const cachedEvent = cursor.value as Event
          if (cachedEvent && matchFilters(filters, cachedEvent)) {
            events.push(cachedEvent)
          }
          cursor.continue()
        } else {
          transaction.commit()
          // Sort by created_at descending
          events.sort((a, b) => b.created_at - a.created_at)

          // Apply limit from filters if specified
          const limit = Math.min(...filters.map((f) => f.limit ?? Infinity))
          if (limit !== Infinity && limit > 0) {
            resolve(events.slice(0, limit))
          } else {
            resolve(events)
          }
        }
      }

      request.onerror = () => {
        transaction.commit()
        resolve([])
      }
    })
  }

  /**
   * Clean up expired cached events.
   * Removes events older than the specified number of days.
   */
  async cleanupExpiredCache(maxAgeDays: number = 7): Promise<number> {
    await this.initPromise
    if (!this.db) {
      return 0
    }

    const expirationTimestamp = Math.floor(Date.now() / 1000) - maxAgeDays * 24 * 60 * 60

    return new Promise((resolve) => {
      const transaction = this.db!.transaction(StoreNames.CACHED_EVENTS, 'readwrite')
      const store = transaction.objectStore(StoreNames.CACHED_EVENTS)
      const index = store.index('created_at')
      const range = IDBKeyRange.upperBound(expirationTimestamp)
      const request = index.openCursor(range)
      let deletedCount = 0

      request.onsuccess = (event) => {
        const cursor = (event.target as IDBRequest).result
        if (cursor) {
          cursor.delete()
          deletedCount++
          cursor.continue()
        } else {
          transaction.commit()
          resolve(deletedCount)
        }
      }

      request.onerror = () => {
        transaction.commit()
        resolve(deletedCount)
      }
    })
  }

  /**
   * Get the count of cached events.
   */
  async getCachedEventCount(): Promise<number> {
    await this.initPromise
    if (!this.db) {
      return 0
    }

    return new Promise((resolve) => {
      const transaction = this.db!.transaction(StoreNames.CACHED_EVENTS, 'readonly')
      const store = transaction.objectStore(StoreNames.CACHED_EVENTS)
      const request = store.count()

      request.onsuccess = () => {
        transaction.commit()
        resolve(request.result)
      }

      request.onerror = () => {
        transaction.commit()
        resolve(0)
      }
    })
  }

  private async cleanUp() {
    await this.initPromise
    if (!this.db) {
      return
    }

    const stores = [
      {
        name: StoreNames.PROFILE_EVENTS,
        expirationTimestamp: Date.now() - 1000 * 60 * 60 * 24 * 30 // 30 day
      },
      {
        name: StoreNames.RELAY_LIST_EVENTS,
        expirationTimestamp: Date.now() - 1000 * 60 * 60 * 24 * 30 // 30 day
      },
      {
        name: StoreNames.FOLLOW_LIST_EVENTS,
        expirationTimestamp: Date.now() - 1000 * 60 * 60 * 24 * 30 // 30 day
      },
      {
        name: StoreNames.BLOSSOM_SERVER_LIST_EVENTS,
        expirationTimestamp: Date.now() - 1000 * 60 * 60 * 24 * 30 // 30 day
      },
      {
        name: StoreNames.RELAY_INFOS,
        expirationTimestamp: Date.now() - 1000 * 60 * 60 * 24 * 30 // 30 day
      },
      {
        name: StoreNames.PIN_LIST_EVENTS,
        expirationTimestamp: Date.now() - 1000 * 60 * 60 * 24 * 30 // 30 days
      }
    ]
    // Also clean up cached events (separate cleanup due to different key structure)
    this.cleanupExpiredCache(7) // 7 days for cached events
    const transaction = this.db!.transaction(
      stores.map((store) => store.name),
      'readwrite'
    )
    await Promise.allSettled(
      stores.map(({ name, expirationTimestamp }) => {
        if (expirationTimestamp < 0) {
          return Promise.resolve()
        }
        return new Promise<void>((resolve, reject) => {
          const store = transaction.objectStore(name)
          const request = store.openCursor()
          request.onsuccess = (event) => {
            const cursor = (event.target as IDBRequest).result
            if (cursor) {
              const value: TValue = cursor.value
              if (value.addedAt < expirationTimestamp) {
                cursor.delete()
              }
              cursor.continue()
            } else {
              resolve()
            }
          }

          request.onerror = (event) => {
            reject(event)
          }
        })
      })
    )
  }

  // ── Relay Stats CRUD ──

  async putRelayStats(key: string, value: unknown): Promise<void> {
    await this.initPromise
    if (!this.db) return
    const transaction = this.db.transaction(StoreNames.RELAY_STATS, 'readwrite')
    const store = transaction.objectStore(StoreNames.RELAY_STATS)
    store.put({ key, value, addedAt: Date.now() })
  }

  async getAllRelayStats(): Promise<Array<{ key: string; value: unknown }>> {
    await this.initPromise
    if (!this.db) return []
    return new Promise((resolve, reject) => {
      const transaction = this.db!.transaction(StoreNames.RELAY_STATS, 'readonly')
      const store = transaction.objectStore(StoreNames.RELAY_STATS)
      const request = store.getAll()
      request.onsuccess = () => resolve(request.result ?? [])
      request.onerror = () => reject(request.error)
    })
  }

  async deleteRelayStats(key: string): Promise<void> {
    await this.initPromise
    if (!this.db) return
    const transaction = this.db.transaction(StoreNames.RELAY_STATS, 'readwrite')
    const store = transaction.objectStore(StoreNames.RELAY_STATS)
    store.delete(key)
  }

  // ── Managed Relays CRUD ──

  async putManagedRelay(key: string, value: unknown): Promise<void> {
    await this.initPromise
    if (!this.db) return
    const transaction = this.db.transaction(StoreNames.MANAGED_RELAYS, 'readwrite')
    const store = transaction.objectStore(StoreNames.MANAGED_RELAYS)
    store.put({ key, value, addedAt: Date.now() })
  }

  async getAllManagedRelays(): Promise<Array<{ key: string; value: unknown }>> {
    await this.initPromise
    if (!this.db) return []
    return new Promise((resolve, reject) => {
      const transaction = this.db!.transaction(StoreNames.MANAGED_RELAYS, 'readonly')
      const store = transaction.objectStore(StoreNames.MANAGED_RELAYS)
      const request = store.getAll()
      request.onsuccess = () => resolve(request.result ?? [])
      request.onerror = () => reject(request.error)
    })
  }

  async deleteManagedRelay(key: string): Promise<void> {
    await this.initPromise
    if (!this.db) return
    const transaction = this.db.transaction(StoreNames.MANAGED_RELAYS, 'readwrite')
    const store = transaction.objectStore(StoreNames.MANAGED_RELAYS)
    store.delete(key)
  }
}

const instance = IndexedDbService.getInstance()
export default instance
