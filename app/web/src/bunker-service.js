/**
 * BunkerService - NIP-46 Remote Signer
 *
 * Implements the signer side of NIP-46 protocol.
 * Listens for signing requests from remote clients and responds using
 * the user's private key stored in ORLY.
 *
 * Protocol:
 * - Kind 24133 events for request/response
 * - NIP-04 encryption for payloads
 * - Methods: connect, get_public_key, sign_event, nip04_encrypt, nip04_decrypt, ping
 */

import { nip04 } from 'nostr-tools';
import { bytesToHex, hexToBytes } from '@noble/hashes/utils';
import { secp256k1 } from '@noble/curves/secp256k1';
import { encodeToken } from './cashu-client.js';

// NIP-46 methods
const NIP46_METHOD = {
    CONNECT: 'connect',
    GET_PUBLIC_KEY: 'get_public_key',
    SIGN_EVENT: 'sign_event',
    NIP04_ENCRYPT: 'nip04_encrypt',
    NIP04_DECRYPT: 'nip04_decrypt',
    PING: 'ping'
};

/**
 * Generate a random hex string.
 */
function generateRandomHex(bytes = 16) {
    const arr = new Uint8Array(bytes);
    crypto.getRandomValues(arr);
    return bytesToHex(arr);
}

/**
 * BunkerService class - implements NIP-46 signer protocol.
 */
export class BunkerService {
    /**
     * @param {string} relayUrl - WebSocket URL of the relay
     * @param {string} userPubkey - User's public key (hex)
     * @param {Uint8Array} userPrivkey - User's private key (32 bytes)
     */
    constructor(relayUrl, userPubkey, userPrivkey) {
        this.relayUrl = relayUrl;
        this.userPubkey = userPubkey;
        this.userPrivkey = userPrivkey;
        this.ws = null;
        this.connected = false;
        this.allowedSecrets = new Set();
        this.connectedClients = new Map(); // pubkey -> { connectedAt, lastActivity }
        this.requestLog = [];
        this.heartbeatInterval = null;
        this.subscriptionId = null;
        this.catToken = null;

        // Callbacks
        this.onClientConnected = null;
        this.onClientDisconnected = null;
        this.onRequest = null;
        this.onStatusChange = null;
    }

    /**
     * Add an allowed connection secret.
     */
    addAllowedSecret(secret) {
        this.allowedSecrets.add(secret);
    }

    /**
     * Remove an allowed secret.
     */
    removeAllowedSecret(secret) {
        this.allowedSecrets.delete(secret);
    }

    /**
     * Set CAT token for WebSocket connection.
     */
    setCatToken(token) {
        this.catToken = token;
    }

    /**
     * Connect to the relay and start listening for NIP-46 requests.
     */
    async connect() {
        return new Promise((resolve, reject) => {
            // Build WebSocket URL
            let wsUrl = this.relayUrl;
            if (wsUrl.startsWith('http://')) {
                wsUrl = 'ws://' + wsUrl.slice(7);
            } else if (wsUrl.startsWith('https://')) {
                wsUrl = 'wss://' + wsUrl.slice(8);
            } else if (!wsUrl.startsWith('ws://') && !wsUrl.startsWith('wss://')) {
                wsUrl = 'wss://' + wsUrl;
            }

            // Add CAT token if available
            if (this.catToken) {
                const tokenEncoded = encodeToken(this.catToken);
                const url = new URL(wsUrl);
                url.searchParams.set('token', tokenEncoded);
                wsUrl = url.toString();
            }

            console.log('[BunkerService] Connecting to:', wsUrl.split('?')[0]);

            const ws = new WebSocket(wsUrl);

            const timeout = setTimeout(() => {
                ws.close();
                reject(new Error('Connection timeout'));
            }, 10000);

            ws.onopen = () => {
                clearTimeout(timeout);
                this.ws = ws;
                this.connected = true;
                console.log('[BunkerService] Connected to relay');

                // Subscribe to NIP-46 events for our pubkey
                this.subscriptionId = generateRandomHex(8);
                const sub = JSON.stringify([
                    'REQ',
                    this.subscriptionId,
                    {
                        kinds: [24133],
                        '#p': [this.userPubkey],
                        since: Math.floor(Date.now() / 1000) - 60
                    }
                ]);
                ws.send(sub);
                console.log('[BunkerService] Subscribed to NIP-46 events');

                // Start heartbeat
                this.startHeartbeat();

                if (this.onStatusChange) {
                    this.onStatusChange('connected');
                }

                resolve();
            };

            ws.onerror = (error) => {
                clearTimeout(timeout);
                console.error('[BunkerService] WebSocket error:', error);
                reject(new Error('WebSocket error'));
            };

            ws.onclose = () => {
                this.connected = false;
                this.ws = null;
                this.stopHeartbeat();
                console.log('[BunkerService] Disconnected from relay');

                if (this.onStatusChange) {
                    this.onStatusChange('disconnected');
                }
            };

            ws.onmessage = (event) => {
                this.handleMessage(event.data);
            };
        });
    }

