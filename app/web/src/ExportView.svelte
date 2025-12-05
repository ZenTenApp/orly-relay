<script>
    export let isLoggedIn = false;
    export let currentEffectiveRole = "";
    export let aclMode = "";

    import { createEventDispatcher } from "svelte";
    const dispatch = createEventDispatcher();

    // When ACL is "none", allow access without login
    $: canAccess = aclMode === "none" || isLoggedIn;
    $: canExportAll = aclMode === "none" || currentEffectiveRole === "admin" || currentEffectiveRole === "owner";

    function exportMyEvents() {
        dispatch("exportMyEvents");
    }

    function exportAllEvents() {
        dispatch("exportAllEvents");
    }

    function openLoginModal() {
        dispatch("openLoginModal");
    }
</script>

{#if canAccess}
    {#if isLoggedIn}
        <div class="export-section">
            <h3>Export My Events</h3>
            <p>Download your personal events as a JSONL file.</p>
            <button class="export-btn" on:click={exportMyEvents}>
                📤 Export My Events
            </button>
        </div>
    {/if}
    {#if canExportAll}
        <div class="export-section">
            <h3>Export All Events</h3>
            <p>
                Download the complete database as a JSONL file. This includes
                all events from all users.
            </p>
            <button class="export-btn" on:click={exportAllEvents}>
                📤 Export All Events
            </button>
        </div>
    {/if}
{:else}
    <div class="login-prompt">
        <p>Please log in to access export functionality.</p>
        <button class="login-btn" on:click={openLoginModal}>Log In</button>
    </div>
{/if}

<style>
    .export-section {
        border-radius: 8px;
        padding: 1em;
        margin: 1em;
        width: 100%;
        max-width: 32em;
        box-sizing: border-box;
        background-color: var(--card-bg);
    }

    .export-section h3 {
        margin: 0 0 1rem 0;
        color: var(--text-color);
        font-size: 1.2rem;
        font-weight: 600;
    }

    .export-section p {
        margin: 0 0 1.5rem 0;
        color: var(--text-color);
        opacity: 0.8;
        line-height: 1.4;
    }

    .export-btn {
        background-color: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.75em 1.5em;
        border-radius: 0.5em;
        cursor: pointer;
        font-weight: bold;
        font-size: 0.9em;
        transition: background-color 0.2s;
    }

    .export-btn:hover {
        background-color: var(--accent-hover-color);
    }

    .login-prompt {
        text-align: center;
        padding: 2em;
        background-color: var(--card-bg);
        border-radius: 8px;
        border: 1px solid var(--border-color);
        width: 100%;
        max-width: 32em;
        box-sizing: border-box;
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
        transition: background-color 0.2s;
    }

    .login-btn:hover {
        background-color: var(--accent-hover-color);
    }
</style>
