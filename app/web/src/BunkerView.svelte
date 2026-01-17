<script>
    import { createEventDispatcher, onMount } from "svelte";
    import QRCode from "qrcode";
    import { getBunkerInfo, createNIP98Auth } from "./api.js";
    import { requestToken, encodeToken, TokenScope, getMintInfo } from "./cashu-client.js";
    import { hexToBytes, bytesToHex } from "@noble/hashes/utils";
    import {
        bunkerServiceActive,
        bunkerServiceCatToken,
        bunkerClientTokens,
        bunkerSelectedTokenId,
        bunkerConnectedClients,
        configureBunkerWorker,
        connectBunkerWorker,
        disconnectBunkerWorker,
        addBunkerSecret,
        requestBunkerStatus,
        resetBunkerState
    } from "./stores.js";

    export let isLoggedIn = false;
    export let userPubkey = "";
    export let userSigner = null;
    export let userPrivkey = null; // User's private key for signing (Uint8Array)
    export let currentEffectiveRole = "";

    const dispatch = createEventDispatcher();

    // Local UI state
    let bunkerInfo = null;
    let isLoading = false;
    let error = "";
    let clientQrDataUrl = "";
    let signerQrDataUrl = "";
    let copiedItem = "";
    let bunkerSecret = "";
    let isStartingService = false;

    // Subscribe to global bunker stores
    $: isServiceActive = $bunkerServiceActive;
    $: clientTokens = $bunkerClientTokens;
    $: selectedTokenId = $bunkerSelectedTokenId;
    $: connectedClients = $bunkerConnectedClients;

    // Two-word name generator
    const adjectives = ["brave", "calm", "clever", "cosmic", "cozy", "daring", "eager", "fancy", "gentle", "happy", "jolly", "keen", "lively", "merry", "nimble", "peppy", "quick", "rustic", "shiny", "swift", "tender", "vivid", "witty", "zesty"];
    const nouns = ["badger", "bunny", "coral", "dolphin", "falcon", "gecko", "heron", "iguana", "jaguar", "koala", "lemur", "mango", "narwhal", "otter", "panda", "quail", "rabbit", "salmon", "turtle", "urchin", "viper", "walrus", "yak", "zebra"];

    function generateTokenName() {
        const adj = adjectives[Math.floor(Math.random() * adjectives.length)];
        const noun = nouns[Math.floor(Math.random() * nouns.length)];
        return `${adj}-${noun}`;
    }

    function generateTokenId() {
        return crypto.randomUUID().split('-')[0];
    }

    // Add a new client token
    async function addClientToken(mintInfo, signHttpAuth) {
        const token = await requestToken(
            mintInfo.mintUrl,
            TokenScope.NIP46,
            hexToBytes(userPubkey),
            signHttpAuth,
            [24133]
        );
        const encoded = encodeToken(token);
        const id = generateTokenId();
        const newToken = {
            id,
            name: generateTokenName(),
            token,
            encoded,
            createdAt: Date.now(),
            isExpanded: false
        };
        bunkerClientTokens.update(tokens => [...tokens, newToken]);
        // Select the new token if none selected
        if (!$bunkerSelectedTokenId) {
            bunkerSelectedTokenId.set(id);
        }
        console.log(`Client token "${newToken.name}" created, expires:`, new Date(token.expiry * 1000).toISOString());
        return newToken;
    }

    // Add a new token (called from UI)
    async function handleAddToken() {
        if (!bunkerInfo?.cashu_enabled) return;

        try {
            const mintInfo = await getMintInfo(bunkerInfo.relay_url);
            if (!mintInfo) return;

            const signHttpAuth = async (url, method) => {
                const header = await createNIP98Auth(userSigner, userPubkey, method, url);
                return `Nostr ${header}`;
            };

            await addClientToken(mintInfo, signHttpAuth);
            // Regenerate QR for newly selected token
            await generateQRCodes();
        } catch (err) {
            console.error("Failed to add token:", err);
            error = err.message || "Failed to add token";
        }
    }

    // Revoke/remove a client token
    function revokeToken(tokenId) {
        clientTokens = clientTokens.filter(t => t.id !== tokenId);
        // If we removed the selected token, select another
        if (selectedTokenId === tokenId) {
            selectedTokenId = clientTokens.length > 0 ? clientTokens[0].id : null;
        }
        generateQRCodes();
    }

    // Toggle token details expansion
    function toggleTokenExpand(tokenId) {
        clientTokens = clientTokens.map(t =>
            t.id === tokenId ? { ...t, isExpanded: !t.isExpanded } : t
        );
    }

    // Update token name
    function updateTokenName(tokenId, newName) {
        clientTokens = clientTokens.map(t =>
            t.id === tokenId ? { ...t, name: newName } : t
        );
    }

    // Generate QR code for a specific token
    async function generateTokenQR(token) {
        if (!bunkerInfo || !userPubkey) return null;
        const url = `bunker://${userPubkey}?relay=${encodeURIComponent(bunkerInfo.relay_url)}${bunkerSecret ? `&secret=${bunkerSecret}` : ''}&cat=${token.encoded}`;
        return await QRCode.toDataURL(url, {
            width: 200,
            margin: 2,
            color: { dark: "#000000", light: "#ffffff" }
        });
    }

    $: canAccess = isLoggedIn && userPubkey && (
        currentEffectiveRole === "write" ||
        currentEffectiveRole === "admin" ||
        currentEffectiveRole === "owner"
    );

    // Generate bunker URLs when bunkerInfo and userPubkey are available
    // Get selected token for the bunker URL
    $: selectedToken = clientTokens.find(t => t.id === selectedTokenId);
    $: clientBunkerURL = bunkerInfo && userPubkey && selectedToken ?
        `bunker://${userPubkey}?relay=${encodeURIComponent(bunkerInfo.relay_url)}${bunkerSecret ? `&secret=${bunkerSecret}` : ''}&cat=${selectedToken.encoded}` : "";

    $: signerBunkerURL = bunkerInfo ?
        `nostr+connect://${bunkerInfo.relay_url}` : "";

    onMount(async () => {
        await loadBunkerInfo();
        // Request current status from worker (in case it's already running)
        requestBunkerStatus();
    });

    // Note: No onDestroy cleanup - worker persists across component mounts

    // Start the bunker service (via Web Worker)
    async function startBunkerService() {
        // Prevent starting if already active or starting
        if (isServiceActive || isStartingService) {
            console.log("Service already active or starting, ignoring");
            return;
        }

        if (!userPrivkey || !userPubkey || !bunkerInfo) {
            error = "Missing private key or bunker info";
            return;
        }

        isStartingService = true;
        error = "";

        try {
            let serviceCatTokenEncoded = null;

            // Check if CAT is required and mint tokens
            if (bunkerInfo.cashu_enabled) {
                console.log("CAT required, minting tokens...");
                const mintInfo = await getMintInfo(bunkerInfo.relay_url);
                if (mintInfo) {
                    // Create NIP-98 auth function
                    const signHttpAuth = async (url, method) => {
                        const header = await createNIP98Auth(userSigner, userPubkey, method, url);
                        return `Nostr ${header}`;
                    };

                    // 1. Token for worker's relay connection
                    const serviceCatToken = await requestToken(
                        mintInfo.mintUrl,
                        TokenScope.NIP46,
                        hexToBytes(userPubkey),
                        signHttpAuth,
                        [24133]
                    );
                    serviceCatTokenEncoded = encodeToken(serviceCatToken);
                    bunkerServiceCatToken.set(serviceCatToken);
                    console.log("Service CAT token acquired, expires:", new Date(serviceCatToken.expiry * 1000).toISOString());

                    // 2. Create first client token
                    await addClientToken(mintInfo, signHttpAuth);
                }
            }

            // Configure the worker with user credentials
            const privkeyHex = userPrivkey instanceof Uint8Array ? bytesToHex(userPrivkey) : userPrivkey;
            configureBunkerWorker({
                userPubkey,
                userPrivkey: privkeyHex,
                relayUrl: bunkerInfo.relay_url,
                catTokenEncoded: serviceCatTokenEncoded,
                secrets: bunkerSecret ? [bunkerSecret] : []
            });

            // Connect the worker
            connectBunkerWorker();

            // Regenerate QR codes with CAT token
            await generateQRCodes();

            console.log("Bunker worker started successfully");
        } catch (err) {
            console.error("Failed to start bunker service:", err);
            error = err.message || "Failed to start bunker service";
            resetBunkerState();
        } finally {
            isStartingService = false;
        }
    }

    // Stop the bunker service (via Web Worker)
    function stopBunkerService() {
        resetBunkerState();
        // Regenerate QR codes without CAT token
        generateQRCodes();
    }

    async function loadBunkerInfo() {
        isLoading = true;
        error = "";

        try {
            bunkerInfo = await getBunkerInfo();

            // Generate a random secret for secure connection
            if (!bunkerSecret) {
                bunkerSecret = generateSecret();
            }

            // Generate QR codes
            await generateQRCodes();
        } catch (err) {
            console.error("Error loading bunker info:", err);
            error = err.message || "Failed to load bunker information";
        } finally {
            isLoading = false;
        }
    }

    function generateSecret() {
        const array = new Uint8Array(16);
        crypto.getRandomValues(array);
        return Array.from(array, b => b.toString(16).padStart(2, '0')).join('');
    }

    async function regenerateSecret() {
        bunkerSecret = generateSecret();
        await generateQRCodes();
    }

    async function generateQRCodes() {
        if (clientBunkerURL) {
            clientQrDataUrl = await QRCode.toDataURL(clientBunkerURL, {
                width: 280,
                margin: 2,
                color: { dark: "#000000", light: "#ffffff" }
            });
        }

        if (signerBunkerURL) {
            signerQrDataUrl = await QRCode.toDataURL(signerBunkerURL, {
                width: 280,
                margin: 2,
                color: { dark: "#000000", light: "#ffffff" }
            });
        }
    }

    // Regenerate QR codes when URLs change
    $: if (clientBunkerURL || signerBunkerURL) {
        generateQRCodes();
    }

    function copyToClipboard(text, label) {
        navigator.clipboard.writeText(text);
        copiedItem = label;
        setTimeout(() => {
            copiedItem = "";
        }, 2000);
    }

    function openLoginModal() {
        dispatch("openLoginModal");
    }
