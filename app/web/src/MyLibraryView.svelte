<script>
    import { onMount } from 'svelte';
    import { userLibrary, selectedCategory, libraryLoading, openReader } from './libraryStores.js';
    import { fetchEvents, fetchUserProfile } from './nostr.js';

    export let isLoggedIn = false;
    export let userPubkey = "";
    export let userSigner = null;

    let initialized = false;

    $: categories = $userLibrary?.categories || [];
    $: selectedCatDocs = getDocsForCategory($selectedCategory, $userLibrary);

    function getDocsForCategory(catDtag, lib) {
        if (!lib?.categories) return [];
        if (!catDtag) {
            // Show all documents across all categories
            return lib.categories.flatMap(c => c.publications || []);
        }
        const cat = lib.categories.find(c => c.dtag === catDtag);
        return cat?.publications || [];
    }

    onMount(() => {
        if (isLoggedIn && userPubkey && !initialized) {
            initialized = true;
            loadLibrary();
        }
    });

    async function loadLibrary() {
        if ($libraryLoading || !userPubkey) return;
        libraryLoading.set(true);

        try {
            // Fetch all kind 30040 (publication indices) by user
            const indexEvents = await fetchEvents(
                [{ kinds: [30040], authors: [userPubkey], limit: 100 }],
                { timeout: 15000, useCache: false }
            );

            if (!indexEvents || indexEvents.length === 0) {
                userLibrary.set({ categories: [{ dtag: "uncategorized", title: "Uncategorized", publications: [] }] });
                return;
            }

            // Build category tree from index events
            // Look for a root index (d=library-root) that references category indices
            const rootIndex = indexEvents.find(e =>
                (e.tags || []).find(t => t[0] === "d" && t[1] === "library-root")
            );

            const categories = [];
            const categorizedDtags = new Set();

            if (rootIndex) {
                // Extract category references from root
                const catRefs = (rootIndex.tags || []).filter(t => t[0] === "a");
                for (const ref of catRefs) {
                    const [, coordStr] = ref;
                    // coordStr format: "30040:pubkey:dtag"
                    const parts = coordStr?.split(":");
                    if (!parts || parts.length < 3) continue;
                    const catDtag = parts.slice(2).join(":");

                    const catEvent = indexEvents.find(e =>
                        (e.tags || []).find(t => t[0] === "d" && t[1] === catDtag)
                    );

                    if (catEvent) {
                        const title = (catEvent.tags || []).find(t => t[0] === "title")?.[1] || catDtag;
                        const pubRefs = (catEvent.tags || []).filter(t => t[0] === "a");
                        const publications = [];

                        for (const pubRef of pubRefs) {
                            const pubParts = pubRef[1]?.split(":");
                            if (!pubParts || pubParts.length < 3) continue;
                            const pubDtag = pubParts.slice(2).join(":");
                            categorizedDtags.add(pubDtag);

                            const pubEvent = indexEvents.find(e =>
                                (e.tags || []).find(t => t[0] === "d" && t[1] === pubDtag)
                            );
                            if (pubEvent) {
                                publications.push({
                                    dtag: pubDtag,
                                    title: (pubEvent.tags || []).find(t => t[0] === "title")?.[1] || pubDtag,
                                    event: pubEvent,
                                });
                            }
                        }

                        categories.push({ dtag: catDtag, title, publications });
                        categorizedDtags.add(catDtag);
                    }
                }
            }

            // Add uncategorized publications
            const uncategorized = [];
            for (const ev of indexEvents) {
                const dtag = (ev.tags || []).find(t => t[0] === "d")?.[1];
                if (!dtag || dtag === "library-root" || categorizedDtags.has(dtag)) continue;
                uncategorized.push({
                    dtag,
                    title: (ev.tags || []).find(t => t[0] === "title")?.[1] || dtag,
                    event: ev,
                });
            }

            if (uncategorized.length > 0 || categories.length === 0) {
                categories.push({ dtag: "uncategorized", title: "Uncategorized", publications: uncategorized });
            }

            userLibrary.set({ categories });
        } catch (err) {
            console.error("[Library] Error loading:", err);
        } finally {
            libraryLoading.set(false);
        }
    }

    async function openPublication(pub) {
        if (!pub.event) return;

        try {
            // Fetch sections (kind 30041) referenced by the index
            const sectionRefs = (pub.event.tags || []).filter(t => t[0] === "a" && t[1]?.startsWith("30041:"));
            const sections = [];

            if (sectionRefs.length > 0) {
                // Fetch all 30041 by this author
                const sectionEvents = await fetchEvents(
                    [{ kinds: [30041], authors: [pub.event.pubkey], limit: 100 }],
                    { timeout: 10000, useCache: false }
                );

                // Match and order by reference order
                for (const ref of sectionRefs) {
                    const parts = ref[1]?.split(":");
                    const secDtag = parts?.slice(2).join(":");
                    const secEvent = (sectionEvents || []).find(e =>
                        (e.tags || []).find(t => t[0] === "d" && t[1] === secDtag)
                    );
                    if (secEvent) sections.push(secEvent);
                }
            }

            // Also try kind 30023 (long-form article) if no 30041 sections found
            if (sections.length === 0) {
                const dtag = (pub.event.tags || []).find(t => t[0] === "d")?.[1];
                const articles = await fetchEvents(
                    [{ kinds: [30023], authors: [pub.event.pubkey], "#d": [dtag], limit: 1 }],
                    { timeout: 10000, useCache: false }
                );
                if (articles?.length > 0) sections.push(articles[0]);
            }

            openReader(pub.event, sections);
        } catch (err) {
            console.error("[Library] Error opening publication:", err);
        }
    }

    function selectCategory(dtag) {
        selectedCategory.set($selectedCategory === dtag ? null : dtag);
    }