    /**
     * Start WebSocket heartbeat to keep connection alive.
     */
    startHeartbeat(intervalMs = 30000) {
        this.stopHeartbeat();
        this.heartbeatInterval = setInterval(() => {
            if (this.ws && this.ws.readyState === WebSocket.OPEN) {
                // Send a ping via Nostr protocol (re-subscribe)
                const sub = JSON.stringify([
                    'REQ',
                    this.subscriptionId,
                    {
                        kinds: [24133],
                        '#p': [this.userPubkey],
                        since: Math.floor(Date.now() / 1000) - 60
                    }
                ]);
                this.ws.send(sub);
            }
        }, intervalMs);
    }

    /**
     * Stop WebSocket heartbeat.
     */
    stopHeartbeat() {
        if (this.heartbeatInterval) {
            clearInterval(this.heartbeatInterval);
            this.heartbeatInterval = null;
        }
    }

    /**
     * Disconnect from the relay.
     */
    disconnect() {
        this.stopHeartbeat();
        if (this.ws) {
            // Close subscription
            if (this.subscriptionId) {
                this.ws.send(JSON.stringify(['CLOSE', this.subscriptionId]));
            }
            this.ws.close();
            this.ws = null;
        }
        this.connected = false;
        this.connectedClients.clear();
    }

    /**
     * Handle incoming WebSocket messages.
     */
    async handleMessage(data) {
        try {
            const msg = JSON.parse(data);
            if (!Array.isArray(msg)) return;

            const [type, ...rest] = msg;

            if (type === 'EVENT') {
                const [, event] = rest;
                if (event.kind === 24133) {
                    await this.handleNIP46Request(event);
                }
            } else if (type === 'OK') {
                // Event published confirmation
                console.log('[BunkerService] Event published:', rest[0]?.substring(0, 8));
            } else if (type === 'NOTICE') {
                console.warn('[BunkerService] Relay notice:', rest[0]);
            }
        } catch (err) {
            console.error('[BunkerService] Failed to parse message:', err);
        }
    }

    /**
     * Handle NIP-46 request event.
     */
    async handleNIP46Request(event) {
        try {
            // Decrypt the content with NIP-04
            const privkeyHex = bytesToHex(this.userPrivkey);
            const decrypted = await nip04.decrypt(privkeyHex, event.pubkey, event.content);
            const request = JSON.parse(decrypted);

            console.log('[BunkerService] Received request:', request.method, 'from:', event.pubkey.substring(0, 8));

            // Log the request
            this.requestLog.push({
                id: request.id,
                method: request.method,
                from: event.pubkey,
                timestamp: Date.now()
            });
            if (this.requestLog.length > 100) {
                this.requestLog.shift();
            }

            if (this.onRequest) {
                this.onRequest(request, event.pubkey);
            }

            // Handle the request
            let result = null;
            let error = null;

            try {
                switch (request.method) {
                    case NIP46_METHOD.CONNECT:
                        result = await this.handleConnect(request, event.pubkey);
                        break;
                    case NIP46_METHOD.GET_PUBLIC_KEY:
                        result = await this.handleGetPublicKey(request, event.pubkey);
                        break;
                    case NIP46_METHOD.SIGN_EVENT:
                        result = await this.handleSignEvent(request, event.pubkey);
                        break;
                    case NIP46_METHOD.NIP04_ENCRYPT:
                        result = await this.handleNip04Encrypt(request, event.pubkey);
                        break;
                    case NIP46_METHOD.NIP04_DECRYPT:
                        result = await this.handleNip04Decrypt(request, event.pubkey);
                        break;
                    case NIP46_METHOD.PING:
                        result = 'pong';
                        break;
                    default:
                        error = `Unknown method: ${request.method}`;
                }
            } catch (err) {
                console.error('[BunkerService] Error handling request:', err);
                error = err.message;
            }

            // Send response
            await this.sendResponse(request.id, result, error, event.pubkey);

        } catch (err) {
            console.error('[BunkerService] Failed to handle NIP-46 request:', err);
        }
    }

    /**
     * Handle connect request.
     */
    async handleConnect(request, senderPubkey) {
        const [clientPubkey, secret] = request.params;

        // Validate secret if required
        if (this.allowedSecrets.size > 0) {
            if (!secret || !this.allowedSecrets.has(secret)) {
                throw new Error('Invalid or missing connection secret');
            }
        }

        // Register connected client
        this.connectedClients.set(senderPubkey, {
            clientPubkey: clientPubkey || senderPubkey,
            connectedAt: Date.now(),
            lastActivity: Date.now()
        });

        console.log('[BunkerService] Client connected:', senderPubkey.substring(0, 8));

        if (this.onClientConnected) {
            this.onClientConnected(senderPubkey);
        }

        return 'ack';
    }

