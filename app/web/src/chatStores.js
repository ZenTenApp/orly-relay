import { writable, derived } from 'svelte/store';

// ==================== Chat Navigation ====================

// Active chat sub-tab: "inbox" or "channels"
export const activeChatTab = writable(localStorage.getItem("activeChatTab") || "inbox");
activeChatTab.subscribe(v => localStorage.setItem("activeChatTab", v));

// ==================== Inbox (DMs) ====================

// Map of pubkey -> { messages: [], lastRead: number, unreadCount: number, protocol: "nip04"|"nip17" }
export const conversations = writable(new Map());

// Currently selected conversation partner pubkey (null = no conversation open)
export const selectedConversation = writable(null);

// Loading state for DM fetch
export const inboxLoading = writable(false);

// ==================== Channels (NIP-28) ====================

// Map of channelId -> { metadata: {}, messages: [], lastRead: number, unreadCount: number, joined: boolean }
export const channels = writable(new Map());

// Set of joined channel IDs (persisted)
const storedJoined = localStorage.getItem("joinedChannels");
export const joinedChannels = writable(new Set(storedJoined ? JSON.parse(storedJoined) : []));
joinedChannels.subscribe(set => localStorage.setItem("joinedChannels", JSON.stringify([...set])));

// Currently selected channel ID (null = no channel open)
export const selectedChannel = writable(null);

// Channel discovery loading state
export const channelsLoading = writable(false);

// ==================== Derived ====================

// Total unread DM count
export const totalUnreadDMs = derived(conversations, $convs => {
    let count = 0;
    for (const conv of $convs.values()) {
        count += conv.unreadCount || 0;
    }
    return count;
});

// Total unread channel messages
export const totalUnreadChannels = derived(channels, $chans => {
    let count = 0;
    for (const chan of $chans.values()) {
        if (chan.joined) count += chan.unreadCount || 0;
    }
    return count;
});

// ==================== Actions ====================

/**
 * Mark a conversation as read
 * @param {string} pubkey - Conversation partner pubkey
 */
export function markConversationRead(pubkey) {
    conversations.update(map => {
        const conv = map.get(pubkey);
        if (conv) {
            conv.lastRead = Date.now();
            conv.unreadCount = 0;
            map.set(pubkey, conv);
        }
        return new Map(map);
    });
}

/**
 * Mark a channel as read
 * @param {string} channelId - Channel ID
 */
export function markChannelRead(channelId) {
    channels.update(map => {
        const chan = map.get(channelId);
        if (chan) {
            chan.lastRead = Date.now();
            chan.unreadCount = 0;
            map.set(channelId, chan);
        }
        return new Map(map);
    });
    // Persist last-read timestamps
    localStorage.setItem(`channel-lastread-${channelId}`, Date.now().toString());
}

/**
 * Join a channel
 * @param {string} channelId - Channel ID
 */
export function joinChannel(channelId) {
    joinedChannels.update(set => {
        set.add(channelId);
        return new Set(set);
    });
}

/**
 * Leave a channel
 * @param {string} channelId - Channel ID
 */
export function leaveChannel(channelId) {
    joinedChannels.update(set => {
        set.delete(channelId);
        return new Set(set);
    });
    selectedChannel.update(sel => sel === channelId ? null : sel);
}

/**
 * Reset all chat state (on logout)
 */
export function resetChatState() {
    conversations.set(new Map());
    selectedConversation.set(null);
    channels.set(new Map());
    selectedChannel.set(null);
    inboxLoading.set(false);
    channelsLoading.set(false);
}
