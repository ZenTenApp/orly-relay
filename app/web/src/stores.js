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
