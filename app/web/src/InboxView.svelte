<script>
    import { onMount, onDestroy, tick } from 'svelte';
    import { conversations, selectedConversation, inboxLoading, markConversationRead } from './chatStores.js';
    import { fetchEvents, fetchUserProfile, nostrClient } from './nostr.js';

    export let isLoggedIn = false;
    export let userPubkey = "";
    export let userSigner = null;

    let profiles = new Map();
    let messageInput = "";
    let messagesEnd;
    let sending = false;
    let initialized = false;

    // Sorted conversation list: most recent first
    $: conversationList = getConversationList($conversations);

    function getConversationList(convMap) {
        const list = [];
        for (const [pubkey, conv] of convMap.entries()) {
            if (!conv.messages || conv.messages.length === 0) continue;
            const lastMsg = conv.messages[conv.messages.length - 1];
            list.push({
                pubkey,
                lastMessage: lastMsg,
                unreadCount: conv.unreadCount || 0,
                protocol: conv.protocol || "nip04",
            });
        }
        list.sort((a, b) => (b.lastMessage?.created_at || 0) - (a.lastMessage?.created_at || 0));
        return list;
    }

    // Current conversation messages
    $: currentMessages = $selectedConversation
        ? ($conversations.get($selectedConversation)?.messages || [])
        : [];

    // Auto-scroll on new messages
    $: if (currentMessages.length > 0 && messagesEnd) {
        tick().then(() => {
            if (messagesEnd) messagesEnd.scrollIntoView({ behavior: 'smooth' });
        });
    }

    // Mark as read when selecting conversation
    $: if ($selectedConversation) {
        markConversationRead($selectedConversation);
    }

    onMount(() => {
        if (isLoggedIn && userPubkey && !initialized) {
            initialized = true;
            loadDMs();
        }
    });

    async function loadDMs() {
        if ($inboxLoading || !userPubkey) return;
        inboxLoading.set(true);

        try {
            // Fetch NIP-04 DMs (kind 4) where user is author or tagged
            const [sent, received] = await Promise.all([
                fetchEvents(
                    [{ kinds: [4], authors: [userPubkey], limit: 200 }],
                    { timeout: 15000, useCache: false }
                ),
                fetchEvents(
                    [{ kinds: [4], "#p": [userPubkey], limit: 200 }],
                    { timeout: 15000, useCache: false }
                ),
            ]);

            const allDMs = [...(sent || []), ...(received || [])];
            if (allDMs.length === 0) {
                inboxLoading.set(false);
                return;
            }

            // Group by conversation partner
            const convMap = new Map();
            for (const ev of allDMs) {
                const partner = ev.pubkey === userPubkey
                    ? ev.tags.find(t => t[0] === "p")?.[1]
                    : ev.pubkey;
                if (!partner) continue;

                if (!convMap.has(partner)) {
                    convMap.set(partner, { messages: [], lastRead: 0, unreadCount: 0, protocol: "nip04" });
                }

                // Decrypt content
                let decrypted = ev.content;
                try {
                    if (userSigner?.nip04Decrypt) {
                        decrypted = await userSigner.nip04Decrypt(partner, ev.content);
                    }
                } catch {
                    decrypted = "[encrypted]";
                }

                convMap.get(partner).messages.push({
                    ...ev,
                    decrypted,
                    isMine: ev.pubkey === userPubkey,
                });
            }

            // Sort messages within each conversation
            for (const conv of convMap.values()) {
                conv.messages.sort((a, b) => a.created_at - b.created_at);
            }

            conversations.set(convMap);

            // Load profiles for conversation partners
            const pubkeys = [...convMap.keys()];
            loadProfiles(pubkeys);

        } catch (err) {
            console.error("[Inbox] Error loading DMs:", err);
        } finally {
            inboxLoading.set(false);
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
            } catch {
                // skip
            }
        }
    }

    function getDisplayName(pubkey) {
        const p = profiles.get(pubkey);
        if (p?.name) return p.name;
        if (p?.display_name) return p.display_name;
        return pubkey?.slice(0, 12) + '...';
    }

    function getAvatar(pubkey) {
        return profiles.get(pubkey)?.picture || null;
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

    function selectConversation(pubkey) {
        selectedConversation.set(pubkey);
    }

    function backToList() {
        selectedConversation.set(null);
    }

    async function sendMessage() {
        if (!messageInput.trim() || !$selectedConversation || !userSigner || sending) return;
        sending = true;

        try {
            const plaintext = messageInput.trim();
            const partner = $selectedConversation;

            // Encrypt with NIP-04
            let ciphertext;
            if (userSigner.nip04Encrypt) {
                ciphertext = await userSigner.nip04Encrypt(partner, plaintext);
            } else {
                throw new Error("Signer does not support NIP-04 encryption");
            }

            const event = {
                kind: 4,
                created_at: Math.floor(Date.now() / 1000),
                tags: [["p", partner]],
                content: ciphertext,
            };

            const signedEvent = await userSigner.signEvent(event);
            await nostrClient.publish(signedEvent);

            // Add to local conversation immediately
            conversations.update(map => {
                const conv = map.get(partner) || { messages: [], lastRead: 0, unreadCount: 0, protocol: "nip04" };
                conv.messages.push({
                    ...signedEvent,
                    decrypted: plaintext,
                    isMine: true,
                });
                map.set(partner, conv);
                return new Map(map);
            });

            messageInput = "";
        } catch (err) {
            console.error("[Inbox] Failed to send message:", err);
        } finally {
            sending = false;
        }
    }

    function handleKeydown(e) {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            sendMessage();
        }
    }

    function truncateMessage(text, maxLen = 50) {
        if (!text || text.length <= maxLen) return text;
        return text.slice(0, maxLen) + '...';
    }
