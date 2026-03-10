import { writable, derived } from 'svelte/store';

// ==================== Notification Categories ====================

// Each category: { items: [], unreadCount: number, lastChecked: number }
export const replyNotifications = writable({ items: [], unreadCount: 0, lastChecked: 0 });
export const reactionNotifications = writable({ items: [], unreadCount: 0, lastChecked: 0 });
export const zapNotifications = writable({ items: [], unreadCount: 0, lastChecked: 0 });

// DM and channel unread counts are derived from chatStores (imported by components, not here)
// These are placeholder counts updated by the notification system
export const dmUnreadCount = writable(0);
export const channelUnreadCount = writable(0);

// ==================== Derived ====================

// Total unread across all categories
export const totalUnreadCount = derived(
    [replyNotifications, reactionNotifications, zapNotifications, dmUnreadCount, channelUnreadCount],
    ([$replies, $reactions, $zaps, $dms, $channels]) =>
        $replies.unreadCount + $reactions.unreadCount + $zaps.unreadCount + $dms + $channels
);

// ==================== Actions ====================

/**
 * Add notification items to a category
 * @param {Function} store - The writable store to update
 * @param {Array} items - New notification items
 */
function addNotifications(store, items) {
    store.update(cat => {
        const existingIds = new Set(cat.items.map(i => i.id));
        const newItems = items.filter(i => !existingIds.has(i.id));
        const newUnread = newItems.filter(i => i.created_at > cat.lastChecked).length;
        return {
            items: [...newItems, ...cat.items].sort((a, b) => b.created_at - a.created_at).slice(0, 100),
            unreadCount: cat.unreadCount + newUnread,
            lastChecked: cat.lastChecked
        };
    });
}

export function addReplyNotifications(items) { addNotifications(replyNotifications, items); }
export function addReactionNotifications(items) { addNotifications(reactionNotifications, items); }
export function addZapNotifications(items) { addNotifications(zapNotifications, items); }

/**
 * Mark a category as read
 * @param {string} category - "replies" | "reactions" | "zaps"
 */
export function markCategoryRead(category) {
    const stores = { replies: replyNotifications, reactions: reactionNotifications, zaps: zapNotifications };
    const store = stores[category];
    if (store) {
        store.update(cat => ({ ...cat, unreadCount: 0, lastChecked: Date.now() }));
    }
}

/**
 * Mark all notifications as read
 */
export function markAllRead() {
    const now = Date.now();
    replyNotifications.update(c => ({ ...c, unreadCount: 0, lastChecked: now }));
    reactionNotifications.update(c => ({ ...c, unreadCount: 0, lastChecked: now }));
    zapNotifications.update(c => ({ ...c, unreadCount: 0, lastChecked: now }));
}

/**
 * Reset all notification state (on logout)
 */
export function resetNotificationState() {
    replyNotifications.set({ items: [], unreadCount: 0, lastChecked: 0 });
    reactionNotifications.set({ items: [], unreadCount: 0, lastChecked: 0 });
    zapNotifications.set({ items: [], unreadCount: 0, lastChecked: 0 });
    dmUnreadCount.set(0);
    channelUnreadCount.set(0);
}
