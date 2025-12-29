/**
 * Cashu Token Client
 *
 * Manages Cashu access tokens for bunker authentication.
 * Handles token issuance using blind signature protocol.
 *
 * Token flow:
 * 1. Generate random secret and blinding factor
 * 2. Compute blinded message B_ = hash_to_curve(secret) + r*G
 * 3. Submit B_ to mint with NIP-98 auth
 * 4. Receive blinded signature C_
 * 5. Unblind: C = C_ - r*K (where K is mint's pubkey)
 * 6. Encode token for transmission
 */

import { secp256k1 } from '@noble/curves/secp256k1';
import { sha256 } from '@noble/hashes/sha256';
import { bytesToHex, hexToBytes } from '@noble/hashes/utils';

// Token scopes matching ORLY's token.Scope
export const TokenScope = {
    RELAY: 'relay',
    NIP46: 'nip46',
    BLOSSOM: 'blossom',
    API: 'api'
};

/**
 * Convert bytes to big-endian number.
 */
function bytesToNumberBE(bytes) {
    let n = 0n;
    for (const b of bytes) {
        n = (n << 8n) | BigInt(b);
    }
    return n;
}

/**
 * Hash a message to a point on secp256k1 using try-and-increment.
 * Matches ORLY's Go implementation exactly.
 */
function hashToCurve(message) {
    const domainSeparator = new TextEncoder().encode('Secp256k1_HashToCurve_Cashu_');
    const msgHash = sha256(new Uint8Array([...domainSeparator, ...message]));

    // Try incrementing counter until we get a valid point
    for (let counter = 0; counter < 65536; counter++) {
        // 4-byte little-endian counter
        const counterBytes = new Uint8Array(4);
        new DataView(counterBytes.buffer).setUint32(0, counter, true);

        const toHash = new Uint8Array([...msgHash, ...counterBytes]);
        const hash = sha256(toHash);

        // Try 0x02 prefix (even Y coordinate)
        const compressed = new Uint8Array([0x02, ...hash]);
        try {
            const point = secp256k1.ProjectivePoint.fromHex(compressed);
            if (!point.equals(secp256k1.ProjectivePoint.ZERO)) {
                return compressed;
            }
        } catch {
            // Not a valid point, continue
        }
    }

    throw new Error('Failed to hash to curve after 65536 attempts');
}

/**
 * Create a blinded message from a secret.
 * B_ = Y + r*G where Y = hash_to_curve(secret)
 */
function blind(secret) {
    // Generate random blinding factor r
    const r = secp256k1.utils.randomPrivateKey();

    // Y = hash_to_curve(secret)
    const Y = secp256k1.ProjectivePoint.fromHex(hashToCurve(secret));

    // r*G
    const rG = secp256k1.ProjectivePoint.BASE.multiply(bytesToNumberBE(r));

    // B_ = Y + r*G
    const B_ = Y.add(rG);

    return {
        B_: B_.toRawBytes(true), // Compressed format
        secret,
        r
    };
}

/**
 * Unblind the signature to get the final signature.
 * C = C_ - r*K where K is the mint's public key
 */
function unblind(C_, r, K) {
    const C_point = secp256k1.ProjectivePoint.fromHex(C_);
    const K_point = secp256k1.ProjectivePoint.fromHex(K);

    // r*K
    const rK = K_point.multiply(bytesToNumberBE(r));

    // C = C_ - r*K
    const C = C_point.subtract(rK);

    return C.toRawBytes(true);
}

/**
 * Encode a token to the Cashu format (cashuA prefix + base64url).
 */
export function encodeToken(token) {
    const tokenData = {
        k: token.keysetId,
        s: bytesToHex(token.secret),
        c: bytesToHex(token.signature),
        p: bytesToHex(token.pubkey),
        e: token.expiry,
        sc: token.scope,
        kinds: token.kinds,
        kind_ranges: token.kindRanges
    };

    const json = JSON.stringify(tokenData);
    // Use base64url encoding
    const base64 = btoa(json)
        .replace(/\+/g, '-')
        .replace(/\//g, '_')
        .replace(/=+$/, '');

    return 'cashuA' + base64;
}

/**
 * Decode a token from the Cashu format.
 */
export function decodeToken(encoded) {
    if (!encoded.startsWith('cashuA')) {
        throw new Error('Invalid token prefix, expected cashuA');
    }

    const base64url = encoded.slice(6);
    // Convert base64url to base64
    let base64 = base64url.replace(/-/g, '+').replace(/_/g, '/');
    // Add padding if needed
    while (base64.length % 4 !== 0) {
        base64 += '=';
    }

    const json = atob(base64);
    const data = JSON.parse(json);

    return {
        keysetId: data.k,
        secret: hexToBytes(data.s),
        signature: hexToBytes(data.c),
        pubkey: hexToBytes(data.p),
        expiry: data.e,
        scope: data.sc,
        kinds: data.kinds,
        kindRanges: data.kind_ranges
    };
}

/**
 * Request a new token from the mint.
 * @param {string} mintUrl - The mint URL (e.g., https://relay.example.com)
 * @param {string} scope - Token scope (relay, nip46, blossom, api)
 * @param {Uint8Array} userPubkey - User's public key (32 bytes)
 * @param {Function} signHttpAuth - Function to create NIP-98 auth header
 * @param {number[]} [kinds] - Permitted event kinds
 * @param {[number, number][]} [kindRanges] - Permitted kind ranges
 * @returns {Promise<Object>} - The token object
 */
export async function requestToken(mintUrl, scope, userPubkey, signHttpAuth, kinds, kindRanges) {
    // Generate secret and blind it
    const secret = crypto.getRandomValues(new Uint8Array(32));
    const blindResult = blind(secret);

    // Create request
    const requestBody = {
        blinded_message: bytesToHex(blindResult.B_),
        scope,
        kinds,
        kind_ranges: kindRanges
    };

    // Get NIP-98 auth header
    const authUrl = `${mintUrl}/cashu/mint`;
    const authHeader = await signHttpAuth(authUrl, 'POST');

    // Submit to mint
    const response = await fetch(authUrl, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': authHeader
        },
        body: JSON.stringify(requestBody)
    });

    if (!response.ok) {
        const error = await response.text();
        throw new Error(`Mint request failed: ${error}`);
    }

    const result = await response.json();

    // Unblind the signature
    const C_ = hexToBytes(result.blinded_signature);
    const K = hexToBytes(result.mint_pubkey);
    const signature = unblind(C_, blindResult.r, K);

    return {
        keysetId: result.keyset_id,
        secret: blindResult.secret,
        signature,
        pubkey: userPubkey,
        expiry: result.expiry,
        scope,
        kinds,
        kindRanges
    };
}

/**
 * Check if relay requires CAT and fetch mint info.
 * @param {string} relayUrl - The relay URL
 * @returns {Promise<Object|null>} - Mint info or null if CAT not required
 */
export async function getMintInfo(relayUrl) {
    // Convert to HTTP URL
    let mintUrl = relayUrl
        .replace('wss://', 'https://')
        .replace('ws://', 'http://')
        .replace(/\/$/, '');

    try {
        const response = await fetch(`${mintUrl}/cashu/info`);
        if (!response.ok) {
            return null;
        }
        const info = await response.json();
        info.mintUrl = mintUrl;
        return info;
    } catch {
        return null;
    }
}
