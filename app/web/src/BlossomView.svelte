<script>
    import { createEventDispatcher, onMount } from "svelte";

    export let isLoggedIn = false;
    export let userPubkey = "";
    export let userSigner = null;
    export let currentEffectiveRole = "";

    const dispatch = createEventDispatcher();

    let blobs = [];
    let isLoading = false;
    let error = "";

    // Upload state
    let selectedFiles = [];
    let isUploading = false;
    let uploadProgress = "";
    let fileInput;

    // Modal state
    let showModal = false;
    let selectedBlob = null;
    let zoomLevel = 1;
    const MIN_ZOOM = 0.25;
    const MAX_ZOOM = 4;
    const ZOOM_STEP = 0.25;

    $: canAccess = isLoggedIn && userPubkey;

    // Track if we've loaded once to prevent repeated loads
    let hasLoadedOnce = false;

    /**
     * Create Blossom auth header (kind 24242) per BUD-01 spec
     * @param {object} signer - The signer instance
     * @param {string} verb - The action verb (list, get, upload, delete)
     * @param {string} sha256Hex - Optional SHA256 hash for x tag
     * @returns {Promise<string|null>} Base64 encoded auth header or null
     */
    async function createBlossomAuth(signer, verb, sha256Hex = null) {
        if (!signer) {
            console.log("No signer available for Blossom auth");
            return null;
        }

        try {
            const now = Math.floor(Date.now() / 1000);
            const expiration = now + 60; // 60 seconds from now

            const tags = [
                ["t", verb],
                ["expiration", expiration.toString()],
            ];

            // Add x tag for blob-specific operations
            if (sha256Hex) {
                tags.push(["x", sha256Hex]);
            }

            const authEvent = {
                kind: 24242,
                created_at: now,
                tags: tags,
                content: `Blossom ${verb} operation`,
            };

            const signedEvent = await signer.signEvent(authEvent);
            return btoa(JSON.stringify(signedEvent));
        } catch (err) {
            console.error("Error creating Blossom auth:", err);
            return null;
        }
    }

    onMount(() => {
        if (canAccess && !hasLoadedOnce) {
            hasLoadedOnce = true;
            loadBlobs();
        }
    });

    // Load once when canAccess becomes true (for when user logs in after mount)
    $: if (canAccess && !hasLoadedOnce && !isLoading) {
        hasLoadedOnce = true;
        loadBlobs();
    }

    async function loadBlobs() {
        if (!userPubkey) return;

        isLoading = true;
        error = "";

        try {
            const url = `${window.location.origin}/blossom/list/${userPubkey}`;
            const authHeader = await createBlossomAuth(userSigner, "list");
            const response = await fetch(url, {
                headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
            });

            if (!response.ok) {
                throw new Error(`Failed to load blobs: ${response.statusText}`);
            }

            const data = await response.json();
            // API returns 'uploaded' timestamp per BUD-02 spec
            blobs = Array.isArray(data) ? data : [];
            blobs.sort((a, b) => (b.uploaded || 0) - (a.uploaded || 0));
            console.log("Loaded blobs:", blobs);
        } catch (err) {
            console.error("Error loading blobs:", err);
            error = err.message || "Failed to load blobs";
        } finally {
            isLoading = false;
        }
    }

    function formatSize(bytes) {
        if (!bytes) return "0 B";
        const units = ["B", "KB", "MB", "GB"];
        let i = 0;
        let size = bytes;
        while (size >= 1024 && i < units.length - 1) {
            size /= 1024;
            i++;
        }
        return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
    }

    function formatDate(timestamp) {
        if (!timestamp) return "Unknown";
        return new Date(timestamp * 1000).toLocaleString();
    }

    function truncateHash(hash) {
        if (!hash) return "";
        return `${hash.slice(0, 8)}...${hash.slice(-8)}`;
    }

    function getMimeCategory(mimeType) {
        if (!mimeType) return "unknown";
        if (mimeType.startsWith("image/")) return "image";
        if (mimeType.startsWith("video/")) return "video";
        if (mimeType.startsWith("audio/")) return "audio";
        return "file";
    }

    function getMimeIcon(mimeType) {
        const category = getMimeCategory(mimeType);
        switch (category) {
            case "image": return "";
            case "video": return "";
            case "audio": return "";
            default: return "";
        }
    }

    function openModal(blob) {
        selectedBlob = blob;
        zoomLevel = 1;
        showModal = true;
    }

    function closeModal() {
        showModal = false;
        selectedBlob = null;
        zoomLevel = 1;
    }

    function zoomIn() {
        if (zoomLevel < MAX_ZOOM) {
            zoomLevel = Math.min(MAX_ZOOM, zoomLevel + ZOOM_STEP);
        }
    }

    function zoomOut() {
        if (zoomLevel > MIN_ZOOM) {
            zoomLevel = Math.max(MIN_ZOOM, zoomLevel - ZOOM_STEP);
        }
    }

    function handleKeydown(event) {
        if (!showModal) return;
        if (event.key === "Escape") {
            closeModal();
        } else if (event.key === "+" || event.key === "=") {
            zoomIn();
        } else if (event.key === "-") {
            zoomOut();
        }
    }

    function getBlobUrl(blob) {
        // Prefer the URL from the API response (includes extension for proper MIME handling)
        if (blob.url) {
            // Already an absolute URL - return as-is
            if (blob.url.startsWith("http://") || blob.url.startsWith("https://")) {
                return blob.url;
            }
            // Starts with / - it's a path, prepend origin
            if (blob.url.startsWith("/")) {
                return `${window.location.origin}${blob.url}`;
            }
            // No protocol - looks like host:port/path, add http://
            // This handles cases like "localhost:3334/blossom/..."
            return `http://${blob.url}`;
        }
        // Fallback: construct URL with sha256 only
        return `${window.location.origin}/blossom/${blob.sha256}`;
    }

    function openLoginModal() {
        dispatch("openLoginModal");
    }

    async function deleteBlob(blob) {
        if (!confirm(`Delete blob ${truncateHash(blob.sha256)}?`)) return;

        try {
            const url = `${window.location.origin}/blossom/${blob.sha256}`;
            const authHeader = await createBlossomAuth(userSigner, "delete", blob.sha256);
            const response = await fetch(url, {
                method: "DELETE",
                headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
            });

            if (!response.ok) {
                throw new Error(`Failed to delete: ${response.statusText}`);
            }

            blobs = blobs.filter(b => b.sha256 !== blob.sha256);
            if (selectedBlob?.sha256 === blob.sha256) {
                closeModal();
            }
        } catch (err) {
            console.error("Error deleting blob:", err);
            alert(`Failed to delete blob: ${err.message}`);
        }
    }

    function handleFileSelect(event) {
        selectedFiles = Array.from(event.target.files);
    }

    function triggerFileInput() {
        fileInput?.click();
    }

    async function uploadFiles() {
        if (selectedFiles.length === 0) return;

        isUploading = true;
        error = "";
        const uploaded = [];
        const failed = [];

        for (let i = 0; i < selectedFiles.length; i++) {
            const file = selectedFiles[i];
            uploadProgress = `Uploading ${i + 1}/${selectedFiles.length}: ${file.name}`;

            try {
                const url = `${window.location.origin}/blossom/upload`;
                const authHeader = await createBlossomAuth(userSigner, "upload");

                const response = await fetch(url, {
                    method: "PUT",
                    headers: {
                        "Content-Type": file.type || "application/octet-stream",
                        ...(authHeader ? { Authorization: `Nostr ${authHeader}` } : {}),
                    },
                    body: file,
                });

                if (!response.ok) {
                    const reason = response.headers.get("X-Reason") || response.statusText;
                    throw new Error(reason);
                }

                const descriptor = await response.json();
                console.log("Upload response:", descriptor);
                uploaded.push(descriptor);
            } catch (err) {
                console.error(`Error uploading ${file.name}:`, err);
                failed.push({ name: file.name, error: err.message });
            }
        }

        isUploading = false;
        uploadProgress = "";
        selectedFiles = [];
        if (fileInput) fileInput.value = "";

        if (uploaded.length > 0) {
            await loadBlobs();
        }

        if (failed.length > 0) {
            error = `Failed to upload: ${failed.map(f => f.name).join(", ")}`;
        }
    }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if canAccess}
    <div class="blossom-view">
        <div class="header-section">
            <h3>Blossom Media Storage</h3>
            <button class="refresh-btn" on:click={loadBlobs} disabled={isLoading}>
                {isLoading ? "Loading..." : "Refresh"}
            </button>
        </div>

        <div class="upload-section">
            <input
                type="file"
                multiple
                bind:this={fileInput}
                on:change={handleFileSelect}
                class="file-input-hidden"
            />
            <button class="select-btn" on:click={triggerFileInput} disabled={isUploading}>
                Select Files
            </button>
            {#if selectedFiles.length > 0}
                <span class="selected-count">{selectedFiles.length} file(s) selected</span>
                <button
                    class="upload-btn"
                    on:click={uploadFiles}
                    disabled={isUploading}
                >
                    {isUploading ? uploadProgress : "Upload"}
                </button>
            {/if}
        </div>

        {#if error}
            <div class="error-message">
                {error}
            </div>
        {/if}

        {#if isLoading && blobs.length === 0}
            <div class="loading">Loading blobs...</div>
        {:else if blobs.length === 0}
            <div class="empty-state">
                <p>No files found in your Blossom storage.</p>
            </div>
        {:else}
            <div class="blob-list">
                {#each blobs as blob}
                    <div
                        class="blob-item"
                        on:click={() => openModal(blob)}
                        on:keypress={(e) => e.key === "Enter" && openModal(blob)}
                        role="button"
                        tabindex="0"
                    >
                        <div class="blob-icon">
                            {getMimeIcon(blob.type)}
                        </div>
                        <div class="blob-info">
                            <div class="blob-hash" title={blob.sha256}>
                                {truncateHash(blob.sha256)}
                            </div>
                            <div class="blob-meta">
                                <span class="blob-size">{formatSize(blob.size)}</span>
                                <span class="blob-type">{blob.type || "unknown"}</span>
                            </div>
                        </div>
                        <div class="blob-date">
                            {formatDate(blob.uploaded)}
                        </div>
                        <button
                            class="delete-btn"
                            on:click|stopPropagation={() => deleteBlob(blob)}
                            title="Delete"
                        >
                            X
                        </button>
                    </div>
                {/each}
            </div>
        {/if}
    </div>
{:else}
    <div class="login-prompt">
        <p>Please log in to view your Blossom storage.</p>
        <button class="login-btn" on:click={openLoginModal}>Log In</button>
    </div>
{/if}

{#if showModal && selectedBlob}
    <div
        class="modal-overlay"
        on:click={closeModal}
        on:keypress={(e) => e.key === "Enter" && closeModal()}
        role="button"
        tabindex="0"
    >
        <div
            class="modal-content"
            on:click|stopPropagation
            on:keypress|stopPropagation
            role="dialog"
        >
            <div class="modal-header">
                <div class="modal-title">
                    <span class="modal-hash">{truncateHash(selectedBlob.sha256)}</span>
                    <span class="modal-type">{selectedBlob.type || "unknown"}</span>
                </div>
                <div class="modal-controls">
                    {#if getMimeCategory(selectedBlob.type) === "image"}
                        <button class="zoom-btn" on:click={zoomOut} disabled={zoomLevel <= MIN_ZOOM}>-</button>
                        <span class="zoom-level">{Math.round(zoomLevel * 100)}%</span>
                        <button class="zoom-btn" on:click={zoomIn} disabled={zoomLevel >= MAX_ZOOM}>+</button>
                    {/if}
                    <button class="close-btn" on:click={closeModal}>X</button>
                </div>
            </div>
            <div class="modal-body">
                {#if getMimeCategory(selectedBlob.type) === "image"}
                    <div class="media-container" style="transform: scale({zoomLevel});">
                        <img src={getBlobUrl(selectedBlob)} alt="Blob content" />
                    </div>
                {:else if getMimeCategory(selectedBlob.type) === "video"}
                    <div class="media-container">
                        <video controls src={getBlobUrl(selectedBlob)}>
                            <track kind="captions" />
                        </video>
                    </div>
                {:else if getMimeCategory(selectedBlob.type) === "audio"}
                    <div class="media-container audio">
                        <audio controls src={getBlobUrl(selectedBlob)}></audio>
                    </div>
                {:else}
                    <div class="file-preview">
                        <div class="file-icon">{getMimeIcon(selectedBlob.type)}</div>
                        <p>Preview not available for this file type.</p>
                        <a href={getBlobUrl(selectedBlob)} target="_blank" rel="noopener noreferrer" class="download-link">
                            Download File
                        </a>
                    </div>
                {/if}
            </div>
            <div class="modal-footer">
                <div class="blob-details">
                    <span>Size: {formatSize(selectedBlob.size)}</span>
                    <span>Uploaded: {formatDate(selectedBlob.uploaded)}</span>
                </div>
                <div class="blob-url-section">
                    <input
                        type="text"
                        readonly
                        value={getBlobUrl(selectedBlob)}
                        class="blob-url-input"
                        on:click={(e) => e.target.select()}
                    />
                    <button
                        class="copy-btn"
                        on:click={() => {
                            navigator.clipboard.writeText(getBlobUrl(selectedBlob));
                        }}
                    >
                        Copy
                    </button>
                </div>
                <div class="modal-actions">
                    <a href={getBlobUrl(selectedBlob)} target="_blank" rel="noopener noreferrer" class="action-btn">
                        Open in New Tab
                    </a>
                    <button class="action-btn danger" on:click={() => deleteBlob(selectedBlob)}>
                        Delete
                    </button>
                </div>
            </div>
        </div>
    </div>
{/if}

<style>
    .blossom-view {
        padding: 1em;
        max-width: 900px;
    }

    .header-section {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 1em;
    }

    .header-section h3 {
        margin: 0;
        color: var(--text-color);
    }

    .refresh-btn {
        background-color: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
    }

    .refresh-btn:hover:not(:disabled) {
        background-color: var(--accent-hover-color);
    }

    .refresh-btn:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .upload-section {
        display: flex;
        align-items: center;
        gap: 0.75em;
        padding: 0.75em 1em;
        background-color: var(--card-bg);
        border-radius: 6px;
        margin-bottom: 1em;
        flex-wrap: wrap;
    }

    .file-input-hidden {
        display: none;
    }

    .select-btn {
        background-color: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
    }

    .select-btn:hover:not(:disabled) {
        background-color: var(--accent-hover-color);
    }

    .select-btn:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .selected-count {
        color: var(--text-color);
        font-size: 0.9em;
    }

    .upload-btn {
        background-color: var(--success, #28a745);
        color: white;
        border: none;
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        font-weight: bold;
    }

    .upload-btn:hover:not(:disabled) {
        opacity: 0.9;
    }

    .upload-btn:disabled {
        opacity: 0.7;
        cursor: not-allowed;
    }

    .error-message {
        background-color: var(--warning);
        color: var(--text-color);
        padding: 0.75em 1em;
        border-radius: 4px;
        margin-bottom: 1em;
    }

    .loading, .empty-state {
        text-align: center;
        padding: 2em;
        color: var(--text-color);
        opacity: 0.7;
    }

    .blob-list {
        display: flex;
        flex-direction: column;
        gap: 0.5em;
    }

    .blob-item {
        display: flex;
        align-items: center;
        gap: 1em;
        padding: 0.75em 1em;
        background-color: var(--card-bg);
        border-radius: 6px;
        cursor: pointer;
        transition: background-color 0.2s;
    }

    .blob-item:hover {
        background-color: var(--sidebar-bg);
    }

    .blob-icon {
        font-size: 1.5em;
        width: 2em;
        text-align: center;
    }

    .blob-info {
        flex: 1;
        min-width: 0;
    }

    .blob-hash {
        font-family: monospace;
        font-size: 0.9em;
        color: var(--text-color);
    }

    .blob-meta {
        display: flex;
        gap: 1em;
        font-size: 0.8em;
        color: var(--text-color);
        opacity: 0.7;
        margin-top: 0.25em;
    }

    .blob-date {
        font-size: 0.85em;
        color: var(--text-color);
        opacity: 0.6;
        white-space: nowrap;
    }

    .delete-btn {
        background: transparent;
        border: 1px solid var(--warning);
        color: var(--warning);
        width: 1.75em;
        height: 1.75em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.85em;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .delete-btn:hover {
        background-color: var(--warning);
        color: var(--text-color);
    }

    .login-prompt {
        text-align: center;
        padding: 2em;
        background-color: var(--card-bg);
        border-radius: 8px;
        border: 1px solid var(--border-color);
        max-width: 32em;
        margin: 1em;
    }

    .login-prompt p {
        margin: 0 0 1.5rem 0;
        color: var(--text-color);
        font-size: 1.1rem;
    }

    .login-btn {
        background-color: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.75em 1.5em;
        border-radius: 4px;
        cursor: pointer;
        font-weight: bold;
        font-size: 0.9em;
    }

    .login-btn:hover {
        background-color: var(--accent-hover-color);
    }

    /* Modal styles */
    .modal-overlay {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: rgba(0, 0, 0, 0.8);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 1000;
    }

    .modal-content {
        background-color: var(--bg-color);
        border-radius: 8px;
        max-width: 90vw;
        max-height: 90vh;
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    .modal-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.75em 1em;
        border-bottom: 1px solid var(--border-color);
        background-color: var(--card-bg);
    }

    .modal-title {
        display: flex;
        align-items: center;
        gap: 1em;
    }

    .modal-hash {
        font-family: monospace;
        color: var(--text-color);
    }

    .modal-type {
        font-size: 0.85em;
        color: var(--text-color);
        opacity: 0.7;
    }

    .modal-controls {
        display: flex;
        align-items: center;
        gap: 0.5em;
    }

    .zoom-btn {
        background-color: var(--primary);
        color: var(--text-color);
        border: none;
        width: 2em;
        height: 2em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 1em;
        font-weight: bold;
    }

    .zoom-btn:hover:not(:disabled) {
        background-color: var(--accent-hover-color);
    }

    .zoom-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .zoom-level {
        font-size: 0.85em;
        color: var(--text-color);
        min-width: 3em;
        text-align: center;
    }

    .close-btn {
        background: transparent;
        border: 1px solid var(--border-color);
        color: var(--text-color);
        width: 2em;
        height: 2em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 1em;
        margin-left: 0.5em;
    }

    .close-btn:hover {
        background-color: var(--warning);
        border-color: var(--warning);
    }

    .modal-body {
        flex: 1;
        overflow: auto;
        display: flex;
        align-items: center;
        justify-content: center;
        padding: 1em;
        min-height: 200px;
    }

    .media-container {
        transition: transform 0.2s ease;
        transform-origin: center center;
    }

    .media-container img {
        max-width: 80vw;
        max-height: 70vh;
        object-fit: contain;
    }

    .media-container video {
        max-width: 80vw;
        max-height: 70vh;
    }

    .media-container.audio {
        width: 100%;
        padding: 2em;
    }

    .media-container audio {
        width: 100%;
    }

    .file-preview {
        text-align: center;
        padding: 2em;
        color: var(--text-color);
    }

    .file-icon {
        font-size: 4em;
        margin-bottom: 0.5em;
    }

    .download-link {
        display: inline-block;
        margin-top: 1em;
        padding: 0.75em 1.5em;
        background-color: var(--primary);
        color: var(--text-color);
        text-decoration: none;
        border-radius: 4px;
    }

    .download-link:hover {
        background-color: var(--accent-hover-color);
    }

    .modal-footer {
        display: flex;
        flex-direction: column;
        gap: 0.5em;
        padding: 0.75em 1em;
        border-top: 1px solid var(--border-color);
        background-color: var(--card-bg);
    }

    .blob-details {
        display: flex;
        gap: 1.5em;
        font-size: 0.85em;
        color: var(--text-color);
        opacity: 0.7;
    }

    .blob-url-section {
        display: flex;
        gap: 0.5em;
        width: 100%;
    }

    .blob-url-input {
        flex: 1;
        padding: 0.4em 0.6em;
        font-family: monospace;
        font-size: 0.85em;
        background-color: var(--bg-color);
        color: var(--text-color);
        border: 1px solid var(--border-color);
        border-radius: 4px;
        cursor: text;
    }

    .blob-url-input:focus {
        outline: none;
        border-color: var(--primary);
    }

    .copy-btn {
        padding: 0.4em 0.8em;
        background-color: var(--primary);
        color: var(--text-color);
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.85em;
    }

    .copy-btn:hover {
        background-color: var(--accent-hover-color);
    }

    .modal-actions {
        display: flex;
        gap: 0.5em;
    }

    .action-btn {
        padding: 0.5em 1em;
        background-color: var(--primary);
        color: var(--text-color);
        border: none;
        border-radius: 4px;
        cursor: pointer;
        text-decoration: none;
        font-size: 0.9em;
    }

    .action-btn:hover {
        background-color: var(--accent-hover-color);
    }

    .action-btn.danger {
        background-color: transparent;
        border: 1px solid var(--warning);
        color: var(--warning);
    }

    .action-btn.danger:hover {
        background-color: var(--warning);
        color: var(--text-color);
    }

    @media (max-width: 600px) {
        .blob-item {
            flex-wrap: wrap;
        }

        .blob-date {
            width: 100%;
            margin-top: 0.5em;
            padding-left: 3em;
        }

        .modal-footer {
            flex-direction: column;
            gap: 0.75em;
        }

        .blob-details {
            flex-direction: column;
            gap: 0.25em;
        }
    }
</style>
