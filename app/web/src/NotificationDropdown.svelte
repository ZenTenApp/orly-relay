<script>
    import { notificationDropdownOpen } from './stores.js';
    import {
        replyNotifications, reactionNotifications, zapNotifications,
        totalUnreadCount, markCategoryRead, markAllRead,
        addReplyNotifications, addReactionNotifications, addZapNotifications
    } from './notificationStores.js';
    import { totalUnreadDMs, totalUnreadChannels } from './chatStores.js';
    import { fetchEvents } from './nostr.js';
    import { onMount, onDestroy } from 'svelte';

    export let userPubkey = "";
    export let isLoggedIn = false;

    let fetched = false;

    // Close on outside click
    function handleWindowClick() {
        if ($notificationDropdownOpen) {
            notificationDropdownOpen.set(false);
        }
    }

    // Fetch notifications when opened
    $: if ($notificationDropdownOpen && isLoggedIn && userPubkey && !fetched) {
        fetchNotifications();
    }

    async function fetchNotifications() {
        fetched = true;
        try {
            // Fetch replies/mentions (kind 1 with #p tag)
            const [replies, reactions, zaps] = await Promise.all([
                fetchEvents(
                    [{ kinds: [1], "#p": [userPubkey], limit: 30 }],
                    { timeout: 10000, useCache: false }
                ),
                fetchEvents(
                    [{ kinds: [7], "#p": [userPubkey], limit: 30 }],
                    { timeout: 10000, useCache: false }
                ),
                fetchEvents(
                    [{ kinds: [9735], "#p": [userPubkey], limit: 30 }],
                    { timeout: 10000, useCache: false }
                ),
            ]);

            if (replies?.length) addReplyNotifications(replies);
            if (reactions?.length) addReactionNotifications(reactions);
            if (zaps?.length) addZapNotifications(zaps);
        } catch (err) {
            console.error("[Notifications] Fetch error:", err);
        }
    }

    function formatTime(ts) {
        if (!ts) return '';
        const now = Math.floor(Date.now() / 1000);
        const diff = now - ts;
        if (diff < 60) return 'now';
        if (diff < 3600) return `${Math.floor(diff / 60)}m`;
        if (diff < 86400) return `${Math.floor(diff / 3600)}h`;
        return `${Math.floor(diff / 86400)}d`;
    }

    function truncate(text, len = 60) {
        if (!text || text.length <= len) return text || '';
        return text.slice(0, len) + '...';
    }

    function handleMarkAll() {
        markAllRead();
    }
</script>

<svelte:window on:click={handleWindowClick} />

