<script>
    export let isDarkTheme = false;
    export let tabs = [];
    export let selectedTab = "";

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
</aside>

<style>
    .sidebar {
        position: fixed;
        left: 0;
        top: 2.5em;
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
</style>