</script>

<div class="my-library">
    {#if !isLoggedIn}
        <div class="library-empty">Log in to access your library.</div>
    {:else if $libraryLoading}
        <div class="library-loading"><div class="spinner"></div></div>
    {:else}
        <!-- Category sidebar -->
        <div class="category-sidebar">
            <div class="category-header">Categories</div>
            <button
                class="category-item"
                class:active={!$selectedCategory}
                on:click={() => selectedCategory.set(null)}
            >
                All
            </button>
            {#each categories as cat (cat.dtag)}
                <button
                    class="category-item"
                    class:active={$selectedCategory === cat.dtag}
                    on:click={() => selectCategory(cat.dtag)}
                >
                    <span class="cat-name">{cat.title}</span>
                    <span class="cat-count">{cat.publications?.length || 0}</span>
                </button>
            {/each}
        </div>

        <!-- Document list -->
        <div class="document-list">
            <div class="docs-header">
                <h3>{$selectedCategory ? categories.find(c => c.dtag === $selectedCategory)?.title || 'Documents' : 'All Documents'}</h3>
                <span class="docs-count">{selectedCatDocs.length} document{selectedCatDocs.length !== 1 ? 's' : ''}</span>
            </div>

            {#if selectedCatDocs.length === 0}
                <div class="library-empty">
                    <p>No publications yet.</p>
                    <p class="hint">Create your first publication from the "New" tab in Library.</p>
                </div>
            {:else}
                {#each selectedCatDocs as doc (doc.dtag)}
                    <!-- svelte-ignore a11y-click-events-have-key-events -->
                    <!-- svelte-ignore a11y-no-static-element-interactions -->
                    <div class="doc-card" on:click={() => openPublication(doc)}>
                        <div class="doc-title">{doc.title}</div>
                        {#if doc.event}
                            <div class="doc-meta">
                                <span class="doc-kind">
                                    {doc.event.kind === 30023 ? 'Article' : 'Publication'}
                                </span>
                                <span class="doc-date">
                                    {new Date(doc.event.created_at * 1000).toLocaleDateString()}
                                </span>
                            </div>
                        {/if}
                    </div>
                {/each}
            {/if}
        </div>
    {/if}
</div>

<style>
    .my-library {
        display: flex;
        width: 100%;
        height: 100%;
        overflow: hidden;
    }

    .category-sidebar {
        width: 200px;
        flex-shrink: 0;
        border-right: 1px solid var(--border-color);
        overflow-y: auto;
        padding: 0.5em 0;
    }

    .category-header {
        padding: 0.5em 0.8em;
        font-size: 0.75rem;
        font-weight: 600;
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.5px;
    }

    .category-item {
        display: flex;
        align-items: center;
        justify-content: space-between;
        width: 100%;
        padding: 0.45em 0.8em;
        background: none;
        border: none;
        color: var(--text-color);
        font-size: 0.83rem;
        cursor: pointer;
        text-align: left;
        transition: background 0.1s;
    }

    .category-item:hover {
        background: var(--primary-bg);
    }

    .category-item.active {
        background: var(--primary-bg);
        color: var(--primary);
        font-weight: 600;
    }

    .cat-count {
        font-size: 0.7rem;
        color: var(--text-muted);
    }

    .document-list {
        flex: 1;
        overflow-y: auto;
        min-width: 0;
    }

    .docs-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0.7em 1em;
        border-bottom: 1px solid var(--border-color);
        position: sticky;
        top: 0;
        background: var(--bg-color);
        z-index: 1;
    }

    .docs-header h3 {
        margin: 0;
        font-size: 0.9rem;
        color: var(--text-color);
    }

    .docs-count {
        font-size: 0.75rem;
        color: var(--text-muted);
    }

    .doc-card {
        padding: 0.75em 1em;
        border-bottom: 1px solid var(--border-color);
        cursor: pointer;
        transition: background 0.1s;
    }

    .doc-card:hover {
        background: var(--primary-bg);
    }

    .doc-title {
        font-size: 0.9rem;
        font-weight: 600;
        color: var(--text-color);
        margin-bottom: 0.25em;
    }

    .doc-meta {
        display: flex;
        gap: 0.75em;
        font-size: 0.75rem;
        color: var(--text-muted);
    }

    .doc-kind {
        background: var(--primary-bg);
        padding: 0.1em 0.4em;
        border-radius: 3px;
    }

    .library-empty {
        text-align: center;
        padding: 3em 1em;
        color: var(--text-muted);
        font-size: 0.85rem;
    }

    .library-empty p {
        margin: 0 0 0.3em;
    }

    .hint {
        font-size: 0.78rem;
    }

    .library-loading {
        display: flex;
        justify-content: center;
        align-items: center;
        width: 100%;
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

    @media (max-width: 640px) {
        .category-sidebar {
            display: none;
        }
    }
</style>
