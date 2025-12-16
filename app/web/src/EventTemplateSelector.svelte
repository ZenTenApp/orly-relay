<script>
    import { createEventDispatcher } from "svelte";
    import { eventKinds, kindCategories, createTemplateEvent, searchEventKinds } from "./eventKinds.js";

    export let isOpen = false;
    export let userPubkey = "";

    const dispatch = createEventDispatcher();

    let searchQuery = "";
    let selectedCategory = "all";
    let filteredKinds = eventKinds;

    // Filter kinds based on search and category
    $: {
        let kinds = eventKinds;

        // Apply category filter
        const category = kindCategories.find(c => c.id === selectedCategory);
        if (category) {
            kinds = kinds.filter(category.filter);
        }

        // Apply search filter
        if (searchQuery.trim()) {
            const query = searchQuery.toLowerCase();
            kinds = kinds.filter(k =>
                k.name.toLowerCase().includes(query) ||
                k.description.toLowerCase().includes(query) ||
                k.kind.toString().includes(query) ||
                (k.nip && k.nip.includes(query))
            );
        }

        filteredKinds = kinds;
    }

    function selectKind(kindInfo) {
        const template = createTemplateEvent(kindInfo.kind, userPubkey);
        dispatch("select", {
            kind: kindInfo,
            template: template
        });
        closeModal();
    }

    function closeModal() {
        isOpen = false;
        searchQuery = "";
        selectedCategory = "all";
        dispatch("close");
    }

    function handleKeydown(event) {
        if (event.key === "Escape") {
            closeModal();
        }
    }

    function handleBackdropClick(event) {
        if (event.target === event.currentTarget) {
            closeModal();
        }
    }

    function getKindBadgeClass(kindInfo) {
        if (kindInfo.isAddressable) return "badge-addressable";
        if (kindInfo.isReplaceable) return "badge-replaceable";
        if (kindInfo.kind >= 20000 && kindInfo.kind < 30000) return "badge-ephemeral";
        return "badge-regular";
    }

    function getKindBadgeText(kindInfo) {
        if (kindInfo.isAddressable) return "Addressable";
        if (kindInfo.isReplaceable) return "Replaceable";
        if (kindInfo.kind >= 20000 && kindInfo.kind < 30000) return "Ephemeral";
        return "Regular";
    }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if isOpen}
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="modal-backdrop" on:click={handleBackdropClick}>
        <div class="modal-content">
            <div class="modal-header">
                <h2>Generate Event Template</h2>
                <button class="close-btn" on:click={closeModal}>&times;</button>
            </div>

            <div class="modal-filters">
                <div class="search-box">
                    <input
                        type="text"
                        placeholder="Search by name, description, or kind number..."
                        bind:value={searchQuery}
                        class="search-input"
                    />
                </div>

                <div class="category-tabs">
                    {#each kindCategories as category}
                        <button
                            class="category-tab"
                            class:active={selectedCategory === category.id}
                            on:click={() => selectedCategory = category.id}
                        >
                            {category.name}
                        </button>
                    {/each}
                </div>
            </div>

            <div class="modal-body">
                <div class="kinds-list">
                    {#if filteredKinds.length === 0}
                        <div class="no-results">
                            No event kinds found matching "{searchQuery}"
                        </div>
                    {:else}
                        {#each filteredKinds as kindInfo}
                            <button
                                class="kind-item"
                                on:click={() => selectKind(kindInfo)}
                            >
                                <div class="kind-header">
                                    <span class="kind-number">Kind {kindInfo.kind}</span>
                                    <span class="kind-badge {getKindBadgeClass(kindInfo)}">
                                        {getKindBadgeText(kindInfo)}
                                    </span>
                                    {#if kindInfo.nip && kindInfo.nip !== "XX"}
                                        <span class="nip-badge">NIP-{kindInfo.nip}</span>
                                    {/if}
                                </div>
                                <div class="kind-name">{kindInfo.name}</div>
                                <div class="kind-description">{kindInfo.description}</div>
                            </button>
                        {/each}
                    {/if}
                </div>
            </div>

            <div class="modal-footer">
                <span class="result-count">{filteredKinds.length} event type{filteredKinds.length !== 1 ? 's' : ''}</span>
                <button class="cancel-btn" on:click={closeModal}>Cancel</button>
            </div>
        </div>
    </div>
{/if}

<style>
    .modal-backdrop {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background: rgba(0, 0, 0, 0.7);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 1000;
    }

    .modal-content {
        background: var(--card-bg);
        border-radius: 0.5rem;
        width: 90%;
        max-width: 800px;
        max-height: 85vh;
        display: flex;
        flex-direction: column;
        box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
        border: 1px solid var(--border-color);
    }

    .modal-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 1rem 1.5rem;
        border-bottom: 1px solid var(--border-color);
    }

    .modal-header h2 {
        margin: 0;
        font-size: 1.25rem;
        color: var(--text-color);
    }

    .close-btn {
        background: none;
        border: none;
        font-size: 1.5rem;
        cursor: pointer;
        color: var(--text-color);
        padding: 0;
        width: 2rem;
        height: 2rem;
        display: flex;
        align-items: center;
        justify-content: center;
        border-radius: 0.25rem;
    }

    .close-btn:hover {
        background: var(--button-hover-bg);
    }

    .modal-filters {
        padding: 1rem 1.5rem;
        border-bottom: 1px solid var(--border-color);
    }

    .search-box {
        margin-bottom: 0.75rem;
    }

    .search-input {
        width: 100%;
        padding: 0.75rem;
        border: 1px solid var(--border-color);
        border-radius: 0.25rem;
        background: var(--input-bg);
        color: var(--input-text-color);
        font-size: 0.9rem;
    }

    .search-input:focus {
        outline: none;
        border-color: var(--accent-color);
    }

    .category-tabs {
        display: flex;
        flex-wrap: wrap;
        gap: 0.5rem;
    }

    .category-tab {
        padding: 0.4rem 0.75rem;
        border: 1px solid var(--border-color);
        border-radius: 1rem;
        background: transparent;
        color: var(--text-color);
        font-size: 0.75rem;
        cursor: pointer;
        transition: all 0.2s;
    }

    .category-tab:hover {
        background: var(--button-hover-bg);
    }

    .category-tab.active {
        background: var(--accent-color);
        border-color: var(--accent-color);
        color: white;
    }

    .modal-body {
        flex: 1;
        overflow-y: auto;
        padding: 1rem 1.5rem;
    }

    .kinds-list {
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
    }

    .kind-item {
        display: block;
        width: 100%;
        text-align: left;
        padding: 0.75rem 1rem;
        border: 1px solid var(--border-color);
        border-radius: 0.375rem;
        background: var(--bg-color);
        cursor: pointer;
        transition: all 0.2s;
    }

    .kind-item:hover {
        border-color: var(--accent-color);
        background: var(--button-hover-bg);
    }

    .kind-header {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        margin-bottom: 0.25rem;
    }

    .kind-number {
        font-family: monospace;
        font-size: 0.8rem;
        color: var(--accent-color);
        font-weight: bold;
    }

    .kind-badge {
        font-size: 0.65rem;
        padding: 0.15rem 0.4rem;
        border-radius: 0.25rem;
        font-weight: 500;
    }

    .badge-regular {
        background: #6c757d;
        color: white;
    }

    .badge-replaceable {
        background: #17a2b8;
        color: white;
    }

    .badge-ephemeral {
        background: #ffc107;
        color: black;
    }

    .badge-addressable {
        background: #28a745;
        color: white;
    }

    .nip-badge {
        font-size: 0.65rem;
        padding: 0.15rem 0.4rem;
        border-radius: 0.25rem;
        background: var(--primary);
        color: var(--text-color);
    }

    .kind-name {
        font-weight: 600;
        color: var(--text-color);
        margin-bottom: 0.25rem;
    }

    .kind-description {
        font-size: 0.85rem;
        color: var(--text-color);
        opacity: 0.7;
    }

    .no-results {
        text-align: center;
        padding: 2rem;
        color: var(--text-color);
        opacity: 0.6;
    }

    .modal-footer {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 1rem 1.5rem;
        border-top: 1px solid var(--border-color);
    }

    .result-count {
        font-size: 0.85rem;
        color: var(--text-color);
        opacity: 0.7;
    }

    .cancel-btn {
        padding: 0.5rem 1rem;
        border: 1px solid var(--border-color);
        border-radius: 0.25rem;
        background: var(--button-bg);
        color: var(--button-text);
        cursor: pointer;
        font-size: 0.9rem;
    }

    .cancel-btn:hover {
        background: var(--button-hover-bg);
    }

    @media (max-width: 640px) {
        .modal-content {
            width: 95%;
            max-height: 90vh;
        }

        .category-tabs {
            overflow-x: auto;
            flex-wrap: nowrap;
            padding-bottom: 0.5rem;
        }

        .category-tab {
            white-space: nowrap;
        }
    }
</style>