{#if $notificationDropdownOpen}
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="notification-dropdown" on:click|stopPropagation>
        <div class="notif-header">
            <span class="notif-title">Notifications</span>
            {#if $totalUnreadCount > 0}
                <button class="mark-all-btn" on:click={handleMarkAll}>Mark all read</button>
            {/if}
        </div>

        <div class="notif-body">
            <!-- Replies -->
            <div class="notif-section">
                <div class="section-header">
                    <span class="section-label">Replies</span>
                    {#if $replyNotifications.unreadCount > 0}
                        <span class="section-count">{$replyNotifications.unreadCount}</span>
                        <button class="section-read" on:click={() => markCategoryRead("replies")}>read</button>
                    {/if}
                </div>
                {#if $replyNotifications.items.length === 0}
                    <div class="notif-empty">No replies yet.</div>
                {:else}
                    {#each $replyNotifications.items.slice(0, 10) as item (item.id)}
                        <div class="notif-item">
                            <div class="notif-icon reply-icon">
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z"/></svg>
                            </div>
                            <div class="notif-content">
                                <span class="notif-text">{truncate(item.content)}</span>
                                <span class="notif-time">{formatTime(item.created_at)}</span>
                            </div>
                        </div>
                    {/each}
                {/if}
            </div>

            <!-- Reactions -->
            <div class="notif-section">
                <div class="section-header">
                    <span class="section-label">Reactions</span>
                    {#if $reactionNotifications.unreadCount > 0}
                        <span class="section-count">{$reactionNotifications.unreadCount}</span>
                        <button class="section-read" on:click={() => markCategoryRead("reactions")}>read</button>
                    {/if}
                </div>
                {#if $reactionNotifications.items.length === 0}
                    <div class="notif-empty">No reactions yet.</div>
                {:else}
                    {#each $reactionNotifications.items.slice(0, 10) as item (item.id)}
                        <div class="notif-item">
                            <div class="notif-icon react-icon">
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"/></svg>
                            </div>
                            <div class="notif-content">
                                <span class="notif-text">{item.content || '+'}</span>
                                <span class="notif-time">{formatTime(item.created_at)}</span>
                            </div>
                        </div>
                    {/each}
                {/if}
            </div>

            <!-- Zaps -->
            <div class="notif-section">
                <div class="section-header">
                    <span class="section-label">Zaps</span>
                    {#if $zapNotifications.unreadCount > 0}
                        <span class="section-count">{$zapNotifications.unreadCount}</span>
                        <button class="section-read" on:click={() => markCategoryRead("zaps")}>read</button>
                    {/if}
                </div>
                {#if $zapNotifications.items.length === 0}
                    <div class="notif-empty">No zaps yet.</div>
                {:else}
                    {#each $zapNotifications.items.slice(0, 10) as item (item.id)}
                        <div class="notif-item">
                            <div class="notif-icon zap-icon">
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>
                            </div>
                            <div class="notif-content">
                                <span class="notif-text">Zap received</span>
                                <span class="notif-time">{formatTime(item.created_at)}</span>
                            </div>
                        </div>
                    {/each}
                {/if}
            </div>

            <!-- DMs and Channels summary -->
            {#if $totalUnreadDMs > 0 || $totalUnreadChannels > 0}
                <div class="notif-section">
                    <div class="section-header">
                        <span class="section-label">Messages</span>
                    </div>
                    {#if $totalUnreadDMs > 0}
                        <div class="notif-item">
                            <div class="notif-icon dm-icon">
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="2" y="4" width="20" height="16" rx="2"/><path d="M22 7l-10 7L2 7"/></svg>
                            </div>
                            <div class="notif-content">
                                <span class="notif-text">{$totalUnreadDMs} unread message{$totalUnreadDMs > 1 ? 's' : ''}</span>
                            </div>
                        </div>
                    {/if}
                    {#if $totalUnreadChannels > 0}
                        <div class="notif-item">
                            <div class="notif-icon channel-icon">
                                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M4 9h16M4 15h16M10 3L8 21M16 3l-2 18"/></svg>
                            </div>
                            <div class="notif-content">
                                <span class="notif-text">{$totalUnreadChannels} unread channel message{$totalUnreadChannels > 1 ? 's' : ''}</span>
                            </div>
                        </div>
                    {/if}
                </div>
            {/if}
        </div>
    </div>
{/if}

<style>
    .notification-dropdown {
        position: fixed;
        top: 3em;
        right: 0.5em;
        width: 340px;
        max-height: 70vh;
        background: var(--card-bg, #0a0a0a);
        border: 1px solid var(--border-color);
        border-radius: 10px;
        box-shadow: 0 8px 24px rgba(0, 0, 0, 0.3);
        z-index: 1100;
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    .notif-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0.7em 0.9em;
        border-bottom: 1px solid var(--border-color);
    }

    .notif-title {
        font-weight: 600;
        font-size: 0.9rem;
        color: var(--text-color);
    }

    .mark-all-btn {
        background: none;
        border: none;
        color: var(--primary);
        font-size: 0.75rem;
        cursor: pointer;
    }

    .mark-all-btn:hover {
        text-decoration: underline;
    }

    .notif-body {
        overflow-y: auto;
        max-height: calc(70vh - 3em);
    }

    .notif-section {
        border-bottom: 1px solid var(--border-color);
    }

    .notif-section:last-child {
        border-bottom: none;
    }

    .section-header {
        display: flex;
        align-items: center;
        gap: 0.4em;
        padding: 0.5em 0.9em;
        background: var(--bg-color);
    }

    .section-label {
        font-size: 0.75rem;
        font-weight: 600;
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.5px;
        flex: 1;
    }

    .section-count {
        background: var(--primary);
        color: #000;
        font-size: 0.6rem;
        font-weight: 700;
        min-width: 16px;
        height: 16px;
        border-radius: 8px;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 0 3px;
    }

    .section-read {
        background: none;
        border: none;
        color: var(--text-muted);
        font-size: 0.65rem;
        cursor: pointer;
    }

    .section-read:hover {
        color: var(--primary);
    }

    .notif-item {
        display: flex;
        align-items: flex-start;
        gap: 0.5em;
        padding: 0.5em 0.9em;
        transition: background 0.1s;
    }

    .notif-item:hover {
        background: var(--primary-bg);
    }

    .notif-icon {
        flex-shrink: 0;
        width: 1.1em;
        height: 1.1em;
        margin-top: 0.1em;
    }

    .notif-icon svg {
        width: 100%;
        height: 100%;
    }

    .reply-icon { color: var(--primary); }
    .react-icon { color: #E91E63; }
    .zap-icon { color: var(--primary); }
    .dm-icon { color: var(--text-muted); }
    .channel-icon { color: var(--text-muted); }

    .notif-content {
        flex: 1;
        min-width: 0;
        display: flex;
        flex-direction: column;
    }

    .notif-text {
        font-size: 0.8rem;
        color: var(--text-color);
        line-height: 1.3;
        word-break: break-word;
    }

    .notif-time {
        font-size: 0.65rem;
        color: var(--text-muted);
        margin-top: 0.15em;
    }

    .notif-empty {
        padding: 0.7em 0.9em;
        font-size: 0.78rem;
        color: var(--text-muted);
    }

    @media (max-width: 640px) {
        .notification-dropdown {
            right: 0;
            left: 0;
            width: auto;
            border-radius: 0 0 10px 10px;
        }
    }
</style>