    /**
     * Handle get_public_key request.
     */
    async handleGetPublicKey(request, senderPubkey) {
        // Update last activity
        if (this.connectedClients.has(senderPubkey)) {
            this.connectedClients.get(senderPubkey).lastActivity = Date.now();
        }

        return this.userPubkey;
    }

    /**
     * Handle sign_event request.
     */
    async handleSignEvent(request, senderPubkey) {
        // Check if client is connected
        if (!this.connectedClients.has(senderPubkey)) {
            throw new Error('Not connected');
        }

        // Update last activity
        this.connectedClients.get(senderPubkey).lastActivity = Date.now();

        const [eventJson] = request.params;
        const event = JSON.parse(eventJson);

        // Ensure pubkey matches
        if (event.pubkey && event.pubkey !== this.userPubkey) {
            throw new Error('Event pubkey does not match signer pubkey');
        }

        // Set pubkey if not set
        event.pubkey = this.userPubkey;

        // Calculate event ID
        const serialized = JSON.stringify([
            0,
            event.pubkey,
            event.created_at,
            event.kind,
            event.tags,
            event.content
        ]);
        const hash = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(serialized));
        event.id = bytesToHex(new Uint8Array(hash));

        // Sign the event
        const sig = secp256k1.sign(hexToBytes(event.id), this.userPrivkey);
        event.sig = sig.toCompactHex();

        console.log('[BunkerService] Signed event:', event.id.substring(0, 8), 'kind:', event.kind);

        return JSON.stringify(event);
    }

    /**
     * Handle nip04_encrypt request.
     */
    async handleNip04Encrypt(request, senderPubkey) {
        // Check if client is connected
        if (!this.connectedClients.has(senderPubkey)) {
            throw new Error('Not connected');
        }

        // Update last activity
        this.connectedClients.get(senderPubkey).lastActivity = Date.now();

        const [pubkey, plaintext] = request.params;
        const privkeyHex = bytesToHex(this.userPrivkey);
        const ciphertext = await nip04.encrypt(privkeyHex, pubkey, plaintext);
        return ciphertext;
    }

    /**
     * Handle nip04_decrypt request.
     */
    async handleNip04Decrypt(request, senderPubkey) {
        // Check if client is connected
        if (!this.connectedClients.has(senderPubkey)) {
            throw new Error('Not connected');
        }

        // Update last activity
        this.connectedClients.get(senderPubkey).lastActivity = Date.now();

        const [pubkey, ciphertext] = request.params;
        const privkeyHex = bytesToHex(this.userPrivkey);
        const plaintext = await nip04.decrypt(privkeyHex, pubkey, ciphertext);
        return plaintext;
    }

    /**
     * Send NIP-46 response to client.
     */
    async sendResponse(requestId, result, error, recipientPubkey) {
        if (!this.ws || !this.connected) {
            console.error('[BunkerService] Cannot send response: not connected');
            return;
        }

        const response = {
            id: requestId,
            result: result !== null ? result : undefined,
            error: error !== null ? error : undefined
        };

        // Encrypt response with NIP-04
        const privkeyHex = bytesToHex(this.userPrivkey);
        const encrypted = await nip04.encrypt(privkeyHex, recipientPubkey, JSON.stringify(response));

        // Create response event
        const event = {
            kind: 24133,
            pubkey: this.userPubkey,
            created_at: Math.floor(Date.now() / 1000),
            content: encrypted,
            tags: [['p', recipientPubkey]]
        };

        // Calculate event ID
        const serialized = JSON.stringify([
            0,
            event.pubkey,
            event.created_at,
            event.kind,
            event.tags,
            event.content
        ]);
        const hash = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(serialized));
        event.id = bytesToHex(new Uint8Array(hash));

        // Sign the event
        const sig = secp256k1.sign(hexToBytes(event.id), this.userPrivkey);
        event.sig = sig.toCompactHex();

        // Send to relay
        this.ws.send(JSON.stringify(['EVENT', event]));
        console.log('[BunkerService] Sent response for:', requestId);
    }

    /**
     * Check if the service is connected.
     */
    isConnected() {
        return this.connected;
    }

    /**
     * Get list of connected clients.
     */
    getConnectedClients() {
        return Array.from(this.connectedClients.entries()).map(([pubkey, info]) => ({
            pubkey,
            ...info
        }));
    }

    /**
     * Get request log.
     */
    getRequestLog() {
        return [...this.requestLog];
    }
}
