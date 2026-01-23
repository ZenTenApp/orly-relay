/**
 * API helper functions for ORLY Launcher admin endpoints
 */

/**
 * Get the API base URL (same as current page)
 */
export function getApiBase() {
    return window.location.origin;
}

/**
 * Create NIP-98 authentication header
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @param {string} method - HTTP method
 * @param {string} url - Request URL
 * @returns {Promise<string|null>} Base64 encoded auth header or null
 */
export async function createNIP98Auth(signer, pubkey, method, url) {
    if (!signer || !pubkey) {
        return null;
    }

    try {
        // Create unsigned auth event
        const authEvent = {
            kind: 27235,
            created_at: Math.floor(Date.now() / 1000),
            tags: [
                ["u", url],
                ["method", method.toUpperCase()],
            ],
            content: "",
        };

        // Sign using the signer
        const signedEvent = await signer.signEvent(authEvent);

        // Use URL-safe base64 encoding
        const json = JSON.stringify(signedEvent);
        const base64 = btoa(json).replace(/\+/g, '-').replace(/\//g, '_');
        return base64;
    } catch (error) {
        console.error("createNIP98Auth error:", error);
        return null;
    }
}

/**
 * Make an authenticated API request
 * @param {string} path - API path
 * @param {object} options - Fetch options
 * @param {object} signer - Signer instance
 * @param {string} pubkey - User's pubkey
 * @returns {Promise<Response>}
 */
async function authFetch(path, options = {}, signer, pubkey) {
    const url = `${getApiBase()}${path}`;
    const method = options.method || 'GET';
    const authHeader = await createNIP98Auth(signer, pubkey, method, url);

    const headers = {
        ...options.headers,
    };

    if (authHeader) {
        headers['Authorization'] = `Nostr ${authHeader}`;
    }

    return fetch(url, { ...options, headers });
}

/**
 * Fetch launcher status
 */
export async function fetchStatus(signer, pubkey) {
    const response = await authFetch('/api/status', {}, signer, pubkey);
    if (!response.ok) {
        throw new Error(`Failed to fetch status: ${response.statusText}`);
    }
    return response.json();
}

/**
 * Fetch launcher configuration
 */
export async function fetchConfig(signer, pubkey) {
    const response = await authFetch('/api/config', {}, signer, pubkey);
    if (!response.ok) {
        throw new Error(`Failed to fetch config: ${response.statusText}`);
    }
    return response.json();
}

/**
 * Save launcher configuration
 * @param {object} config - Configuration object to save
 */
export async function saveConfig(signer, pubkey, config) {
    const response = await authFetch('/api/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(config),
    }, signer, pubkey);

    if (!response.ok) {
        const data = await response.json().catch(() => ({}));
        throw new Error(data.message || `Save failed: ${response.statusText}`);
    }
    return response.json();
}

/**
 * Fetch available binaries
 */
export async function fetchBinaries(signer, pubkey) {
    const response = await authFetch('/api/binaries', {}, signer, pubkey);
    if (!response.ok) {
        throw new Error(`Failed to fetch binaries: ${response.statusText}`);
    }
    return response.json();
}

/**
 * Update binaries from URLs
 */
export async function updateBinaries(signer, pubkey, version, urls) {
    const response = await authFetch('/api/update', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ version, urls }),
    }, signer, pubkey);

    if (!response.ok) {
        const data = await response.json();
        throw new Error(data.message || `Update failed: ${response.statusText}`);
    }
    return response.json();
}

/**
 * Restart all services
 */
export async function restartServices(signer, pubkey) {
    const response = await authFetch('/api/restart', {
        method: 'POST',
    }, signer, pubkey);

    if (!response.ok) {
        throw new Error(`Restart failed: ${response.statusText}`);
    }
    return response.json();
}

/**
 * Rollback to previous version
 */
export async function rollbackVersion(signer, pubkey) {
    const response = await authFetch('/api/rollback', {
        method: 'POST',
    }, signer, pubkey);

    if (!response.ok) {
        const data = await response.json();
        throw new Error(data.message || `Rollback failed: ${response.statusText}`);
    }
    return response.json();
}
