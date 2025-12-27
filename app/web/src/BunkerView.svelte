<script>
    import { createEventDispatcher, onMount } from "svelte";
    import QRCode from "qrcode";
    import { getWireGuardConfig, regenerateWireGuard, getBunkerURL, fetchWireGuardStatus, getWireGuardAudit } from "./api.js";

    export let isLoggedIn = false;
    export let userPubkey = "";
    export let userSigner = null;
    export let currentEffectiveRole = "";

    const dispatch = createEventDispatcher();

    // State
    let wgConfig = null;
    let bunkerInfo = null;
    let wgStatus = null;
    let auditData = null;
    let isLoading = false;
    let error = "";
    let wgQrDataUrl = "";
    let bunkerQrDataUrl = "";

    $: canAccess = isLoggedIn && userPubkey && (
        currentEffectiveRole === "write" ||
        currentEffectiveRole === "admin" ||
        currentEffectiveRole === "owner"
    );

    let hasLoadedOnce = false;

    onMount(async () => {
        // Always check status first
        await checkStatus();
        if (canAccess && wgStatus?.available && !hasLoadedOnce) {
            hasLoadedOnce = true;
            await loadConfig();
        }
    });

    $: if (canAccess && wgStatus?.available && !hasLoadedOnce && !isLoading) {
        hasLoadedOnce = true;
        loadConfig();
    }

    async function checkStatus() {
        try {
            wgStatus = await fetchWireGuardStatus();
        } catch (err) {
            console.error("Error checking WireGuard status:", err);
            wgStatus = { available: false };
        }
    }

    async function loadConfig() {
        if (!userSigner || !userPubkey) return;

        isLoading = true;
        error = "";

        try {
            // Load WireGuard config, bunker URL, and audit data in parallel
            const [wgResult, bunkerResult, auditResult] = await Promise.all([
                getWireGuardConfig(userSigner, userPubkey),
                getBunkerURL(userSigner, userPubkey),
                getWireGuardAudit(userSigner, userPubkey).catch(() => null)
            ]);

            wgConfig = wgResult;
            bunkerInfo = bunkerResult;
            auditData = auditResult;

            // Generate QR codes
            if (wgConfig?.config_text) {
                wgQrDataUrl = await QRCode.toDataURL(wgConfig.config_text, {
                    width: 256,
                    margin: 2,
                    color: { dark: "#000000", light: "#ffffff" }
                });
            }

            if (bunkerInfo?.url) {
                bunkerQrDataUrl = await QRCode.toDataURL(bunkerInfo.url, {
                    width: 256,
                    margin: 2,
                    color: { dark: "#000000", light: "#ffffff" }
                });
            }
        } catch (err) {
            console.error("Error loading bunker config:", err);
            error = err.message || "Failed to load configuration";
        } finally {
            isLoading = false;
        }
    }

    function formatDate(timestamp) {
        if (!timestamp) return "Never";
        return new Date(timestamp * 1000).toLocaleString();
    }

    async function handleRegenerate() {
        if (!confirm("Regenerate your WireGuard keys? Your current keys will stop working.")) {
            return;
        }

        isLoading = true;
        error = "";

        try {
            await regenerateWireGuard(userSigner, userPubkey);
            // Reload config after regeneration
            hasLoadedOnce = false;
            await loadConfig();
        } catch (err) {
            console.error("Error regenerating keys:", err);
            error = err.message || "Failed to regenerate keys";
        } finally {
            isLoading = false;
        }
    }

    function copyToClipboard(text, label) {
        navigator.clipboard.writeText(text);
        alert(`${label} copied to clipboard!`);
    }

    function downloadConfig() {
        if (!wgConfig?.config_text) return;

        const blob = new Blob([wgConfig.config_text], { type: "text/plain" });
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = "wg-orly.conf";
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);
    }

    function openLoginModal() {
        dispatch("openLoginModal");
    }
</script>