</script>

<div class="inbox" class:has-selected={$selectedConversation}>
    <!-- Conversation List -->
    <div class="conversation-list" class:hidden-mobile={$selectedConversation}>
        {#if !isLoggedIn}
            <div class="inbox-empty">Log in to see your messages.</div>
        {:else if $inboxLoading}
            <div class="inbox-loading">
                <div class="spinner"></div>
            </div>
        {:else if conversationList.length === 0}
            <div class="inbox-empty">No conversations yet.</div>
        {:else}
            {#each conversationList as conv (conv.pubkey)}
                <!-- svelte-ignore a11y-click-events-have-key-events -->
                <!-- svelte-ignore a11y-no-static-element-interactions -->
                <div
                    class="conversation-item"
                    class:active={$selectedConversation === conv.pubkey}
                    on:click={() => selectConversation(conv.pubkey)}
                >
                    {#if getAvatar(conv.pubkey)}
                        <img src={getAvatar(conv.pubkey)} alt="" class="conv-avatar" />
                    {:else}
                        <div class="conv-avatar-placeholder">
                            {getDisplayName(conv.pubkey).charAt(0).toUpperCase()}
                        </div>
                    {/if}
                    <div class="conv-info">
                        <div class="conv-header-row">
                            <span class="conv-name">{getDisplayName(conv.pubkey)}</span>
                            <span class="conv-time">{formatTime(conv.lastMessage?.created_at)}</span>
                        </div>
                        <div class="conv-preview">
                            {truncateMessage(conv.lastMessage?.decrypted || conv.lastMessage?.content || '')}
                        </div>
                    </div>
                    {#if conv.unreadCount > 0}
                        <span class="conv-badge">{conv.unreadCount}</span>
                    {/if}
                </div>
            {/each}
        {/if}
    </div>

    <!-- Message Thread -->
    {#if $selectedConversation}
        <div class="message-thread">
            <div class="thread-header">
                <button class="back-btn" on:click={backToList}>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 18 9 12 15 6"/></svg>
                </button>
                <div class="thread-user">
                    {#if getAvatar($selectedConversation)}
                        <img src={getAvatar($selectedConversation)} alt="" class="thread-avatar" />
                    {:else}
                        <div class="thread-avatar-placeholder">
                            {getDisplayName($selectedConversation).charAt(0).toUpperCase()}
                        </div>
                    {/if}
                    <span class="thread-name">{getDisplayName($selectedConversation)}</span>
                </div>
            </div>

            <div class="messages-container">
                {#each currentMessages as msg (msg.id)}
                    <div class="message" class:mine={msg.isMine} class:theirs={!msg.isMine}>
                        <div class="message-bubble">
                            <div class="message-text">{msg.decrypted || msg.content}</div>
                            <span class="message-time">{formatTime(msg.created_at)}</span>
                        </div>
                    </div>
                {/each}
                <div bind:this={messagesEnd}></div>
            </div>

            <div class="compose-bar">
                <textarea
                    bind:value={messageInput}
                    on:keydown={handleKeydown}
                    placeholder="Type a message..."
                    rows="1"
                    disabled={sending}
                ></textarea>
                <button class="send-btn" on:click={sendMessage} disabled={sending || !messageInput.trim()}>
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/></svg>
                </button>
            </div>
        </div>
    {/if}
</div>

<style>
    .inbox {
        display: flex;
        width: 100%;
        height: 100%;
        overflow: hidden;
    }

    .conversation-list {
        width: 320px;
        flex-shrink: 0;
        border-right: 1px solid var(--border-color);
        overflow-y: auto;
    }

    .conversation-item {
        display: flex;
        align-items: center;
        gap: 0.6em;
        padding: 0.7em 0.8em;
        cursor: pointer;
        transition: background 0.15s;
        border-bottom: 1px solid var(--border-color);
    }

    .conversation-item:hover {
        background: var(--primary-bg);
    }

    .conversation-item.active {
        background: var(--primary-bg);
    }

    .conv-avatar, .conv-avatar-placeholder {
        width: 40px;
        height: 40px;
        border-radius: 50%;
        flex-shrink: 0;
    }

    .conv-avatar {
        object-fit: cover;
    }

    .conv-avatar-placeholder {
        background: var(--primary);
        color: #000;
        display: flex;
        align-items: center;
        justify-content: center;
        font-weight: bold;
        font-size: 0.85rem;
    }

    .conv-info {
        flex: 1;
        min-width: 0;
    }

    .conv-header-row {
        display: flex;
        justify-content: space-between;
        align-items: baseline;
        gap: 0.5em;
    }

    .conv-name {
        font-weight: 600;
        font-size: 0.85rem;
        color: var(--text-color);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .conv-time {
        font-size: 0.7rem;
        color: var(--text-muted);
        flex-shrink: 0;
    }

    .conv-preview {
        font-size: 0.78rem;
        color: var(--text-muted);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        margin-top: 0.15em;
    }

    .conv-badge {
        background: var(--primary);
        color: #000;
        font-size: 0.7rem;
        font-weight: bold;
        min-width: 18px;
        height: 18px;
        border-radius: 9px;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 0 4px;
        flex-shrink: 0;
    }

    .message-thread {
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

    .thread-user {
        display: flex;
        align-items: center;
        gap: 0.5em;
    }

    .thread-avatar, .thread-avatar-placeholder {
        width: 28px;
        height: 28px;
        border-radius: 50%;
    }

    .thread-avatar {
        object-fit: cover;
    }

    .thread-avatar-placeholder {
        background: var(--primary);
        color: #000;
        display: flex;
        align-items: center;
        justify-content: center;
        font-weight: bold;
        font-size: 0.7rem;
    }

    .thread-name {
        font-weight: 600;
        font-size: 0.85rem;
        color: var(--text-color);
    }

    .messages-container {
        flex: 1;
        overflow-y: auto;
        padding: 0.75em;
        display: flex;
        flex-direction: column;
        gap: 0.3em;
    }

    .message {
        display: flex;
        max-width: 75%;
    }

    .message.mine {
        align-self: flex-end;
    }

    .message.theirs {
        align-self: flex-start;
    }

    .message-bubble {
        padding: 0.5em 0.75em;
        border-radius: 12px;
        font-size: 0.85rem;
        line-height: 1.4;
        word-break: break-word;
    }

    .mine .message-bubble {
        background: var(--primary);
        color: #000;
        border-bottom-right-radius: 4px;
    }

    .theirs .message-bubble {
        background: var(--card-bg, #1a1a1a);
        color: var(--text-color);
        border-bottom-left-radius: 4px;
    }

    .message-time {
        display: block;
        font-size: 0.65rem;
        opacity: 0.6;
        margin-top: 0.2em;
        text-align: right;
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

    .inbox-empty {
        text-align: center;
        padding: 3em 1em;
        color: var(--text-muted);
        font-size: 0.85rem;
    }

    .inbox-loading {
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

    /* Mobile: single-pane */
    @media (max-width: 640px) {
        .conversation-list {
            width: 100%;
            border-right: none;
        }

        .conversation-list.hidden-mobile {
            display: none;
        }

        .inbox:not(.has-selected) .message-thread {
            display: none;
        }

        .inbox.has-selected .conversation-list {
            display: none;
        }

        .back-btn {
            display: flex;
        }
    }
</style>
