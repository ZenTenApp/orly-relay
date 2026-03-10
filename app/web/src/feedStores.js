import { writable } from 'svelte/store';

// ==================== Feed State ====================

// Array of kind 1 events from follows, sorted by created_at desc
export const feedNotes = writable([]);

// Pagination state
export const feedLoading = writable(false);
export const feedHasMore = writable(true);
export const feedOldestTimestamp = writable(Math.floor(Date.now() / 1000));

// ==================== Actions ====================

/**
 * Prepend new notes to the feed (for real-time updates)
 * @param {Array} notes - New events to prepend
 */
export function prependFeedNotes(notes) {
    feedNotes.update(existing => {
        const ids = new Set(existing.map(e => e.id));
        const newNotes = notes.filter(n => !ids.has(n.id));
        return [...newNotes, ...existing].sort((a, b) => b.created_at - a.created_at);
    });
}

/**
 * Append older notes (for pagination)
 * @param {Array} notes - Older events to append
 */
export function appendFeedNotes(notes) {
    if (notes.length === 0) {
        feedHasMore.set(false);
        return;
    }
    feedNotes.update(existing => {
        const ids = new Set(existing.map(e => e.id));
        const newNotes = notes.filter(n => !ids.has(n.id));
        const merged = [...existing, ...newNotes].sort((a, b) => b.created_at - a.created_at);
        return merged;
    });
    // Update oldest timestamp for next page
    const oldest = notes.reduce((min, n) => Math.min(min, n.created_at), Infinity);
    feedOldestTimestamp.set(oldest);
}

/**
 * Reset feed state (on logout or relay change)
 */
export function resetFeedState() {
    feedNotes.set([]);
    feedLoading.set(false);
    feedHasMore.set(true);
    feedOldestTimestamp.set(Math.floor(Date.now() / 1000));
}
