<script>
    import { createEventDispatcher } from "svelte";
    import { connectToRelay, normalizeWsUrl } from "./config.js";
    import { relayInfo, relayConnectionStatus, relayUrl, savedRelays, saveRelay, removeRelay } from "./stores.js";

    const dispatch = createEventDispatcher();

    export let showModal = false;
    export let isDarkTheme = false;

    let urlInput = "";
    let isConnecting = false;
    let errorMessage = "";
    let connectingUrl = "";

    function closeModal() {
        showModal = false;
        errorMessage = "";
        dispatch("close");
    }

    async function handleConnect(url = null) {
        const targetUrl = url || urlInput.trim();
        if (!targetUrl) {
            errorMessage = "Please enter a relay URL";
            return;
        }

        isConnecting = true;
        connectingUrl = targetUrl;
        errorMessage = "";

        try {
            const result = await connectToRelay(targetUrl);

            if (result.success) {
                // Save with the wss:// URL as the display name
                const wsUrl = normalizeWsUrl(targetUrl);
                saveRelay(targetUrl, wsUrl);
                urlInput = ""; // Clear input on success
                dispatch("connected", { info: result.info });
                closeModal();
            } else {
                errorMessage = result.error || "Failed to connect";
            }
        } catch (error) {
            errorMessage = error.message || "Connection failed";
        } finally {
            isConnecting = false;
            connectingUrl = "";
        }
    }

    async function handleAddRelay() {
        const targetUrl = urlInput.trim();
        if (!targetUrl) {
            errorMessage = "Please enter a relay URL";
            return;
        }

        isConnecting = true;
        errorMessage = "";

        try {
            const result = await connectToRelay(targetUrl);

            if (result.success) {
                const wsUrl = normalizeWsUrl(targetUrl);
                saveRelay(targetUrl, wsUrl);
                urlInput = "";
                dispatch("connected", { info: result.info });
                // Don't close modal - stay open to manage relays
            } else {
                errorMessage = result.error || "Failed to connect";
            }
        } catch (error) {
            errorMessage = error.message || "Connection failed";
        } finally {
            isConnecting = false;
        }
    }

    function handleRemoveRelay(url, event) {
        event.stopPropagation();
        removeRelay(url);
    }

    function handleKeydown(event) {
        if (event.key === "Enter" && !isConnecting) {
            handleAddRelay();
        } else if (event.key === "Escape") {
            closeModal();
        }
    }

    function isCurrentRelay(url) {
        return $relayUrl === url && $relayConnectionStatus === "connected";
    }

    // Reset input when modal opens
    $: if (showModal) {
        urlInput = "";
        errorMessage = "";
    }
</script>

