<script>
    import { onMount, tick } from 'svelte';
    import {
        channels, joinedChannels, selectedChannel, channelsLoading,
        markChannelRead, joinChannel, leaveChannel
    } from './chatStores.js';
    import { fetchEvents, fetchUserProfile, nostrClient } from './nostr.js';

    export let isLoggedIn = false;
    export let userPubkey = "";
    export let userSigner = null;

    let profiles = new Map();
    let messageInput = "";
    let messagesEnd;
    let sending = false;
    let initialized = false;
    let showLeaveConfirm = null;
    let showDiscovery = false;
    let discoveryChannels = [];
    let discoveryLoading = false;

    // Channel list: joined channels sorted by most recent message
    $: channelList = getChannelList($channels, $joinedChannels);

    function getChannelList(chanMap, joined) {
        const list = [];
        for (const [id, chan] of chanMap.entries()) {
            if (!joined.has(id)) continue;
            const lastMsg = chan.messages?.length > 0 ? chan.messages[chan.messages.length - 1] : null;
            list.push({
                id,
                name: chan.metadata?.name || id.slice(0, 12) + '...',
                about: chan.metadata?.about || '',
                picture: chan.metadata?.picture || null,
                lastMessage: lastMsg,
                unreadCount: chan.unreadCount || 0,
            });
        }
        list.sort((a, b) => (b.lastMessage?.created_at || 0) - (a.lastMessage?.created_at || 0));
        return list;
    }

    // Current channel messages
    $: currentChannel = $selectedChannel ? $channels.get($selectedChannel) : null;
    $: currentMessages = currentChannel?.messages || [];
    $: currentMeta = currentChannel?.metadata || {};

    // Auto-scroll
    $: if (currentMessages.length > 0 && messagesEnd) {
        tick().then(() => {
            if (messagesEnd) messagesEnd.scrollIntoView({ behavior: 'smooth' });
        });
    }

    // Mark as read
    $: if ($selectedChannel) {
        markChannelRead($selectedChannel);
    }

    onMount(() => {
        if (isLoggedIn && !initialized) {
            initialized = true;
            loadJoinedChannels();
        }
    });

    async function loadJoinedChannels() {
        if ($channelsLoading) return;
        channelsLoading.set(true);

        try {
            const joined = [...$joinedChannels];
            if (joined.length === 0) {
                channelsLoading.set(false);
                return;
            }

            // Fetch channel metadata (kind 40 = creation, kind 41 = metadata update)
            const metaEvents = await fetchEvents(
                [{ kinds: [40], ids: joined, limit: 100 }],
                { timeout: 10000, useCache: false }
            );

            // Also fetch kind 41 metadata updates
            const metaUpdates = await fetchEvents(
                [{ kinds: [41], "#e": joined, limit: 100 }],
                { timeout: 10000, useCache: false }
            );

            const chanMap = new Map($channels);
            // Process kind 40 (channel creation)
            for (const ev of (metaEvents || [])) {
                try {
                    const meta = JSON.parse(ev.content);
                    const existing = chanMap.get(ev.id) || { messages: [], lastRead: 0, unreadCount: 0, joined: true };
                    existing.metadata = meta;
                    existing.metadata._creator = ev.pubkey;
                    existing.joined = true;
                    chanMap.set(ev.id, existing);
                } catch { /* skip malformed */ }
            }

            // Process kind 41 (metadata updates, override kind 40)
            for (const ev of (metaUpdates || [])) {
                const channelId = ev.tags.find(t => t[0] === "e")?.[1];
                if (!channelId || !chanMap.has(channelId)) continue;
                try {
                    const meta = JSON.parse(ev.content);
                    const existing = chanMap.get(channelId);
                    // Only apply if from channel creator
                    if (existing.metadata?._creator === ev.pubkey) {
                        existing.metadata = { ...existing.metadata, ...meta, _creator: ev.pubkey };
                        chanMap.set(channelId, existing);
                    }
                } catch { /* skip */ }
            }

            // Fetch recent messages for joined channels (kind 42)
            const msgEvents = await fetchEvents(
                [{ kinds: [42], "#e": joined, limit: 200 }],
                { timeout: 15000, useCache: false }
            );

            // Sort messages into channels
            const profilePubkeys = new Set();
            for (const ev of (msgEvents || [])) {
                const rootTag = ev.tags.find(t => t[0] === "e" && (t[3] === "root" || !t[3]));
                const channelId = rootTag?.[1];
                if (!channelId || !chanMap.has(channelId)) continue;

                const chan = chanMap.get(channelId);
                if (!chan.messages.find(m => m.id === ev.id)) {
                    chan.messages.push(ev);
                    profilePubkeys.add(ev.pubkey);
                }
            }

            // Sort messages by timestamp
            for (const chan of chanMap.values()) {
                chan.messages.sort((a, b) => a.created_at - b.created_at);
            }

            channels.set(chanMap);
            loadProfiles([...profilePubkeys]);
        } catch (err) {
            console.error("[Channels] Error loading channels:", err);
        } finally {
            channelsLoading.set(false);
        }
    }

    async function discoverChannels() {
        showDiscovery = true;
        discoveryLoading = true;
        discoveryChannels = [];

        try {
            const events = await fetchEvents(
                [{ kinds: [40], limit: 50 }],
                { timeout: 10000, useCache: false }
            );

            for (const ev of (events || [])) {
                if ($joinedChannels.has(ev.id)) continue;
                try {
                    const meta = JSON.parse(ev.content);
                    discoveryChannels.push({
                        id: ev.id,
                        name: meta.name || ev.id.slice(0, 12) + '...',
                        about: meta.about || '',
                        picture: meta.picture || null,
                        creator: ev.pubkey,
                    });
                } catch { /* skip */ }
            }
        } catch (err) {
            console.error("[Channels] Discovery error:", err);
        } finally {
            discoveryLoading = false;
        }
    }

    function handleJoinChannel(channelId) {
        joinChannel(channelId);
        showDiscovery = false;
        loadJoinedChannels();
    }

    function confirmLeave(channelId) {
        showLeaveConfirm = channelId;
    }

    function handleLeave() {
        if (showLeaveConfirm) {
            leaveChannel(showLeaveConfirm);
            channels.update(map => {
                map.delete(showLeaveConfirm);
                return new Map(map);
            });
            showLeaveConfirm = null;
        }
    }

    async function loadProfiles(pubkeys) {
        for (const pk of pubkeys) {
            if (profiles.has(pk)) continue;
            try {
                const profile = await fetchUserProfile(pk);
                if (profile) {
                    profiles.set(pk, profile);
                    profiles = profiles;
                }
            } catch { /* skip */ }
        }
    }

    function getDisplayName(pubkey) {
        const p = profiles.get(pubkey);
        if (p?.name) return p.name;
        if (p?.display_name) return p.display_name;
        return pubkey?.slice(0, 10) + '...';
    }

    function formatTime(ts) {
        if (!ts) return '';
        const d = new Date(ts * 1000);
        const now = new Date();
        if (d.toDateString() === now.toDateString()) {
            return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
        }
        return d.toLocaleDateString([], { month: 'short', day: 'numeric' });
    }

    function selectChannel(id) {
        selectedChannel.set(id);
    }

    function backToList() {
        selectedChannel.set(null);
    }

    async function sendChannelMessage() {
        if (!messageInput.trim() || !$selectedChannel || !userSigner || sending) return;
        sending = true;

        try {
            const content = messageInput.trim();
            const channelId = $selectedChannel;

            const event = {
                kind: 42,
                created_at: Math.floor(Date.now() / 1000),
                tags: [
                    ["e", channelId, "", "root"],
                ],
                content,
            };

            const signedEvent = await userSigner.signEvent(event);
            await nostrClient.publish(signedEvent);

            // Add locally
            channels.update(map => {
                const chan = map.get(channelId);
                if (chan && !chan.messages.find(m => m.id === signedEvent.id)) {
                    chan.messages.push(signedEvent);
                    map.set(channelId, { ...chan });
                }
                return new Map(map);
            });

            messageInput = "";
        } catch (err) {
            console.error("[Channels] Send error:", err);
        } finally {
            sending = false;
        }
    }

    function handleKeydown(e) {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            sendChannelMessage();
        }
    }
