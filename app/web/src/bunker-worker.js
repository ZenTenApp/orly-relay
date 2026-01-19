/**
 * BunkerWorker - Web Worker for persistent NIP-46 bunker service
 *
 * Runs in a separate thread to maintain WebSocket connection
 * regardless of UI component lifecycle.
 */

import { nip04 } from 'nostr-tools';
import { bytesToHex, hexToBytes } from '@noble/hashes/utils';
import { secp256k1 } from '@noble/curves/secp256k1';

// State
let ws = null;
let connected = false;
let userPubkey = null;
let userPrivkey = null;
let relayUrl = null;
let subscriptionId = null;
let heartbeatInterval = null;
let allowedSecrets = new Set();
let connectedClients = new Map();

// NIP-46 methods
const NIP46_METHOD = {
    CONNECT: 'connect',
    GET_PUBLIC_KEY: 'get_public_key',
    SIGN_EVENT: 'sign_event',
    NIP04_ENCRYPT: 'nip04_encrypt',
    NIP04_DECRYPT: 'nip04_decrypt',
    PING: 'ping'
};

function generateRandomHex(bytes = 16) {
    const arr = new Uint8Array(bytes);
    crypto.getRandomValues(arr);
    return bytesToHex(arr);
}

function postStatus(status, data = {}) {
    self.postMessage({ type: 'status', status, ...data });
}

function postError(error) {
    self.postMessage({ type: 'error', error });
}

function postClientsUpdate() {
    const clients = Array.from(connectedClients.entries()).map(([pubkey, info]) => ({
        pubkey,
        ...info
    }));
    self.postMessage({ type: 'clients', clients });
}

async function connect() {
    if (connected || !relayUrl || !userPubkey || !userPrivkey) {
        postError('Missing configuration or already connected');
        return;
    }

    return new Promise((resolve, reject) => {
        let wsUrl = relayUrl;
        if (wsUrl.startsWith('http://')) {
            wsUrl = 'ws://' + wsUrl.slice(7);
        } else if (wsUrl.startsWith('https://')) {
            wsUrl = 'wss://' + wsUrl.slice(8);
        } else if (!wsUrl.startsWith('ws://') && !wsUrl.startsWith('wss://')) {
            wsUrl = 'wss://' + wsUrl;
        }

        console.log('[BunkerWorker] Connecting to:', wsUrl);

        ws = new WebSocket(wsUrl);

        const timeout = setTimeout(() => {
            ws.close();
            postError('Connection timeout');
            reject(new Error('Connection timeout'));
        }, 10000);

        ws.onopen = () => {
            clearTimeout(timeout);
            connected = true;
            console.log('[BunkerWorker] Connected to relay');

            // Subscribe to NIP-46 events
            subscriptionId = generateRandomHex(8);
            const sub = JSON.stringify([
                'REQ',
                subscriptionId,
                {
                    kinds: [24133],
                    '#p': [userPubkey],
                    since: Math.floor(Date.now() / 1000) - 60
                }
            ]);
            ws.send(sub);

            startHeartbeat();
            postStatus('connected');
            resolve();
        };

        ws.onerror = (error) => {
            clearTimeout(timeout);
            console.error('[BunkerWorker] WebSocket error:', error);
            postError('WebSocket error');
            reject(new Error('WebSocket error'));
        };

        ws.onclose = () => {
            connected = false;
            ws = null;
            stopHeartbeat();
            console.log('[BunkerWorker] Disconnected from relay');
            postStatus('disconnected');
        };

        ws.onmessage = (event) => {
            handleMessage(event.data);
        };
    });
}

function disconnect() {
    stopHeartbeat();
    if (ws) {
        if (subscriptionId) {
            ws.send(JSON.stringify(['CLOSE', subscriptionId]));
        }
        ws.close();
        ws = null;
    }
    connected = false;
    connectedClients.clear();
    postStatus('disconnected');
    postClientsUpdate();
}

function startHeartbeat(intervalMs = 30000) {
    stopHeartbeat();
    heartbeatInterval = setInterval(() => {
        if (ws && ws.readyState === WebSocket.OPEN) {
            const sub = JSON.stringify([
                'REQ',
                subscriptionId,
                {
                    kinds: [24133],
                    '#p': [userPubkey],
                    since: Math.floor(Date.now() / 1000) - 60
                }
            ]);
            ws.send(sub);
        }
    }, intervalMs);
}

function stopHeartbeat() {
    if (heartbeatInterval) {
        clearInterval(heartbeatInterval);
        heartbeatInterval = null;
    }
}

async function handleMessage(data) {
    try {
        const msg = JSON.parse(data);
        if (!Array.isArray(msg)) return;

        const [type, ...rest] = msg;

        if (type === 'EVENT') {
            const [, event] = rest;
            if (event.kind === 24133) {
                await handleNIP46Request(event);
            }
        } else if (type === 'OK') {
            console.log('[BunkerWorker] Event published:', rest[0]?.substring(0, 8));
        } else if (type === 'NOTICE') {
            console.warn('[BunkerWorker] Relay notice:', rest[0]);
        }
    } catch (err) {
        console.error('[BunkerWorker] Failed to parse message:', err);
    }
}

