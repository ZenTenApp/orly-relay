import { SimplePool } from 'nostr-tools/pool';
import { EventStore } from 'applesauce-core';
import { PrivateKeySigner } from 'applesauce-signers';
import { getDefaultRelays, FALLBACK_RELAYS } from "./constants.js";

// Dedicated pool for fallback relay queries (separate from main pool to avoid conflicts)
let fallbackPool = null;

function getFallbackPool() {
  if (!fallbackPool) {
    fallbackPool = new SimplePool();
  }
  return fallbackPool;
}

// Nostr client wrapper using nostr-tools
class NostrClient {
  constructor() {
    this.pool = new SimplePool();
    this.eventStore = new EventStore();
    this.isConnected = false;
    this.signer = null;
    // Use dynamic relay list (supports standalone mode)
    this.relays = [...getDefaultRelays()];
  }

  // Refresh relay list from config (call when relay URL changes)
  refreshRelays() {
    const newRelays = getDefaultRelays();
    if (JSON.stringify(this.relays) !== JSON.stringify(newRelays)) {
      console.log("Relay list updated:", newRelays);
      this.relays = [...newRelays];
    }
  }

  // Reset client for new relay (close old connections, refresh relay list, create new pool)
  reset() {
    console.log("[NostrClient] Resetting for new relay...");
    // Close ALL existing connections by destroying the pool
    if (this.pool) {
      try {
        // Close connections to old relays first
        this.pool.close(this.relays);
      } catch (e) {
        console.warn("[NostrClient] Error closing old relay connections:", e);
      }
      // Destroy the pool reference completely
      this.pool = null;
    }
    // Create completely fresh pool
    this.pool = new SimplePool();
    this.isConnected = false;
    // Refresh relay list
    this.relays = [...getDefaultRelays()];
    console.log("[NostrClient] Reset complete, new relays:", this.relays);
  }

  async connect() {
    console.log("Starting connection to", this.relays.length, "relays...");
    
    try {
      // SimplePool doesn't require explicit connect
      this.isConnected = true;
      console.log("✓ Successfully initialized relay pool");
      
      // Wait a bit for connections to stabilize
      await new Promise((resolve) => setTimeout(resolve, 1000));
    } catch (error) {
      console.error("✗ Connection failed:", error);
      throw error;
    }
  }

  async connectToRelay(relayUrl) {
    console.log(`Adding relay: ${relayUrl}`);
    
    try {
      if (!this.relays.includes(relayUrl)) {
        this.relays.push(relayUrl);
      }
      console.log(`✓ Successfully added relay ${relayUrl}`);
      return true;
    } catch (error) {
      console.error(`✗ Failed to add relay ${relayUrl}:`, error);
      return false;
    }
  }

  subscribe(filters, callback) {
    console.log("Creating subscription with filters:", filters);
    
    const sub = this.pool.subscribeMany(
      this.relays,
      filters,
      {
        onevent(event) {
          console.log("Event received:", event);
          callback(event);
        },
        oneose() {
          console.log("EOSE received");
          window.dispatchEvent(new CustomEvent('nostr-eose', {
            detail: { subscriptionId: sub.id }
          }));
        }
      }
    );

    return sub;
  }

  unsubscribe(subscription) {
    console.log(`Closing subscription`);
    if (subscription && subscription.close) {
      subscription.close();
    }
  }

  disconnect() {
    console.log("Disconnecting relay pool");
    if (this.pool) {
      this.pool.close(this.relays);
    }
    this.isConnected = false;
  }

  // Publish an event
  async publish(event, specificRelays = null) {
    if (!this.isConnected) {
      console.warn("Not connected to any relays, attempting to connect first");
      await this.connect();
    }

    try {
      const relaysToUse = specificRelays || this.relays;
      const promises = this.pool.publish(relaysToUse, event);
      const results = await Promise.allSettled(promises);

      // Count successes and failures
      let okCount = 0;
      let errorCount = 0;
      for (const result of results) {
        if (result.status === 'fulfilled') {
          okCount++;
          console.log("✓ Event accepted by relay");
        } else {
          errorCount++;
          console.warn("✗ Relay rejected event:", result.reason);
        }
      }

      if (okCount === 0) {
        throw new Error(`Event rejected by all ${errorCount} relays`);
      }

      console.log(`✓ Event published: ${okCount} OK, ${errorCount} failed`);

      // Store the published event in IndexedDB
      await putEvents([event]);
      console.log("Event stored in IndexedDB");

      return { success: true, okCount, errorCount, event };
    } catch (error) {
      console.error("✗ Failed to publish event:", error);
      throw error;
    }
  }

