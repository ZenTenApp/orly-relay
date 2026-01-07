<script>
    export let isLoggedIn = false;
    export let userRole = "";
    export let userSigner = null;
    export let userPubkey = "";

    import { createEventDispatcher, onMount } from "svelte";
    import * as api from "./api.js";
    import { copyToClipboard, showCopyFeedback } from "./utils.js";

    const dispatch = createEventDispatcher();

    // State
    let nrcEnabled = false;
    let badgerRequired = false;
    let connections = [];
    let config = {};
    let isLoading = false;
    let message = "";
    let messageType = "info";

    // New connection form
    let newLabel = "";
    let newUseCashu = false;

    // URI display modal
    let showURIModal = false;
    let currentURI = "";
    let currentLabel = "";

    onMount(async () => {
        await loadNRCConfig();
    });

    async function loadNRCConfig() {
        try {
            const result = await api.fetchNRCConfig();
            nrcEnabled = result.enabled;
            badgerRequired = result.badger_required;

            if (nrcEnabled && isLoggedIn && userRole === "owner") {
                await loadConnections();
            }
        } catch (error) {
            console.error("Failed to load NRC config:", error);
        }
    }

    async function loadConnections() {
        if (!isLoggedIn || !userSigner || !userPubkey) return;

        isLoading = true;
        try {
            const result = await api.fetchNRCConnections(userSigner, userPubkey);
            connections = result.connections || [];
            config = result.config || {};
        } catch (error) {
            setMessage(`Failed to load connections: ${error.message}`, "error");
        } finally {
            isLoading = false;
        }
    }

    async function createConnection() {
        if (!newLabel.trim()) {
            setMessage("Please enter a label for the connection", "error");
            return;
        }

        isLoading = true;
        try {
            const result = await api.createNRCConnection(userSigner, userPubkey, newLabel.trim(), newUseCashu);

            // Show the URI modal with the new connection
            currentURI = result.uri;
            currentLabel = result.label;
            showURIModal = true;

            // Reset form
            newLabel = "";
            newUseCashu = false;

            // Reload connections
            await loadConnections();
            setMessage(`Connection "${result.label}" created successfully`, "success");
        } catch (error) {
            setMessage(`Failed to create connection: ${error.message}`, "error");
        } finally {
            isLoading = false;
        }
    }

    async function deleteConnection(connId, label) {
        if (!confirm(`Are you sure you want to delete the connection "${label}"? This will revoke access for any device using this connection.`)) {
            return;
        }

        isLoading = true;
        try {
            await api.deleteNRCConnection(userSigner, userPubkey, connId);
            await loadConnections();
            setMessage(`Connection "${label}" deleted`, "success");
        } catch (error) {
            setMessage(`Failed to delete connection: ${error.message}`, "error");
        } finally {
            isLoading = false;
        }
    }

    async function showConnectionURI(connId, label) {
        isLoading = true;
        try {
            const result = await api.getNRCConnectionURI(userSigner, userPubkey, connId);
            currentURI = result.uri;
            currentLabel = label;
            showURIModal = true;
        } catch (error) {
            setMessage(`Failed to get URI: ${error.message}`, "error");
        } finally {
            isLoading = false;
        }
    }

    async function copyURIToClipboard(event) {
        const success = await copyToClipboard(currentURI);
        const button = event.target.closest("button");
        showCopyFeedback(button, success);
        if (!success) {
            setMessage("Failed to copy to clipboard", "error");
        }
    }

    function closeURIModal() {
        showURIModal = false;
        currentURI = "";
        currentLabel = "";
    }

    function setMessage(msg, type = "info") {
        message = msg;
        messageType = type;
        // Auto-clear after 5 seconds
        setTimeout(() => {
            if (message === msg) {
                message = "";
            }
        }, 5000);
    }

    function formatTimestamp(ts) {
        if (!ts) return "Never";
        return new Date(ts * 1000).toLocaleString();
    }

    function openLoginModal() {
        dispatch("openLoginModal");
    }

    // Reload when login state changes
    $: if (isLoggedIn && userRole === "owner" && nrcEnabled) {
        loadConnections();
    }
</script>

