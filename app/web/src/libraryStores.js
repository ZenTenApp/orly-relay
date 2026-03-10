import { writable } from 'svelte/store';

// ==================== Library Navigation ====================

// Which library sub-view is active: "my-library" | "bookmarks" | "editor" | "reader"
export const activeLibraryView = writable("my-library");

// ==================== My Library ====================

// User's root library index (kind 30040, d="library-root") resolved into a tree
// Shape: { categories: [{ dtag, title, publications: [{ dtag, title, kind, sectionCount }] }] }
export const userLibrary = writable(null);

// Selected category d-tag (null = show all)
export const selectedCategory = writable(null);

// Selected publication d-tag (null = none, opens reader when set)
export const selectedPublication = writable(null);

// Loading state
export const libraryLoading = writable(false);

// ==================== Publication Reader ====================

// Current publication index event (kind 30040)
export const readerIndex = writable(null);

// Current publication sections (kind 30041[]) in order
export const readerSections = writable([]);

// Currently viewed section index
export const readerCurrentSection = writable(0);

// ==================== Publication Editor ====================

// Editor state
export const editorState = writable({
    mode: "new",       // "new" | "edit"
    format: "markdown", // "markdown" | "asciidoc"
    title: "",
    dtag: "",
    sections: [],       // [{ dtag, title, content, modified }]
    currentSection: 0,
    isDirty: false,
    categoryDtag: null, // category to attach to
});

// ==================== Viewing Another User's Library ====================

// Hex pubkey of the user whose library we're viewing (null = own library)
export const viewingPubkey = writable(null);

// Their resolved library tree (same shape as userLibrary)
export const viewingLibrary = writable(null);

// ==================== Bookmarks ====================

// User's bookmark list (kind 10003) - array of event references
export const bookmarkList = writable([]);

// User's bookmark sets (kind 30003) - Map of d-tag -> { title, items[] }
export const bookmarkSets = writable(new Map());

// Bookmark loading state
export const bookmarksLoading = writable(false);

// ==================== Actions ====================

/**
 * Reset library state (on logout)
 */
export function resetLibraryState() {
    userLibrary.set(null);
    selectedCategory.set(null);
    selectedPublication.set(null);
    libraryLoading.set(false);
    readerIndex.set(null);
    readerSections.set([]);
    readerCurrentSection.set(0);
    editorState.set({
        mode: "new",
        format: "markdown",
        title: "",
        dtag: "",
        sections: [],
        currentSection: 0,
        isDirty: false,
        categoryDtag: null,
    });
    viewingPubkey.set(null);
    viewingLibrary.set(null);
    bookmarkList.set([]);
    bookmarkSets.set(new Map());
    bookmarksLoading.set(false);
}

/**
 * Open the publication reader
 * @param {object} indexEvent - Kind 30040 event
 * @param {Array} sections - Kind 30041 events in order
 */
export function openReader(indexEvent, sections) {
    readerIndex.set(indexEvent);
    readerSections.set(sections);
    readerCurrentSection.set(0);
    activeLibraryView.set("reader");
}

/**
 * Open the publication editor for a new document
 * @param {string} format - "markdown" or "asciidoc"
 * @param {string|null} categoryDtag - Category to attach to
 */
export function openNewEditor(format = "markdown", categoryDtag = null) {
    editorState.set({
        mode: "new",
        format,
        title: "",
        dtag: "",
        sections: [{ dtag: "", title: "Section 1", content: "", modified: false }],
        currentSection: 0,
        isDirty: false,
        categoryDtag,
    });
    activeLibraryView.set("editor");
}

/**
 * Open the publication editor for an existing document
 * @param {object} indexEvent - Kind 30040 event
 * @param {Array} sections - Kind 30041 events
 */
export function openExistingEditor(indexEvent, sections) {
    const format = (indexEvent.tags || []).find(t => t[0] === "format")?.[1] || "markdown";
    const title = (indexEvent.tags || []).find(t => t[0] === "title")?.[1] || "";
    const dtag = (indexEvent.tags || []).find(t => t[0] === "d")?.[1] || "";
    editorState.set({
        mode: "edit",
        format,
        title,
        dtag,
        sections: sections.map(s => ({
            dtag: (s.tags || []).find(t => t[0] === "d")?.[1] || "",
            title: (s.tags || []).find(t => t[0] === "title")?.[1] || "",
            content: s.content || "",
            modified: false,
        })),
        currentSection: 0,
        isDirty: false,
        categoryDtag: null,
    });
    activeLibraryView.set("editor");
}
