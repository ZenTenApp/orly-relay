<script>
    import { readerIndex, readerSections, readerCurrentSection, activeLibraryView } from './libraryStores.js';

    export let isLoggedIn = false;
    export let userPubkey = "";

    $: index = $readerIndex;
    $: sections = $readerSections;
    $: currentIdx = $readerCurrentSection;
    $: currentSection = sections[currentIdx] || null;
    $: totalSections = sections.length;

    // Extract metadata from index event
    $: title = index ? (index.tags || []).find(t => t[0] === "title")?.[1] || "Untitled" : "";
    $: format = index ? (index.tags || []).find(t => t[0] === "format")?.[1] || "markdown" : "markdown";

    // TOC entries from index a-tags or section titles
    $: tocEntries = sections.map((s, i) => ({
        index: i,
        title: (s.tags || []).find(t => t[0] === "title")?.[1] || `Section ${i + 1}`,
    }));

    // Simple markdown-to-HTML renderer
    function renderContent(content, fmt) {
        if (!content) return '<p class="empty-section">Empty section.</p>';

        // HTML escape first
        let text = content
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;');

        if (fmt === "asciidoc") {
            // Basic AsciiDoc rendering
            text = text.replace(/^= (.+)$/gm, '<h1>$1</h1>');
            text = text.replace(/^== (.+)$/gm, '<h2>$1</h2>');
            text = text.replace(/^=== (.+)$/gm, '<h3>$1</h3>');
            text = text.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
            text = text.replace(/\*(.+?)\*/g, '<em>$1</em>');
            text = text.replace(/`(.+?)`/g, '<code>$1</code>');
            text = text.replace(/\n\n/g, '</p><p>');
            text = `<p>${text}</p>`;
        } else {
            // Markdown rendering
            text = text.replace(/^### (.+)$/gm, '<h3>$1</h3>');
            text = text.replace(/^## (.+)$/gm, '<h2>$1</h2>');
            text = text.replace(/^# (.+)$/gm, '<h1>$1</h1>');
            text = text.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>');
            text = text.replace(/\*(.+?)\*/g, '<em>$1</em>');
            text = text.replace(/`(.+?)`/g, '<code>$1</code>');
            text = text.replace(/^\- (.+)$/gm, '<li>$1</li>');
            text = text.replace(/(<li>.*<\/li>)/s, '<ul>$1</ul>');
            text = text.replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2" target="_blank" rel="noopener">$1</a>');
            text = text.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, '<img src="$2" alt="$1" class="content-image" />');
            text = text.replace(/\n\n/g, '</p><p>');
            text = `<p>${text}</p>`;
        }

        // Convert remaining newlines to <br>
        text = text.replace(/\n/g, '<br>');

        return text;
    }

    $: renderedContent = currentSection ? renderContent(currentSection.content, format) : '';
    $: sectionTitle = currentSection
        ? (currentSection.tags || []).find(t => t[0] === "title")?.[1] || `Section ${currentIdx + 1}`
        : '';

    function goToSection(idx) {
        readerCurrentSection.set(idx);
    }

    function prevSection() {
        if (currentIdx > 0) readerCurrentSection.set(currentIdx - 1);
    }

    function nextSection() {
        if (currentIdx < totalSections - 1) readerCurrentSection.set(currentIdx + 1);
    }

    function backToLibrary() {
        activeLibraryView.set("my-library");
    }
</script>

<div class="reader">
    <!-- TOC sidebar -->
    <div class="reader-toc">
        <button class="back-link" on:click={backToLibrary}>
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="15 18 9 12 15 6"/></svg>
            Back
        </button>
        <div class="toc-title">{title}</div>
        {#each tocEntries as entry (entry.index)}
            <button
                class="toc-item"
                class:active={currentIdx === entry.index}
                on:click={() => goToSection(entry.index)}
            >
                {entry.title}
            </button>
        {/each}
    </div>

    <!-- Content panel -->
    <div class="reader-content">
        <div class="content-header">
            <h2>{sectionTitle}</h2>
            {#if totalSections > 1}
                <span class="section-indicator">{currentIdx + 1} / {totalSections}</span>
            {/if}
        </div>

        <div class="content-body">
            {#if sections.length === 0}
                <div class="reader-empty">No content available for this publication.</div>
            {:else}
                <div class="rendered-content">{@html renderedContent}</div>
            {/if}
        </div>

        {#if totalSections > 1}
            <div class="section-nav">
                <button class="nav-btn" on:click={prevSection} disabled={currentIdx === 0}>
                    Previous
                </button>
                <button class="nav-btn" on:click={nextSection} disabled={currentIdx >= totalSections - 1}>
                    Next
                </button>
            </div>
        {/if}
    </div>
</div>

<style>
    .reader {
        display: flex;
        width: 100%;
        height: 100%;
        overflow: hidden;
    }

    .reader-toc {
        width: 220px;
        flex-shrink: 0;
        border-right: 1px solid var(--border-color);
        overflow-y: auto;
        padding: 0.5em 0;
    }

    .back-link {
        display: flex;
        align-items: center;
        gap: 0.3em;
        padding: 0.5em 0.8em;
        background: none;
        border: none;
        color: var(--primary);
        font-size: 0.8rem;
        cursor: pointer;
        margin-bottom: 0.5em;
    }

    .back-link svg {
        width: 0.9em;
        height: 0.9em;
    }

    .back-link:hover {
        text-decoration: underline;
    }

    .toc-title {
        padding: 0.3em 0.8em 0.6em;
        font-weight: 700;
        font-size: 0.9rem;
        color: var(--text-color);
        border-bottom: 1px solid var(--border-color);
        margin-bottom: 0.3em;
    }

    .toc-item {
        display: block;
        width: 100%;
        padding: 0.4em 0.8em;
        background: none;
        border: none;
        color: var(--text-color);
        font-size: 0.8rem;
        cursor: pointer;
        text-align: left;
        transition: background 0.1s;
        border-left: 2px solid transparent;
    }

    .toc-item:hover {
        background: var(--primary-bg);
    }

    .toc-item.active {
        background: var(--primary-bg);
        border-left-color: var(--primary);
        color: var(--primary);
        font-weight: 600;
    }

    .reader-content {
        flex: 1;
        overflow-y: auto;
        display: flex;
        flex-direction: column;
        min-width: 0;
    }

    .content-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0.75em 1.5em;
        border-bottom: 1px solid var(--border-color);
        position: sticky;
        top: 0;
        background: var(--bg-color);
        z-index: 1;
    }

    .content-header h2 {
        margin: 0;
        font-size: 1rem;
        color: var(--text-color);
    }

    .section-indicator {
        font-size: 0.75rem;
        color: var(--text-muted);
    }

    .content-body {
        flex: 1;
        padding: 1.5em;
        max-width: 720px;
    }

    .rendered-content {
        font-size: 0.9rem;
        line-height: 1.7;
        color: var(--text-color);
    }

    :global(.rendered-content h1) { font-size: 1.4rem; margin: 1em 0 0.5em; color: var(--text-color); }
    :global(.rendered-content h2) { font-size: 1.2rem; margin: 1em 0 0.4em; color: var(--text-color); }
    :global(.rendered-content h3) { font-size: 1rem; margin: 0.8em 0 0.3em; color: var(--text-color); }
    :global(.rendered-content p) { margin: 0 0 0.8em; }
    :global(.rendered-content code) {
        background: var(--card-bg, #1a1a1a);
        padding: 0.15em 0.35em;
        border-radius: 3px;
        font-size: 0.85em;
    }
    :global(.rendered-content a) { color: var(--primary); }
    :global(.rendered-content ul) { margin: 0.5em 0; padding-left: 1.5em; }
    :global(.rendered-content li) { margin: 0.2em 0; }
    :global(.rendered-content .content-image) { max-width: 100%; border-radius: 6px; margin: 0.5em 0; }
    :global(.rendered-content .empty-section) { color: var(--text-muted); font-style: italic; }

    .section-nav {
        display: flex;
        justify-content: space-between;
        padding: 1em 1.5em;
        border-top: 1px solid var(--border-color);
    }

    .nav-btn {
        background: var(--button-bg);
        border: 1px solid var(--border-color);
        border-radius: 6px;
        padding: 0.4em 1em;
        color: var(--text-color);
        font-size: 0.8rem;
        cursor: pointer;
    }

    .nav-btn:hover:not(:disabled) {
        background: var(--button-hover-bg);
    }

    .nav-btn:disabled {
        opacity: 0.4;
        cursor: not-allowed;
    }

    .reader-empty {
        padding: 3em 1.5em;
        text-align: center;
        color: var(--text-muted);
    }

    @media (max-width: 640px) {
        .reader-toc {
            display: none;
        }

        .content-body {
            padding: 1em;
        }
    }
</style>
