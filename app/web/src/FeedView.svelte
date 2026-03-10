<script>
    import NoteCard from './NoteCard.svelte';
    import { feedNotes, feedLoading, feedHasMore, feedOldestTimestamp, prependFeedNotes, appendFeedNotes, resetFeedState } from './feedStores.js';
    import { fetchEvents, fetchUserContactList, fetchUserProfile } from './nostr.js';
    import { onMount, onDestroy } from 'svelte';

    export let isLoggedIn = false;
    export let userPubkey = "";
    export let userContactList = null;

    // Profile cache: pubkey -> profile object
    let profiles = new Map();
    let initialized = false;
    let feedContainer;

    // Extract follow pubkeys from contact list (kind 3)
    $: followPubkeys = extractFollows(userContactList);

    function extractFollows(contactList) {
        if (!contactList?.tags) return [];
        return contactList.tags
            .filter(t => t[0] === 'p' && t[1])
            .map(t => t[1]);
    }

    onMount(() => {
        if (isLoggedIn && followPubkeys.length > 0 && $feedNotes.length === 0) {
            loadFeed();
        }
    });

    // Reload feed when follows change
    $: if (isLoggedIn && followPubkeys.length > 0 && !initialized) {
        initialized = true;
        if ($feedNotes.length === 0) {
            loadFeed();
        }
    }

    async function loadFeed() {
        if ($feedLoading || followPubkeys.length === 0) return;
        feedLoading.set(true);

        try {
            const events = await fetchEvents(
                [{ kinds: [1], authors: followPubkeys, limit: 40 }],
                { timeout: 15000, useCache: false }
            );

            if (events && events.length > 0) {
                prependFeedNotes(events);
                loadProfiles(events);
            }
        } catch (err) {
            console.error("[Feed] Error loading feed:", err);
        } finally {
            feedLoading.set(false);
        }
    }

    async function loadMore() {
        if ($feedLoading || !$feedHasMore || followPubkeys.length === 0) return;
        feedLoading.set(true);

        try {
            const events = await fetchEvents(
                [{ kinds: [1], authors: followPubkeys, until: $feedOldestTimestamp, limit: 40 }],
                { timeout: 15000, useCache: false }
            );

            if (events) {
                appendFeedNotes(events);
                loadProfiles(events);
            }
        } catch (err) {
            console.error("[Feed] Error loading more:", err);
        } finally {
            feedLoading.set(false);
        }
    }

    function handleScroll(e) {
        const el = e.target;
        if (el.scrollHeight - el.scrollTop - el.clientHeight < 200) {
            loadMore();
        }
    }

    // Batch-load profiles for note authors we haven't cached
    async function loadProfiles(events) {
        const missing = new Set();
        for (const ev of events) {
            if (ev.pubkey && !profiles.has(ev.pubkey)) {
                missing.add(ev.pubkey);
            }
        }

        for (const pk of missing) {
            try {
                const profile = await fetchUserProfile(pk);
                if (profile) {
                    profiles.set(pk, profile);
                    profiles = profiles; // trigger reactivity
                }
            } catch {
                // Silently skip failed profile fetches
            }
        }
    }

    function handleRefresh() {
        resetFeedState();
        initialized = false;
        loadFeed();
    }
</script>

<div class="feed-view" on:scroll={handleScroll} bind:this={feedContainer}>
    <div class="feed-header">
        <h2>Feed</h2>
        <button class="refresh-btn" on:click={handleRefresh} disabled={$feedLoading}>
            {$feedLoading ? '...' : 'Refresh'}
        </button>
    </div>

    {#if !isLoggedIn}
        <div class="feed-empty">
            <p>Log in to see your feed.</p>
        </div>
    {:else if followPubkeys.length === 0}
        <div class="feed-empty">
            <p>You aren't following anyone yet.</p>
            <p class="feed-hint">Follow some people to see their notes here.</p>
        </div>
    {:else}
        {#each $feedNotes as note (note.id)}
            <NoteCard event={note} {userPubkey} {profiles} />
        {/each}

        {#if $feedLoading}
            <div class="feed-loading">
                <div class="spinner"></div>
            </div>
        {/if}

        {#if !$feedHasMore && $feedNotes.length > 0}
            <div class="feed-end">No more notes.</div>
        {/if}

        {#if !$feedLoading && $feedNotes.length === 0}
            <div class="feed-empty">
                <p>No notes from your follows yet.</p>
            </div>
        {/if}
    {/if}
</div>

<style>
    .feed-view {
        width: 100%;
        max-width: 640px;
        height: 100%;
        overflow-y: auto;
        margin: 0 auto;
    }

    .feed-header {
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

    .feed-header h2 {
        margin: 0;
        font-size: 1.1rem;
        color: var(--text-color);
    }

    .refresh-btn {
        background: var(--button-bg);
        border: 1px solid var(--border-color);
        border-radius: 6px;
        padding: 0.3em 0.75em;
        font-size: 0.8rem;
        cursor: pointer;
        color: var(--text-color);
        transition: background 0.15s;
    }

    .refresh-btn:hover:not(:disabled) {
        background: var(--button-hover-bg);
    }

    .refresh-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .feed-empty {
        text-align: center;
        padding: 3em 1em;
        color: var(--text-muted);
    }

    .feed-empty p {
        margin: 0 0 0.5em;
    }

    .feed-hint {
        font-size: 0.85rem;
    }

    .feed-loading {
        display: flex;
        justify-content: center;
        padding: 1.5em;
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

    .feed-end {
        text-align: center;
        padding: 1.5em;
        color: var(--text-muted);
        font-size: 0.85rem;
    }
</style>
