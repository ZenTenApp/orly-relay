<script>
    export let isLoggedIn = false;
    export let userRole = "";
    export let userPubkey = "";
    export let filteredEvents = [];
    export let expandedEvents = new Set();
    export let isLoadingEvents = false;
    export let showOnlyMyEvents = false;

    import { createEventDispatcher } from "svelte";
    const dispatch = createEventDispatcher();

    function handleScroll(event) {
        dispatch("scroll", event);
    }

    function toggleEventExpansion(eventId) {
        dispatch("toggleEventExpansion", eventId);
    }

    function deleteEvent(eventId) {
        dispatch("deleteEvent", eventId);
    }

    function copyEventToClipboard(event, e) {
        dispatch("copyEventToClipboard", { event, e });
    }

    function handleToggleChange() {
        dispatch("toggleChange");
    }

    function loadAllEvents(refresh, authors) {
        dispatch("loadAllEvents", { refresh, authors });
    }

    function truncatePubkey(pubkey) {
        if (!pubkey) return "";
        return pubkey.slice(0, 8) + "..." + pubkey.slice(-8);
    }

    function getKindName(kind) {
        const kindNames = {
            0: "Profile",
            1: "Text Note",
            2: "Recommend Relay",
            3: "Contacts",
            4: "Encrypted DM",
            5: "Delete",
            6: "Repost",
            7: "Reaction",
            8: "Badge Award",
            16: "Generic Repost",
            40: "Channel Creation",
            41: "Channel Metadata",
            42: "Channel Message",
            43: "Channel Hide Message",
            44: "Channel Mute User",
            1984: "Reporting",
            9734: "Zap Request",
            9735: "Zap",
            10000: "Mute List",
            10001: "Pin List",
            10002: "Relay List",
            22242: "Client Auth",
            24133: "Nostr Connect",
            27235: "HTTP Auth",
            30000: "Categorized People",
            30001: "Categorized Bookmarks",
            30008: "Profile Badges",
            30009: "Badge Definition",
            30017: "Create or update a stall",
            30018: "Create or update a product",
            30023: "Long-form Content",
            30024: "Draft Long-form Content",
            30078: "Application-specific Data",
            30311: "Live Event",
            30315: "User Statuses",
            30402: "Classified Listing",
            30403: "Draft Classified Listing",
            31922: "Date-Based Calendar Event",
            31923: "Time-Based Calendar Event",
            31924: "Calendar",
            31925: "Calendar Event RSVP",
            31989: "Handler recommendation",
            31990: "Handler information",
            34550: "Community Definition",
        };
        return kindNames[kind] || `Kind ${kind}`;
    }

    function formatTimestamp(timestamp) {
        return new Date(timestamp * 1000).toLocaleString();
    }

    function truncateContent(content) {
        if (!content) return "";
        return content.length > 100 ? content.slice(0, 100) + "..." : content;
    }
</script>