async function handleNIP46Request(event) {
    try {
        const privkeyHex = bytesToHex(userPrivkey);
        const decrypted = await nip04.decrypt(privkeyHex, event.pubkey, event.content);
        const request = JSON.parse(decrypted);

        console.log('[BunkerWorker] Received request:', request.method, 'from:', event.pubkey.substring(0, 8));

        // Log to main thread
        self.postMessage({
            type: 'request',
            method: request.method,
            from: event.pubkey,
            timestamp: Date.now()
        });

        let result = null;
        let error = null;

        try {
            switch (request.method) {
                case NIP46_METHOD.CONNECT:
                    result = await handleConnect(request, event.pubkey);
                    break;
                case NIP46_METHOD.GET_PUBLIC_KEY:
                    result = handleGetPublicKey(event.pubkey);
                    break;
                case NIP46_METHOD.SIGN_EVENT:
                    result = await handleSignEvent(request, event.pubkey);
                    break;
                case NIP46_METHOD.NIP04_ENCRYPT:
                    result = await handleNip04Encrypt(request, event.pubkey);
                    break;
                case NIP46_METHOD.NIP04_DECRYPT:
                    result = await handleNip04Decrypt(request, event.pubkey);
                    break;
                case NIP46_METHOD.PING:
                    result = 'pong';
                    break;
                default:
                    error = `Unknown method: ${request.method}`;
            }
        } catch (err) {
            console.error('[BunkerWorker] Error handling request:', err);
            error = err.message;
        }

        await sendResponse(request.id, result, error, event.pubkey);

    } catch (err) {
        console.error('[BunkerWorker] Failed to handle NIP-46 request:', err);
    }
}

async function handleConnect(request, senderPubkey) {
    const [clientPubkey, secret] = request.params;

    if (allowedSecrets.size > 0) {
        if (!secret || !allowedSecrets.has(secret)) {
            throw new Error('Invalid or missing connection secret');
        }
    }

    connectedClients.set(senderPubkey, {
        clientPubkey: clientPubkey || senderPubkey,
        connectedAt: Date.now(),
        lastActivity: Date.now()
    });

    console.log('[BunkerWorker] Client connected:', senderPubkey.substring(0, 8));
    postClientsUpdate();

    return 'ack';
}

function handleGetPublicKey(senderPubkey) {
    if (connectedClients.has(senderPubkey)) {
        connectedClients.get(senderPubkey).lastActivity = Date.now();
    }
    return userPubkey;
}

async function handleSignEvent(request, senderPubkey) {
    if (!connectedClients.has(senderPubkey)) {
        throw new Error('Not connected');
    }

    connectedClients.get(senderPubkey).lastActivity = Date.now();

    const [eventJson] = request.params;
    const event = JSON.parse(eventJson);

    if (event.pubkey && event.pubkey !== userPubkey) {
        throw new Error('Event pubkey does not match signer pubkey');
    }

    event.pubkey = userPubkey;

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

    const sig = secp256k1.sign(hexToBytes(event.id), userPrivkey);
    event.sig = sig.toCompactHex();

    console.log('[BunkerWorker] Signed event:', event.id.substring(0, 8), 'kind:', event.kind);

    return JSON.stringify(event);
}

async function handleNip04Encrypt(request, senderPubkey) {
    if (!connectedClients.has(senderPubkey)) {
        throw new Error('Not connected');
    }

    connectedClients.get(senderPubkey).lastActivity = Date.now();

    const [pubkey, plaintext] = request.params;
    const privkeyHex = bytesToHex(userPrivkey);
    return await nip04.encrypt(privkeyHex, pubkey, plaintext);
}

async function handleNip04Decrypt(request, senderPubkey) {
    if (!connectedClients.has(senderPubkey)) {
        throw new Error('Not connected');
    }

    connectedClients.get(senderPubkey).lastActivity = Date.now();

    const [pubkey, ciphertext] = request.params;
    const privkeyHex = bytesToHex(userPrivkey);
    return await nip04.decrypt(privkeyHex, pubkey, ciphertext);
}

async function sendResponse(requestId, result, error, recipientPubkey) {
    if (!ws || !connected) {
        console.error('[BunkerWorker] Cannot send response: not connected');
        return;
    }

    const response = {
        id: requestId,
        result: result !== null ? result : undefined,
        error: error !== null ? error : undefined
    };

    const privkeyHex = bytesToHex(userPrivkey);
    const encrypted = await nip04.encrypt(privkeyHex, recipientPubkey, JSON.stringify(response));

    const event = {
        kind: 24133,
        pubkey: userPubkey,
        created_at: Math.floor(Date.now() / 1000),
        content: encrypted,
        tags: [['p', recipientPubkey]]
    };

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

    const sig = secp256k1.sign(hexToBytes(event.id), userPrivkey);
    event.sig = sig.toCompactHex();

    ws.send(JSON.stringify(['EVENT', event]));
    console.log('[BunkerWorker] Sent response for:', requestId);
}

// Message handler from main thread
self.onmessage = async (event) => {
    const { type, ...data } = event.data;

    switch (type) {
        case 'configure':
            userPubkey = data.userPubkey;
            userPrivkey = data.userPrivkey ? hexToBytes(data.userPrivkey) : null;
            relayUrl = data.relayUrl;
            if (data.secrets) {
                allowedSecrets = new Set(data.secrets);
            }
            console.log('[BunkerWorker] Configured for pubkey:', userPubkey?.substring(0, 8));
            break;

        case 'connect':
            try {
                await connect();
            } catch (err) {
                postError(err.message);
            }
            break;

        case 'disconnect':
            disconnect();
            break;

        case 'addSecret':
            allowedSecrets.add(data.secret);
            break;

        case 'removeSecret':
            allowedSecrets.delete(data.secret);
            break;

        case 'getStatus':
            postStatus(connected ? 'connected' : 'disconnected');
            postClientsUpdate();
            break;

        default:
            console.warn('[BunkerWorker] Unknown message type:', type);
    }
};

console.log('[BunkerWorker] Worker initialized');
