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
            <svg class="gitea-icon" viewBox="0 0 640 640" fill="currentColor" xmlns="http://www.w3.org/2000/svg">
                <path d="M395.9 484.2l-126.9-61c-12.5-6-17.9-21.2-11.8-33.8l61-126.9c6-12.5 21.2-17.9 33.8-11.8 17.2 8.3 27.1 13 27.1 13l-.1-51.6s-.1-30.3 21.7-32.1c-.1 0 74-3 74-3s30.1-1.5 32.1 22.4c0 0 3.9 74.6 3.9 74.6s1.2 22.8-23.2 30c0 0-13.6 3.9-20.6 6-13.6 4.1-26.1-9-24.6-24.6l3.5-42.9-31.3 66.5-31.4 65.2c-4.2 8.7-1.1 18.8 7 24.2l32.9 21.1c11 7 .2 18.2-7.7 37.1l-19.4 48.6z"/>
                <path d="M286.1 499.2l127-60.9c12.5-6 17.9-21.3 11.9-33.8l-61-126.9c-6-12.5-21.2-17.9-33.8-11.9-17.2 8.3-27.1 13-27.1 13l.1-51.6s.1-30.3-21.7-32.1c.1 0-74-3-74-3s-30.1-1.5-32.1 22.4c0 0-3.9 74.6-3.9 74.6s-1.2 22.8 23.2 30.1c0 0 13.6 3.9 20.6 5.9 13.6 4.1 26.1-9 24.6-24.6l-3.5-42.9 31.3 66.5 31.4 65.2c4.2 8.8 1 18.8-7 24.2l-32.9 21.1c-11 7.1-.2 18.3 7.7 37.1l19.2 48.6z"/>
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
        color: var(--muted-foreground);
        text-decoration: none;
        font-size: 0.8em;
        transition: color 0.2s;
    }

    .version-link:hover {
        color: var(--text-color);
    }

    .gitea-icon {
        width: 1.2em;
        height: 1.2em;
        flex-shrink: 0;
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
