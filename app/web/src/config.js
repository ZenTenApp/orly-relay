/**
 * Relay configuration module for dual-mode operation (embedded vs standalone)
 *
 * Embedded mode: Dashboard served from same origin as relay (no CORS needed)
 * Standalone mode: Dashboard hosted separately, connects to remote relay (CORS required)
 */

import { get } from 'svelte/store';
import { relayUrl, isStandaloneMode, relayInfo, relayConnectionStatus } from './stores.js';

// Build-time configuration (set via rollup replace plugin)
const BUILD_STANDALONE_MODE = typeof process !== 'undefined' &&
    process.env && process.env.STANDALONE_MODE === 'true';
const BUILD_DEFAULT_RELAY_URL = typeof process !== 'undefined' &&
    process.env && process.env.DEFAULT_RELAY_URL || '';

/**
 * Initialize configuration on app startup
 * Call this from main.js before rendering App
 */
export function initConfig() {
    // Detect standalone mode:
    // 1. Explicitly built as standalone
    // 2. Has a configured relay URL in localStorage
    // 3. Running from file:// protocol
    // 4. Not running on a typical relay port (3334) - likely a static server
    const hasStoredRelay = !!localStorage.getItem("relayUrl");
    const isFileProtocol = window.location.protocol === 'file:';
    const isNonRelayPort = !['3334', '443', '80', ''].includes(window.location.port);

    const standalone = BUILD_STANDALONE_MODE || hasStoredRelay || isFileProtocol || isNonRelayPort;
    isStandaloneMode.set(standalone);

    // Set default relay URL from build config if not already set
    if (BUILD_DEFAULT_RELAY_URL && !get(relayUrl)) {
        relayUrl.set(BUILD_DEFAULT_RELAY_URL);
    }

    console.log('[config] Initialized:', {
        standaloneMode: standalone,
        buildStandalone: BUILD_STANDALONE_MODE,
        hasStoredRelay,
        isNonRelayPort,
        port: window.location.port,
        relayUrl: get(relayUrl) || '(same origin)'
    });
}

/**
 * Get the HTTP base URL for API calls
 * @returns {string} Base URL (e.g., "https://relay.example.com")
 */
export function getApiBase() {
    const url = get(relayUrl);
    if (url) {
        return normalizeHttpUrl(url);
    }
    return window.location.origin;
}

/**
 * Get the WebSocket URL for relay connection
 * @returns {string} WebSocket URL (e.g., "wss://relay.example.com/")
 */
export function getWsUrl() {
    const url = get(relayUrl);
    if (url) {
        return normalizeWsUrl(url);
    }
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${protocol}//${window.location.host}/`;
}

/**
 * Get array of relay URLs for nostr-tools SimplePool
 * @returns {string[]} Array with single relay URL
 */
export function getRelayUrls() {
    return [getWsUrl()];
}

/**
 * Check if running in standalone mode
 * @returns {boolean}
 */
export function isStandalone() {
    return get(isStandaloneMode);
}

/**
 * Check if a relay URL is configured (either stored or same-origin)
 * @returns {boolean}
 */
export function hasRelayConfigured() {
    // In embedded mode, always configured (same origin)
    // In standalone mode, need explicit URL
    if (!get(isStandaloneMode)) {
        return true;
    }
    return !!get(relayUrl);
}

/**
 * Set the relay URL and trigger connection
 * @param {string} url - Relay URL (http/https/ws/wss)
 */
export function setRelayUrl(url) {
    const normalized = url ? normalizeHttpUrl(url) : '';
    relayUrl.set(normalized);

    if (normalized) {
        // Mark as standalone since we have an explicit URL
        isStandaloneMode.set(true);
    }
}

/**
 * Fetch and validate relay info via NIP-11
 * @param {string} [url] - Optional URL to check (defaults to current relay)
 * @returns {Promise<object|null>} Relay info or null on error
 */
export async function fetchRelayInfoFromUrl(url) {
    const baseUrl = url ? normalizeHttpUrl(url) : getApiBase();

    try {
        relayConnectionStatus.set("connecting");

        const response = await fetch(baseUrl, {
            headers: {
                Accept: "application/nostr+json",
            },
        });

        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }

        const info = await response.json();

        // Validate it looks like relay info
        if (!info.name && !info.supported_nips) {
            throw new Error("Invalid relay info response");
        }

        relayInfo.set(info);
        relayConnectionStatus.set("connected");

        return info;
    } catch (error) {
        console.error('[config] Failed to fetch relay info:', error);
        relayConnectionStatus.set("error");
        relayInfo.set(null);
        return null;
    }
}

