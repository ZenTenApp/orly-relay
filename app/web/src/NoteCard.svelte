<script>
    import { createEventDispatcher } from 'svelte';

    export let event = null;
    export let userPubkey = "";
    export let profiles = new Map();

    const dispatch = createEventDispatcher();

    $: authorProfile = profiles.get(event?.pubkey) || null;
    $: displayName = getDisplayName(authorProfile, event?.pubkey);
    $: timeAgo = formatTimeAgo(event?.created_at);
    $: parsedContent = parseContent(event?.content || "");
    $: isOwnNote = event?.pubkey === userPubkey;

    function getDisplayName(profile, pubkey) {
        if (profile?.name) return profile.name;
        if (profile?.display_name) return profile.display_name;
        if (pubkey) return pubkey.slice(0, 8) + '...';
        return 'Anonymous';
    }

    function formatTimeAgo(timestamp) {
        if (!timestamp) return '';
        const now = Math.floor(Date.now() / 1000);
        const diff = now - timestamp;
        if (diff < 60) return `${diff}s`;
        if (diff < 3600) return `${Math.floor(diff / 60)}m`;
        if (diff < 86400) return `${Math.floor(diff / 3600)}h`;
        if (diff < 604800) return `${Math.floor(diff / 86400)}d`;
        return new Date(timestamp * 1000).toLocaleDateString();
    }

    function parseContent(content) {
        // Escape HTML first
        let text = content
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;');

        // Convert URLs to links
        text = text.replace(
            /(https?:\/\/[^\s<]+)/g,
            '<a href="$1" target="_blank" rel="noopener noreferrer" class="note-link">$1</a>'
        );

        // Convert nostr: links to styled spans
        text = text.replace(
            /nostr:(npub1[a-z0-9]+|note1[a-z0-9]+|nevent1[a-z0-9]+|nprofile1[a-z0-9]+|naddr1[a-z0-9]+)/g,
            '<span class="nostr-ref">$&</span>'
        );

        // Convert newlines to <br>
        text = text.replace(/\n/g, '<br>');

        return text;
    }

    // Extract image URLs from content
    $: images = extractImages(event?.content || "");

    function extractImages(content) {
        const urlRegex = /https?:\/\/[^\s<]+\.(jpg|jpeg|png|gif|webp|svg)(\?[^\s<]*)?/gi;
        return [...(content.matchAll(urlRegex) || [])].map(m => m[0]);
    }

    // Content warning check
    $: contentWarning = event?.tags?.find(t => t[0] === 'content-warning')?.[1] || null;
    let showWarned = false;

    function handleReply() {
        dispatch('reply', event);
    }

    function handleReaction() {
        dispatch('reaction', event);
    }

    function handleRepost() {
        dispatch('repost', event);
    }

    function handleZap() {
        dispatch('zap', event);
    }
</script>

