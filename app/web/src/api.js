/**
 * API helper functions for ORLY relay management endpoints
 */

import { getApiBase } from './config.js';

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
        console.log("createNIP98Auth: No signer or pubkey available", { hasSigner: !!signer, hasPubkey: !!pubkey });
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

        console.log("createNIP98Auth: Signing event for", method, url);

        // Sign using the signer
        const signedEvent = await signer.signEvent(authEvent);
        console.log("createNIP98Auth: Signed event:", {
            id: signedEvent.id,
            pubkey: signedEvent.pubkey,
            kind: signedEvent.kind,
            created_at: signedEvent.created_at,
            tags: signedEvent.tags,
            hasSig: !!signedEvent.sig
        });

        // Use URL-safe base64 encoding (replace + with -, / with _)
        const json = JSON.stringify(signedEvent);
        const base64 = btoa(json).replace(/\+/g, '-').replace(/\//g, '_');
        return base64;
    } catch (error) {
        console.error("createNIP98Auth: Error:", error);
        return null;
    }
}

/**
 * Fetch user role from the relay
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @returns {Promise<string>} User role
 */
export async function fetchUserRole(signer, pubkey) {
    try {
        const url = `${getApiBase()}/api/role`;
        const authHeader = await createNIP98Auth(signer, pubkey, "GET", url);
        const response = await fetch(url, {
            headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
        });
        if (response.ok) {
            const data = await response.json();
            return data.role || "";
        }
    } catch (error) {
        console.error("Error fetching user role:", error);
    }
    return "";
}

/**
 * Fetch ACL mode from the relay
 * @returns {Promise<string>} ACL mode
 */
export async function fetchACLMode() {
    try {
        const response = await fetch(`${getApiBase()}/api/acl-mode`);
        if (response.ok) {
            const data = await response.json();
            return data.mode || "";
        }
    } catch (error) {
        console.error("Error fetching ACL mode:", error);
    }
    return "";
}

// ==================== Sprocket API ====================

/**
 * Load sprocket configuration
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @returns {Promise<object>} Sprocket config data
 */
