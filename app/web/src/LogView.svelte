<script>
    import { createEventDispatcher, onMount, onDestroy } from "svelte";

    export let isLoggedIn = false;
    export let userRole = "";
    export let userSigner = null;

    const dispatch = createEventDispatcher();

    let logs = [];
    let isLoading = false;
    let hasMore = true;
    let offset = 0;
    let totalLogs = 0;
    let error = "";
    let currentLogLevel = "info";
    let selectedLevel = "info";

    const LOG_LEVELS = ["trace", "debug", "info", "warn", "error", "fatal"];
    const LIMIT = 100;

    let scrollContainer;
    let loadMoreTrigger;
    let observer;

    $: canAccess = isLoggedIn && userRole === "owner";

    onMount(() => {
        if (canAccess) {
            loadLogs(true);
            loadLogLevel();
            setupIntersectionObserver();
        }
    });

    onDestroy(() => {
        if (observer) {
            observer.disconnect();
        }
    });

    $: if (canAccess && logs.length === 0 && !isLoading) {
        loadLogs(true);
        loadLogLevel();
    }

    function setupIntersectionObserver() {
        if (!loadMoreTrigger) return;

        observer = new IntersectionObserver(
            (entries) => {
                if (entries[0].isIntersecting && hasMore && !isLoading) {
                    loadMoreLogs();
                }
            },
            { threshold: 0.1 }
        );

        observer.observe(loadMoreTrigger);
    }

    async function createAuthHeader(method = "GET", path = "/api/logs") {
        if (!userSigner) return null;

        try {
            const now = Math.floor(Date.now() / 1000);
            const authEvent = {
                kind: 27235,
                created_at: now,
                tags: [
                    ["u", `${window.location.origin}${path}`],
                    ["method", method],
                ],
                content: "",
            };

            const signedEvent = await userSigner.signEvent(authEvent);
            return btoa(JSON.stringify(signedEvent));
        } catch (err) {
            console.error("Error creating auth header:", err);
            return null;
        }
    }

    async function loadLogs(refresh = false) {
        if (isLoading) return;

        isLoading = true;
        error = "";

        if (refresh) {
            offset = 0;
            logs = [];
        }

        try {
            const path = `/api/logs?offset=${offset}&limit=${LIMIT}`;
            const authHeader = await createAuthHeader("GET", path);
            const url = `${window.location.origin}${path}`;
            const response = await fetch(url, {
                headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
            });

            if (!response.ok) {
                throw new Error(`Failed to load logs: ${response.statusText}`);
            }

            const data = await response.json();
            if (refresh) {
                logs = data.logs || [];
            } else {
                logs = [...logs, ...(data.logs || [])];
            }
            totalLogs = data.total || 0;
            hasMore = data.has_more || false;
            offset = logs.length;
        } catch (err) {
            console.error("Error loading logs:", err);
            error = err.message || "Failed to load logs";
        } finally {
            isLoading = false;
        }
    }

    function loadMoreLogs() {
        if (hasMore && !isLoading) {
            loadLogs(false);
        }
    }

    async function loadLogLevel() {
        try {
            const response = await fetch(`${window.location.origin}/api/logs/level`);
            if (response.ok) {
                const data = await response.json();
                currentLogLevel = data.level || "info";
                selectedLevel = currentLogLevel;
            }
        } catch (err) {
            console.error("Error loading log level:", err);
        }
    }

    async function setLogLevel() {
        if (selectedLevel === currentLogLevel) return;

        try {
            const authHeader = await createAuthHeader("POST", "/api/logs/level");
            const response = await fetch(`${window.location.origin}/api/logs/level`, {
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    ...(authHeader ? { Authorization: `Nostr ${authHeader}` } : {}),
                },
                body: JSON.stringify({ level: selectedLevel }),
            });

            if (!response.ok) {
                throw new Error(`Failed to set log level: ${response.statusText}`);
            }

            const data = await response.json();
            currentLogLevel = data.level;
            selectedLevel = currentLogLevel;
        } catch (err) {
            console.error("Error setting log level:", err);
            error = err.message || "Failed to set log level";
            selectedLevel = currentLogLevel;
        }
    }

    async function clearLogs() {
        if (!confirm("Are you sure you want to clear all logs?")) return;

        try {
            const authHeader = await createAuthHeader("POST", "/api/logs/clear");
            const response = await fetch(`${window.location.origin}/api/logs/clear`, {
                method: "POST",
                headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
            });

            if (!response.ok) {
                throw new Error(`Failed to clear logs: ${response.statusText}`);
            }

            logs = [];
            offset = 0;
            hasMore = false;
            totalLogs = 0;
        } catch (err) {
            console.error("Error clearing logs:", err);
            error = err.message || "Failed to clear logs";
        }
    }

    function formatTimestamp(timestamp) {
        if (!timestamp) return "";
        const date = new Date(timestamp);
        return date.toLocaleString();
    }

    function getLevelClass(level) {
        switch (level?.toUpperCase()) {
            case "TRC":
            case "TRACE":
                return "level-trace";
            case "DBG":
            case "DEBUG":
                return "level-debug";
            case "INF":
            case "INFO":
                return "level-info";
            case "WRN":
            case "WARN":
                return "level-warn";
            case "ERR":
            case "ERROR":
                return "level-error";
            case "FTL":
            case "FATAL":
                return "level-fatal";
            default:
                return "level-info";
        }
    }

    function openLoginModal() {
        dispatch("openLoginModal");
    }
</script>