  // Get pool for advanced usage
  getPool() {
    return this.pool;
  }

  // Get event store
  getEventStore() {
    return this.eventStore;
  }

  // Get signer
  getSigner() {
    return this.signer;
  }

  // Set signer
  setSigner(signer) {
    this.signer = signer;
  }
}

// Create a global client instance
export const nostrClient = new NostrClient();

// Export the class for creating new instances
export { NostrClient };

// Export signer classes
export { PrivateKeySigner };

// Export NIP-07 helper
export class Nip07Signer {
  async getPublicKey() {
    if (window.nostr) {
      return await window.nostr.getPublicKey();
    }
    throw new Error('NIP-07 extension not found');
  }

  async signEvent(event) {
    if (window.nostr) {
      return await window.nostr.signEvent(event);
    }
    throw new Error('NIP-07 extension not found');
  }

  async nip04Encrypt(pubkey, plaintext) {
    if (window.nostr && window.nostr.nip04) {
      return await window.nostr.nip04.encrypt(pubkey, plaintext);
    }
    throw new Error('NIP-07 extension does not support NIP-04');
  }

  async nip04Decrypt(pubkey, ciphertext) {
    if (window.nostr && window.nostr.nip04) {
      return await window.nostr.nip04.decrypt(pubkey, ciphertext);
    }
    throw new Error('NIP-07 extension does not support NIP-04');
  }

  async nip44Encrypt(pubkey, plaintext) {
    if (window.nostr && window.nostr.nip44) {
      return await window.nostr.nip44.encrypt(pubkey, plaintext);
    }
    throw new Error('NIP-07 extension does not support NIP-44');
  }

  async nip44Decrypt(pubkey, ciphertext) {
    if (window.nostr && window.nostr.nip44) {
      return await window.nostr.nip44.decrypt(pubkey, ciphertext);
    }
    throw new Error('NIP-07 extension does not support NIP-44');
  }
}

// Merge two event arrays, deduplicating by event id
// Newer events (by created_at) take precedence for same id
function mergeAndDeduplicateEvents(cached, relay) {
  const eventMap = new Map();

  // Add cached events first
  for (const event of cached) {
    eventMap.set(event.id, event);
  }

  // Add/update with relay events (they may be newer)
  for (const event of relay) {
    const existing = eventMap.get(event.id);
    if (!existing || event.created_at >= existing.created_at) {
      eventMap.set(event.id, event);
    }
  }

  // Return sorted by created_at descending (newest first)
  return Array.from(eventMap.values()).sort((a, b) => b.created_at - a.created_at);
}

// IndexedDB helpers for unified event storage
// This provides a local cache that all components can access
const DB_NAME = "nostrCache";
const DB_VERSION = 2; // Incremented for new indexes
const STORE_EVENTS = "events";

function openDB() {
  return new Promise((resolve, reject) => {
    try {
      const req = indexedDB.open(DB_NAME, DB_VERSION);
      req.onupgradeneeded = (event) => {
        const db = req.result;
        const oldVersion = event.oldVersion;
        
        // Create or update the events store
        let store;
        if (!db.objectStoreNames.contains(STORE_EVENTS)) {
          store = db.createObjectStore(STORE_EVENTS, { keyPath: "id" });
        } else {
          // Get existing store during upgrade
          store = req.transaction.objectStore(STORE_EVENTS);
        }
        
        // Create indexes if they don't exist
        if (!store.indexNames.contains("byKindAuthor")) {
          store.createIndex("byKindAuthor", ["kind", "pubkey"], {
            unique: false,
          });
        }
        if (!store.indexNames.contains("byKindAuthorCreated")) {
          store.createIndex(
            "byKindAuthorCreated",
            ["kind", "pubkey", "created_at"],
            { unique: false },
          );
        }
        if (!store.indexNames.contains("byKind")) {
          store.createIndex("byKind", "kind", { unique: false });
        }
        if (!store.indexNames.contains("byAuthor")) {
          store.createIndex("byAuthor", "pubkey", { unique: false });
        }
        if (!store.indexNames.contains("byCreatedAt")) {
          store.createIndex("byCreatedAt", "created_at", { unique: false });
        }
      };
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    } catch (e) {
      console.error("Failed to open IndexedDB", e);
      reject(e);
    }
  });
}

