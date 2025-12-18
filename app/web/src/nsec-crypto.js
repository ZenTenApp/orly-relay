/**
 * Secure nsec encryption/decryption using Argon2id + AES-GCM
 *
 * - Argon2id key derivation with ~3 second computation time (runs in Web Worker)
 * - AES-256-GCM authenticated encryption
 * - Validates bech32 nsec format and checksum on decryption
 */

import { argon2id } from "hash-wasm";
import { decode as nip19Decode } from "nostr-tools/nip19";

// Argon2id parameters tuned for ~3 second derivation on typical hardware
const ARGON2_CONFIG = {
    parallelism: 4,      // 4 threads
    iterations: 8,       // Time cost
    memorySize: 262144,  // 256 MB memory
    hashLength: 32,      // 256-bit key for AES-256
    outputType: "binary"
};

// Worker singleton and message ID counter
let worker = null;
let messageId = 0;
const pendingRequests = new Map();

/**
 * Get or create the Argon2 worker
 */
function getWorker() {
    if (worker) return worker;

    // Inline worker code - includes hash-wasm import via importScripts alternative
    // Since we can't easily import ES modules in workers, we'll use a different approach
    // We'll run argon2id in chunks with yielding to allow UI updates

    const workerCode = `
        importScripts('https://cdn.jsdelivr.net/npm/hash-wasm@4.11.0/dist/argon2.umd.min.js');

        const ARGON2_CONFIG = {
            parallelism: 4,
            iterations: 8,
            memorySize: 262144,
            hashLength: 32,
            outputType: "binary"
        };

        self.onmessage = async function(e) {
            const { password, salt, id } = e.data;

            try {
                const result = await hashwasm.argon2id({
                    password: password,
                    salt: new Uint8Array(salt),
                    ...ARGON2_CONFIG
                });

                self.postMessage({
                    id,
                    success: true,
                    result: Array.from(result)
                });
            } catch (error) {
                self.postMessage({
                    id,
                    success: false,
                    error: error.message
                });
            }
        };
    `;

    const blob = new Blob([workerCode], { type: 'application/javascript' });
    worker = new Worker(URL.createObjectURL(blob));

    worker.onmessage = function(e) {
        const { id, success, result, error } = e.data;
        const pending = pendingRequests.get(id);
        if (pending) {
            pendingRequests.delete(id);
            if (success) {
                pending.resolve(new Uint8Array(result));
            } else {
                pending.reject(new Error(error));
            }
        }
    };

    worker.onerror = function(e) {
        console.error('Argon2 worker error:', e);
    };

    return worker;
}

/**
 * Derive an encryption key from password using Argon2id (in Web Worker)
 * @param {string} password - User's password
 * @param {Uint8Array} salt - Random 32-byte salt
 * @returns {Promise<Uint8Array>} - 32-byte derived key
 */
export async function deriveKey(password, salt) {
    // Try to use worker, fall back to main thread if it fails
    try {
        const w = getWorker();
        const id = ++messageId;

        return new Promise((resolve, reject) => {
            pendingRequests.set(id, { resolve, reject });
            w.postMessage({
                id,
                password,
                salt: Array.from(salt)
            });
        });
    } catch (e) {
        // Fallback to main thread (will block UI but at least works)
        console.warn('Worker failed, falling back to main thread:', e);
        const result = await argon2id({
            password: password,
            salt: salt,
            ...ARGON2_CONFIG
        });
        return result;
    }
}

/**
 * Encrypt an nsec with a password
 * @param {string} nsec - The nsec in bech32 format (nsec1...)
 * @param {string} password - User's password
 * @returns {Promise<string>} - Base64 encoded encrypted data (salt + iv + ciphertext)
 */