/**
 * Connect to a new relay URL
 * Validates via NIP-11 first, falls back to WebSocket test if CORS blocks NIP-11
 * @param {string} url - Relay URL
 * @returns {Promise<{success: boolean, info?: object, error?: string}>}
 */
export async function connectToRelay(url) {
    console.log('[config] connectToRelay called with:', url);
    if (!url) {
        return { success: false, error: "URL is required" };
    }

    const normalized = normalizeHttpUrl(url);
    console.log('[config] Normalized HTTP URL:', normalized);

    // Try to fetch relay info to validate
    const info = await fetchRelayInfoFromUrl(normalized);
    console.log('[config] fetchRelayInfoFromUrl returned:', info ? 'success' : 'null');

    if (info) {
        // NIP-11 worked, store the URL
        setRelayUrl(normalized);
        return { success: true, info };
    }

    // NIP-11 failed (likely CORS), try WebSocket connection test
    console.log('[config] NIP-11 failed, trying WebSocket connection test');
    const wsUrl = normalizeWsUrl(url);
    console.log('[config] Normalized WS URL:', wsUrl);
    const wsResult = await testWebSocketConnection(wsUrl);
    console.log('[config] WebSocket test complete:', wsResult);

    if (wsResult.success) {
        // WebSocket worked, store the URL
        setRelayUrl(normalized);
        relayConnectionStatus.set("connected");
        // Create minimal relay info
        const minimalInfo = { name: wsUrl };
        relayInfo.set(minimalInfo);
        return { success: true, info: minimalInfo };
    }

    return { success: false, error: wsResult.error || "Could not connect to relay" };
}

/**
 * Test WebSocket connection to a relay
 * @param {string} wsUrl - WebSocket URL
 * @returns {Promise<{success: boolean, error?: string}>}
 */
async function testWebSocketConnection(wsUrl) {
    console.log('[config] Testing WebSocket connection to:', wsUrl);
    return new Promise((resolve) => {
        let resolved = false;
        let ws = null;

        const safeResolve = (result) => {
            if (!resolved) {
                resolved = true;
                console.log('[config] WebSocket test result:', result);
                resolve(result);
            }
        };

        const timeout = setTimeout(() => {
            console.log('[config] WebSocket connection timed out');
            if (ws) ws.close();
            safeResolve({ success: false, error: "Connection timed out" });
        }, 5000);

        try {
            ws = new WebSocket(wsUrl);

            ws.onopen = () => {
                console.log('[config] WebSocket connected successfully');
                clearTimeout(timeout);
                ws.close();
                safeResolve({ success: true });
            };

            ws.onerror = (error) => {
                console.log('[config] WebSocket error:', error);
                clearTimeout(timeout);
                safeResolve({ success: false, error: "WebSocket connection failed" });
            };

            ws.onclose = (event) => {
                console.log('[config] WebSocket closed:', event.code, event.reason);
                clearTimeout(timeout);
                if (event.code !== 1000 && !resolved) {
                    safeResolve({ success: false, error: `Connection closed: ${event.reason || 'code ' + event.code}` });
                }
            };
        } catch (err) {
            console.error('[config] WebSocket creation error:', err);
            clearTimeout(timeout);
            safeResolve({ success: false, error: err.message || "Failed to create WebSocket" });
        }
    });
}

// ==================== URL Normalization Helpers ====================

/**
 * Normalize URL to HTTP(S) format
 * @param {string} url
 * @returns {string}
 */
function normalizeHttpUrl(url) {
    let normalized = url.trim();

    // Convert WebSocket URLs to HTTP
    if (normalized.startsWith('wss://')) {
        normalized = 'https://' + normalized.slice(6);
    } else if (normalized.startsWith('ws://')) {
        normalized = 'http://' + normalized.slice(5);
    }

    // Add protocol if missing
    if (!normalized.startsWith('http://') && !normalized.startsWith('https://')) {
        normalized = 'https://' + normalized;
    }

    // Remove trailing slash
    return normalized.replace(/\/$/, '');
}

/**
 * Normalize URL to WebSocket format
 * @param {string} url
 * @returns {string}
 */
export function normalizeWsUrl(url) {
    let normalized = url.trim();

    // Convert HTTP URLs to WebSocket
    if (normalized.startsWith('https://')) {
        normalized = 'wss://' + normalized.slice(8);
    } else if (normalized.startsWith('http://')) {
        normalized = 'ws://' + normalized.slice(7);
    }

    // Add protocol if missing
    if (!normalized.startsWith('ws://') && !normalized.startsWith('wss://')) {
        normalized = 'wss://' + normalized;
    }

    // Ensure trailing slash for relay URL
    if (!normalized.endsWith('/')) {
        normalized += '/';
    }

    return normalized;
}