async function getLatestProfileEvent(pubkey) {
  try {
    const db = await openDB();
    return await new Promise((resolve, reject) => {
      const tx = db.transaction(STORE_EVENTS, "readonly");
      const idx = tx.objectStore(STORE_EVENTS).index("byKindAuthorCreated");
      const range = IDBKeyRange.bound(
        [0, pubkey, -Infinity],
        [0, pubkey, Infinity],
      );
      const req = idx.openCursor(range, "prev"); // newest first
      req.onsuccess = () => {
        const cursor = req.result;
        resolve(cursor ? cursor.value : null);
      };
      req.onerror = () => reject(req.error);
    });
  } catch (e) {
    console.warn("IDB getLatestProfileEvent failed", e);
    return null;
  }
}

async function putEvent(event) {
  try {
    const db = await openDB();
    await new Promise((resolve, reject) => {
      const tx = db.transaction(STORE_EVENTS, "readwrite");
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
      tx.objectStore(STORE_EVENTS).put(event);
    });
  } catch (e) {
    console.warn("IDB putEvent failed", e);
  }
}

// Store multiple events in IndexedDB
async function putEvents(events) {
  if (!events || events.length === 0) return;
  
  try {
    const db = await openDB();
    await new Promise((resolve, reject) => {
      const tx = db.transaction(STORE_EVENTS, "readwrite");
      tx.oncomplete = () => resolve();
      tx.onerror = () => reject(tx.error);
      
      const store = tx.objectStore(STORE_EVENTS);
      for (const event of events) {
        store.put(event);
      }
    });
    console.log(`Stored ${events.length} events in IndexedDB`);
  } catch (e) {
    console.warn("IDB putEvents failed", e);
  }
}

// Query events from IndexedDB by filters
async function queryEventsFromDB(filters) {
  try {
    const db = await openDB();
    const results = [];
    
    console.log("QueryEventsFromDB: Starting query with filters:", filters);
    
    for (const filter of filters) {
      console.log("QueryEventsFromDB: Processing filter:", filter);
      
      const events = await new Promise((resolve, reject) => {
        const tx = db.transaction(STORE_EVENTS, "readonly");
        const store = tx.objectStore(STORE_EVENTS);
        const allEvents = [];
        
        // Determine which index to use based on filter
        let req;
        if (filter.kinds && filter.kinds.length > 0 && filter.authors && filter.authors.length > 0) {
          // Use byKindAuthor index for the most specific query
          const kind = filter.kinds[0];
          const author = filter.authors[0];
          console.log(`QueryEventsFromDB: Using byKindAuthorCreated index for kind=${kind}, author=${author.substring(0, 8)}...`);
          
          const idx = store.index("byKindAuthorCreated");
          const range = IDBKeyRange.bound(
            [kind, author, -Infinity],
            [kind, author, Infinity]
          );
          req = idx.openCursor(range, "prev"); // newest first
        } else if (filter.kinds && filter.kinds.length > 0) {
          // Use byKind index
          console.log(`QueryEventsFromDB: Using byKind index for kind=${filter.kinds[0]}`);
          const idx = store.index("byKind");
          req = idx.openCursor(IDBKeyRange.only(filter.kinds[0]));
        } else if (filter.authors && filter.authors.length > 0) {
          // Use byAuthor index
          console.log(`QueryEventsFromDB: Using byAuthor index for author=${filter.authors[0].substring(0, 8)}...`);
          const idx = store.index("byAuthor");
          req = idx.openCursor(IDBKeyRange.only(filter.authors[0]));
        } else {
          // Scan all events
          console.log("QueryEventsFromDB: Scanning all events (no specific index)");
          req = store.openCursor();
        }
        
        req.onsuccess = (event) => {
          const cursor = event.target.result;
          if (cursor) {
            const evt = cursor.value;
            
            // Apply additional filters
            let matches = true;
            
            // Filter by kinds
            if (filter.kinds && filter.kinds.length > 0 && !filter.kinds.includes(evt.kind)) {
              matches = false;
            }
            
            // Filter by authors
            if (filter.authors && filter.authors.length > 0 && !filter.authors.includes(evt.pubkey)) {
              matches = false;
            }
            
            // Filter by since
            if (filter.since && evt.created_at < filter.since) {
              matches = false;
            }
            
            // Filter by until
            if (filter.until && evt.created_at > filter.until) {
              matches = false;
            }
            
            // Filter by IDs
            if (filter.ids && filter.ids.length > 0 && !filter.ids.includes(evt.id)) {
              matches = false;
            }
            
            if (matches) {
              allEvents.push(evt);
            }
            
            // Apply limit
            if (filter.limit && allEvents.length >= filter.limit) {
              console.log(`QueryEventsFromDB: Reached limit of ${filter.limit}, found ${allEvents.length} matching events`);
              resolve(allEvents);
              return;
            }
            
            cursor.continue();
          } else {
            console.log(`QueryEventsFromDB: Cursor exhausted, found ${allEvents.length} matching events`);
            resolve(allEvents);
          }
        };
        
        req.onerror = () => {
          console.error("QueryEventsFromDB: Cursor error:", req.error);
          reject(req.error);
        };
      });
      
      console.log(`QueryEventsFromDB: Found ${events.length} events for this filter`);
      results.push(...events);
    }
    
    // Sort by created_at (newest first) and apply global limit
    results.sort((a, b) => b.created_at - a.created_at);
    
    console.log(`QueryEventsFromDB: Returning ${results.length} total events`);
    return results;
  } catch (e) {
    console.error("QueryEventsFromDB failed:", e);
    return [];
  }
}