export async function loadSprocketConfig(signer, pubkey) {
    const url = `${getApiBase()}/api/sprocket/config`;
    const authHeader = await createNIP98Auth(signer, pubkey, "GET", url);
    const response = await fetch(url, {
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (!response.ok) throw new Error(`Failed to load config: ${response.statusText}`);
    return await response.json();
}

/**
 * Load sprocket status
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @returns {Promise<object>} Sprocket status data
 */
export async function loadSprocketStatus(signer, pubkey) {
    const url = `${getApiBase()}/api/sprocket/status`;
    const authHeader = await createNIP98Auth(signer, pubkey, "GET", url);
    const response = await fetch(url, {
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (!response.ok) throw new Error(`Failed to load status: ${response.statusText}`);
    return await response.json();
}

/**
 * Load sprocket script
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @returns {Promise<string>} Sprocket script content
 */
export async function loadSprocketScript(signer, pubkey) {
    const url = `${getApiBase()}/api/sprocket`;
    const authHeader = await createNIP98Auth(signer, pubkey, "GET", url);
    const response = await fetch(url, {
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (response.status === 404) return "";
    if (!response.ok) throw new Error(`Failed to load sprocket: ${response.statusText}`);
    return await response.text();
}

/**
 * Save sprocket script
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @param {string} script - Script content
 * @returns {Promise<object>} Save result
 */
export async function saveSprocketScript(signer, pubkey, script) {
    const url = `${getApiBase()}/api/sprocket`;
    const authHeader = await createNIP98Auth(signer, pubkey, "PUT", url);
    const response = await fetch(url, {
        method: "PUT",
        headers: {
            "Content-Type": "text/plain",
            ...(authHeader ? { Authorization: `Nostr ${authHeader}` } : {}),
        },
        body: script,
    });
    if (!response.ok) throw new Error(`Failed to save: ${response.statusText}`);
    return await response.json();
}

/**
 * Restart sprocket
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @returns {Promise<object>} Restart result
 */
export async function restartSprocket(signer, pubkey) {
    const url = `${getApiBase()}/api/sprocket/restart`;
    const authHeader = await createNIP98Auth(signer, pubkey, "POST", url);
    const response = await fetch(url, {
        method: "POST",
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (!response.ok) throw new Error(`Failed to restart: ${response.statusText}`);
    return await response.json();
}

/**
 * Delete sprocket
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @returns {Promise<object>} Delete result
 */
export async function deleteSprocket(signer, pubkey) {
    const url = `${getApiBase()}/api/sprocket`;
    const authHeader = await createNIP98Auth(signer, pubkey, "DELETE", url);
    const response = await fetch(url, {
        method: "DELETE",
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (!response.ok) throw new Error(`Failed to delete: ${response.statusText}`);
    return await response.json();
}

/**
 * Load sprocket versions
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @returns {Promise<Array>} Version list
 */
export async function loadSprocketVersions(signer, pubkey) {
    const url = `${getApiBase()}/api/sprocket/versions`;
    const authHeader = await createNIP98Auth(signer, pubkey, "GET", url);
    const response = await fetch(url, {
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (!response.ok) throw new Error(`Failed to load versions: ${response.statusText}`);
    return await response.json();
}

/**
 * Load specific sprocket version
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @param {string} version - Version filename
 * @returns {Promise<string>} Version content
 */
export async function loadSprocketVersion(signer, pubkey, version) {
    const url = `${getApiBase()}/api/sprocket/versions/${encodeURIComponent(version)}`;
    const authHeader = await createNIP98Auth(signer, pubkey, "GET", url);
    const response = await fetch(url, {
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (!response.ok) throw new Error(`Failed to load version: ${response.statusText}`);
    return await response.text();
}

/**
 * Delete sprocket version
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @param {string} filename - Version filename
 * @returns {Promise<object>} Delete result
 */
export async function deleteSprocketVersion(signer, pubkey, filename) {
    const url = `${getApiBase()}/api/sprocket/versions/${encodeURIComponent(filename)}`;
    const authHeader = await createNIP98Auth(signer, pubkey, "DELETE", url);
    const response = await fetch(url, {
        method: "DELETE",
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (!response.ok) throw new Error(`Failed to delete version: ${response.statusText}`);
    return await response.json();
}

/**
 * Upload sprocket script file
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @param {File} file - File to upload
 * @returns {Promise<object>} Upload result
 */
export async function uploadSprocketScript(signer, pubkey, file) {
    const content = await file.text();
    return await saveSprocketScript(signer, pubkey, content);
}

// ==================== Policy API ====================

/**
 * Load policy configuration
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @returns {Promise<object>} Policy config
 */
export async function loadPolicyConfig(signer, pubkey) {
    const url = `${getApiBase()}/api/policy/config`;
    const authHeader = await createNIP98Auth(signer, pubkey, "GET", url);
    const response = await fetch(url, {
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (!response.ok) throw new Error(`Failed to load policy config: ${response.statusText}`);
    return await response.json();
}

/**
 * Load policy JSON
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @returns {Promise<object>} Policy JSON
 */
export async function loadPolicy(signer, pubkey) {
    const url = `${getApiBase()}/api/policy`;
    const authHeader = await createNIP98Auth(signer, pubkey, "GET", url);
    const response = await fetch(url, {
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (!response.ok) throw new Error(`Failed to load policy: ${response.statusText}`);
    return await response.json();
}

/**
 * Validate policy JSON
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @param {string} policyJson - Policy JSON string
 * @returns {Promise<object>} Validation result
 */
export async function validatePolicy(signer, pubkey, policyJson) {
    const url = `${getApiBase()}/api/policy/validate`;
    const authHeader = await createNIP98Auth(signer, pubkey, "POST", url);
    const response = await fetch(url, {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
            ...(authHeader ? { Authorization: `Nostr ${authHeader}` } : {}),
        },
        body: policyJson,
    });
    return await response.json();
}

/**
 * Fetch policy follows whitelist
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @returns {Promise<Array>} List of followed pubkeys
 */
export async function fetchPolicyFollows(signer, pubkey) {
    const url = `${getApiBase()}/api/policy/follows`;
    const authHeader = await createNIP98Auth(signer, pubkey, "GET", url);
    const response = await fetch(url, {
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (!response.ok) throw new Error(`Failed to fetch follows: ${response.statusText}`);
    const data = await response.json();
    return data.follows || [];
}

// ==================== Relay Info API ====================

/**
 * Fetch relay info document (NIP-11)
 * @returns {Promise<object>} Relay info including version
 */
export async function fetchRelayInfo() {
    try {
        const response = await fetch(getApiBase(), {
            headers: {
                Accept: "application/nostr+json",
            },
        });
        if (response.ok) {
            return await response.json();
        }
    } catch (error) {
        console.error("Error fetching relay info:", error);
    }
    return null;
}

// ==================== Export/Import API ====================

/**
 * Export events
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @param {Array} authorPubkeys - Filter by authors (empty for all)
 * @returns {Promise<Blob>} JSONL blob
 */
export async function exportEvents(signer, pubkey, authorPubkeys = []) {
    const url = `${getApiBase()}/api/export`;
    const authHeader = await createNIP98Auth(signer, pubkey, "POST", url);

    const response = await fetch(url, {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
            ...(authHeader ? { Authorization: `Nostr ${authHeader}` } : {}),
        },
        body: JSON.stringify({ pubkeys: authorPubkeys }),
    });

    if (!response.ok) throw new Error(`Export failed: ${response.statusText}`);
    return await response.blob();
}

/**
 * Import events from file
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @param {File} file - JSONL file to import
 * @returns {Promise<object>} Import result
 */
export async function importEvents(signer, pubkey, file) {
    const url = `${getApiBase()}/api/import`;
    const authHeader = await createNIP98Auth(signer, pubkey, "POST", url);

    const formData = new FormData();
    formData.append("file", file);

    const response = await fetch(url, {
        method: "POST",
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
        body: formData,
    });

    if (!response.ok) throw new Error(`Import failed: ${response.statusText}`);
    return await response.json();
}

// ==================== WireGuard API ====================

/**
 * Fetch WireGuard status
 * @returns {Promise<object>} WireGuard status
 */
export async function fetchWireGuardStatus() {
    try {
        const response = await fetch(`${getApiBase()}/api/wireguard/status`);
        if (response.ok) {
            return await response.json();
        }
    } catch (error) {
        console.error("Error fetching WireGuard status:", error);
    }
    return { wireguard_enabled: false, bunker_enabled: false, available: false };
}

/**
 * Get WireGuard configuration for the authenticated user
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @returns {Promise<object>} WireGuard config
 */
export async function getWireGuardConfig(signer, pubkey) {
    const url = `${getApiBase()}/api/wireguard/config`;
    const authHeader = await createNIP98Auth(signer, pubkey, "GET", url);
    const response = await fetch(url, {
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (!response.ok) {
        const error = await response.text();
        throw new Error(error || `Failed to get WireGuard config: ${response.statusText}`);
    }
    return await response.json();
}

/**
 * Regenerate WireGuard keypair for the authenticated user
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @returns {Promise<object>} Regeneration result
 */
export async function regenerateWireGuard(signer, pubkey) {
    const url = `${getApiBase()}/api/wireguard/regenerate`;
    const authHeader = await createNIP98Auth(signer, pubkey, "POST", url);
    const response = await fetch(url, {
        method: "POST",
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (!response.ok) {
        const error = await response.text();
        throw new Error(error || `Failed to regenerate WireGuard: ${response.statusText}`);
    }
    return await response.json();
}


/**
 * Get WireGuard audit log (revoked keys and access attempts)
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @returns {Promise<object>} Audit data with revoked_keys and access_logs
 */
export async function getWireGuardAudit(signer, pubkey) {
    const url = `${getApiBase()}/api/wireguard/audit`;
    const authHeader = await createNIP98Auth(signer, pubkey, "GET", url);
    const response = await fetch(url, {
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (!response.ok) {
        const error = await response.text();
        throw new Error(error || `Failed to get audit log: ${response.statusText}`);
    }
    return await response.json();
}

// ==================== NRC (Nostr Relay Connect) API ====================

/**
 * Get NRC configuration status (no auth required)
 * @returns {Promise<object>} NRC config status
 */
export async function fetchNRCConfig() {
    const apiBase = getApiBase();
    console.log("[api] fetchNRCConfig using base URL:", apiBase);
    try {
        const response = await fetch(`${apiBase}/api/nrc/config`);
        if (response.ok) {
            return await response.json();
        }
    } catch (error) {
        console.error("Error fetching NRC config:", error);
    }
    return { enabled: false, badger_required: true };
}

/**
 * Get all NRC connections
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @returns {Promise<object>} Connections list and config
 */
export async function fetchNRCConnections(signer, pubkey) {
    const url = `${getApiBase()}/api/nrc/connections`;
    const authHeader = await createNIP98Auth(signer, pubkey, "GET", url);
    const response = await fetch(url, {
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (!response.ok) {
        const error = await response.text();
        throw new Error(error || `Failed to get NRC connections: ${response.statusText}`);
    }
    return await response.json();
}

/**
 * Create a new NRC connection
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @param {string} label - Connection label
 * @returns {Promise<object>} Created connection with URI
 */
export async function createNRCConnection(signer, pubkey, label) {
    const url = `${getApiBase()}/api/nrc/connections`;
    const authHeader = await createNIP98Auth(signer, pubkey, "POST", url);
    const response = await fetch(url, {
        method: "POST",
        headers: {
            "Content-Type": "application/json",
            ...(authHeader ? { Authorization: `Nostr ${authHeader}` } : {}),
        },
        body: JSON.stringify({ label }),
    });
    if (!response.ok) {
        const error = await response.text();
        throw new Error(error || `Failed to create NRC connection: ${response.statusText}`);
    }
    return await response.json();
}

/**
 * Delete an NRC connection
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @param {string} connId - Connection ID to delete
 * @returns {Promise<object>} Delete result
 */
export async function deleteNRCConnection(signer, pubkey, connId) {
    const url = `${getApiBase()}/api/nrc/connections/${connId}`;
    const authHeader = await createNIP98Auth(signer, pubkey, "DELETE", url);
    const response = await fetch(url, {
        method: "DELETE",
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (!response.ok) {
        const error = await response.text();
        throw new Error(error || `Failed to delete NRC connection: ${response.statusText}`);
    }
    return await response.json();
}

/**
 * Get connection URI for an NRC connection
 * @param {object} signer - The signer instance
 * @param {string} pubkey - User's pubkey
 * @param {string} connId - Connection ID
 * @returns {Promise<object>} Connection URI
 */
export async function getNRCConnectionURI(signer, pubkey, connId) {
    const url = `${getApiBase()}/api/nrc/connections/${connId}/uri`;
    const authHeader = await createNIP98Auth(signer, pubkey, "GET", url);
    const response = await fetch(url, {
        headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
    });
    if (!response.ok) {
        const error = await response.text();
        throw new Error(error || `Failed to get NRC URI: ${response.statusText}`);
    }
    return await response.json();
}