{#if canAccess}
    <div class="log-view">
        <div class="header-section">
            <h3>Logs</h3>
            <div class="header-controls">
                <div class="level-selector">
                    <label for="log-level">Level:</label>
                    <select
                        id="log-level"
                        bind:value={selectedLevel}
                        on:change={setLogLevel}
                    >
                        {#each LOG_LEVELS as level}
                            <option value={level}>{level}</option>
                        {/each}
                    </select>
                </div>
                <button class="clear-btn" on:click={clearLogs} disabled={isLoading || logs.length === 0}>
                    Clear
                </button>
                <button class="refresh-btn" on:click={() => loadLogs(true)} disabled={isLoading}>
                    🔄 {isLoading ? "Loading..." : "Refresh"}
                </button>
            </div>
        </div>

        {#if error}
            <div class="error-message">{error}</div>
        {/if}

        <div class="log-info">
            Showing {logs.length} of {totalLogs} logs (Level: {currentLogLevel})
        </div>

        <div class="log-list" bind:this={scrollContainer}>
            {#if logs.length === 0 && !isLoading}
                <div class="empty-state">
                    <p>No logs available.</p>
                </div>
            {:else}
                {#each logs as log}
                    <div class="log-entry">
                        <span class="log-timestamp">{formatTimestamp(log.timestamp)}</span>
                        <span class="log-level {getLevelClass(log.level)}">{log.level}</span>
                        {#if log.file}
                            <span class="log-location">{log.file}:{log.line}</span>
                        {/if}
                        <span class="log-message">{log.message}</span>
                    </div>
                {/each}
                <div bind:this={loadMoreTrigger} class="load-more-trigger">
                    {#if isLoading}
                        <span>Loading more...</span>
                    {:else if hasMore}
                        <span>Scroll for more</span>
                    {:else}
                        <span>End of logs</span>
                    {/if}
                </div>
            {/if}
        </div>
    </div>
{:else}
    <div class="login-prompt">
        <p>Log viewer is only available to relay owners.</p>
        {#if !isLoggedIn}
            <button class="login-btn" on:click={openLoginModal}>Log In</button>
        {:else}
            <p class="access-denied">Your role ({userRole}) does not have access to this feature.</p>
        {/if}
    </div>
{/if}

<style>
    .log-view {
        padding: 1em;
        box-sizing: border-box;
        width: 100%;
    }

    .header-section {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 1em;
        flex-wrap: wrap;
        gap: 0.5em;
    }

    .header-section h3 {
        margin: 0;
        color: var(--text-color);
    }

    .header-controls {
        display: flex;
        align-items: center;
        gap: 0.75em;
        flex-wrap: wrap;
    }

    .level-selector {
        display: flex;
        align-items: center;
        gap: 0.5em;
    }

    .level-selector label {
        color: var(--text-color);
        font-size: 0.9em;
    }

    .level-selector select {
        padding: 0.4em 0.6em;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background-color: var(--card-bg);
        color: var(--text-color);
        font-size: 0.9em;
    }

    .clear-btn {
        background-color: transparent;
        border: 1px solid var(--warning);
        color: var(--warning);
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
    }

    .clear-btn:hover:not(:disabled) {
        background-color: var(--warning);
        color: var(--text-color);
    }

    .clear-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
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

    .log-info {
        font-size: 0.85em;
        color: var(--text-color);
        opacity: 0.7;
        margin-bottom: 0.75em;
    }

    .log-list {
        display: flex;
        flex-direction: column;
        gap: 0.25em;
        width: 100%;
    }

    .log-entry {
        display: flex;
        align-items: flex-start;
        gap: 0.75em;
        padding: 0.5em 0.75em;
        background-color: var(--card-bg);
        border-radius: 4px;
        font-family: monospace;
        font-size: 0.85em;
        word-break: break-word;
    }

    .log-timestamp {
        color: var(--text-color);
        opacity: 0.6;
        white-space: nowrap;
        flex-shrink: 0;
    }

    .log-level {
        font-weight: bold;
        padding: 0.1em 0.4em;
        border-radius: 3px;
        text-transform: uppercase;
        flex-shrink: 0;
        min-width: 3.5em;
        text-align: center;
    }

    .level-trace {
        background-color: #6c757d;
        color: white;
    }

    .level-debug {
        background-color: #17a2b8;
        color: white;
    }

    .level-info {
        background-color: #28a745;
        color: white;
    }

    .level-warn {
        background-color: #ffc107;
        color: #212529;
    }

    .level-error {
        background-color: #dc3545;
        color: white;
    }

    .level-fatal {
        background-color: #721c24;
        color: white;
    }

    .log-location {
        color: var(--text-color);
        opacity: 0.5;
        flex-shrink: 0;
    }

    .log-message {
        color: var(--text-color);
        flex: 1;
    }

    .load-more-trigger {
        padding: 1em;
        text-align: center;
        color: var(--text-color);
        opacity: 0.6;
        font-size: 0.9em;
    }

    .empty-state {
        text-align: center;
        padding: 2em;
        color: var(--text-color);
        opacity: 0.7;
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

    .access-denied {
        font-size: 0.9em;
        opacity: 0.7;
    }

    @media (max-width: 600px) {
        .header-section {
            flex-direction: column;
            align-items: flex-start;
        }

        .header-controls {
            width: 100%;
            justify-content: flex-end;
        }

        .log-entry {
            flex-wrap: wrap;
        }

        .log-timestamp {
            width: 100%;
            margin-bottom: 0.25em;
        }
    }
</style>