function parseProfileFromEvent(event) {
  try {
    const profile = JSON.parse(event.content || "{}");
    return {
      name: profile.name || profile.display_name || "",
      picture: profile.picture || "",
      banner: profile.banner || "",
      about: profile.about || "",
      nip05: profile.nip05 || "",
      lud16: profile.lud16 || profile.lud06 || "",
    };
  } catch (e) {
    return {
      name: "",
      picture: "",
      banner: "",
      about: "",
      nip05: "",
      lud16: "",
    };
  }
}

// Fetch user profile metadata (kind 0)
export async function fetchUserProfile(pubkey) {
  console.log(`Starting profile fetch for pubkey: ${pubkey}`);
  console.log(`[fetchUserProfile] Current relay list:`, nostrClient.relays);

  // 1) Try cached profile first and resolve immediately if present
  try {
    const cachedEvent = await getLatestProfileEvent(pubkey);
    if (cachedEvent) {
      console.log("Using cached profile event");
      const profile = parseProfileFromEvent(cachedEvent);
      return profile;
    }
  } catch (e) {
    console.warn("Failed to load cached profile", e);
  }

  const filters = [{
    kinds: [0],
    authors: [pubkey],
    limit: 1
  }];

  // 2) Fetch profile from local relay first
  try {
    const events = await fetchEvents(filters, { timeout: 10000 });

    if (events.length > 0) {
      const profileEvent = events[0];
      console.log("Profile fetched from local relay:", profileEvent);
      return processProfileEvent(profileEvent, pubkey);
    }
  } catch (error) {
    console.warn("Failed to fetch profile from local relay:", error);
  }

  // 3) Try fallback relays if local relay doesn't have the profile
  console.log("Profile not found on local relay, trying fallback relays:", FALLBACK_RELAYS);
  try {
    const profileEvent = await fetchProfileFromFallbackRelays(pubkey, filters);
    if (profileEvent) {
      return processProfileEvent(profileEvent, pubkey);
    }
  } catch (error) {
    console.warn("Failed to fetch profile from fallback relays:", error);
  }

  // 4) No profile found anywhere
  console.log("No profile found for pubkey:", pubkey);
  return null;
}

