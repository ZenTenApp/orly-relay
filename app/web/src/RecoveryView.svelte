<script>
    export let recoverySelectedKind = null;
    export let recoveryCustomKind = "";
    export let isLoadingRecovery = false;
    export let recoveryEvents = [];
    export let recoveryHasMore = false;

    import { createEventDispatcher } from "svelte";
    const dispatch = createEventDispatcher();

    const replaceableKinds = [
        { value: 0, label: "Profile (0)" },
        { value: 3, label: "Contacts (3)" },
        { value: 10000, label: "Mute List (10000)" },
        { value: 10001, label: "Pin List (10001)" },
        { value: 10002, label: "Relay List (10002)" },
        { value: 30000, label: "Categorized People (30000)" },
        { value: 30001, label: "Categorized Bookmarks (30001)" },
        { value: 30008, label: "Profile Badges (30008)" },
        { value: 30009, label: "Badge Definition (30009)" },
        { value: 30017, label: "Create or update a stall (30017)" },
        { value: 30018, label: "Create or update a product (30018)" },
        { value: 30023, label: "Long-form Content (30023)" },
        { value: 30024, label: "Draft Long-form Content (30024)" },
        { value: 30078, label: "Application-specific Data (30078)" },
        { value: 30311, label: "Live Event (30311)" },
        { value: 30315, label: "User Statuses (30315)" },
        { value: 30402, label: "Classified Listing (30402)" },
        { value: 30403, label: "Draft Classified Listing (30403)" },
        { value: 31922, label: "Date-Based Calendar Event (31922)" },
        { value: 31923, label: "Time-Based Calendar Event (31923)" },
        { value: 31924, label: "Calendar (31924)" },
        { value: 31925, label: "Calendar Event RSVP (31925)" },
        { value: 31989, label: "Handler recommendation (31989)" },
        { value: 31990, label: "Handler information (31990)" },
        { value: 34550, label: "Community Definition (34550)" },
    ];

    function selectRecoveryKind() {
        dispatch("selectRecoveryKind");
    }

    function handleCustomKindInput() {
        dispatch("handleCustomKindInput");
    }

    function loadRecoveryEvents() {
        dispatch("loadRecoveryEvents");
    }

    function repostEventToAll(event) {
        dispatch("repostEventToAll", event);
    }

    function repostEvent(event) {
        dispatch("repostEvent", event);
    }

    function copyEventToClipboard(event, e) {
        dispatch("copyEventToClipboard", { event, e });
    }

    function isCurrentVersion(event) {
        // This logic would need to be passed from parent or implemented here
        // For now, just return false for old versions
        return false;
    }
</script>

