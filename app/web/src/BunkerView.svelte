<script>
    import { createEventDispatcher, onMount } from "svelte";
    import QRCode from "qrcode";
    import { getBunkerInfo } from "./api.js";
    import { bytesToHex } from "@noble/hashes/utils";
    import {
        bunkerServiceActive,
        bunkerConnectedClients,
        configureBunkerWorker,
        connectBunkerWorker,
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
    $: connectedClients = $bunkerConnectedClients;

    $: canAccess = isLoggedIn && userPubkey && (
        currentEffectiveRole === "write" ||
        currentEffectiveRole === "admin" ||
        currentEffectiveRole === "owner"
    );

    // Generate bunker URLs when bunkerInfo and userPubkey are available
    $: clientBunkerURL = bunkerInfo && userPubkey ?
        `bunker://${userPubkey}?relay=${encodeURIComponent(bunkerInfo.relay_url)}${bunkerSecret ? `&secret=${bunkerSecret}` : ''}` : "";

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
            // Configure the worker with user credentials
            const privkeyHex = userPrivkey instanceof Uint8Array ? bytesToHex(userPrivkey) : userPrivkey;
            configureBunkerWorker({
                userPubkey,
                userPrivkey: privkeyHex,
                relayUrl: bunkerInfo.relay_url,
                secrets: bunkerSecret ? [bunkerSecret] : []
            });

            // Connect the worker
            connectBunkerWorker();

            // Regenerate QR codes
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
    }
</style>