// Helper to fetch profile from fallback relays
async function fetchProfileFromFallbackRelays(pubkey, filters) {
  console.log(`[fetchProfileFromFallbackRelays] Querying fallback relays:`, FALLBACK_RELAYS);
  console.log(`[fetchProfileFromFallbackRelays] Using filters:`, JSON.stringify(filters));
  return new Promise((resolve) => {
    const events = [];
    const pool = getFallbackPool();
    let sub;

    const timeoutId = setTimeout(() => {
      if (sub) sub.close();
      // Return the most recent profile event
      if (events.length > 0) {
        events.sort((a, b) => b.created_at - a.created_at);
        resolve(events[0]);
      } else {
        resolve(null);
      }
    }, 5000);

    sub = pool.subscribeMany(
      FALLBACK_RELAYS,
      filters,
      {
        onevent(event) {
          console.log("[fetchProfileFromFallbackRelays] Event received:", event.id?.substring(0, 8), "kind:", event.kind, "pubkey:", event.pubkey?.substring(0, 8));
          events.push(event);
        },
        oneose() {
          console.log(`[fetchProfileFromFallbackRelays] EOSE received, got ${events.length} events`);
          clearTimeout(timeoutId);
          if (sub) sub.close();
          if (events.length > 0) {
            events.sort((a, b) => b.created_at - a.created_at);
            console.log("[fetchProfileFromFallbackRelays] Returning best event:", events[0].id?.substring(0, 8));
            resolve(events[0]);
          } else {
            console.log("[fetchProfileFromFallbackRelays] No events found");
            resolve(null);
          }
        }
      }
    );
  });
}

// Helper to process and cache a profile event
async function processProfileEvent(profileEvent, pubkey) {
  // Cache the event
  await putEvent(profileEvent);

  // Publish the profile event to the local relay
  try {
    console.log("Publishing profile event to local relay:", profileEvent.id);
    await nostrClient.publish(profileEvent);
    console.log("Profile event successfully saved to local relay");
  } catch (publishError) {
    console.warn("Failed to publish profile to local relay:", publishError);
  }

  // Parse profile data
  const profile = parseProfileFromEvent(profileEvent);

  // Notify listeners that an updated profile is available
  try {
    if (typeof window !== "undefined" && window.dispatchEvent) {
      window.dispatchEvent(
        new CustomEvent("profile-updated", {
          detail: { pubkey, profile, event: profileEvent },
        }),
      );
    }
  } catch (e) {
    console.warn("Failed to dispatch profile-updated event", e);
  }

  return profile;
}

// Fetch user's relay list (NIP-65 kind 10002)
export async function fetchUserRelayList(pubkey) {
  console.log(`[nostr] Fetching relay list for pubkey: ${pubkey?.substring(0, 8)}...`);

  const filters = [{
    kinds: [10002],
    authors: [pubkey],
    limit: 1
  }];

  // Try local relay first
  try {
    const events = await fetchEvents(filters, { timeout: 10000, useCache: true });
    if (events.length > 0) {
      const relayListEvent = events.sort((a, b) => b.created_at - a.created_at)[0];
      console.log("[nostr] Relay list found on local relay");
      return parseRelayListFromEvent(relayListEvent);
    }
  } catch (error) {
    console.warn("[nostr] Failed to fetch relay list from local relay:", error);
  }

  // Try fallback relays
  console.log("[nostr] Relay list not found locally, trying fallback relays...");
  try {
    const relayListEvent = await fetchFromFallbackRelays(filters);
    if (relayListEvent) {
      // Cache and publish to local relay
      await putEvent(relayListEvent);
      try {
        await nostrClient.publish(relayListEvent);
      } catch (e) {
        console.warn("[nostr] Failed to publish relay list to local relay:", e);
      }
      return parseRelayListFromEvent(relayListEvent);
    }
  } catch (error) {
    console.warn("[nostr] Failed to fetch relay list from fallback relays:", error);
  }

  console.log("[nostr] No relay list found for pubkey");
  return null;
}

// Parse relay list from kind 10002 event
function parseRelayListFromEvent(event) {
  if (!event || event.kind !== 10002) return null;

  const relays = {
    read: [],
    write: [],
    all: []
  };

  for (const tag of event.tags) {
    if (tag[0] === 'r' && tag[1]) {
      const url = tag[1];
      const marker = tag[2]; // 'read', 'write', or undefined (both)

      if (marker === 'read') {
        relays.read.push(url);
      } else if (marker === 'write') {
        relays.write.push(url);
      } else {
        // No marker means both read and write
        relays.read.push(url);
        relays.write.push(url);
      }
      relays.all.push({ url, read: marker !== 'write', write: marker !== 'read' });
    }
  }

  console.log(`[nostr] Parsed relay list: ${relays.all.length} relays`);
  return relays;
}

