import { writable, derived } from 'svelte/store';

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

// ==================== Bunker Worker State ====================
// Persists across component mounts/unmounts using Web Worker

export const bunkerWorker = writable(null);
export const bunkerServiceActive = writable(false);
export const bunkerServiceCatToken = writable(null);
export const bunkerClientTokens = writable([]);  // [{id, name, token, encoded, createdAt, isExpanded}]
export const bunkerSelectedTokenId = writable(null);
export const bunkerConnectedClients = writable([]);

// Internal worker reference (not reactive)
let _bunkerWorker = null;

/**
 * Initialize the bunker worker
 */
export function initBunkerWorker() {
    if (_bunkerWorker) {
        return _bunkerWorker;
    }

    _bunkerWorker = new Worker('/bunker-worker.js');

    _bunkerWorker.onmessage = (event) => {
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
                console.log('[BunkerStore] NIP-46 request:', data.method, 'from:', data.from?.substring(0, 8));
                break;
        }
    };

    _bunkerWorker.onerror = (error) => {
        console.error('[BunkerStore] Worker error:', error);
        bunkerServiceActive.set(false);
    };

    bunkerWorker.set(_bunkerWorker);
    return _bunkerWorker;
}

/**
 * Get or create the bunker worker
 */
export function getBunkerWorker() {
    return _bunkerWorker || initBunkerWorker();
}

/**
 * Configure the bunker worker
 */
export function configureBunkerWorker(config) {
    const worker = getBunkerWorker();
    worker.postMessage({ type: 'configure', ...config });
}

/**
 * Start the bunker worker connection
 */
export function connectBunkerWorker() {
    const worker = getBunkerWorker();
    worker.postMessage({ type: 'connect' });
}

/**
 * Stop the bunker worker connection
 */
export function disconnectBunkerWorker() {
    if (_bunkerWorker) {
        _bunkerWorker.postMessage({ type: 'disconnect' });
    }
}

/**
 * Add a secret to the worker
 */
export function addBunkerSecret(secret) {
    const worker = getBunkerWorker();
    worker.postMessage({ type: 'addSecret', secret });
}

/**
 * Request status from worker
 */
export function requestBunkerStatus() {
    if (_bunkerWorker) {
        _bunkerWorker.postMessage({ type: 'getStatus' });
    }
}

/**
 * Reset bunker state (call on logout or stop)
 */
export function resetBunkerState() {
    disconnectBunkerWorker();
    bunkerServiceActive.set(false);
    bunkerServiceCatToken.set(null);
    bunkerClientTokens.set([]);
    bunkerSelectedTokenId.set(null);
    bunkerConnectedClients.set([]);
}

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
