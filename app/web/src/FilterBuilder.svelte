<script>
    import { createEventDispatcher, onDestroy } from "svelte";
    import { KIND_NAMES, isValidPubkey, isValidEventId, isValidTagName, formatDateTimeLocal, parseDateTimeLocal } from "./helpers.tsx";

    const dispatch = createEventDispatcher();

    // Filter state
    export let searchText = "";
    export let selectedKinds = [];
    export let pubkeys = [];
    export let eventIds = [];
    export let tags = [];
    export let sinceTimestamp = null;
    export let untilTimestamp = null;
    export let limit = null;

    // JSON editor state
    export let showJsonEditor = false;
    let jsonEditorValue = "";
    let jsonError = "";

    // UI state
    let showKindsPicker = false;
    let kindSearchQuery = "";
    let newPubkey = "";
    let newEventId = "";
    let newTagName = "";
    let newTagValue = "";
    let pubkeyError = "";
    let eventIdError = "";
    let tagNameError = "";

    // Debounce timer
    let debounceTimer = null;
    const DEBOUNCE_MS = 1000;
    let initialized = false;

    onDestroy(() => {
        if (debounceTimer) clearTimeout(debounceTimer);
    });

    // Build filter object from current state
    function buildFilterObject() {
        const filter = {};
        if (selectedKinds.length > 0) filter.kinds = selectedKinds;
        if (pubkeys.length > 0) filter.authors = pubkeys;
        if (eventIds.length > 0) filter.ids = eventIds;
        if (sinceTimestamp) filter.since = sinceTimestamp;
        if (untilTimestamp) filter.until = untilTimestamp;
        if (limit) filter.limit = limit;
        if (searchText) filter.search = searchText;
        // Tags
        tags.forEach(tag => {
            const tagKey = `#${tag.name}`;
            if (!filter[tagKey]) filter[tagKey] = [];
            filter[tagKey].push(tag.value);
        });
        return filter;
    }

    // Update JSON editor when filter state changes
    $: if (showJsonEditor) {
        const filter = buildFilterObject();
        jsonEditorValue = JSON.stringify(filter, null, 2);
    }

    // Debounced auto-apply when any filter value changes (skip initial mount)
    $: {
        // Track all filter values
        const _ = [searchText, selectedKinds, pubkeys, eventIds, tags, sinceTimestamp, untilTimestamp, limit];
        if (initialized) {
            debouncedApply();
        } else {
            initialized = true;
        }
    }

    function debouncedApply() {
        if (debounceTimer) clearTimeout(debounceTimer);
        debounceTimer = setTimeout(() => {
            applyFilters();
        }, DEBOUNCE_MS);
    }

    function applyJsonFilter() {
        try {
            const parsed = JSON.parse(jsonEditorValue);
            jsonError = "";

            // Update state from parsed JSON
            selectedKinds = parsed.kinds || [];
            pubkeys = parsed.authors || [];
            eventIds = parsed.ids || [];
            sinceTimestamp = parsed.since || null;
            untilTimestamp = parsed.until || null;
            limit = parsed.limit || null;
            searchText = parsed.search || "";

            // Extract tags
            tags = [];
            Object.keys(parsed).forEach(key => {
                if (key.startsWith('#') && key.length === 2) {
                    const tagName = key.slice(1);
                    const values = Array.isArray(parsed[key]) ? parsed[key] : [parsed[key]];
                    values.forEach(value => {
                        tags.push({ name: tagName, value: String(value) });
                    });
                }
            });
            tags = tags; // trigger reactivity

            // Apply immediately (skip debounce)
            if (debounceTimer) clearTimeout(debounceTimer);
            applyFilters();
        } catch (e) {
            jsonError = "Invalid JSON: " + e.message;
        }
    }

    // Get all available kinds as array
    $: availableKinds = Object.entries(KIND_NAMES).map(([kind, name]) => ({
        kind: parseInt(kind),
        name: name
    })).sort((a, b) => a.kind - b.kind);

    // Filter kinds by search query
    $: filteredKinds = availableKinds.filter(k => 
        k.kind.toString().includes(kindSearchQuery) || 
        k.name.toLowerCase().includes(kindSearchQuery.toLowerCase())
    );

    function toggleKind(kind) {
        if (selectedKinds.includes(kind)) {
            selectedKinds = selectedKinds.filter(k => k !== kind);
        } else {
            selectedKinds = [...selectedKinds, kind].sort((a, b) => a - b);
        }
    }

    function removeKind(kind) {
        selectedKinds = selectedKinds.filter(k => k !== kind);
    }

    function addPubkey() {
        const trimmed = newPubkey.trim();
        if (!trimmed) return;
        
        if (!isValidPubkey(trimmed)) {
            pubkeyError = "Invalid pubkey: must be 64 character hex string";
            return;
        }
        
        if (pubkeys.includes(trimmed)) {
            pubkeyError = "Pubkey already added";
            return;
        }
        
        pubkeys = [...pubkeys, trimmed];
        newPubkey = "";
        pubkeyError = "";
    }

    function removePubkey(pubkey) {
        pubkeys = pubkeys.filter(p => p !== pubkey);
    }

    function addEventId() {
        const trimmed = newEventId.trim();
        if (!trimmed) return;
        
        if (!isValidEventId(trimmed)) {
            eventIdError = "Invalid event ID: must be 64 character hex string";
            return;
        }
        
        if (eventIds.includes(trimmed)) {
            eventIdError = "Event ID already added";
            return;
        }
        
        eventIds = [...eventIds, trimmed];
        newEventId = "";
        eventIdError = "";
    }

    function removeEventId(eventId) {
        eventIds = eventIds.filter(id => id !== eventId);
    }

    function addTag() {
        const trimmedName = newTagName.trim();
        const trimmedValue = newTagValue.trim();
        
        if (!trimmedName || !trimmedValue) return;
        
        if (!isValidTagName(trimmedName)) {
            tagNameError = "Invalid tag name: must be single letter a-z or A-Z";
            return;
        }
        
        // Check if this exact tag already exists
        if (tags.some(t => t.name === trimmedName && t.value === trimmedValue)) {
            tagNameError = "Tag already added";
            return;
        }
        
        tags = [...tags, { name: trimmedName, value: trimmedValue }];
        newTagName = "";
        newTagValue = "";
        tagNameError = "";
    }

    function removeTag(index) {
        tags = tags.filter((_, i) => i !== index);
    }

    function clearAllFilters() {
        searchText = "";
        selectedKinds = [];
        pubkeys = [];
        eventIds = [];
        tags = [];
        sinceTimestamp = null;
        untilTimestamp = null;
        limit = null;
        dispatch("clear");
    }

    function applyFilters() {
        dispatch("apply", {
            searchText,
            selectedKinds,
            pubkeys,
            eventIds,
            tags,
            sinceTimestamp,
            untilTimestamp,
            limit
        });
    }

    // Format timestamp for input
    function getFormattedSince() {
        return sinceTimestamp ? formatDateTimeLocal(sinceTimestamp) : "";
    }

    function getFormattedUntil() {
        return untilTimestamp ? formatDateTimeLocal(untilTimestamp) : "";
    }

    function handleSinceChange(event) {
        const value = event.target.value;
        sinceTimestamp = value ? parseDateTimeLocal(value) : null;
    }

    function handleUntilChange(event) {
        const value = event.target.value;
        untilTimestamp = value ? parseDateTimeLocal(value) : null;
    }