// Generic helper to fetch from fallback relays
async function fetchFromFallbackRelays(filters) {
  return new Promise((resolve) => {
    const events = [];
    const pool = getFallbackPool();
    let sub;

    const timeoutId = setTimeout(() => {
      if (sub) sub.close();
      if (events.length > 0) {
        events.sort((a, b) => b.created_at - a.created_at);
        resolve(events[0]);
      } else {
        resolve(null);
      }
    }, 5000);

    sub = pool.subscribeMany(
      FALLBACK_RELAYS,
      filters,
      {
        onevent(event) {
          events.push(event);
        },
        oneose() {
          clearTimeout(timeoutId);
          if (sub) sub.close();
          if (events.length > 0) {
            events.sort((a, b) => b.created_at - a.created_at);
            resolve(events[0]);
          } else {
            resolve(null);
          }
        }
      }
    );
  });
}

// Fetch user's contact list (kind 3) - includes follows and may have relay hints
export async function fetchUserContactList(pubkey) {
  console.log(`[nostr] Fetching contact list for pubkey: ${pubkey?.substring(0, 8)}...`);

  const filters = [{
    kinds: [3],
    authors: [pubkey],
    limit: 1
  }];

  // Try local relay first
  try {
    const events = await fetchEvents(filters, { timeout: 10000, useCache: true });
    if (events.length > 0) {
      const contactEvent = events.sort((a, b) => b.created_at - a.created_at)[0];
      console.log("[nostr] Contact list found on local relay");
      return parseContactListFromEvent(contactEvent);
    }
  } catch (error) {
    console.warn("[nostr] Failed to fetch contact list from local relay:", error);
  }

  // Try fallback relays
  console.log("[nostr] Contact list not found locally, trying fallback relays...");
  try {
    const contactEvent = await fetchFromFallbackRelays(filters);
    if (contactEvent) {
      await putEvent(contactEvent);
      try {
        await nostrClient.publish(contactEvent);
      } catch (e) {
        console.warn("[nostr] Failed to publish contact list to local relay:", e);
      }
      return parseContactListFromEvent(contactEvent);
    }
  } catch (error) {
    console.warn("[nostr] Failed to fetch contact list from fallback relays:", error);
  }

  console.log("[nostr] No contact list found for pubkey");
  return null;
}

// Parse contact list from kind 3 event
function parseContactListFromEvent(event) {
  if (!event || event.kind !== 3) return null;

  const follows = [];
  const relayHints = {};

  for (const tag of event.tags) {
    if (tag[0] === 'p' && tag[1]) {
      const pubkey = tag[1];
      const relayUrl = tag[2] || null;
      const petname = tag[3] || null;

      follows.push({ pubkey, relayUrl, petname });

      if (relayUrl) {
        if (!relayHints[relayUrl]) {
          relayHints[relayUrl] = [];
        }
        relayHints[relayUrl].push(pubkey);
      }
    }
  }

  // Also parse the content field which may contain relay preferences (legacy format)
  let legacyRelays = {};
  try {
    if (event.content) {
      legacyRelays = JSON.parse(event.content);
    }
  } catch (e) {
    // Content is not JSON, ignore
  }

  console.log(`[nostr] Parsed contact list: ${follows.length} follows, ${Object.keys(relayHints).length} relay hints`);
  return { follows, relayHints, legacyRelays, event };
}

