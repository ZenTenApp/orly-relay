<script>
    import { onMount } from 'svelte';
    import { bookmarkList, bookmarksLoading } from './libraryStores.js';
    import { fetchEvents, fetchUserProfile } from './nostr.js';

    export let isLoggedIn = false;
    export let userPubkey = "";

    let initialized = false;
    let resolvedBookmarks = [];

    onMount(() => {
        if (isLoggedIn && userPubkey && !initialized) {
            initialized = true;
            loadBookmarks();
        }
    });

    async function loadBookmarks() {
        if ($bookmarksLoading || !userPubkey) return;
        bookmarksLoading.set(true);

        try {
            // Fetch kind 10003 (bookmark list)
            const events = await fetchEvents(
                [{ kinds: [10003], authors: [userPubkey], limit: 1 }],
                { timeout: 10000, useCache: false }
            );

            if (!events || events.length === 0) {
                bookmarksLoading.set(false);
                return;
            }

            // Most recent kind 10003
            const bookmarkEvent = events.sort((a, b) => b.created_at - a.created_at)[0];

            // Extract bookmarked event IDs from e-tags and a-tags
            const eTags = (bookmarkEvent.tags || []).filter(t => t[0] === "e");
            const aTags = (bookmarkEvent.tags || []).filter(t => t[0] === "a");

            bookmarkList.set([...eTags, ...aTags]);

            // Resolve e-tag bookmarks (fetch the actual events)
            if (eTags.length > 0) {
                const ids = eTags.map(t => t[1]).filter(Boolean);
                const resolved = await fetchEvents(
                    [{ ids, limit: 100 }],
                    { timeout: 10000, useCache: false }
                );

                resolvedBookmarks = (resolved || []).sort((a, b) => b.created_at - a.created_at);
            }
        } catch (err) {
            console.error("[Bookmarks] Error:", err);
        } finally {
            bookmarksLoading.set(false);
        }
    }

    function getKindLabel(kind) {
        switch (kind) {
            case 1: return 'Note';
            case 30023: return 'Article';
            case 30040: return 'Publication';
            case 30041: return 'Section';
            default: return `Kind ${kind}`;
        }
    }

    function truncate(text, len = 120) {
        if (!text || text.length <= len) return text || '';
        return text.slice(0, len) + '...';
    }

    function formatDate(ts) {
        if (!ts) return '';
        return new Date(ts * 1000).toLocaleDateString();
    }
</script>

<div class="bookmarks-view">
    <div class="bookmarks-header">
        <h2>Bookmarks</h2>
        <span class="bookmark-count">{$bookmarkList.length} item{$bookmarkList.length !== 1 ? 's' : ''}</span>
    </div>

    {#if !isLoggedIn}
        <div class="bookmarks-empty">Log in to see your bookmarks.</div>
    {:else if $bookmarksLoading}
        <div class="bookmarks-loading"><div class="spinner"></div></div>
    {:else if resolvedBookmarks.length === 0 && $bookmarkList.length === 0}
        <div class="bookmarks-empty">
            <p>No bookmarks yet.</p>
            <p class="hint">Bookmark notes and articles to find them here.</p>
        </div>
    {:else}
        <div class="bookmark-list">
            {#each resolvedBookmarks as item (item.id)}
                <div class="bookmark-item">
                    <div class="bookmark-kind">{getKindLabel(item.kind)}</div>
                    <div class="bookmark-content">{truncate(item.content)}</div>
                    <div class="bookmark-meta">
                        <span class="bookmark-author">{item.pubkey?.slice(0, 10)}...</span>
                        <span class="bookmark-date">{formatDate(item.created_at)}</span>
                    </div>
                </div>
            {/each}

            <!-- Unresolved a-tag bookmarks -->
            {#each $bookmarkList.filter(t => t[0] === "a") as tag}
                <div class="bookmark-item">
                    <div class="bookmark-kind">Reference</div>
                    <div class="bookmark-content bookmark-ref">{tag[1]}</div>
                </div>
            {/each}
        </div>
    {/if}
</div>

<style>
    .bookmarks-view {
        width: 100%;
        max-width: 640px;
        height: 100%;
        overflow-y: auto;
        margin: 0 auto;
    }

    .bookmarks-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0.75em 1em;
        border-bottom: 1px solid var(--border-color);
        position: sticky;
        top: 0;
        background: var(--bg-color);
        z-index: 1;
    }

    .bookmarks-header h2 {
        margin: 0;
        font-size: 1.1rem;
        color: var(--text-color);
    }

    .bookmark-count {
        font-size: 0.75rem;
        color: var(--text-muted);
    }

    .bookmark-list {
        padding: 0;
    }

    .bookmark-item {
        padding: 0.75em 1em;
        border-bottom: 1px solid var(--border-color);
        transition: background 0.1s;
    }

    .bookmark-item:hover {
        background: var(--primary-bg);
    }

    .bookmark-kind {
        font-size: 0.7rem;
        color: var(--primary);
        font-weight: 600;
        text-transform: uppercase;
        letter-spacing: 0.5px;
        margin-bottom: 0.25em;
    }

    .bookmark-content {
        font-size: 0.85rem;
        color: var(--text-color);
        line-height: 1.4;
        word-break: break-word;
    }

    .bookmark-ref {
        font-family: monospace;
        font-size: 0.75rem;
        color: var(--text-muted);
    }

    .bookmark-meta {
        display: flex;
        gap: 0.75em;
        margin-top: 0.3em;
        font-size: 0.72rem;
        color: var(--text-muted);
    }

    .bookmarks-empty {
        text-align: center;
        padding: 3em 1em;
        color: var(--text-muted);
        font-size: 0.85rem;
    }

    .bookmarks-empty p {
        margin: 0 0 0.3em;
    }

    .hint {
        font-size: 0.78rem;
    }

    .bookmarks-loading {
        display: flex;
        justify-content: center;
        padding: 3em;
    }

    .spinner {
        width: 24px;
        height: 24px;
        border: 2px solid var(--border-color);
        border-top-color: var(--primary);
        border-radius: 50%;
        animation: spin 0.8s linear infinite;
    }

    @keyframes spin {
        to { transform: rotate(360deg); }
    }
</style>