<div class="recovery-tab">
    <div>
        <h3>Event Recovery</h3>
        <p>Search and recover old versions of replaceable events</p>
    </div>

    <div class="recovery-controls-card">
        <div class="recovery-controls">
            <div class="kind-selector">
                <label for="recovery-kind">Select Event Kind:</label>
                <select
                    id="recovery-kind"
                    bind:value={recoverySelectedKind}
                    on:change={selectRecoveryKind}
                >
                    <option value={null}>Choose a replaceable kind...</option>
                    {#each replaceableKinds as kind}
                        <option value={kind.value}>{kind.label}</option>
                    {/each}
                </select>
            </div>

            <div class="custom-kind-input">
                <label for="custom-kind">Or enter custom kind number:</label>
                <input
                    id="custom-kind"
                    type="number"
                    bind:value={recoveryCustomKind}
                    on:input={handleCustomKindInput}
                    placeholder="e.g., 10001"
                    min="0"
                />
            </div>
        </div>
    </div>

    {#if (recoverySelectedKind !== null && recoverySelectedKind !== undefined && recoverySelectedKind >= 0) || (recoveryCustomKind !== "" && parseInt(recoveryCustomKind) >= 0)}
        <div class="recovery-results">
            {#if isLoadingRecovery}
                <div class="loading">Loading events...</div>
            {:else if recoveryEvents.length === 0}
                <div class="no-events">No events found for this kind</div>
            {:else}
                <div class="events-list">
                    {#each recoveryEvents as event}
                        {@const isCurrent = isCurrentVersion(event)}
                        <div class="event-item" class:old-version={!isCurrent}>
                            <div class="event-header">
                                <div class="event-header-left">
                                    <span class="event-kind">
                                        {#if isCurrent}
                                            Current Version{/if}</span
                                    >
                                    <span class="event-timestamp">
                                        {new Date(
                                            event.created_at * 1000,
                                        ).toLocaleString()}
                                    </span>
                                </div>
                                <div class="event-header-actions">
                                    {#if !isCurrent}
                                        <button
                                            class="repost-all-button"
                                            on:click={() =>
                                                repostEventToAll(event)}
                                        >
                                            🌐 Repost to All
                                        </button>
                                        <button
                                            class="repost-button"
                                            on:click={() => repostEvent(event)}
                                        >
                                            🔄 Repost
                                        </button>
                                    {/if}
                                    <button
                                        class="copy-json-btn"
                                        on:click|stopPropagation={(e) =>
                                            copyEventToClipboard(event, e)}
                                    >
                                        📋 Copy JSON
                                    </button>
                                </div>
                            </div>

                            <div class="event-content">
                                <pre class="event-json">{JSON.stringify(
                                        event,
                                        null,
                                        2,
                                    )}</pre>
                            </div>
                        </div>
                    {/each}
                </div>

                {#if recoveryHasMore}
                    <button
                        class="load-more"
                        on:click={loadRecoveryEvents}
                        disabled={isLoadingRecovery}
                    >
                        Load More Events
                    </button>
                {/if}
            {/if}
        </div>
    {/if}
</div>

<style>
    .recovery-tab {
        width: 100%;
        max-width: 1200px;
        margin: 0;
        padding: 20px;
        background: var(--header-bg);
        color: var(--text-color);
        border-radius: 8px;
        box-sizing: border-box;
    }

    .recovery-tab h3 {
        margin: 0 0 0.5rem 0;
        color: var(--text-color);
        font-size: 1.5rem;
        font-weight: 600;
    }

    .recovery-tab p {
        margin: 0 0 1.5rem 0;
        color: var(--text-color);
        opacity: 0.8;
        line-height: 1.4;
    }

    .recovery-controls-card {
        background-color: var(--card-bg);
        border-radius: 0.5em;
        padding: 1em;
        margin-bottom: 1.5rem;
    }

    .recovery-controls {
        display: flex;
        flex-direction: column;
        gap: 1em;
    }

    .kind-selector,
    .custom-kind-input {
        display: flex;
        flex-direction: column;
        gap: 0.5em;
    }

    .kind-selector label,
    .custom-kind-input label {
        font-weight: 600;
        color: var(--text-color);
    }

    .kind-selector select,
    .custom-kind-input input {
        padding: 0.5em;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--input-bg);
        color: var(--input-text-color);
        font-size: 0.9em;
    }

    .recovery-results {
        margin-top: 1.5rem;
    }

    .loading,
    .no-events {
        text-align: center;
        padding: 2em;
        color: var(--text-color);
        opacity: 0.7;
    }

    .events-list {
        display: flex;
        flex-direction: column;
        gap: 1em;
    }

    .event-item {
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 8px;
        padding: 1em;
    }

    .event-item.old-version {
        border-color: var(--warning);
        background: var(--warning-bg);
    }

    .event-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 1em;
        flex-wrap: wrap;
        gap: 1em;
    }

    .event-header-left {
        display: flex;
        flex-direction: column;
        gap: 0.25em;
    }

    .event-kind {
        font-weight: 600;
        color: var(--primary);
    }

    .event-timestamp {
        font-size: 0.9em;
        color: var(--text-color);
        opacity: 0.7;
    }

    .event-header-actions {
        display: flex;
        gap: 0.5em;
        flex-wrap: wrap;
    }

    .repost-all-button,
    .repost-button,
    .copy-json-btn {
        background: var(--accent-color);
        color: var(--accent-hover-color);
        border: none;
        padding: 0.5em;
        border-radius: 0.5em;
        cursor: pointer;
        font-size: 0.8em;
        transition: background-color 0.2s;
    }

    .repost-all-button:hover,
    .repost-button:hover,
    .copy-json-btn:hover {
        background: var(--accent-hover-color);
    }

    .event-content {
        margin-top: 1em;
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

    .load-more {
        width: 100%;
        padding: 12px;
        background: var(--primary);
        color: var(--text-color);
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 1em;
        margin-top: 20px;
        transition: background 0.2s ease;
    }

    .load-more:hover:not(:disabled) {
        background: var(--accent-hover-color);
    }

    .load-more:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }
</style>
