<script>
    export let isLoggedIn = false;
    export let currentEffectiveRole = "";
    export let selectedFile = null;
    export let aclMode = "";

    import { createEventDispatcher } from "svelte";
    const dispatch = createEventDispatcher();

    // When ACL is "none", allow access without login
    $: canImport = aclMode === "none" || (isLoggedIn && (currentEffectiveRole === "admin" || currentEffectiveRole === "owner"));

    function handleFileSelect(event) {
        dispatch("fileSelect", event);
    }

    function importEvents() {
        dispatch("importEvents");
    }

    function openLoginModal() {
        dispatch("openLoginModal");
    }
</script>

<div class="import-section">
    {#if canImport}
        <h3>Import Events</h3>
        <p>Upload a JSONL file to import events into the database.</p>
        <div class="recovery-controls-card">
            <input
                type="file"
                id="import-file"
                accept=".jsonl,.txt"
                on:change={handleFileSelect}
            />
            <button
                class="import-btn"
                on:click={importEvents}
                disabled={!selectedFile}
            >
                Import Events
            </button>
        </div>
    {:else if isLoggedIn}
        <div class="permission-denied">
            <h3 class="recovery-header">Import Events</h3>
            <p class="recovery-description">
                Admin or owner permission required for import functionality.
            </p>
        </div>
    {:else}
        <div class="login-prompt">
            <h3 class="recovery-header">Import Events</h3>
            <p class="recovery-description">
                Please log in to access import functionality.
            </p>
            <button class="login-btn" on:click={openLoginModal}>Log In</button>
        </div>
    {/if}
</div>

<style>
    .import-section {
        background: transparent;
        padding: 1em;
        border-radius: 8px;
        margin-bottom: 1.5rem;
        width: 32em;
    }

    .import-section h3 {
        margin: 0 0 1rem 0;
        color: var(--text-color);
        font-size: 1.2rem;
        font-weight: 600;
    }

    .import-section p {
        margin: 0 0 1.5rem 0;
        color: var(--text-color);
        opacity: 0.8;
        line-height: 1.4;
    }

    .recovery-controls-card {
        background-color: var(--card-bg);
        padding: 1em;
        border: 0;
        display: flex;
        flex-direction: column;
        border-radius: 0.5em;
        gap: 1em;
    }

    #import-file {
        padding: 0.5em;
        border: 0;
        background: var(--input-bg);
        color: var(--input-text-color);
    }

    .import-btn {
        background-color: var(--primary);
        color: var(--text-color);
        border-radius: 0.5em;
        padding: 0.75em 1.5em;
        border: 0;
        cursor: pointer;
        font-weight: bold;
        font-size: 0.9em;
        transition: background-color 0.2s;
        align-self: flex-start;
    }

    .import-btn:hover:not(:disabled) {
        background-color: var(--accent-hover-color);
    }

    .import-btn:disabled {
        background-color: var(--secondary);
        cursor: not-allowed;
    }

    .permission-denied,
    .login-prompt {
        text-align: center;
        padding: 2em;
        background-color: var(--card-bg);
        border: 0;
    }

    .recovery-header {
        margin: 0 0 1rem 0;
        color: var(--text-color);
        font-size: 1.2rem;
        font-weight: 600;
    }

    .recovery-description {
        margin: 0 0 1.5rem 0;
        color: var(--text-color);
        line-height: 1.4;
    }

    .login-btn {
        background-color: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.75em 1.5em;
        border: 0;
        cursor: pointer;
        font-weight: bold;
        font-size: 0.9em;
        transition: background-color 0.2s;
    }

    .login-btn:hover {
        background-color: var(--accent-hover-color);
    }
</style>