{#if showModal}
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="modal-overlay" on:click={closeModal}>
        <div
            class="modal"
            class:dark={isDarkTheme}
            on:click|stopPropagation
        >
            <div class="modal-header">
                <h2>Relay Manager</h2>
                <button class="close-btn" on:click={closeModal}>&times;</button>
            </div>

            <div class="modal-content">
                <!-- Add new relay section at top -->
                <div class="add-relay-section">
                    <div class="section-header">Add Relay</div>
                    <div class="input-row">
                        <input
                            type="text"
                            placeholder="wss://relay.example.com"
                            bind:value={urlInput}
                            on:keydown={handleKeydown}
                            disabled={isConnecting}
                            class="url-input"
                        />
                        <button
                            class="add-btn"
                            on:click={handleAddRelay}
                            disabled={isConnecting || !urlInput.trim()}
                        >
                            {#if isConnecting && !connectingUrl}
                                Adding...
                            {:else}
                                Add
                            {/if}
                        </button>
                    </div>
                </div>

                {#if errorMessage}
                    <div class="error-message">
                        {errorMessage}
                    </div>
                {/if}

                <!-- Saved relays list -->
                <div class="saved-relays-section">
                    <div class="section-header">Saved Relays</div>
                    {#if $savedRelays.length > 0}
                        <div class="saved-relays-list">
                            {#each $savedRelays as relay}
                                <div
                                    class="relay-item"
                                    class:current={isCurrentRelay(relay.url)}
                                    class:connecting={connectingUrl === relay.url}
                                >
                                    <button
                                        class="relay-connect-btn"
                                        on:click={() => handleConnect(relay.url)}
                                        disabled={isConnecting}
                                        title="Click to connect"
                                    >
                                        <span class="relay-status-dot" class:connected={isCurrentRelay(relay.url)}></span>
                                        <span class="relay-url-label">{relay.name}</span>
                                        {#if isCurrentRelay(relay.url)}
                                            <span class="current-badge">Connected</span>
                                        {:else if connectingUrl === relay.url}
                                            <span class="connecting-badge">Connecting...</span>
                                        {/if}
                                    </button>
                                    <button
                                        class="relay-remove-btn"
                                        on:click={(e) => handleRemoveRelay(relay.url, e)}
                                        title="Remove relay"
                                        disabled={isConnecting}
                                    >
                                        Remove
                                    </button>
                                </div>
                            {/each}
                        </div>
                    {:else}
                        <div class="empty-state">
                            No saved relays. Add one above to get started.
                        </div>
                    {/if}
                </div>

                <div class="button-group">
                    <button class="done-btn" on:click={closeModal}>
                        Done
                    </button>
                </div>
            </div>
        </div>
    </div>
{/if}

<style>
    .modal-overlay {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background-color: rgba(0, 0, 0, 0.5);
        display: flex;
        justify-content: center;
        align-items: center;
        z-index: 1000;
    }

    .modal {
        background: var(--bg-color);
        border-radius: 8px;
        box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
        width: 90%;
        max-width: 550px;
        max-height: 90vh;
        overflow-y: auto;
        border: 1px solid var(--border-color);
    }

    .modal-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 16px 20px;
        border-bottom: 1px solid var(--border-color);
    }

    .modal-header h2 {
        margin: 0;
        color: var(--text-color);
        font-size: 1.25rem;
    }

    .close-btn {
        background: none;
        border: none;
        font-size: 1.5rem;
        cursor: pointer;
        color: var(--text-color);
        padding: 0;
        width: 30px;
        height: 30px;
        display: flex;
        align-items: center;
        justify-content: center;
        border-radius: 50%;
        transition: background-color 0.2s;
    }

    .close-btn:hover {
        background-color: var(--tab-hover-bg);
    }

    .modal-content {
        padding: 16px 20px;
        display: flex;
        flex-direction: column;
        gap: 16px;
    }

    .section-header {
        font-size: 0.85rem;
        color: var(--muted-foreground);
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.5px;
        margin-bottom: 8px;
    }

    .add-relay-section {
        padding-bottom: 16px;
        border-bottom: 1px solid var(--border-color);
    }

    .input-row {
        display: flex;
        gap: 8px;
    }

    .url-input {
        flex: 1;
        padding: 10px 12px;
        border: 1px solid var(--input-border);
        border-radius: 6px;
        font-size: 0.95rem;
        font-family: monospace;
        background: var(--bg-color);
        color: var(--text-color);
    }

    .url-input:focus {
        outline: none;
        border-color: var(--primary);
    }

    .url-input:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .add-btn {
        padding: 10px 20px;
        background: var(--primary);
        color: white;
        border: none;
        border-radius: 6px;
        cursor: pointer;
        font-size: 0.95rem;
        font-weight: 500;
        white-space: nowrap;
        transition: background-color 0.2s;
    }

    .add-btn:hover:not(:disabled) {
        background: #00acc1;
    }

    .add-btn:disabled {
        background: #ccc;
        cursor: not-allowed;
    }

    .error-message {
        padding: 10px 12px;
        background: #fee2e2;
        color: #dc2626;
        border-radius: 6px;
        font-size: 0.9rem;
    }

    .dark .error-message {
        background: #450a0a;
        color: #fca5a5;
    }

    .saved-relays-section {
        flex: 1;
    }

    .saved-relays-list {
        display: flex;
        flex-direction: column;
        gap: 6px;
    }

    .relay-item {
        display: flex;
        align-items: center;
        gap: 8px;
        padding: 4px;
        border-radius: 6px;
        background: var(--muted);
        transition: background-color 0.2s;
    }

    .relay-item.current {
        background: rgba(16, 185, 129, 0.15);
    }

    .relay-item.connecting {
        background: rgba(234, 179, 8, 0.15);
    }

    .relay-connect-btn {
        flex: 1;
        display: flex;
        align-items: center;
        gap: 10px;
        padding: 10px 12px;
        background: transparent;
        border: none;
        cursor: pointer;
        text-align: left;
        border-radius: 4px;
        transition: background-color 0.15s;
    }

    .relay-connect-btn:hover:not(:disabled) {
        background: var(--tab-hover-bg);
    }

    .relay-connect-btn:disabled {
        cursor: not-allowed;
        opacity: 0.7;
    }

    .relay-status-dot {
        width: 8px;
        height: 8px;
        border-radius: 50%;
        background: var(--muted-foreground);
        flex-shrink: 0;
    }

    .relay-status-dot.connected {
        background: var(--success);
    }

    .relay-url-label {
        flex: 1;
        color: var(--text-color);
        font-family: monospace;
        font-size: 0.9rem;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .current-badge {
        font-size: 0.7rem;
        padding: 2px 8px;
        background: var(--success);
        color: white;
        border-radius: 4px;
        font-weight: 500;
        flex-shrink: 0;
    }

    .connecting-badge {
        font-size: 0.7rem;
        padding: 2px 8px;
        background: var(--warning);
        color: white;
        border-radius: 4px;
        font-weight: 500;
        flex-shrink: 0;
    }

    .relay-remove-btn {
        padding: 6px 12px;
        background: transparent;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        color: var(--muted-foreground);
        cursor: pointer;
        font-size: 0.8rem;
        transition: background-color 0.2s, color 0.2s, border-color 0.2s;
        flex-shrink: 0;
    }

    .relay-remove-btn:hover:not(:disabled) {
        background: var(--danger);
        border-color: var(--danger);
        color: white;
    }

    .relay-remove-btn:disabled {
        cursor: not-allowed;
        opacity: 0.5;
    }

    .empty-state {
        padding: 20px;
        text-align: center;
        color: var(--muted-foreground);
        font-size: 0.9rem;
    }

    .button-group {
        display: flex;
        justify-content: flex-end;
        margin-top: 8px;
        padding-top: 16px;
        border-top: 1px solid var(--border-color);
    }

    .done-btn {
        padding: 10px 24px;
        background: var(--primary);
        color: white;
        border: none;
        border-radius: 6px;
        cursor: pointer;
        font-size: 0.95rem;
        font-weight: 500;
        transition: background-color 0.2s;
    }

    .done-btn:hover {
        background: #00acc1;
    }
</style>