</script>

<div class="channels" class:has-selected={$selectedChannel}>
    <!-- Channel List -->
    <div class="channel-list" class:hidden-mobile={$selectedChannel}>
        <div class="channel-list-header">
            <span>Channels</span>
            <button class="discover-btn" on:click={discoverChannels}>+</button>
        </div>

        {#if !isLoggedIn}
            <div class="channels-empty">Log in to use channels.</div>
        {:else if $channelsLoading}
            <div class="channels-loading"><div class="spinner"></div></div>
        {:else if channelList.length === 0}
            <div class="channels-empty">
                No channels joined.
                <button class="discover-link" on:click={discoverChannels}>Discover channels</button>
            </div>
        {:else}
            {#each channelList as chan (chan.id)}
                <!-- svelte-ignore a11y-click-events-have-key-events -->
                <!-- svelte-ignore a11y-no-static-element-interactions -->
                <div
                    class="channel-item"
                    class:active={$selectedChannel === chan.id}
                    on:click={() => selectChannel(chan.id)}
                >
                    <div class="channel-icon">#</div>
                    <div class="channel-info">
                        <span class="channel-name">{chan.name}</span>
                    </div>
                    {#if chan.unreadCount > 0}
                        <span class="channel-badge">{chan.unreadCount}</span>
                    {/if}
                    <button
                        class="channel-leave"
                        on:click|stopPropagation={() => confirmLeave(chan.id)}
                        title="Leave channel"
                    >x</button>
                </div>
            {/each}
        {/if}
    </div>

    <!-- Channel Thread -->
    {#if $selectedChannel}
        <div class="channel-thread">
            <div class="thread-header">
                <button class="back-btn" on:click={backToList}>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 18 9 12 15 6"/></svg>
                </button>
                <div class="thread-channel-info">
                    <span class="thread-channel-name"># {currentMeta.name || $selectedChannel.slice(0, 12)}</span>
                    {#if currentMeta.about}
                        <span class="thread-channel-about">{currentMeta.about}</span>
                    {/if}
                </div>
            </div>

            <div class="messages-container">
                {#each currentMessages as msg (msg.id)}
                    <div class="channel-message">
                        <div class="msg-author">
                            <span class="msg-name">{getDisplayName(msg.pubkey)}</span>
                            <span class="msg-time">{formatTime(msg.created_at)}</span>
                        </div>
                        <div class="msg-content">{msg.content}</div>
                    </div>
                {/each}
                <div bind:this={messagesEnd}></div>
            </div>

            <div class="compose-bar">
                <textarea
                    bind:value={messageInput}
                    on:keydown={handleKeydown}
                    placeholder="Message #{currentMeta.name || 'channel'}..."
                    rows="1"
                    disabled={sending}
                ></textarea>
                <button class="send-btn" on:click={sendChannelMessage} disabled={sending || !messageInput.trim()}>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>
                </button>
            </div>
        </div>
    {/if}

    <!-- Discovery Overlay -->
    {#if showDiscovery}
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="discovery-overlay" on:click={() => showDiscovery = false}>
            <div class="discovery-panel" on:click|stopPropagation>
                <div class="discovery-header">
                    <h3>Discover Channels</h3>
                    <button class="discovery-close" on:click={() => showDiscovery = false}>x</button>
                </div>
                {#if discoveryLoading}
                    <div class="channels-loading"><div class="spinner"></div></div>
                {:else if discoveryChannels.length === 0}
                    <div class="channels-empty">No new channels found.</div>
                {:else}
                    <div class="discovery-list">
                        {#each discoveryChannels as chan (chan.id)}
                            <div class="discovery-item">
                                <div class="discovery-info">
                                    <span class="discovery-name"># {chan.name}</span>
                                    {#if chan.about}
                                        <span class="discovery-about">{chan.about}</span>
                                    {/if}
                                </div>
                                <button class="join-btn" on:click={() => handleJoinChannel(chan.id)}>Join</button>
                            </div>
                        {/each}
                    </div>
                {/if}
            </div>
        </div>
    {/if}

    <!-- Leave Confirmation -->
    {#if showLeaveConfirm}
        <!-- svelte-ignore a11y-click-events-have-key-events -->
        <!-- svelte-ignore a11y-no-static-element-interactions -->
        <div class="discovery-overlay" on:click={() => showLeaveConfirm = null}>
            <div class="confirm-panel" on:click|stopPropagation>
                <p>Leave this channel?</p>
                <div class="confirm-actions">
                    <button class="cancel-btn" on:click={() => showLeaveConfirm = null}>Cancel</button>
                    <button class="leave-btn" on:click={handleLeave}>Leave</button>
                </div>
            </div>
        </div>
    {/if}
</div>

<style>
    .channels {
        display: flex;
        width: 100%;
        height: 100%;
        overflow: hidden;
        position: relative;
    }

    .channel-list {
        width: 280px;
        flex-shrink: 0;
        border-right: 1px solid var(--border-color);
        overflow-y: auto;
        display: flex;
        flex-direction: column;
    }

    .channel-list-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0.6em 0.8em;
        border-bottom: 1px solid var(--border-color);
        font-weight: 600;
        font-size: 0.85rem;
        color: var(--text-color);
    }

    .discover-btn {
        background: var(--button-bg);
        border: 1px solid var(--border-color);
        border-radius: 4px;
        color: var(--text-color);
        cursor: pointer;
        font-size: 1rem;
        width: 24px;
        height: 24px;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 0;
    }

    .discover-btn:hover {
        background: var(--button-hover-bg);
    }

    .channel-item {
        display: flex;
        align-items: center;
        gap: 0.4em;
        padding: 0.5em 0.8em;
        cursor: pointer;
        transition: background 0.15s;
    }

    .channel-item:hover {
        background: var(--primary-bg);
    }

    .channel-item.active {
        background: var(--primary-bg);
    }

    .channel-icon {
        color: var(--text-muted);
        font-weight: bold;
        font-size: 0.9rem;
        width: 20px;
        text-align: center;
        flex-shrink: 0;
    }

    .channel-info {
        flex: 1;
        min-width: 0;
    }

    .channel-name {
        font-size: 0.85rem;
        color: var(--text-color);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .channel-badge {
        background: var(--primary);
        color: #000;
        font-size: 0.65rem;
        font-weight: bold;
        min-width: 16px;
        height: 16px;
        border-radius: 8px;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 0 3px;
        flex-shrink: 0;
    }

    .channel-leave {
        background: none;
        border: none;
        color: var(--text-muted);
        cursor: pointer;
        font-size: 0.75rem;
        padding: 0.1em 0.3em;
        border-radius: 3px;
        opacity: 0;
        transition: opacity 0.15s;
    }

    .channel-item:hover .channel-leave {
        opacity: 1;
    }

    .channel-leave:hover {
        background: var(--danger, #ef4444);
        color: #fff;
    }

    .channel-thread {
        flex: 1;
        display: flex;
        flex-direction: column;
        min-width: 0;
    }

    .thread-header {
        display: flex;
        align-items: center;
        gap: 0.5em;
        padding: 0.6em 0.8em;
        border-bottom: 1px solid var(--border-color);
        background: var(--bg-color);
    }

    .back-btn {
        display: none;
        background: none;
        border: none;
        color: var(--text-muted);
        cursor: pointer;
        padding: 0.2em;
    }

    .back-btn svg {
        width: 1.2em;
        height: 1.2em;
    }

    .thread-channel-info {
        display: flex;
        flex-direction: column;
    }

    .thread-channel-name {
        font-weight: 600;
        font-size: 0.85rem;
        color: var(--text-color);
    }

    .thread-channel-about {
        font-size: 0.7rem;
        color: var(--text-muted);
    }

    .messages-container {
        flex: 1;
        overflow-y: auto;
        padding: 0.75em;
        display: flex;
        flex-direction: column;
        gap: 0.5em;
    }

    .channel-message {
        padding: 0.3em 0;
    }

    .msg-author {
        display: flex;
        align-items: baseline;
        gap: 0.4em;
        margin-bottom: 0.1em;
    }

    .msg-name {
        font-weight: 600;
        font-size: 0.8rem;
        color: var(--text-color);
    }

    .msg-time {
        font-size: 0.65rem;
        color: var(--text-muted);
    }

    .msg-content {
        font-size: 0.85rem;
        color: var(--text-color);
        line-height: 1.4;
        word-break: break-word;
    }

    .compose-bar {
        display: flex;
        align-items: flex-end;
        gap: 0.4em;
        padding: 0.5em 0.75em;
        border-top: 1px solid var(--border-color);
        background: var(--bg-color);
    }

    .compose-bar textarea {
        flex: 1;
        background: var(--card-bg, #1a1a1a);
        border: 1px solid var(--border-color);
        border-radius: 8px;
        padding: 0.5em 0.75em;
        color: var(--text-color);
        font-size: 0.85rem;
        resize: none;
        outline: none;
        max-height: 120px;
        font-family: inherit;
    }

    .compose-bar textarea::placeholder {
        color: var(--text-muted);
    }

    .send-btn {
        background: var(--primary);
        border: none;
        border-radius: 50%;
        width: 34px;
        height: 34px;
        display: flex;
        align-items: center;
        justify-content: center;
        cursor: pointer;
        flex-shrink: 0;
        color: #000;
        transition: opacity 0.15s;
    }

    .send-btn:disabled {
        opacity: 0.4;
        cursor: not-allowed;
    }

    .send-btn svg {
        width: 1em;
        height: 1em;
    }

    .channels-empty {
        text-align: center;
        padding: 2em 1em;
        color: var(--text-muted);
        font-size: 0.85rem;
    }

    .discover-link {
        display: block;
        margin-top: 0.5em;
        background: none;
        border: none;
        color: var(--primary);
        cursor: pointer;
        font-size: 0.85rem;
    }

    .discover-link:hover {
        text-decoration: underline;
    }

    .channels-loading {
        display: flex;
        justify-content: center;
        padding: 2em;
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

    /* Discovery overlay */
    .discovery-overlay {
        position: absolute;
        inset: 0;
        background: rgba(0, 0, 0, 0.5);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 10;
    }

    .discovery-panel {
        background: var(--card-bg, #1a1a1a);
        border: 1px solid var(--border-color);
        border-radius: 10px;
        width: 90%;
        max-width: 400px;
        max-height: 70%;
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    .discovery-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0.75em 1em;
        border-bottom: 1px solid var(--border-color);
    }

    .discovery-header h3 {
        margin: 0;
        font-size: 0.9rem;
        color: var(--text-color);
    }

    .discovery-close {
        background: none;
        border: none;
        color: var(--text-muted);
        cursor: pointer;
        font-size: 1.1rem;
    }

    .discovery-list {
        overflow-y: auto;
        padding: 0.5em;
    }

    .discovery-item {
        display: flex;
        align-items: center;
        gap: 0.5em;
        padding: 0.5em;
        border-radius: 6px;
    }

    .discovery-item:hover {
        background: var(--primary-bg);
    }

    .discovery-info {
        flex: 1;
        min-width: 0;
    }

    .discovery-name {
        font-size: 0.85rem;
        font-weight: 600;
        color: var(--text-color);
        display: block;
    }

    .discovery-about {
        font-size: 0.75rem;
        color: var(--text-muted);
        display: block;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .join-btn {
        background: var(--primary);
        border: none;
        border-radius: 6px;
        color: #000;
        font-size: 0.8rem;
        font-weight: 600;
        padding: 0.3em 0.8em;
        cursor: pointer;
        flex-shrink: 0;
    }

    .join-btn:hover {
        opacity: 0.9;
    }

    /* Confirm modal */
    .confirm-panel {
        background: var(--card-bg, #1a1a1a);
        border: 1px solid var(--border-color);
        border-radius: 10px;
        padding: 1.5em;
        text-align: center;
    }

    .confirm-panel p {
        margin: 0 0 1em;
        color: var(--text-color);
        font-size: 0.9rem;
    }

    .confirm-actions {
        display: flex;
        gap: 0.5em;
        justify-content: center;
    }

    .cancel-btn {
        background: var(--button-bg);
        border: 1px solid var(--border-color);
        border-radius: 6px;
        color: var(--text-color);
        padding: 0.4em 1em;
        cursor: pointer;
        font-size: 0.85rem;
    }

    .leave-btn {
        background: var(--danger, #ef4444);
        border: none;
        border-radius: 6px;
        color: #fff;
        padding: 0.4em 1em;
        cursor: pointer;
        font-size: 0.85rem;
    }

    /* Mobile */
    @media (max-width: 640px) {
        .channel-list {
            width: 100%;
            border-right: none;
        }

        .channel-list.hidden-mobile {
            display: none;
        }

        .channels:not(.has-selected) .channel-thread {
            display: none;
        }

        .channels.has-selected .channel-list {
            display: none;
        }

        .back-btn {
            display: flex;
        }
    }
</style>
