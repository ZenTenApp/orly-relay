<script>
    import { searchActive } from './stores.js';
    import { createEventDispatcher, onMount, tick } from 'svelte';

    const dispatch = createEventDispatcher();

    let searchQuery = "";
    let inputEl;

    $: if ($searchActive) {
        tick().then(() => {
            if (inputEl) inputEl.focus();
        });
    }

    function close() {
        searchActive.set(false);
        searchQuery = "";
    }

    function handleKeydown(e) {
        if (e.key === 'Escape') close();
    }

    function handleSubmit() {
        if (searchQuery.trim()) {
            dispatch('search', searchQuery.trim());
        }
    }
</script>

{#if $searchActive}
    <div class="search-overlay">
        <div class="search-bar">
            <svg class="search-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="11" cy="11" r="8" />
                <path d="M21 21l-4.35-4.35" />
            </svg>
            <input
                bind:this={inputEl}
                bind:value={searchQuery}
                on:keydown={(e) => { if (e.key === 'Escape') close(); if (e.key === 'Enter') handleSubmit(); }}
                type="text"
                placeholder="Search notes, profiles, channels, publications..."
                class="search-input"
            />
            <button class="search-close" on:click={close}>x</button>
        </div>

        {#if searchQuery.trim().length > 0}
            <div class="search-results-panel">
                <div class="search-placeholder">
                    Search results will appear here.
                </div>
            </div>
        {/if}
    </div>
{/if}

<style>
    .search-overlay {
        position: fixed;
        top: 0;
        left: 200px;
        right: 0;
        bottom: 0;
        z-index: 500;
        display: flex;
        flex-direction: column;
    }

    .search-bar {
        display: flex;
        align-items: center;
        gap: 0.5em;
        height: 3em;
        padding: 0 0.75em;
        background: var(--header-bg);
        border-bottom: 1px solid var(--border-color);
    }

    .search-icon {
        width: 1.1em;
        height: 1.1em;
        color: var(--text-muted);
        flex-shrink: 0;
    }

    .search-input {
        flex: 1;
        background: none;
        border: none;
        outline: none;
        color: var(--text-color);
        font-size: 0.9rem;
        padding: 0.4em 0;
    }

    .search-input::placeholder {
        color: var(--text-muted);
    }

    .search-close {
        background: none;
        border: none;
        color: var(--text-muted);
        cursor: pointer;
        font-size: 1.1rem;
        padding: 0.25em 0.4em;
        border-radius: 4px;
        transition: background 0.15s;
    }

    .search-close:hover {
        background: var(--button-hover-bg);
        color: var(--text-color);
    }

    .search-results-panel {
        flex: 1;
        background: var(--bg-color);
        overflow-y: auto;
    }

    .search-placeholder {
        text-align: center;
        padding: 3em 1em;
        color: var(--text-muted);
        font-size: 0.9rem;
    }

    @media (max-width: 1280px) {
        .search-overlay {
            left: 60px;
        }
    }

    @media (max-width: 640px) {
        .search-overlay {
            left: 0;
        }
    }
</style>