</script>

{#if !bunkerInfo?.available}
    <div class="bunker-view">
        <div class="unavailable-message">
            <h3>Remote Signing Not Available</h3>
            <p>This relay does not have bunker mode enabled, or ACL mode is set to "none".</p>
            <p class="hint">Remote signing requires the relay operator to enable ACL mode "follows" or "managed".</p>
        </div>
    </div>
{:else if canAccess}
    <div class="bunker-view">
        <div class="header-section">
            <h3>Remote Signing (NIP-46 Bunker)</h3>
            <button class="refresh-btn" on:click={loadBunkerInfo} disabled={isLoading}>
                {isLoading ? "Loading..." : "Refresh"}
            </button>
        </div>

        {#if error}
            <div class="error-message">{error}</div>
        {/if}

        {#if bunkerInfo?.cashu_enabled && bunkerInfo?.acl_mode !== "none"}
            <div class="cat-warning">
                <strong>CAT Required:</strong> This relay requires Cashu Access Tokens (CAT) for bunker connections.
                Your client must support CAT authentication or connections will be rejected.
            </div>
        {/if}

        {#if isLoading && !bunkerInfo}
            <div class="loading">Loading bunker information...</div>
        {:else if bunkerInfo}
            <div class="instructions">
                <p><strong>How it works:</strong> Start the bunker service to allow remote apps (like Smesh) to request signatures from your ORLY account.
                Share the QR code or bunker URL with your client app.</p>
            </div>

            <!-- Service Control -->
            <div class="service-control">
                <div class="service-header">
                    <h4>Bunker Service</h4>
                    <div class="service-status" class:active={isServiceActive}>
                        <span class="status-dot"></span>
                        {isServiceActive ? 'Active' : 'Inactive'}
                    </div>
                </div>

                {#if !userPrivkey}
                    <div class="no-privkey-warning">
                        Bunker service requires nsec login. Please log in with your private key to enable remote signing.
                    </div>
                {:else}
                    <div class="service-actions">
                        {#if isServiceActive}
                            <button class="stop-btn" on:click={stopBunkerService}>
                                Stop Service
                            </button>
                        {:else}
                            <button class="start-btn" on:click={startBunkerService} disabled={isStartingService}>
                                {isStartingService ? 'Starting...' : 'Start Service'}
                            </button>
                        {/if}
                    </div>

                    {#if isServiceActive && connectedClients.length > 0}
                        <div class="connected-clients">
                            <h5>Connected Clients ({connectedClients.length})</h5>
                            {#each connectedClients as client}
                                <div class="client-entry">
                                    <code>{client.pubkey.substring(0, 16)}...</code>
                                    <span class="client-time">Connected {new Date(client.connectedAt).toLocaleTimeString()}</span>
                                </div>
                            {/each}
                        </div>
                    {/if}

                {/if}
            </div>

            <!-- Client Tokens Table - show if tokens exist, even if temporarily disconnected -->
            {#if clientTokens.length > 0}
                <div class="tokens-section">
                    <div class="tokens-header">
                        <h4>Client Tokens</h4>
                        <button class="add-token-btn" on:click={handleAddToken}>+ Add Token</button>
                    </div>
                    <p class="tokens-desc">Each device/app gets its own token. Tokens can be individually revoked.</p>

                    <div class="tokens-table">
                        {#each clientTokens as tokenEntry (tokenEntry.id)}
                            <div class="token-row" class:expanded={tokenEntry.isExpanded}>
                                <div class="token-main" on:click={() => toggleTokenExpand(tokenEntry.id)} on:keypress={(e) => e.key === 'Enter' && toggleTokenExpand(tokenEntry.id)} role="button" tabindex="0">
                                    <span class="expand-icon">{tokenEntry.isExpanded ? '▼' : '▶'}</span>
                                    <input
                                        type="text"
                                        class="token-name-input"
                                        value={tokenEntry.name}
                                        on:input={(e) => updateTokenName(tokenEntry.id, e.target.value)}
                                        on:click|stopPropagation
                                        placeholder="Token name"
                                    />
                                    <span class="token-created">
                                        {new Date(tokenEntry.createdAt).toLocaleDateString()}
                                    </span>
                                    <span class="token-expiry">
                                        Expires: {new Date(tokenEntry.token.expiry * 1000).toLocaleDateString()}
                                    </span>
                                    <button
                                        class="revoke-btn"
                                        on:click|stopPropagation={() => revokeToken(tokenEntry.id)}
                                        title="Revoke this token"
                                    >
                                        Revoke
                                    </button>
                                </div>

                                {#if tokenEntry.isExpanded}
                                    <div class="token-details">
                                        {#await generateTokenQR(tokenEntry)}
                                            <div class="qr-placeholder small">Loading QR...</div>
                                        {:then qrDataUrl}
                                            <div class="token-detail-content">
                                                <div
                                                    class="qr-container small clickable"
                                                    on:click={() => {
                                                        const url = `bunker://${userPubkey}?relay=${encodeURIComponent(bunkerInfo.relay_url)}${bunkerSecret ? `&secret=${bunkerSecret}` : ''}&cat=${tokenEntry.encoded}`;
                                                        copyToClipboard(url, tokenEntry.id);
                                                    }}
                                                    on:keypress={(e) => {
                                                        if (e.key === 'Enter') {
                                                            const url = `bunker://${userPubkey}?relay=${encodeURIComponent(bunkerInfo.relay_url)}${bunkerSecret ? `&secret=${bunkerSecret}` : ''}&cat=${tokenEntry.encoded}`;
                                                            copyToClipboard(url, tokenEntry.id);
                                                        }
                                                    }}
                                                    role="button"
                                                    tabindex="0"
                                                    title="Click to copy bunker URL"
                                                >
                                                    <img src={qrDataUrl} alt="Token QR Code" class="qr-code small" />
                                                    <div class="qr-overlay" class:visible={copiedItem === tokenEntry.id}>
                                                        Copied!
                                                    </div>
                                                </div>
                                                <div class="token-info">
                                                    <div class="info-item">
                                                        <span class="label">Created:</span>
                                                        <span>{new Date(tokenEntry.createdAt).toLocaleString()}</span>
                                                    </div>
                                                    <div class="info-item">
                                                        <span class="label">Expires:</span>
                                                        <span>{new Date(tokenEntry.token.expiry * 1000).toLocaleString()}</span>
                                                    </div>
                                                    <div class="info-item url-item">
                                                        <span class="label">Bunker URL:</span>
                                                        <code class="bunker-url small">{`bunker://${userPubkey}?relay=${encodeURIComponent(bunkerInfo.relay_url)}${bunkerSecret ? `&secret=${bunkerSecret}` : ''}&cat=${tokenEntry.encoded}`}</code>
                                                    </div>
                                                    <div class="copy-hint">Click QR code to copy URL</div>
                                                </div>
                                            </div>
                                        {:catch}
                                            <div class="error-message">Failed to generate QR</div>
                                        {/await}
                                    </div>
                                {/if}
                            </div>
                        {/each}
                    </div>
                </div>
            {/if}

            <!-- Connection Info -->
            <div class="connection-info">
                <h4>Connection Details</h4>
                <div class="info-row">
                    <span class="label">Relay:</span>
                    <code>{bunkerInfo.relay_url}</code>
                    <button class="copy-btn" on:click={() => copyToClipboard(bunkerInfo.relay_url, "relay")}>
                        {copiedItem === "relay" ? "Copied!" : "Copy"}
                    </button>
                </div>
                <div class="info-row">
                    <span class="label">Your npub:</span>
                    <code class="npub">{userPubkey}</code>
                </div>
                <div class="info-row">
                    <span class="label">Secret:</span>
                    <code class="secret">{bunkerSecret}</code>
                    <button class="copy-btn" on:click={regenerateSecret}>Regenerate</button>
                </div>
            </div>
        {/if}
    </div>
{:else if isLoggedIn}
    <div class="bunker-view">
        <div class="access-denied">
            <h3>Access Denied</h3>
            <p>You need write access to use remote signing. Your current access level: <strong>{currentEffectiveRole || "read-only"}</strong></p>
        </div>
    </div>
{:else}
    <div class="login-prompt">
        <p>Please log in to access remote signing.</p>
        <button class="login-btn" on:click={openLoginModal}>Log In</button>
    </div>
{/if}

<style>
    .bunker-view {
        padding: 1em;
        box-sizing: border-box;
    }

    .header-section {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 1em;
    }

    .header-section h3 {
        margin: 0;
        color: var(--text-color);
    }

    .refresh-btn {
        background-color: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
    }

    .refresh-btn:hover:not(:disabled) {
        background-color: var(--accent-hover-color);
    }

    .refresh-btn:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .error-message {
        background-color: var(--warning);
        color: var(--text-color);
        padding: 0.75em 1em;
        border-radius: 4px;
        margin-bottom: 1em;
    }

    .cat-warning {
        background-color: rgba(255, 193, 7, 0.15);
        border: 1px solid rgba(255, 193, 7, 0.5);
        color: var(--text-color);
        padding: 0.75em 1em;
        border-radius: 4px;
        margin-bottom: 1em;
        font-size: 0.95em;
    }

    .loading {
        text-align: center;
        padding: 2em;
        color: var(--text-color);
        opacity: 0.7;
    }

    .instructions {
        background-color: var(--card-bg);
        padding: 1em;
        border-radius: 6px;
        margin-bottom: 1.5em;
    }

    .instructions p {
        margin: 0;
        color: var(--text-color);
    }

    /* Service Control Styles */
    .service-control {
        background-color: var(--card-bg);
        padding: 1.25em;
        border-radius: 8px;
        margin-bottom: 1.5em;
    }

    .service-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 1em;
    }

    .service-header h4 {
        margin: 0;
        color: var(--text-color);
    }

    .service-status {
        display: flex;
        align-items: center;
        gap: 0.5em;
        font-size: 0.9em;
        color: var(--text-color);
        opacity: 0.7;
    }

    .service-status.active {
        opacity: 1;
        color: #4ade80;
    }

    .status-dot {
        width: 10px;
        height: 10px;
        border-radius: 50%;
        background-color: #6b7280;
    }

    .service-status.active .status-dot {
        background-color: #4ade80;
        box-shadow: 0 0 8px rgba(74, 222, 128, 0.5);
    }

    .service-actions {
        margin-bottom: 1em;
    }

    .start-btn, .stop-btn {
        padding: 0.75em 1.5em;
        border: none;
        border-radius: 6px;
        font-size: 1em;
        font-weight: 500;
        cursor: pointer;
        transition: background-color 0.2s;
    }

    .start-btn {
        background-color: #4ade80;
        color: #0a0a0a;
    }

    .start-btn:hover:not(:disabled) {
        background-color: #22c55e;
    }

    .start-btn:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .stop-btn {
        background-color: #ef4444;
        color: white;
    }

    .stop-btn:hover {
        background-color: #dc2626;
    }

    .no-privkey-warning {
        background-color: rgba(255, 193, 7, 0.15);
        border: 1px solid rgba(255, 193, 7, 0.5);
        color: var(--text-color);
        padding: 0.75em 1em;
        border-radius: 4px;
        font-size: 0.95em;
    }

    .connected-clients {
        margin-top: 1em;
        padding-top: 1em;
        border-top: 1px solid var(--border-color);
    }

    .connected-clients h5 {
        margin: 0 0 0.5em 0;
        color: var(--text-color);
        font-size: 0.9em;
    }

    .client-entry {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.5em;
        background-color: var(--bg-color);
        border-radius: 4px;
        margin-bottom: 0.5em;
    }

    .client-entry code {
        font-size: 0.85em;
    }

    .client-time {
        font-size: 0.8em;
        opacity: 0.7;
    }

    .cat-info {
        display: flex;
        align-items: center;
        gap: 1em;
        margin-top: 1em;
        padding-top: 1em;
        border-top: 1px solid var(--border-color);
    }

    .cat-badge {
        background-color: rgba(74, 222, 128, 0.2);
        color: #4ade80;
        padding: 0.25em 0.75em;
        border-radius: 4px;
        font-size: 0.85em;
        font-weight: 500;
    }

    .cat-expiry {
        font-size: 0.85em;
        opacity: 0.7;
    }

    .qr-sections {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
        gap: 1.5em;
        margin-bottom: 1.5em;
    }

    .qr-section {
        background-color: var(--card-bg);
        padding: 1.25em;
        border-radius: 8px;
    }

    .qr-section h4 {
        margin: 0 0 0.5em 0;
        color: var(--text-color);
    }

    .section-desc {
        margin: 0 0 1em 0;
        color: var(--text-color);
        opacity: 0.8;
        font-size: 0.95em;
    }

    .qr-container {
        display: flex;
        justify-content: center;
        margin: 1em 0;
        position: relative;
    }

    .qr-container.clickable {
        cursor: pointer;
        transition: transform 0.1s;
    }

    .qr-container.clickable:hover {
        transform: scale(1.02);
    }

    .qr-container.clickable:active {
        transform: scale(0.98);
    }

    .qr-code {
        border-radius: 8px;
        background: white;
        padding: 8px;
    }

    .qr-overlay {
        position: absolute;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        background-color: rgba(0, 0, 0, 0.85);
        color: #4ade80;
        padding: 0.75em 1.5em;
        border-radius: 8px;
        font-weight: 600;
        font-size: 1.1em;
        opacity: 0;
        transition: opacity 0.2s;
        pointer-events: none;
    }

    .qr-overlay.visible {
        opacity: 1;
    }

    .qr-placeholder {
        width: 280px;
        height: 280px;
        display: flex;
        align-items: center;
        justify-content: center;
        background-color: var(--bg-color);
        border-radius: 8px;
        color: var(--text-color);
        opacity: 0.5;
    }

    .url-display {
        text-align: center;
        margin-top: 0.5em;
    }

    .bunker-url {
        font-family: monospace;
        font-size: 0.75em;
        word-break: break-all;
        padding: 0.5em;
        background-color: var(--bg-color);
        border-radius: 4px;
        display: inline-block;
        max-width: 100%;
        color: var(--text-color);
    }

    .copy-hint {
        text-align: center;
        font-size: 0.8em;
        color: var(--text-color);
        opacity: 0.6;
        margin-top: 0.5em;
    }

    .connection-info {
        background-color: var(--card-bg);
        padding: 1.25em;
        border-radius: 8px;
        margin-bottom: 1.5em;
    }

    .connection-info h4 {
        margin: 0 0 1em 0;
        color: var(--text-color);
    }

    .info-row {
        display: flex;
        align-items: center;
        gap: 0.5em;
        margin-bottom: 0.75em;
        flex-wrap: wrap;
    }

    .info-row:last-child {
        margin-bottom: 0;
    }

    .label {
        color: var(--text-color);
        opacity: 0.7;
        min-width: 80px;
    }

    code {
        font-family: monospace;
        padding: 0.25em 0.5em;
        background-color: var(--bg-color);
        border-radius: 4px;
        color: var(--text-color);
        word-break: break-all;
    }

    .npub, .secret {
        font-size: 0.85em;
    }

    .copy-btn {
        padding: 0.3em 0.6em;
        background-color: var(--primary);
        color: var(--text-color);
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.8em;
    }

    .copy-btn:hover {
        background-color: var(--accent-hover-color);
    }

    .unavailable-message, .access-denied {
        text-align: center;
        padding: 2em;
        background-color: var(--card-bg);
        border-radius: 8px;
    }

    .unavailable-message h3, .access-denied h3 {
        margin: 0 0 0.5em 0;
        color: var(--text-color);
    }

    .unavailable-message p, .access-denied p {
        margin: 0.5em 0;
        color: var(--text-color);
        opacity: 0.8;
    }

    .hint {
        font-size: 0.9em;
        opacity: 0.6 !important;
    }

    .login-prompt {
        text-align: center;
        padding: 2em;
        background-color: var(--card-bg);
        border-radius: 8px;
        border: 1px solid var(--border-color);
        max-width: 32em;
        margin: 1em;
    }

    .login-prompt p {
        margin: 0 0 1.5rem 0;
        color: var(--text-color);
        font-size: 1.1rem;
    }

    .login-btn {
        background-color: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.75em 1.5em;
        border-radius: 4px;
        cursor: pointer;
        font-weight: bold;
        font-size: 0.9em;
    }

    .login-btn:hover {
        background-color: var(--accent-hover-color);
    }

    /* Token table styles */
    .tokens-section {
        background-color: var(--card-bg);
        padding: 1.25em;
        border-radius: 8px;
        margin-bottom: 1.5em;
    }

    .tokens-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 0.5em;
    }

    .tokens-header h4 {
        margin: 0;
        color: var(--text-color);
    }

    .tokens-desc {
        margin: 0 0 1em 0;
        font-size: 0.9em;
        color: var(--text-color);
        opacity: 0.7;
    }

    .add-token-btn {
        background-color: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.4em 0.8em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.85em;
    }

    .add-token-btn:hover {
        background-color: var(--accent-hover-color);
    }

    .tokens-table {
        display: flex;
        flex-direction: column;
        gap: 0.5em;
    }

    .token-row {
        background-color: var(--bg-color);
        border-radius: 6px;
        overflow: hidden;
    }

    .token-row.expanded {
        border: 1px solid var(--border-color);
    }

    .token-main {
        display: flex;
        align-items: center;
        gap: 0.75em;
        padding: 0.75em;
        cursor: pointer;
        transition: background-color 0.15s;
    }

    .token-main:hover {
        background-color: var(--card-bg);
    }

    .expand-icon {
        font-size: 0.7em;
        color: var(--text-color);
        opacity: 0.6;
        width: 1em;
    }

    .token-name-input {
        flex: 1;
        min-width: 100px;
        max-width: 180px;
        background-color: transparent;
        border: 1px solid transparent;
        border-radius: 4px;
        padding: 0.3em 0.5em;
        font-size: 0.95em;
        font-weight: 500;
        color: var(--text-color);
    }

    .token-name-input:hover {
        border-color: var(--border-color);
    }

    .token-name-input:focus {
        outline: none;
        border-color: var(--primary);
        background-color: var(--card-bg);
    }

    .token-created, .token-expiry {
        font-size: 0.8em;
        color: var(--text-color);
        opacity: 0.7;
    }

    .token-expiry {
        margin-left: auto;
    }

    .revoke-btn {
        background-color: #ef4444;
        color: white;
        border: none;
        padding: 0.3em 0.6em;
        border-radius: 4px;
        font-size: 0.8em;
        cursor: pointer;
    }

    .revoke-btn:hover {
        background-color: #dc2626;
    }

    .token-details {
        padding: 1em;
        border-top: 1px solid var(--border-color);
        background-color: var(--card-bg);
    }

    .token-detail-content {
        display: flex;
        gap: 1.5em;
        align-items: flex-start;
    }

    .qr-container.small {
        flex-shrink: 0;
    }

    .qr-code.small {
        width: 150px;
        height: 150px;
    }

    .qr-placeholder.small {
        width: 150px;
        height: 150px;
        font-size: 0.85em;
    }

    .token-info {
        flex: 1;
        display: flex;
        flex-direction: column;
        gap: 0.5em;
    }

    .info-item {
        display: flex;
        gap: 0.5em;
        font-size: 0.9em;
    }

    .info-item .label {
        color: var(--text-color);
        opacity: 0.7;
        min-width: 70px;
    }

    .info-item.url-item {
        flex-direction: column;
        gap: 0.25em;
    }

    .bunker-url.small {
        font-size: 0.7em;
        padding: 0.5em;
        word-break: break-all;
    }

    @media (max-width: 600px) {
        .qr-sections {
            grid-template-columns: 1fr;
        }

        .bunker-url {
            font-size: 0.65em;
        }

        .info-row {
            flex-direction: column;
            align-items: flex-start;
        }

        .token-main {
            flex-wrap: wrap;
            gap: 0.5em;
        }

        .token-name-input {
            order: 1;
            flex: 1 1 100%;
            max-width: none;
        }

        .expand-icon {
            order: 0;
        }

        .token-created {
            order: 2;
        }

        .token-expiry {
            order: 3;
            margin-left: 0;
        }

        .revoke-btn {
            order: 4;
        }

        .token-detail-content {
            flex-direction: column;
            align-items: center;
        }

        .token-info {
            width: 100%;
        }
    }
</style>