{#if event}
    <article class="note-card">
        <div class="note-header">
            <div class="note-author">
                {#if authorProfile?.picture}
                    <img src={authorProfile.picture} alt="" class="author-avatar" />
                {:else}
                    <div class="author-avatar-placeholder">
                        {displayName.charAt(0).toUpperCase()}
                    </div>
                {/if}
                <div class="author-info">
                    <span class="author-name">{displayName}</span>
                    {#if authorProfile?.nip05}
                        <span class="author-nip05">{authorProfile.nip05}</span>
                    {/if}
                </div>
            </div>
            <span class="note-time" title={new Date(event.created_at * 1000).toLocaleString()}>
                {timeAgo}
            </span>
        </div>

        <div class="note-body">
            {#if contentWarning && !showWarned}
                <div class="content-warning">
                    <span>CW: {contentWarning}</span>
                    <button on:click={() => showWarned = true}>Show</button>
                </div>
            {:else}
                <div class="note-text">{@html parsedContent}</div>
                {#if images.length > 0}
                    <div class="note-images" class:gallery={images.length > 1}>
                        {#each images as src}
                            <img {src} alt="" class="note-image" loading="lazy" />
                        {/each}
                    </div>
                {/if}
            {/if}
        </div>

        <div class="note-actions">
            <button class="action-btn reply-btn" on:click={handleReply} title="Reply">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z"/></svg>
            </button>
            <button class="action-btn repost-btn" on:click={handleRepost} title="Repost">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="17 1 21 5 17 9"/><path d="M3 11V9a4 4 0 0 1 4-4h14"/><polyline points="7 23 3 19 7 15"/><path d="M21 13v2a4 4 0 0 1-4 4H3"/></svg>
            </button>
            <button class="action-btn react-btn" on:click={handleReaction} title="React">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z"/></svg>
            </button>
            <button class="action-btn zap-btn" on:click={handleZap} title="Zap">
                <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/></svg>
            </button>
        </div>
    </article>
{/if}

<style>
    .note-card {
        border-bottom: 1px solid var(--border-color);
        padding: 0.75em 1em;
        transition: background 0.15s;
    }

    .note-card:hover {
        background: var(--primary-bg);
    }

    .note-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        margin-bottom: 0.4em;
    }

    .note-author {
        display: flex;
        align-items: center;
        gap: 0.5em;
        min-width: 0;
    }

    .author-avatar {
        width: 36px;
        height: 36px;
        border-radius: 50%;
        object-fit: cover;
        flex-shrink: 0;
    }

    .author-avatar-placeholder {
        width: 36px;
        height: 36px;
        border-radius: 50%;
        background: var(--primary);
        color: #000;
        display: flex;
        align-items: center;
        justify-content: center;
        font-weight: bold;
        font-size: 0.85rem;
        flex-shrink: 0;
    }

    .author-info {
        display: flex;
        flex-direction: column;
        min-width: 0;
    }

    .author-name {
        font-weight: 600;
        font-size: 0.85rem;
        color: var(--text-color);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .author-nip05 {
        font-size: 0.7rem;
        color: var(--text-muted);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .note-time {
        font-size: 0.75rem;
        color: var(--text-muted);
        flex-shrink: 0;
        margin-left: 0.5em;
    }

    .note-body {
        margin-bottom: 0.5em;
    }

    .note-text {
        font-size: 0.9rem;
        line-height: 1.5;
        color: var(--text-color);
        word-break: break-word;
        overflow-wrap: break-word;
    }

    :global(.note-link) {
        color: var(--primary);
        text-decoration: none;
        word-break: break-all;
    }

    :global(.note-link:hover) {
        text-decoration: underline;
    }

    :global(.nostr-ref) {
        color: var(--primary);
        font-size: 0.8em;
        background: var(--primary-bg);
        padding: 0.1em 0.3em;
        border-radius: 3px;
        word-break: break-all;
    }

    .note-images {
        margin-top: 0.5em;
        border-radius: 8px;
        overflow: hidden;
    }

    .note-images.gallery {
        display: grid;
        grid-template-columns: repeat(2, 1fr);
        gap: 2px;
    }

    .note-image {
        width: 100%;
        max-height: 400px;
        object-fit: cover;
        display: block;
    }

    .content-warning {
        display: flex;
        align-items: center;
        gap: 0.75em;
        padding: 0.6em;
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 6px;
        font-size: 0.85rem;
        color: var(--text-muted);
    }

    .content-warning button {
        background: var(--button-bg);
        border: 1px solid var(--border-color);
        border-radius: 4px;
        padding: 0.2em 0.6em;
        font-size: 0.8rem;
        cursor: pointer;
        color: var(--text-color);
    }

    .note-actions {
        display: flex;
        gap: 0.5em;
    }

    .action-btn {
        display: flex;
        align-items: center;
        justify-content: center;
        background: none;
        border: none;
        cursor: pointer;
        padding: 0.3em;
        border-radius: 50%;
        color: var(--text-muted);
        transition: color 0.15s, background 0.15s;
    }

    .action-btn svg {
        width: 1em;
        height: 1em;
    }

    .action-btn:hover {
        background: var(--primary-bg);
    }

    .reply-btn:hover { color: var(--primary); }
    .repost-btn:hover { color: var(--success); }
    .react-btn:hover { color: #E91E63; }
    .zap-btn:hover { color: var(--primary); }
</style>