export async function encryptNsec(nsec, password) {
    // Validate nsec format first
    if (!nsec.startsWith("nsec1")) {
        throw new Error("Invalid nsec format - must start with nsec1");
    }

    // Validate bech32 checksum
    try {
        const decoded = nip19Decode(nsec);
        if (decoded.type !== "nsec") {
            throw new Error("Invalid nsec - wrong type");
        }
    } catch (e) {
        throw new Error("Invalid nsec - bech32 checksum failed");
    }

    // Generate random salt and IV
    const salt = crypto.getRandomValues(new Uint8Array(32));
    const iv = crypto.getRandomValues(new Uint8Array(12));

    // Derive key using Argon2id (~3 seconds, in worker)
    const keyBytes = await deriveKey(password, salt);

    // Import key for AES-GCM
    const key = await crypto.subtle.importKey(
        "raw",
        keyBytes,
        { name: "AES-GCM" },
        false,
        ["encrypt"]
    );

    // Encrypt the nsec
    const encoder = new TextEncoder();
    const encrypted = await crypto.subtle.encrypt(
        { name: "AES-GCM", iv: iv },
        key,
        encoder.encode(nsec)
    );

    // Combine salt + iv + ciphertext and encode as base64
    const combined = new Uint8Array(salt.length + iv.length + encrypted.byteLength);
    combined.set(salt, 0);
    combined.set(iv, salt.length);
    combined.set(new Uint8Array(encrypted), salt.length + iv.length);

    return btoa(String.fromCharCode(...combined));
}

/**
 * Decrypt an nsec with a password
 * @param {string} encryptedData - Base64 encoded encrypted data
 * @param {string} password - User's password
 * @returns {Promise<string>} - The decrypted nsec in bech32 format
 * @throws {Error} - If password is wrong or data is corrupted
 */
export async function decryptNsec(encryptedData, password) {
    // Decode base64
    const combined = new Uint8Array(
        atob(encryptedData).split("").map(c => c.charCodeAt(0))
    );

    // Validate minimum length (32 salt + 12 iv + 16 auth tag + some ciphertext)
    if (combined.length < 60) {
        throw new Error("Invalid encrypted data - too short");
    }

    // Extract salt, iv, and ciphertext
    const salt = combined.slice(0, 32);
    const iv = combined.slice(32, 44);
    const ciphertext = combined.slice(44);

    // Derive key using Argon2id (~3 seconds, in worker)
    const keyBytes = await deriveKey(password, salt);

    // Import key for AES-GCM
    const key = await crypto.subtle.importKey(
        "raw",
        keyBytes,
        { name: "AES-GCM" },
        false,
        ["decrypt"]
    );

    // Decrypt
    let decrypted;
    try {
        decrypted = await crypto.subtle.decrypt(
            { name: "AES-GCM", iv: iv },
            key,
            ciphertext
        );
    } catch (e) {
        throw new Error("Decryption failed - invalid password or corrupted data");
    }

    const decoder = new TextDecoder();
    const nsec = decoder.decode(decrypted);

    // Validate the decrypted nsec has correct bech32 format and checksum
    if (!nsec.startsWith("nsec1")) {
        throw new Error("Decryption produced invalid data - not an nsec");
    }

    try {
        const decoded = nip19Decode(nsec);
        if (decoded.type !== "nsec") {
            throw new Error("Decryption produced invalid nsec type");
        }
    } catch (e) {
        throw new Error("Decryption produced invalid nsec - bech32 checksum failed");
    }

    return nsec;
}

/**
 * Check if a string is a valid nsec (validates bech32 format and checksum)
 * @param {string} nsec - The string to validate
 * @returns {boolean} - True if valid nsec
 */
export function isValidNsec(nsec) {
    if (!nsec || !nsec.startsWith("nsec1")) {
        return false;
    }
    try {
        const decoded = nip19Decode(nsec);
        return decoded.type === "nsec";
    } catch {
        return false;
    }
}

/**
 * Terminate the worker (call when done to free resources)
 */
export function terminateWorker() {
    if (worker) {
        worker.terminate();
        worker = null;
    }
}