</script>

<div class="filter-builder">
    <div class="filter-content">
        <div class="filter-grid">
            <!-- Search text -->
            <label for="search-text">Search Text (NIP-50)</label>
            <div class="field-content">
                <input
                    id="search-text"
                    type="text"
                    bind:value={searchText}
                    placeholder="Search events..."
                    class="filter-input"
                />
            </div>

        <!-- Kinds picker -->
        <label>Event Kinds</label>
        <div class="field-content">
            <button
                class="picker-toggle-btn"
                on:click={() => showKindsPicker = !showKindsPicker}
            >
                {showKindsPicker ? "▼" : "▶"} Select Kinds ({selectedKinds.length} selected)
            </button>

            {#if showKindsPicker}
                <div class="kinds-picker">
                    <input
                        type="text"
                        bind:value={kindSearchQuery}
                        placeholder="Search kinds..."
                        class="filter-input kind-search"
                    />
                    <div class="kinds-list">
                        {#each filteredKinds as { kind, name }}
                            <label class="kind-checkbox">
                                <input
                                    type="checkbox"
                                    checked={selectedKinds.includes(kind)}
                                    on:change={() => toggleKind(kind)}
                                />
                                <span class="kind-number">{kind}</span>
                                <span class="kind-name">{name}</span>
                            </label>
                        {/each}
                    </div>
                </div>
            {/if}

            {#if selectedKinds.length > 0}
                <div class="chips-container">
                    {#each selectedKinds as kind}
                        <div class="chip">
                            <span class="chip-text">{kind}: {KIND_NAMES[kind] || `Kind ${kind}`}</span>
                            <button class="chip-remove" on:click={() => removeKind(kind)}>×</button>
                        </div>
                    {/each}
                </div>
            {/if}
        </div>

        <!-- Authors/Pubkeys -->
        <label>Authors (Pubkeys)</label>
        <div class="field-content">
            <div class="input-group">
                <input
                    type="text"
                    bind:value={newPubkey}
                    placeholder="64 character hex pubkey..."
                    class="filter-input"
                    maxlength="64"
                    on:keydown={(e) => e.key === 'Enter' && addPubkey()}
                />
                <button class="add-btn" on:click={addPubkey}>Add</button>
            </div>
            {#if pubkeyError}
                <div class="error-message">{pubkeyError}</div>
            {/if}
            {#if pubkeys.length > 0}
                <div class="list-items">
                    {#each pubkeys as pubkey}
                        <div class="list-item">
                            <span class="list-item-text">{pubkey}</span>
                            <button class="list-item-remove" on:click={() => removePubkey(pubkey)}>×</button>
                        </div>
                    {/each}
                </div>
            {/if}
        </div>

        <!-- Event IDs -->
        <label>Event IDs</label>
        <div class="field-content">
            <div class="input-group">
                <input
                    type="text"
                    bind:value={newEventId}
                    placeholder="64 character hex event ID..."
                    class="filter-input"
                    maxlength="64"
                    on:keydown={(e) => e.key === 'Enter' && addEventId()}
                />
                <button class="add-btn" on:click={addEventId}>Add</button>
            </div>
            {#if eventIdError}
                <div class="error-message">{eventIdError}</div>
            {/if}
            {#if eventIds.length > 0}
                <div class="list-items">
                    {#each eventIds as eventId}
                        <div class="list-item">
                            <span class="list-item-text">{eventId}</span>
                            <button class="list-item-remove" on:click={() => removeEventId(eventId)}>×</button>
                        </div>
                    {/each}
                </div>
            {/if}
        </div>

        <!-- Tags -->
        <label>Tags (#e, #p, #a)</label>
        <div class="field-content">
            <div class="tag-input-group">
                <span class="hash-prefix">#</span>
                <input
                    type="text"
                    bind:value={newTagName}
                    placeholder="Tag"
                    class="filter-input tag-name-input"
                    maxlength="1"
                />
                <input
                    type="text"
                    bind:value={newTagValue}
                    placeholder="Value..."
                    class="filter-input tag-value-input"
                    on:keydown={(e) => e.key === 'Enter' && addTag()}
                />
                <button class="add-btn" on:click={addTag}>Add</button>
            </div>
            {#if tagNameError}
                <div class="error-message">{tagNameError}</div>
            {/if}
            {#if tags.length > 0}
                <div class="list-items">
                    {#each tags as tag, index}
                        <div class="list-item">
                            <span class="list-item-text">#{tag.name}: {tag.value}</span>
                            <button class="list-item-remove" on:click={() => removeTag(index)}>×</button>
                        </div>
                    {/each}
                </div>
            {/if}
        </div>

        <!-- Since timestamp -->
        <label for="since-timestamp">Since</label>
        <div class="field-content timestamp-field">
            <input
                id="since-timestamp"
                type="datetime-local"
                value={getFormattedSince()}
                on:change={handleSinceChange}
                class="filter-input"
            />
            {#if sinceTimestamp}
                <button class="clear-timestamp-btn" on:click={() => sinceTimestamp = null}>×</button>
            {/if}
        </div>

        <!-- Until timestamp -->
        <label for="until-timestamp">Until</label>
        <div class="field-content timestamp-field">
            <input
                id="until-timestamp"
                type="datetime-local"
                value={getFormattedUntil()}
                on:change={handleUntilChange}
                class="filter-input"
            />
            {#if untilTimestamp}
                <button class="clear-timestamp-btn" on:click={() => untilTimestamp = null}>×</button>
            {/if}
        </div>

        <!-- Limit -->
        <label for="limit">Limit</label>
        <div class="field-content">
            <input
                id="limit"
                type="number"
                bind:value={limit}
                placeholder="Max events to return"
                class="filter-input"
                min="1"
            />
        </div>

        <!-- JSON Editor (shown when toggled) - spans both columns -->
        {#if showJsonEditor}
            <div class="json-editor-section">
                <label for="json-editor">Filter JSON</label>
                <textarea
                    id="json-editor"
                    class="json-editor"
                    bind:value={jsonEditorValue}
                    placeholder={'{"kinds": [1], "limit": 100}'}
                    rows="8"
                ></textarea>
                {#if jsonError}
                    <div class="json-error">{jsonError}</div>
                {/if}
                <button class="apply-json-btn" on:click={applyJsonFilter}>Apply JSON</button>
            </div>
        {/if}
        </div>
    </div>
    <div class="clear-column">
        <button class="clear-all-btn" on:click={clearAllFilters} title="Clear all filters">🧹</button>
        <div class="spacer"></div>
        <button
            class="json-toggle-btn"
            class:active={showJsonEditor}
            on:click={() => dispatch("toggleJson")}
            title="Edit filter JSON"
        >&lt;/&gt;</button>
    </div>
</div>

<style>
    .filter-builder {
        padding: 1em;
        background: var(--bg-color);
        border-bottom: 1px solid var(--border-color);
        display: flex;
        gap: 1em;
    }

    .filter-content {
        flex: 1;
        min-width: 0;
    }

    .clear-column {
        display: flex;
        flex-direction: column;
        gap: 0.5em;
        flex-shrink: 0;
        width: 2.5em;
    }

    .clear-column .spacer {
        flex: 1;
    }

    .clear-all-btn,
    .json-toggle-btn {
        background: var(--secondary);
        color: var(--text-color);
        border: none;
        padding: 0;
        border-radius: 4px;
        cursor: pointer;
        font-size: 1em;
        transition: filter 0.2s, background-color 0.2s;
        width: 100%;
        aspect-ratio: 1;
        display: flex;
        align-items: center;
        justify-content: center;
        box-sizing: border-box;
    }

    .clear-all-btn {
        background: var(--danger);
    }

    .clear-all-btn:hover {
        filter: brightness(1.2);
    }

    .json-toggle-btn {
        font-family: monospace;
        font-weight: 600;
        background: var(--primary);
    }

    .json-toggle-btn:hover {
        background: var(--accent-hover-color);
    }

    .json-toggle-btn.active {
        background: var(--accent-hover-color);
    }

    .filter-grid {
        display: grid;
        grid-template-columns: auto 1fr;
        gap: 0.5em 1em;
        align-items: start;
    }

    .filter-grid > label {
        font-weight: 600;
        color: var(--text-color);
        font-size: 0.9em;
        padding-top: 0.6em;
        white-space: nowrap;
    }

    .field-content {
        min-width: 0;
    }

    .filter-input {
        width: 100%;
        padding: 0.6em;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--input-bg);
        color: var(--input-text-color);
        font-size: 0.9em;
        box-sizing: border-box;
    }

    .filter-input:focus {
        outline: none;
        border-color: var(--primary);
        box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.15);
    }

    .picker-toggle-btn {
        width: 100%;
        padding: 0.6em;
        background: var(--secondary);
        color: var(--text-color);
        border: 1px solid var(--border-color);
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        text-align: left;
        transition: background-color 0.2s;
    }

    .picker-toggle-btn:hover {
        background: var(--accent-hover-color);
    }

    .kinds-picker {
        margin-top: 0.5em;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        padding: 0.5em;
        background: var(--card-bg);
    }

    .kind-search {
        margin-bottom: 0.5em;
    }

    .kinds-list {
        max-height: 300px;
        overflow-y: auto;
    }

    .kind-checkbox {
        display: flex;
        align-items: center;
        padding: 0.4em;
        cursor: pointer;
        border-radius: 4px;
        transition: background-color 0.2s;
    }

    .kind-checkbox:hover {
        background: var(--bg-color);
    }

    .kind-checkbox input[type="checkbox"] {
        margin-right: 0.5em;
        cursor: pointer;
    }

    .kind-number {
        background: var(--primary);
        color: var(--text-color);
        padding: 0.1em 0.4em;
        border-radius: 3px;
        font-size: 0.8em;
        font-weight: 600;
        font-family: monospace;
        margin-right: 0.5em;
        min-width: 40px;
        text-align: center;
        display: inline-block;
    }

    .kind-name {
        font-size: 0.85em;
        color: var(--text-color);
    }

    .chips-container {
        display: flex;
        flex-wrap: wrap;
        gap: 0.5em;
        margin-top: 0.5em;
    }

    .chip {
        display: inline-flex;
        align-items: center;
        background: var(--primary);
        color: var(--text-color);
        padding: 0.2em 0.5em;
        border-radius: 0.5em;
        font-size: 0.7em;
        font-weight: 500;
        text-transform: uppercase;
        letter-spacing: 0.5px;
        gap: 0.4em;
    }

    .chip-text {
        line-height: 1;
    }

    .chip-remove {
        background: transparent;
        border: none;
        color: var(--text-color);
        cursor: pointer;
        padding: 0;
        font-size: 1em;
        line-height: 1;
        opacity: 0.8;
        transition: opacity 0.2s;
    }

    .chip-remove:hover {
        opacity: 1;
    }

    .input-group {
        display: flex;
        gap: 0.5em;
    }

    .input-group .filter-input {
        flex: 1;
    }

    .add-btn {
        background: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.6em 1.2em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        font-weight: 600;
        transition: background-color 0.2s;
        white-space: nowrap;
    }

    .add-btn:hover {
        background: var(--accent-hover-color);
    }

    .error-message {
        color: var(--danger);
        font-size: 0.85em;
        margin-top: 0.25em;
    }

    .list-items {
        margin-top: 0.5em;
        display: flex;
        flex-direction: column;
        gap: 0.5em;
    }

    .list-item {
        display: flex;
        align-items: center;
        padding: 0.5em;
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 4px;
        gap: 0.5em;
    }

    .list-item-text {
        flex: 1;
        font-family: monospace;
        font-size: 0.85em;
        color: var(--text-color);
        word-break: break-all;
    }

    .list-item-remove {
        background: var(--danger);
        color: var(--text-color);
        border: none;
        padding: 0.25em 0.5em;
        border-radius: 3px;
        cursor: pointer;
        font-size: 1.2em;
        line-height: 1;
        transition: background-color 0.2s;
    }

    .list-item-remove:hover {
        filter: brightness(0.9);
    }

    .tag-input-group {
        display: flex;
        gap: 0.5em;
        align-items: center;
    }

    .hash-prefix {
        font-weight: 700;
        font-size: 1.2em;
        color: var(--text-color);
    }

    .tag-name-input {
        width: 50px;
    }

    .tag-value-input {
        flex: 1;
    }

    .timestamp-field {
        position: relative;
        display: flex;
        align-items: center;
        gap: 0.5em;
    }

    .timestamp-field .filter-input {
        flex: 1;
    }

    .clear-timestamp-btn {
        background: var(--danger);
        color: var(--text-color);
        border: none;
        padding: 0.25em 0.5em;
        border-radius: 3px;
        cursor: pointer;
        font-size: 1em;
        line-height: 1;
        transition: background-color 0.2s;
        flex-shrink: 0;
    }

    .clear-timestamp-btn:hover {
        filter: brightness(0.9);
    }

    .json-editor-section {
        grid-column: 1 / -1;
        margin-top: 0.5em;
        padding-top: 1em;
        border-top: 1px solid var(--border-color);
    }

    .json-editor-section label {
        display: block;
        font-weight: 600;
        color: var(--text-color);
        font-size: 0.9em;
        margin-bottom: 0.5em;
    }

    .json-editor {
        width: 100%;
        padding: 0.6em;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--input-bg);
        color: var(--input-text-color);
        font-family: monospace;
        font-size: 0.85em;
        resize: vertical;
        box-sizing: border-box;
    }

    .json-editor:focus {
        outline: none;
        border-color: var(--primary);
        box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.15);
    }

    .json-error {
        color: var(--danger);
        font-size: 0.85em;
        margin-top: 0.25em;
    }

    .apply-json-btn {
        margin-top: 0.5em;
        background: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        font-weight: 600;
        transition: background-color 0.2s;
    }

    .apply-json-btn:hover {
        background: var(--accent-hover-color);
    }

    /* Responsive design */
    @media (max-width: 768px) {
        .filter-grid {
            grid-template-columns: 1fr;
        }

        .filter-grid > label {
            padding-top: 0;
            padding-bottom: 0.25em;
        }
    }
</style>