// Fetch events
export async function fetchEvents(filters, options = {}) {
  console.log(`Starting event fetch with filters:`, JSON.stringify(filters, null, 2));
  console.log(`Current relays:`, nostrClient.relays);

  // Ensure client is connected
  if (!nostrClient.isConnected || nostrClient.relays.length === 0) {
    console.warn("Client not connected, initializing...");
    await initializeNostrClient();
  }

  const {
    timeout = 30000,
    useCache = true, // Option to query from cache first
  } = options;

  // Try to get cached events first if requested
  let cachedEvents = [];
  if (useCache) {
    try {
      cachedEvents = await queryEventsFromDB(filters);
      if (cachedEvents.length > 0) {
        console.log(`Found ${cachedEvents.length} cached events in IndexedDB`);
      }
    } catch (e) {
      console.warn("Failed to query cached events", e);
    }
  }

  return new Promise((resolve, reject) => {
    const relayEvents = [];
    let sub = null;

    const timeoutId = setTimeout(() => {
      console.log(`Timeout reached after ${timeout}ms, returning ${relayEvents.length} relay events`);
      if (sub) sub.close();

      // Store all received events in IndexedDB before resolving
      if (relayEvents.length > 0) {
        putEvents(relayEvents).catch(e => console.warn("Failed to cache events", e));
      }

      // Merge cached events with relay events, deduplicate by id
      const mergedEvents = mergeAndDeduplicateEvents(cachedEvents, relayEvents);
      resolve(mergedEvents);
    }, timeout);

    try {
      // Generate a subscription ID for logging
      const subId = Math.random().toString(36).substring(7);

      // Validate filters before sending
      if (!Array.isArray(filters) || filters.length === 0) {
        console.error(`❌ Invalid filters: not an array or empty`, filters);
        resolve(cachedEvents);
        return;
      }

      // Ensure each filter is a valid object
      const validFilters = filters.filter(f => f && typeof f === 'object' && !Array.isArray(f));
      if (validFilters.length !== filters.length) {
        console.warn(`⚠️ Some filters were invalid, filtered ${filters.length} -> ${validFilters.length}`, filters);
      }

      if (validFilters.length === 0) {
        console.error(`❌ No valid filters remaining`);
        resolve(cachedEvents);
        return;
      }

      console.log(`📤 REQ [${subId}] to ${nostrClient.relays.join(', ')}:`, JSON.stringify(["REQ", subId, ...validFilters], null, 2));

      sub = nostrClient.pool.subscribeMany(
        nostrClient.relays,
        validFilters,
        {
          onevent(event) {
            console.log(`📥 EVENT received for REQ [${subId}]:`, {
              id: event.id?.substring(0, 8) + '...',
              kind: event.kind,
              pubkey: event.pubkey?.substring(0, 8) + '...',
              created_at: event.created_at,
              content_preview: event.content?.substring(0, 50)
            });
            relayEvents.push(event);

            // Store event immediately in IndexedDB
            putEvent(event).catch(e => console.warn("Failed to cache event", e));
          },
          oneose() {
            console.log(`✅ EOSE received for REQ [${subId}], got ${relayEvents.length} relay events`);
            clearTimeout(timeoutId);
            if (sub) sub.close();

            // Store all events in IndexedDB before resolving
            if (relayEvents.length > 0) {
              putEvents(relayEvents).catch(e => console.warn("Failed to cache events", e));
            }

            // Merge cached events with relay events, deduplicate by id
            const mergedEvents = mergeAndDeduplicateEvents(cachedEvents, relayEvents);
            console.log(`Merged ${cachedEvents.length} cached + ${relayEvents.length} relay = ${mergedEvents.length} total events`);
            resolve(mergedEvents);
          }
        }
      );
    } catch (error) {
      clearTimeout(timeoutId);
      console.error("Failed to fetch events:", error);
      reject(error);
    }
  });
}

// Fetch all events with timestamp-based pagination (including delete events)
export async function fetchAllEvents(options = {}) {
  const {
    limit = 100,
    since = null,
    until = null,
    authors = null,
    kinds = null,
    ...rest
  } = options;

  const now = Math.floor(Date.now() / 1000);
  const fiveYearsAgo = now - (5 * 365 * 24 * 60 * 60);

  // Start with 5 years if no since specified
  const initialSince = since || fiveYearsAgo;

  const filters = [{ ...rest }];
  filters[0].since = initialSince;
  if (until) filters[0].until = until;
  if (authors) filters[0].authors = authors;
  if (kinds) filters[0].kinds = kinds;
  if (limit) filters[0].limit = limit;

  const events = await fetchEvents(filters, {
    timeout: 30000
  });

  return events;
}

// Fetch user's events with timestamp-based pagination
export async function fetchUserEvents(pubkey, options = {}) {
  const {
    limit = 100,
    since = null,
    until = null
  } = options;

  const filters = [{
    authors: [pubkey]
  }];
  
  if (since) filters[0].since = since;
  if (until) filters[0].until = until;
  if (limit) filters[0].limit = limit;
  
  const events = await fetchEvents(filters, { 
    timeout: 30000 
  });
  
  return events;
}

// NIP-50 search function
export async function searchEvents(searchQuery, options = {}) {
  const {
    limit = 100,
    since = null,
    until = null,
    kinds = null
  } = options;

  const filters = [{
    search: searchQuery
  }];
  
  if (since) filters[0].since = since;
  if (until) filters[0].until = until;
  if (kinds) filters[0].kinds = kinds;
  if (limit) filters[0].limit = limit;
  
  const events = await fetchEvents(filters, { 
    timeout: 30000 
  });
  
  return events;
}

