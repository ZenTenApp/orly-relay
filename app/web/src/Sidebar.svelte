<script>
    export let isDarkTheme = false;
    export let tabs = [];
    export let selectedTab = "";
    export let version = "";

    import { createEventDispatcher } from "svelte";
    const dispatch = createEventDispatcher();

    function selectTab(tabId) {
        dispatch("selectTab", tabId);
    }

    function closeSearchTab(tabId) {
        dispatch("closeSearchTab", tabId);
    }
</script>

<aside class="sidebar" class:dark-theme={isDarkTheme}>
    <div class="sidebar-content">
        <div class="tabs">
            {#each tabs as tab}
                <button
                    class="tab"
                    class:active={selectedTab === tab.id}
                    on:click={() => selectTab(tab.id)}
                >
                    <span class="tab-icon">{tab.icon}</span>
                    <span class="tab-label">{tab.label}</span>
                    {#if tab.isSearchTab}
                        <span
                            class="tab-close-icon"
                            on:click|stopPropagation={() =>
                                closeSearchTab(tab.id)}
                            on:keydown={(e) =>
                                e.key === "Enter" && closeSearchTab(tab.id)}
                            role="button"
                            tabindex="0">✕</span
                        >
                    {/if}
                </button>
            {/each}
        </div>
    </div>
    {#if version}
        <a href="https://next.orly.dev" target="_blank" rel="noopener noreferrer" class="version-link">
            <svg class="version-icon" viewBox="0 0 24 24" fill="currentColor" xmlns="http://www.w3.org/2000/svg">
                <!-- Mug body -->
                <path d="M5 6h12v2h1.5c1.38 0 2.5 1.12 2.5 2.5v1c0 1.38-1.12 2.5-2.5 2.5H17v1c0 1.66-1.34 3-3 3H8c-1.66 0-3-1.34-3-3V6zm12 6h1.5c.28 0 .5-.22.5-.5v-1c0-.28-.22-.5-.5-.5H17v2z"/>
                <!-- Leaf on mug -->
                <path d="M9 9c1.5 0 3 .5 3 2.5S10.5 14 9 14c0-1.5.5-3.5 2-4.5" stroke="currentColor" stroke-width="1" fill="none"/>
            </svg>
            <span class="version-text">v{version}</span>
        </a>
    {/if}
</aside>

<style>
    .sidebar {
        position: fixed;
        left: 0;
        top: 3em;
        width: 200px;
        bottom: 0;
        background: var(--sidebar-bg);
        overflow-y: auto;
        z-index: 100;
    }

    .sidebar-content {
        padding: 0;
        background: var(--sidebar-bg);
    }

    .tabs {
        display: flex;
        flex-direction: column;
        padding: 0;
    }

    .tab {
        display: flex;
        align-items: center;
        padding: 0.75em;
        padding-left: 1em;
        background: transparent;
        color: var(--text-color);
        border: none;
        cursor: pointer;
        transition: background-color 0.2s;
        gap: 0.75rem;
        text-align: left;
        width: 100%;
    }

    .tab:hover {
        background-color: var(--bg-color);
    }

    .tab.active {
        background-color: var(--bg-color);
    }

    .tab-icon {
        font-size: 1.2em;
        flex-shrink: 0;
        width: 1.5em;
        text-align: center;
    }

    .tab-label {
        font-size: 0.9em;
        font-weight: 500;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        flex: 1;
    }

    .tab-close-icon {
        cursor: pointer;
        transition: opacity 0.2s;
        font-size: 0.8em;
        margin-left: auto;
        padding: 0.25rem;
        flex-shrink: 0;
    }

    .tab-close-icon:hover {
        opacity: 0.7;
        background-color: var(--warning);
        color: var(--text-color);
    }

    @media (max-width: 1280px) {
        .sidebar {
            width: 60px;
        }

        .tab-label {
            display: none;
        }

        .tab-close-icon {
            display: none;
        }

        .tab {
            /* Keep left alignment so icons stay in same position */
            justify-content: flex-start;
        }
    }

    @media (max-width: 640px) {
        .sidebar {
            width: 160px;
        }

        .tab-label {
            display: block;
        }

        .tab {
            justify-content: flex-start;
        }
    }

    .version-link {
        position: absolute;
        bottom: 0;
        left: 0;
        right: 0;
        display: flex;
        align-items: center;
        gap: 0.5rem;
        padding: 0.75em 1em;
        color: var(--text-color);
        text-decoration: none;
        font-size: 0.8em;
        transition: background-color 0.2s, color 0.2s;
        background: transparent;
    }

    .version-link:hover {
        background-color: var(--bg-color);
    }

    .version-icon {
        width: 1.2em;
        height: 1.2em;
        flex-shrink: 0;
        color: #4a9c5d;
    }

    .version-text {
        white-space: nowrap;
    }

    @media (max-width: 1280px) {
        .version-text {
            display: none;
        }

        .version-link {
            justify-content: flex-start;
            padding-left: 1.25em;
        }
    }

    @media (max-width: 640px) {
        .version-text {
            display: inline;
        }
    }
</style>