{#if !wgStatus?.available}
    <div class="bunker-view">
        <div class="unavailable-message">
            <h3>Remote Signing Not Available</h3>
            <p>This relay does not have WireGuard/Bunker enabled, or ACL mode is set to "none".</p>
            <p class="hint">Remote signing requires the relay operator to enable WireGuard VPN and use ACL mode "follows" or "managed".</p>
        </div>
    </div>
{:else if canAccess}
    <div class="bunker-view">
        <div class="header-section">
            <h3>Remote Signing (Bunker)</h3>
            <button class="refresh-btn" on:click={loadConfig} disabled={isLoading}>
                {isLoading ? "Loading..." : "Refresh"}
            </button>
        </div>

        {#if error}
            <div class="error-message">{error}</div>
        {/if}

        {#if isLoading && !wgConfig}
            <div class="loading">Loading configuration...</div>
        {:else if wgConfig}
            <div class="instructions">
                <p><strong>How it works:</strong> Connect to the relay's private VPN, then use Amber to sign events remotely.</p>
            </div>

            <div class="config-sections">
                <!-- Step 1: WireGuard -->
                <section class="config-section">
                    <h4>Step 1: Install WireGuard</h4>
                    <p class="section-desc">Download the WireGuard app for your device:</p>

                    <div class="client-links">
                        <a href="https://play.google.com/store/apps/details?id=com.wireguard.android" target="_blank" rel="noopener noreferrer" class="client-link">
                            <span class="client-icon">Android</span>
                            <span class="client-store">Google Play</span>
                        </a>
                        <a href="https://f-droid.org/packages/com.wireguard.android/" target="_blank" rel="noopener noreferrer" class="client-link">
                            <span class="client-icon">Android</span>
                            <span class="client-store">F-Droid</span>
                        </a>
                        <a href="https://apps.apple.com/app/wireguard/id1441195209" target="_blank" rel="noopener noreferrer" class="client-link">
                            <span class="client-icon">iOS</span>
                            <span class="client-store">App Store</span>
                        </a>
                        <a href="https://www.wireguard.com/install/" target="_blank" rel="noopener noreferrer" class="client-link">
                            <span class="client-icon">Desktop</span>
                            <span class="client-store">Windows/Mac/Linux</span>
                        </a>
                    </div>
                </section>

                <!-- Step 2: WireGuard Config -->
                <section class="config-section">
                    <h4>Step 2: Add VPN Configuration</h4>
                    <p class="section-desc">Scan this QR code with the WireGuard app:</p>

                    <div class="qr-container">
                        {#if wgQrDataUrl}
                            <img src={wgQrDataUrl} alt="WireGuard Configuration QR Code" class="qr-code" />
                        {:else}
                            <div class="qr-placeholder">Generating QR...</div>
                        {/if}
                    </div>

                    <div class="config-actions">
                        <button on:click={() => copyToClipboard(wgConfig.config_text, "Config")}>Copy Config</button>
                        <button on:click={downloadConfig}>Download .conf</button>
                    </div>

                    <details class="config-text-details">
                        <summary>Show raw config</summary>
                        <pre class="config-text">{wgConfig.config_text}</pre>
                    </details>
                </section>

                <!-- Step 3: Connect VPN -->
                <section class="config-section">
                    <h4>Step 3: Connect to VPN</h4>
                    <p class="section-desc">After importing the config, toggle the VPN connection ON in the WireGuard app.</p>
                    <div class="ip-info">
                        <span class="label">Your VPN IP:</span>
                        <code>{wgConfig.interface.address}</code>
                    </div>
                </section>

                <!-- Step 4: Bunker URL -->
                {#if bunkerInfo}
                    <section class="config-section">
                        <h4>Step 4: Add Bunker to Amber</h4>
                        <p class="section-desc">With VPN connected, scan this QR code in <a href="https://github.com/greenart7c3/Amber" target="_blank" rel="noopener noreferrer">Amber</a>:</p>

                        <div class="qr-container">
                            {#if bunkerQrDataUrl}
                                <img src={bunkerQrDataUrl} alt="Bunker URL QR Code" class="qr-code" />
                            {:else}
                                <div class="qr-placeholder">Generating QR...</div>
                            {/if}
                        </div>

                        <div class="bunker-url-container">
                            <code class="bunker-url">{bunkerInfo.url}</code>
                            <button on:click={() => copyToClipboard(bunkerInfo.url, "Bunker URL")}>Copy</button>
                        </div>

                        <div class="relay-info">
                            <span class="label">Relay npub:</span>
                            <code class="npub">{bunkerInfo.relay_npub}</code>
                        </div>
                    </section>
                {/if}

                <!-- Amber links -->
                <section class="config-section">
                    <h4>Get Amber (NIP-46 Signer)</h4>
                    <p class="section-desc">Amber is an Android app for secure remote signing:</p>

                    <div class="client-links">
                        <a href="https://play.google.com/store/apps/details?id=com.greenart7c3.nostrsigner" target="_blank" rel="noopener noreferrer" class="client-link">
                            <span class="client-icon">Amber</span>
                            <span class="client-store">Google Play</span>
                        </a>
                        <a href="https://github.com/greenart7c3/Amber/releases" target="_blank" rel="noopener noreferrer" class="client-link">
                            <span class="client-icon">Amber</span>
                            <span class="client-store">GitHub APK</span>
                        </a>
                    </div>
                </section>
            </div>

            <!-- Danger zone -->
            <div class="danger-zone">
                <h4>Danger Zone</h4>
                <p>Regenerate your WireGuard keys if you believe they've been compromised.</p>
                <button class="danger-btn" on:click={handleRegenerate} disabled={isLoading}>
                    Regenerate Keys
                </button>
            </div>

            <!-- Audit Log Section -->
            {#if auditData && (auditData.revoked_keys?.length > 0 || auditData.access_logs?.length > 0)}
                <div class="audit-section">
                    <h4>Key History & Access Log</h4>
                    <p class="audit-desc">Monitor activity on your old WireGuard keys. High access counts might indicate you left something connected or someone copied your credentials.</p>

                    {#if auditData.revoked_keys?.length > 0}
                        <div class="audit-subsection">
                            <h5>Revoked Keys</h5>
                            <div class="audit-table-container">
                                <table class="audit-table">
                                    <thead>
                                        <tr>
                                            <th>Client IP</th>
                                            <th>Created</th>
                                            <th>Revoked</th>
                                            <th>Access Count</th>
                                            <th>Last Access</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {#each auditData.revoked_keys as key}
                                            <tr class:warning={key.access_count > 0}>
                                                <td><code>{key.client_ip}</code></td>
                                                <td>{formatDate(key.created_at)}</td>
                                                <td>{formatDate(key.revoked_at)}</td>
                                                <td class:highlight={key.access_count > 0}>{key.access_count}</td>
                                                <td>{formatDate(key.last_access_at)}</td>
                                            </tr>
                                        {/each}
                                    </tbody>
                                </table>
                            </div>
                        </div>
                    {/if}

                    {#if auditData.access_logs?.length > 0}
                        <div class="audit-subsection">
                            <h5>Recent Access Attempts (Obsolete Addresses)</h5>
                            <div class="audit-table-container">
                                <table class="audit-table">
                                    <thead>
                                        <tr>
                                            <th>Client IP</th>
                                            <th>Time</th>
                                            <th>Remote Address</th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        {#each auditData.access_logs as log}
                                            <tr>
                                                <td><code>{log.client_ip}</code></td>
                                                <td>{formatDate(log.timestamp)}</td>
                                                <td><code>{log.remote_addr}</code></td>
                                            </tr>
                                        {/each}
                                    </tbody>
                                </table>
                            </div>
                        </div>
                    {/if}
                </div>
            {/if}
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

    .config-sections {
        display: flex;
        flex-direction: column;
        gap: 1.5em;
    }

    .config-section {
        background-color: var(--card-bg);
        padding: 1.25em;
        border-radius: 8px;
    }

    .config-section h4 {
        margin: 0 0 0.5em 0;
        color: var(--text-color);
    }

    .section-desc {
        margin: 0 0 1em 0;
        color: var(--text-color);
        opacity: 0.8;
        font-size: 0.95em;
    }

    .section-desc a {
        color: var(--primary);
    }

    .client-links {
        display: flex;
        flex-wrap: wrap;
        gap: 0.75em;
    }

    .client-link {
        display: flex;
        flex-direction: column;
        align-items: center;
        padding: 0.75em 1em;
        background-color: var(--bg-color);
        border: 1px solid var(--border-color);
        border-radius: 6px;
        text-decoration: none;
        color: var(--text-color);
        transition: border-color 0.2s, background-color 0.2s;
        min-width: 100px;
    }

    .client-link:hover {
        border-color: var(--primary);
        background-color: var(--sidebar-bg);
    }

    .client-icon {
        font-weight: 500;
        margin-bottom: 0.25em;
    }

    .client-store {
        font-size: 0.8em;
        opacity: 0.7;
    }

    .qr-container {
        display: flex;
        justify-content: center;
        margin: 1em 0;
    }

    .qr-code {
        border-radius: 8px;
        background: white;
        padding: 8px;
    }

    .qr-placeholder {
        width: 256px;
        height: 256px;
        display: flex;
        align-items: center;
        justify-content: center;
        background-color: var(--bg-color);
        border-radius: 8px;
        color: var(--text-color);
        opacity: 0.5;
    }

    .config-actions {
        display: flex;
        justify-content: center;
        gap: 0.75em;
        margin-top: 1em;
    }

    .config-actions button {
        padding: 0.5em 1em;
        background-color: var(--primary);
        color: var(--text-color);
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
    }

    .config-actions button:hover {
        background-color: var(--accent-hover-color);
    }

    .config-text-details {
        margin-top: 1em;
    }

    .config-text-details summary {
        cursor: pointer;
        color: var(--text-color);
        opacity: 0.8;
        font-size: 0.9em;
    }

    .config-text {
        margin-top: 0.5em;
        padding: 1em;
        background-color: var(--bg-color);
        border-radius: 4px;
        font-size: 0.85em;
        overflow-x: auto;
        white-space: pre;
        color: var(--text-color);
    }

    .ip-info, .relay-info {
        display: flex;
        align-items: center;
        gap: 0.5em;
        margin-top: 0.5em;
    }

    .label {
        color: var(--text-color);
        opacity: 0.7;
    }

    code {
        font-family: monospace;
        padding: 0.25em 0.5em;
        background-color: var(--bg-color);
        border-radius: 4px;
        color: var(--text-color);
    }

    .bunker-url-container {
        display: flex;
        align-items: center;
        gap: 0.5em;
        justify-content: center;
        flex-wrap: wrap;
    }

    .bunker-url {
        word-break: break-all;
        max-width: 400px;
    }

    .bunker-url-container button {
        padding: 0.4em 0.8em;
        background-color: var(--primary);
        color: var(--text-color);
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.85em;
    }

    .bunker-url-container button:hover {
        background-color: var(--accent-hover-color);
    }

    .npub {
        word-break: break-all;
        font-size: 0.85em;
    }

    .danger-zone {
        margin-top: 2em;
        padding: 1.25em;
        border: 1px solid var(--warning);
        border-radius: 8px;
        background-color: rgba(255, 100, 100, 0.05);
    }

    .danger-zone h4 {
        margin: 0 0 0.5em 0;
        color: var(--warning);
    }

    .danger-zone p {
        margin: 0 0 1em 0;
        color: var(--text-color);
        opacity: 0.8;
        font-size: 0.95em;
    }

    .danger-btn {
        background-color: transparent;
        border: 1px solid var(--warning);
        color: var(--warning);
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
    }

    .danger-btn:hover:not(:disabled) {
        background-color: var(--warning);
        color: var(--text-color);
    }

    .danger-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
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

    /* Audit section styles */
    .audit-section {
        margin-top: 2em;
        padding: 1.25em;
        border: 1px solid var(--border-color);
        border-radius: 8px;
        background-color: var(--card-bg);
    }

    .audit-section h4 {
        margin: 0 0 0.5em 0;
        color: var(--text-color);
    }

    .audit-desc {
        margin: 0 0 1em 0;
        color: var(--text-color);
        opacity: 0.8;
        font-size: 0.9em;
    }

    .audit-subsection {
        margin-bottom: 1.5em;
    }

    .audit-subsection:last-child {
        margin-bottom: 0;
    }

    .audit-subsection h5 {
        margin: 0 0 0.5em 0;
        color: var(--text-color);
        font-size: 0.95em;
    }

    .audit-table-container {
        overflow-x: auto;
    }

    .audit-table {
        width: 100%;
        border-collapse: collapse;
        font-size: 0.85em;
    }

    .audit-table th,
    .audit-table td {
        padding: 0.5em 0.75em;
        text-align: left;
        border-bottom: 1px solid var(--border-color);
    }

    .audit-table th {
        background-color: var(--bg-color);
        color: var(--text-color);
        font-weight: 500;
    }

    .audit-table td {
        color: var(--text-color);
    }

    .audit-table td code {
        font-size: 0.9em;
        padding: 0.15em 0.3em;
    }

    .audit-table tr.warning {
        background-color: rgba(255, 100, 100, 0.1);
    }

    .audit-table td.highlight {
        color: var(--warning);
        font-weight: 600;
    }

    @media (max-width: 600px) {
        .client-links {
            flex-direction: column;
        }

        .client-link {
            width: 100%;
        }

        .bunker-url {
            font-size: 0.75em;
        }

        .audit-table {
            font-size: 0.75em;
        }

        .audit-table th,
        .audit-table td {
            padding: 0.4em 0.5em;
        }
    }
</style>