<div class="events-view-container">
    {#if isLoggedIn && (userRole === "read" || userRole === "write" || userRole === "admin" || userRole === "owner")}
        <div class="events-view-content" on:scroll={handleScroll}>
            {#if filteredEvents.length > 0}
                {#each filteredEvents as event}
                    <div
                        class="events-view-item"
                        class:expanded={expandedEvents.has(event.id)}
                    >
                        <div
                            class="events-view-row"
                            on:click={() => toggleEventExpansion(event.id)}
                            on:keydown={(e) =>
                                e.key === "Enter" &&
                                toggleEventExpansion(event.id)}
                            role="button"
                            tabindex="0"
                        >
                            <div class="events-view-avatar">
                                <div class="avatar-placeholder">👤</div>
                            </div>
                            <div class="events-view-info">
                                <div class="events-view-author">
                                    {truncatePubkey(event.pubkey)}
                                </div>
                                <div class="events-view-kind">
                                    <span
                                        class="kind-number"
                                        class:delete-event={event.kind === 5}
                                        >{event.kind}</span
                                    >
                                    <span class="kind-name"
                                        >{getKindName(event.kind)}</span
                                    >
                                </div>
                            </div>
                            <div class="events-view-content">
                                <div class="event-timestamp">
                                    {formatTimestamp(event.created_at)}
                                </div>
                                {#if event.kind === 5}
                                    <div class="delete-event-info">
                                        <span class="delete-event-label"
                                            >🗑️ Delete Event</span
                                        >
                                        {#if event.tags && event.tags.length > 0}
                                            <div class="delete-targets">
                                                {#each event.tags.filter((tag) => tag[0] === "e") as eTag}
                                                    <span class="delete-target"
                                                        >Target: {eTag[1].slice(
                                                            0,
                                                            8,
                                                        )}...{eTag[1].slice(
                                                            -8,
                                                        )}</span
                                                    >
                                                {/each}
                                            </div>
                                        {/if}
                                    </div>
                                {:else}
                                    <div class="event-content-single-line">
                                        {truncateContent(event.content)}
                                    </div>
                                {/if}
                            </div>
                            {#if event.kind !== 5 && (userRole === "admin" || userRole === "owner" || (userRole === "write" && event.pubkey && event.pubkey === userPubkey))}
                                <button
                                    class="delete-btn"
                                    on:click|stopPropagation={() =>
                                        deleteEvent(event.id)}
                                >
                                    🗑️
                                </button>
                            {/if}
                        </div>
                        {#if expandedEvents.has(event.id)}
                            <div class="events-view-details">
                                <div class="json-container">
                                    <pre class="event-json">{JSON.stringify(
                                            event,
                                            null,
                                            2,
                                        )}</pre>
                                    <button
                                        class="copy-json-btn"
                                        on:click|stopPropagation={(e) =>
                                            copyEventToClipboard(event, e)}
                                        title="Copy minified JSON to clipboard"
                                    >
                                        📋
                                    </button>
                                </div>
                            </div>
                        {/if}
                    </div>
                {/each}
            {:else if !isLoadingEvents}
                <div class="no-events">
                    <p>No events found.</p>
                </div>
            {/if}

            {#if isLoadingEvents}
                <div class="loading-events">
                    <div class="spinner"></div>
                    <p>Loading events...</p>
                </div>
            {/if}
        </div>
    {:else}
        <div class="permission-denied">
            <p>
                ❌ Read, write, admin, or owner permission required to view all
                events.
            </p>
        </div>
    {/if}
    {#if isLoggedIn && (userRole === "read" || userRole === "write" || userRole === "admin" || userRole === "owner")}
        <div class="events-view-header">
            <div class="events-view-toggle">
                <label class="toggle-container">
                    <input
                        type="checkbox"
                        bind:checked={showOnlyMyEvents}
                        on:change={() => handleToggleChange()}
                    />
                    <span class="toggle-slider"></span>
                    <span class="toggle-label">Only show my events</span>
                </label>
            </div>
            <div class="events-view-buttons">
                <button
                    class="refresh-btn"
                    on:click={() => {
                        const authors =
                            showOnlyMyEvents && userPubkey
                                ? [userPubkey]
                                : null;
                        loadAllEvents(false, authors);
                    }}
                    disabled={isLoadingEvents}
                >
                    🔄 Load More
                </button>
                <button
                    class="reload-btn"
                    on:click={() => {
                        const authors =
                            showOnlyMyEvents && userPubkey
                                ? [userPubkey]
                                : null;
                        loadAllEvents(true, authors);
                    }}
                    disabled={isLoadingEvents}
                >
                    {#if isLoadingEvents}
                        <div class="spinner"></div>
                    {:else}
                        🔄
                    {/if}
                </button>
            </div>
        </div>
    {/if}
</div>

<style>
    .events-view-container {
        width: 100%;
        height: 100%;
        display: flex;
        flex-direction: column;
        box-sizing: border-box;
    }

    .events-view-content {
        flex: 1;
        overflow-y: auto;
        padding: 0;
    }

    .events-view-item {
        border: 0;
        margin: 0;
        transition: all 0.2s ease;
    }

    .events-view-item:hover {
        padding: 0;
    }

    .events-view-row {
        display: flex;
        align-items: center;
        padding: 0.5em;
        cursor: pointer;
        gap: 1em;
    }

    .events-view-avatar {
        flex-shrink: 0;
    }

    .avatar-placeholder {
        width: 40px;
        height: 40px;
        border-radius: 50%;
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 1.2em;
        border: 0;
    }

    .events-view-info {
        flex-shrink: 0;
        min-width: 120px;
    }

    .events-view-author {
        font-weight: 600;
        color: var(--text-color);
        font-size: 0.9em;
        font-family: monospace;
    }

    .events-view-kind {
        display: flex;
        align-items: center;
        gap: 0.5em;
        margin-top: 0.25em;
    }

    .kind-number {
        background: var(--primary);
        color: var(--text-color);
        padding: 0.1em 0.4em;
        border: 0;
        font-size: 0.7em;
        font-weight: 600;
        font-family: monospace;
    }

    .kind-number.delete-event {
        background: var(--danger);
    }

    .kind-name {
        font-size: 0.8em;
        color: var(--text-color);
        opacity: 0.8;
    }

    .event-timestamp {
        font-size: 0.8em;
        color: var(--text-color);
        opacity: 0.6;
        margin-bottom: 0.5em;
    }

    .delete-event-info {
        background: var(--danger-bg);
        padding: 0.5em;
        border-radius: 4px;
        border: 1px solid var(--danger);
    }

    .delete-event-label {
        font-weight: 600;
        color: var(--danger);
        display: block;
        margin-bottom: 0.25em;
    }

    .delete-targets {
        display: flex;
        flex-wrap: wrap;
        gap: 0.25em;
    }

    .delete-target {
        background: var(--danger);
        color: var(--text-color);
        padding: 0.1em 0.3em;
        border-radius: 0.2rem;
        font-size: 0.7em;
        font-family: monospace;
    }

    .event-content-single-line {
        color: var(--text-color);
        line-height: 1.4;
        word-wrap: break-word;
    }

    .delete-btn {
        background: var(--danger);
        color: var(--text-color);
        border: none;
        padding: 0.5em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        flex-shrink: 0;
        transition: background-color 0.2s;
    }

    .delete-btn:hover {
        background: var(--danger);
        filter: brightness(0.9);
    }

    .events-view-details {
        padding: 0;
        background: var(--bg-color);
    }

    .json-container {
        position: relative;
    }

    .event-json {
        background: var(--code-bg);
        padding: 1em;
        border: 0;
        font-size: 0.8em;
        line-height: 1.4;
        overflow-x: auto;
        margin: 0;
        color: var(--code-text);
    }

    .copy-json-btn {
        position: absolute;
        top: 1em;
        right: 1em;
        background: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 1em;
        cursor: pointer;
        font-size: 0.8em;
        opacity: 0.8;
        transition: opacity 0.2s;
    }

    .copy-json-btn:hover {
        opacity: 1;
    }

    .no-events {
        text-align: center;
        padding: 2em;
        color: var(--text-color);
        opacity: 0.7;
    }

    .loading-events {
        text-align: center;
        padding: 2em;
        color: var(--text-color);
    }

    .spinner {
        width: 20px;
        height: 20px;
        border: 0;
        border-radius: 50%;
        animation: spin 1s linear infinite;
        margin: 0 auto 1em;
    }

    @keyframes spin {
        0% {
            transform: rotate(0deg);
        }
        100% {
            transform: rotate(360deg);
        }
    }

    .permission-denied {
        text-align: center;
        padding: 2em;
        background-color: var(--card-bg);
        border-radius: 8px;
        border: 1px solid var(--border-color);
        color: var(--text-color);
    }

    .events-view-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.5em;
        border: 0;
        background: var(--header-bg);
    }

    .events-view-toggle {
        display: flex;
        align-items: center;
    }

    .toggle-container {
        display: flex;
        align-items: center;
        gap: 0.5em;
        cursor: pointer;
    }

    .toggle-container input[type="checkbox"] {
        display: none;
    }

    .toggle-slider {
        width: 40px;
        height: 20px;
        background: var(--border-color);
        border-radius: 10px;
        position: relative;
        transition: background 0.2s;
    }

    .toggle-slider::before {
        content: "";
        position: absolute;
        width: 16px;
        height: 16px;
        background: var(--text-color);
        border-radius: 50%;
        top: 2px;
        left: 2px;
        transition: transform 0.2s;
    }

    .toggle-container input:checked + .toggle-slider {
        background: var(--primary);
    }

    .toggle-container input:checked + .toggle-slider::before {
        transform: translateX(20px);
    }

    .toggle-label {
        font-size: 0.9em;
        color: var(--text-color);
    }

    .events-view-buttons {
        display: flex;
        gap: 0.5em;
    }

    .refresh-btn,
    .reload-btn {
        background: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        transition: background-color 0.2s;
        display: flex;
        align-items: center;
        gap: 0.25em;
    }

    .refresh-btn:hover:not(:disabled),
    .reload-btn:hover:not(:disabled) {
        background: var(--accent-hover-color);
    }

    .refresh-btn:disabled,
    .reload-btn:disabled {
        background: var(--secondary);
        cursor: not-allowed;
    }
</style>