// Fetch a specific event by ID
export async function fetchEventById(eventId, options = {}) {
  const {
    timeout = 10000,
  } = options;

  console.log(`Fetching event by ID: ${eventId}`);

  try {
    const filters = [{
      ids: [eventId]
    }];

    console.log('Fetching event with filters:', filters);

    const events = await fetchEvents(filters, { timeout });

    console.log(`Fetched ${events.length} events`);
    
    // Return the first event if found, null otherwise
    return events.length > 0 ? events[0] : null;
  } catch (error) {
    console.error("Failed to fetch event by ID:", error);
    throw error;
  }
}

// Fetch delete events that target a specific event ID
export async function fetchDeleteEventsByTarget(eventId, options = {}) {
  const {
    timeout = 10000
  } = options;

  console.log(`Fetching delete events for target: ${eventId}`);

  try {
    const filters = [{
      kinds: [5], // Kind 5 is deletion
      '#e': [eventId] // e-tag referencing the target event
    }];

    console.log('Fetching delete events with filters:', filters);

    const events = await fetchEvents(filters, { timeout });

    console.log(`Fetched ${events.length} delete events`);
    
    return events;
  } catch (error) {
    console.error("Failed to fetch delete events:", error);
    throw error;
  }
}

// Initialize client connection
export async function initializeNostrClient() {
  // Refresh relay list to pick up any changes (important for standalone mode)
  nostrClient.refreshRelays();
  await nostrClient.connect();
}

// Query events from cache and relay combined
// This is the main function components should use
export async function queryEvents(filters, options = {}) {
  const {
    timeout = 30000,
    cacheFirst = true, // Try cache first before hitting relay
    cacheOnly = false,  // Only use cache, don't query relay
  } = options;
  
  let cachedEvents = [];
  
  // Try cache first
  if (cacheFirst || cacheOnly) {
    try {
      cachedEvents = await queryEventsFromDB(filters);
      console.log(`Found ${cachedEvents.length} events in cache`);
      
      if (cacheOnly || cachedEvents.length > 0) {
        return cachedEvents;
      }
    } catch (e) {
      console.warn("Failed to query cache", e);
    }
  }
  
  // If cache didn't have results and we're not cache-only, query relay
  if (!cacheOnly) {
    const relayEvents = await fetchEvents(filters, { timeout, useCache: false });
    console.log(`Fetched ${relayEvents.length} events from relay`);
    return relayEvents;
  }
  
  return cachedEvents;
}

// Export cache query function for direct access
export { queryEventsFromDB };

// Clear the IndexedDB cache (call when switching relays)
export async function clearIndexedDBCache() {
  console.log("[nostr] Clearing IndexedDB cache...");
  try {
    const db = await openDB();
    const tx = db.transaction(STORE_EVENTS, "readwrite");
    const store = tx.objectStore(STORE_EVENTS);
    await new Promise((resolve, reject) => {
      const req = store.clear();
      req.onsuccess = () => resolve();
      req.onerror = () => reject(req.error);
    });
    console.log("[nostr] IndexedDB cache cleared");
  } catch (e) {
    console.warn("[nostr] Failed to clear IndexedDB cache", e);
  }
}

// Debug function to check database contents
export async function debugIndexedDB() {
  try {
    const db = await openDB();
    const tx = db.transaction(STORE_EVENTS, "readonly");
    const store = tx.objectStore(STORE_EVENTS);
    
    const allEvents = await new Promise((resolve, reject) => {
      const req = store.getAll();
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
    
    const byKind = allEvents.reduce((acc, e) => {
      acc[e.kind] = (acc[e.kind] || 0) + 1;
      return acc;
    }, {});
    
    console.log("===== IndexedDB Contents =====");
    console.log(`Total events: ${allEvents.length}`);
    console.log("Events by kind:", byKind);
    console.log("Kind 0 events:", allEvents.filter(e => e.kind === 0));
    console.log("All event IDs:", allEvents.map(e => ({ id: e.id.substring(0, 8), kind: e.kind, pubkey: e.pubkey.substring(0, 8) })));
    console.log("==============================");
    
    return {
      total: allEvents.length,
      byKind,
      events: allEvents
    };
  } catch (e) {
    console.error("Failed to debug IndexedDB:", e);
    return null;
  }
}
