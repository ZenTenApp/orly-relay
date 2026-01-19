import { writable, derived } from 'svelte/store';

// ==================== Relay Connection State ====================

// Configured relay URL (empty = use same origin / embedded mode)
export const relayUrl = writable(localStorage.getItem("relayUrl") || "");
export const isStandaloneMode = writable(false);
export const relayInfo = writable(null); // NIP-11 relay info
export const relayConnectionStatus = writable("disconnected"); // disconnected, connecting, connected, error
export const isOrlyRelay = writable(true); // true if connected to ORLY relay with API endpoints

// Saved relays list - each entry: { url: string, name: string, lastConnected?: number }
const storedRelays = localStorage.getItem("savedRelays");
export const savedRelays = writable(storedRelays ? JSON.parse(storedRelays) : []);

// Persist relay URL to localStorage
relayUrl.subscribe(url => {
    if (url) {
        localStorage.setItem("relayUrl", url);
    } else {
        localStorage.removeItem("relayUrl");
    }
});

// Persist saved relays to localStorage
savedRelays.subscribe(relays => {
    localStorage.setItem("savedRelays", JSON.stringify(relays));
});

// ==================== User/Auth State ====================

export const isLoggedIn = writable(false);
export const userPubkey = writable("");
export const userProfile = writable(null);
export const userRole = writable("");
export const userSigner = writable(null);
export const authMethod = writable("");

// View-as role for permission testing
export const viewAsRole = writable("");

// Derived: effective role (actual or view-as)
export const currentEffectiveRole = derived(
    [userRole, viewAsRole],
    ([$userRole, $viewAsRole]) => $viewAsRole || $userRole
);

// ==================== UI State ====================

export const isDarkTheme = writable(false);
export const showLoginModal = writable(false);
export const showSettingsDrawer = writable(false);
export const selectedTab = writable(localStorage.getItem("selectedTab") || "export");
export const showFilterBuilder = writable(false);

// ==================== ACL State ====================

export const aclMode = writable("");
export const isPolicyAdmin = writable(false);
export const policyEnabled = writable(false);

// ==================== Events Cache ====================

export const globalEventsCache = writable([]);
export const globalCacheTimestamp = writable(0);

// ==================== Search State ====================

export const searchQuery = writable("");
export const searchTabs = writable([]);
export const searchResults = writable(new Map());

// ==================== Helper Functions ====================

/**
 * Reset all auth-related stores on logout
 */
export function resetAuthState() {
    isLoggedIn.set(false);
    userPubkey.set("");
    userProfile.set(null);
    userRole.set("");
    userSigner.set(null);
    authMethod.set("");
    viewAsRole.set("");
    isPolicyAdmin.set(false);
}

/**
 * Clear the events cache
 */
export function clearEventsCache() {
    globalEventsCache.set([]);
    globalCacheTimestamp.set(0);
}

/**
 * Update the events cache
 * @param {Array} events - Events to cache
 */
export function updateEventsCache(events) {
    globalEventsCache.set(events);
    globalCacheTimestamp.set(Date.now());
}

/**
 * Check if cache is still valid
 * @param {number} cacheDuration - Cache duration in ms
 * @returns {boolean}
 */
export function isCacheValid(cacheDuration = 5 * 60 * 1000) {
    let timestamp;
    globalCacheTimestamp.subscribe(v => timestamp = v)();
    return Date.now() - timestamp < cacheDuration;
}

/**
 * Clear relay connection and reset to embedded mode
 */
export function clearRelayConnection() {
    relayUrl.set("");
    relayInfo.set(null);
    relayConnectionStatus.set("disconnected");
    // Also clear auth state since we're changing relays
    resetAuthState();
    clearEventsCache();
}

/**
 * Add or update a relay in the saved relays list
 * @param {string} url - Relay URL
 * @param {string} name - Relay name (from NIP-11 or user input)
 */
export function saveRelay(url, name) {
    savedRelays.update(relays => {
        const existing = relays.findIndex(r => r.url === url);
        const entry = { url, name, lastConnected: Date.now() };
        if (existing >= 0) {
            relays[existing] = entry;
        } else {
            relays.unshift(entry);
        }
        return relays;
    });
}

/**
 * Remove a relay from the saved relays list
 * @param {string} url - Relay URL to remove
 */
export function removeRelay(url) {
    savedRelays.update(relays => relays.filter(r => r.url !== url));
}

/**
 * Update the last connected timestamp for a relay
 * @param {string} url - Relay URL
 */
export function touchRelay(url) {
    savedRelays.update(relays => {
        const relay = relays.find(r => r.url === url);
        if (relay) {
            relay.lastConnected = Date.now();
        }
        return relays;
    });
}

// ==================== Bunker Service State ====================

export const bunkerServiceActive = writable(false);
export const bunkerConnectedClients = writable([]);

// Bunker worker instance (persists across component mounts)
let bunkerWorker = null;

/**
 * Get or create the bunker worker
 */
function getBunkerWorker() {
    if (!bunkerWorker) {
        bunkerWorker = new Worker(new URL('./bunker-worker.js', import.meta.url), { type: 'module' });
        bunkerWorker.onmessage = (event) => {
            const { type, ...data } = event.data;
            switch (type) {
                case 'status':
                    bunkerServiceActive.set(data.status === 'connected');
                    break;
                case 'clients':
                    bunkerConnectedClients.set(data.clients || []);
                    break;
                case 'error':
                    console.error('[BunkerStore] Worker error:', data.error);
                    break;
                case 'request':
                    console.log('[BunkerStore] Request:', data.method, 'from:', data.from);
                    break;
            }
        };
    }
    return bunkerWorker;
}

/**
 * Configure the bunker worker
 */
export function configureBunkerWorker(config) {
    const worker = getBunkerWorker();
    worker.postMessage({ type: 'configure', ...config });
}

/**
 * Connect the bunker worker
 */
export function connectBunkerWorker() {
    const worker = getBunkerWorker();
    worker.postMessage({ type: 'connect' });
}

/**
 * Disconnect the bunker worker
 */
export function disconnectBunkerWorker() {
    const worker = getBunkerWorker();
    worker.postMessage({ type: 'disconnect' });
}

/**
 * Add a secret to the bunker worker
 */
export function addBunkerSecret(secret) {
    const worker = getBunkerWorker();
    worker.postMessage({ type: 'addSecret', secret });
}

/**
 * Request current bunker status
 */
export function requestBunkerStatus() {
    const worker = getBunkerWorker();
    worker.postMessage({ type: 'getStatus' });
}

/**
 * Reset bunker state
 */
export function resetBunkerState() {
    disconnectBunkerWorker();
    bunkerServiceActive.set(false);
    bunkerConnectedClients.set([]);
}
