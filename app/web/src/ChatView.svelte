<script>
    import { activeChatTab } from './chatStores.js';
    import InboxView from './InboxView.svelte';
    import ChannelsView from './ChannelsView.svelte';

    export let isLoggedIn = false;
    export let userPubkey = "";
    export let userSigner = null;
</script>

<div class="chat-view">
    <div class="chat-tabs">
        <button
            class="chat-tab"
            class:active={$activeChatTab === "inbox"}
            on:click={() => activeChatTab.set("inbox")}
        >
            Inbox
        </button>
        <button
            class="chat-tab"
            class:active={$activeChatTab === "channels"}
            on:click={() => activeChatTab.set("channels")}
        >
            Channels
        </button>
    </div>

    <div class="chat-content">
        {#if $activeChatTab === "inbox"}
            <InboxView {isLoggedIn} {userPubkey} {userSigner} />
        {:else}
            <ChannelsView {isLoggedIn} {userPubkey} {userSigner} />
        {/if}
    </div>
</div>

<style>
    .chat-view {
        width: 100%;
        height: 100%;
        display: flex;
        flex-direction: column;
    }

    .chat-tabs {
        display: flex;
        border-bottom: 1px solid var(--border-color);
        background: var(--bg-color);
        position: sticky;
        top: 0;
        z-index: 1;
    }

    .chat-tab {
        flex: 1;
        padding: 0.7em 1em;
        background: none;
        border: none;
        border-bottom: 2px solid transparent;
        color: var(--text-muted);
        font-size: 0.85rem;
        font-weight: 500;
        cursor: pointer;
        transition: color 0.15s, border-color 0.15s;
    }

    .chat-tab:hover {
        color: var(--text-color);
    }

    .chat-tab.active {
        color: var(--primary);
        border-bottom-color: var(--primary);
    }

    .chat-content {
        flex: 1;
        overflow: hidden;
        display: flex;
    }
</style>