<div class="relay-connect-view">
    <h2>Relay Connect</h2>
    <p class="description">
        Nostr Relay Connect (NRC) allows remote access to this relay through a public relay tunnel.
        Create connection strings for your devices to sync securely.
    </p>

    {#if !nrcEnabled}
        <div class="not-enabled">
            {#if badgerRequired}
                <p>NRC requires the Badger database backend.</p>
                <p>Set <code>ORLY_DB_TYPE=badger</code> to enable NRC functionality.</p>
            {:else}
                <p>NRC is not enabled on this relay.</p>
                <p>Set <code>ORLY_NRC_ENABLED=true</code> and configure <code>ORLY_NRC_RENDEZVOUS_URL</code> to enable.</p>
            {/if}
        </div>
    {:else if !isLoggedIn}
        <div class="login-prompt">
            <p>Please log in to manage relay connections.</p>
            <button class="login-btn" on:click={openLoginModal}>Log In</button>
        </div>
    {:else if userRole !== "owner"}
        <div class="permission-denied">
            <p>Owner permission required for relay connection management.</p>
            <p>Current role: <strong>{userRole || "none"}</strong></p>
        </div>
    {:else}
        <!-- Config status -->
        <div class="config-status">
            <div class="status-item">
                <span class="status-label">Status:</span>
                <span class="status-value enabled">Enabled</span>
            </div>
            <div class="status-item">
                <span class="status-label">Rendezvous:</span>
                <span class="status-value">{config.rendezvous_url || "Not configured"}</span>
            </div>
            {#if config.mint_url}
                <div class="status-item">
                    <span class="status-label">Cashu Mint:</span>
                    <span class="status-value">{config.mint_url}</span>
                </div>
            {/if}
        </div>

        <!-- Create new connection -->
        <div class="section">
            <h3>Create New Connection</h3>
            <div class="create-form">
                <div class="form-group">
                    <label for="new-label">Device Label</label>
                    <input
                        type="text"
                        id="new-label"
                        bind:value={newLabel}
                        placeholder="e.g., Phone, Laptop, Tablet"
                        disabled={isLoading}
                    />
                </div>
                <div class="form-group checkbox-group">
                    <label>
                        <input
                            type="checkbox"
                            bind:checked={newUseCashu}
                            disabled={isLoading || !config.mint_url}
                        />
                        Include CAT (Cashu Access Token)
                        {#if !config.mint_url}
                            <span class="hint">(requires Cashu mint)</span>
                        {/if}
                    </label>
                </div>
                <button
                    class="create-btn"
                    on:click={createConnection}
                    disabled={isLoading || !newLabel.trim()}
                >
                    + Create Connection
                </button>
            </div>
        </div>

        <!-- Connections list -->
        <div class="section">
            <h3>Connections ({connections.length})</h3>
            {#if connections.length === 0}
                <p class="no-connections">No connections yet. Create one to get started.</p>
            {:else}
                <div class="connections-list">
                    {#each connections as conn}
                        <div class="connection-item">
                            <div class="connection-info">
                                <div class="connection-label">{conn.label}</div>
                                <div class="connection-details">
                                    <span class="detail">ID: {conn.id.substring(0, 8)}...</span>
                                    <span class="detail">Created: {formatTimestamp(conn.created_at)}</span>
                                    {#if conn.last_used}
                                        <span class="detail">Last used: {formatTimestamp(conn.last_used)}</span>
                                    {/if}
                                    {#if conn.use_cashu}
                                        <span class="badge cashu">CAT</span>
                                    {/if}
                                </div>
                            </div>
                            <div class="connection-actions">
                                <button
                                    class="action-btn show-uri-btn"
                                    on:click={() => showConnectionURI(conn.id, conn.label)}
                                    disabled={isLoading}
                                    title="Show connection URI"
                                >
                                    Show URI
                                </button>
                                <button
                                    class="action-btn delete-btn"
                                    on:click={() => deleteConnection(conn.id, conn.label)}
                                    disabled={isLoading}
                                    title="Delete connection"
                                >
                                    Delete
                                </button>
                            </div>
                        </div>
                    {/each}
                </div>
            {/if}

            <button
                class="refresh-btn"
                on:click={loadConnections}
                disabled={isLoading}
            >
                Refresh
            </button>
        </div>

        {#if message}
            <div class="message" class:error={messageType === "error"} class:success={messageType === "success"}>
                {message}
            </div>
        {/if}
    {/if}
</div>

<!-- URI Modal -->
{#if showURIModal}
    <div class="modal-overlay" on:click={closeURIModal}>
        <div class="modal" on:click|stopPropagation>
            <h3>Connection URI for "{currentLabel}"</h3>
            <p class="modal-description">
                Copy this URI to your Nostr client to connect to this relay remotely.
                Keep it secret - anyone with this URI can access your relay.
            </p>
            <div class="uri-display">
                <textarea readonly>{currentURI}</textarea>
            </div>
            <div class="modal-actions">
                <button class="copy-btn" on:click={copyURIToClipboard}>
                    Copy to Clipboard
                </button>
                <button class="close-btn" on:click={closeURIModal}>
                    Close
                </button>
            </div>
        </div>
    </div>
{/if}

<style>
    .relay-connect-view {
        width: 100%;
        max-width: 800px;
        margin: 0;
        padding: 20px;
        background: var(--header-bg);
        color: var(--text-color);
        border-radius: 8px;
    }

    .relay-connect-view h2 {
        margin: 0 0 0.5rem 0;
        color: var(--text-color);
        font-size: 1.8rem;
        font-weight: 600;
    }

    .description {
        color: var(--muted-foreground);
        margin-bottom: 1.5rem;
        line-height: 1.5;
    }

    .section {
        background-color: var(--card-bg);
        border-radius: 8px;
        padding: 1em;
        margin-bottom: 1.5rem;
        border: 1px solid var(--border-color);
    }

    .section h3 {
        margin: 0 0 1rem 0;
        color: var(--text-color);
        font-size: 1.1rem;
        font-weight: 600;
    }

    .config-status {
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
        margin-bottom: 1.5rem;
        padding: 1rem;
        background: var(--card-bg);
        border-radius: 8px;
        border: 1px solid var(--border-color);
    }

    .status-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
    }

    .status-label {
        font-weight: 600;
        color: var(--text-color);
    }

    .status-value {
        color: var(--muted-foreground);
        font-family: monospace;
        font-size: 0.9em;
    }

    .status-value.enabled {
        color: var(--success);
    }

    /* Create form */
    .create-form {
        display: flex;
        flex-direction: column;
        gap: 1rem;
    }

    .form-group {
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
    }

    .form-group label {
        font-weight: 500;
        color: var(--text-color);
    }

    .form-group input[type="text"] {
        padding: 0.75em;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--input-bg);
        color: var(--input-text-color);
        font-size: 1em;
    }

    .checkbox-group {
        flex-direction: row;
        align-items: center;
    }

    .checkbox-group label {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        cursor: pointer;
    }

    .checkbox-group input[type="checkbox"] {
        width: 1.2em;
        height: 1.2em;
    }

    .hint {
        color: var(--muted-foreground);
        font-size: 0.85em;
    }

    .create-btn {
        background: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.75em 1.5em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 1em;
        font-weight: 500;
        align-self: flex-start;
        transition: background-color 0.2s;
    }

    .create-btn:hover:not(:disabled) {
        background: var(--accent-hover-color);
    }

    .create-btn:disabled {
        background: var(--secondary);
        cursor: not-allowed;
    }

    /* Connections list */
    .connections-list {
        display: flex;
        flex-direction: column;
        gap: 0.75rem;
        margin-bottom: 1rem;
    }

    .connection-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 1rem;
        background: var(--bg-color);
        border: 1px solid var(--border-color);
        border-radius: 4px;
    }

    .connection-info {
        flex: 1;
    }

    .connection-label {
        font-weight: 600;
        color: var(--text-color);
        margin-bottom: 0.25rem;
    }

    .connection-details {
        display: flex;
        flex-wrap: wrap;
        gap: 0.75rem;
        font-size: 0.85em;
        color: var(--muted-foreground);
    }

    .badge {
        background: var(--primary);
        color: var(--text-color);
        padding: 0.1em 0.4em;
        border-radius: 0.25rem;
        font-size: 0.75em;
        font-weight: 600;
    }

    .badge.cashu {
        background: var(--warning);
    }

    .connection-actions {
        display: flex;
        gap: 0.5rem;
    }

    .action-btn {
        background: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        transition: background-color 0.2s;
    }

    .action-btn:hover:not(:disabled) {
        background: var(--accent-hover-color);
    }

    .action-btn:disabled {
        background: var(--secondary);
        cursor: not-allowed;
    }

    .show-uri-btn {
        background: var(--info);
    }

    .show-uri-btn:hover:not(:disabled) {
        filter: brightness(0.9);
    }

    .delete-btn {
        background: var(--danger);
    }

    .delete-btn:hover:not(:disabled) {
        filter: brightness(0.9);
    }

    .refresh-btn {
        background: var(--secondary);
        color: var(--text-color);
        border: none;
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        transition: background-color 0.2s;
    }

    .refresh-btn:hover:not(:disabled) {
        filter: brightness(0.9);
    }

    .refresh-btn:disabled {
        cursor: not-allowed;
        opacity: 0.6;
    }

    .no-connections {
        color: var(--muted-foreground);
        text-align: center;
        padding: 2rem;
    }

    /* Message */
    .message {
        padding: 1rem;
        border-radius: 4px;
        margin-top: 1rem;
        background: var(--info-bg, #e7f3ff);
        color: var(--info-text, #0066cc);
        border: 1px solid var(--info, #0066cc);
    }

    .message.error {
        background: var(--danger-bg);
        color: var(--danger-text);
        border-color: var(--danger);
    }

    .message.success {
        background: var(--success-bg);
        color: var(--success-text);
        border-color: var(--success);
    }

    /* Modal */
    .modal-overlay {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background: rgba(0, 0, 0, 0.6);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 1000;
    }

    .modal {
        background: var(--card-bg);
        border-radius: 8px;
        padding: 1.5rem;
        max-width: 600px;
        width: 90%;
        max-height: 80vh;
        overflow: auto;
        border: 1px solid var(--border-color);
    }

    .modal h3 {
        margin: 0 0 0.5rem 0;
        color: var(--text-color);
    }

    .modal-description {
        color: var(--muted-foreground);
        margin-bottom: 1rem;
        font-size: 0.9em;
        line-height: 1.5;
    }

    .uri-display textarea {
        width: 100%;
        height: 120px;
        padding: 0.75em;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--input-bg);
        color: var(--input-text-color);
        font-family: monospace;
        font-size: 0.85em;
        resize: none;
        word-break: break-all;
    }

    .modal-actions {
        display: flex;
        gap: 0.5rem;
        margin-top: 1rem;
        justify-content: flex-end;
    }

    .copy-btn {
        background: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.75em 1.5em;
        border-radius: 4px;
        cursor: pointer;
        font-weight: 500;
        transition: background-color 0.2s;
    }

    .copy-btn:hover {
        background: var(--accent-hover-color);
    }

    .close-btn {
        background: var(--secondary);
        color: var(--text-color);
        border: none;
        padding: 0.75em 1.5em;
        border-radius: 4px;
        cursor: pointer;
        font-weight: 500;
        transition: background-color 0.2s;
    }

    .close-btn:hover {
        filter: brightness(0.9);
    }

    /* States */
    .not-enabled,
    .permission-denied,
    .login-prompt {
        text-align: center;
        padding: 2em;
        background-color: var(--card-bg);
        border-radius: 8px;
        border: 1px solid var(--border-color);
        color: var(--text-color);
    }

    .not-enabled p,
    .permission-denied p,
    .login-prompt p {
        margin: 0 0 1rem 0;
        line-height: 1.4;
    }

    .not-enabled code {
        background: var(--code-bg);
        padding: 0.2em 0.4em;
        border-radius: 0.25rem;
        font-family: monospace;
        font-size: 0.9em;
    }

    .login-btn {
        background: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.75em 1.5em;
        border-radius: 4px;
        cursor: pointer;
        font-weight: bold;
        font-size: 0.9em;
        transition: background-color 0.2s;
    }

    .login-btn:hover {
        background: var(--accent-hover-color);
    }
</style>
