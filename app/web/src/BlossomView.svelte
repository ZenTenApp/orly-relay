<script>
    import { createEventDispatcher, onMount } from "svelte";
    import { npubEncode } from "nostr-tools/nip19";
    import { SimplePool } from "nostr-tools/pool";
    import { fetchUserProfile, nostrClient } from "./nostr.js";
    import { getApiBase, getRelayUrls } from "./config.js";
    import { publishEventWithAuth } from "./websocket-auth.js";

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

    // Responsive variants state
    let blobVariants = [];
    let isLoadingVariants = false;
    let copiedVariant = null;
    let isDeletingVariants = false;
    let responsiveBlobs = new Set();  // Original hashes that have variants
    let variantHashes = new Set();    // Variant hashes to filter from list

    // TTL cache for variant fetches - prevents stale IndexedDB fallback
    // Map<blobHash, { timestamp: number, hasVariants: boolean }>
    const variantFetchCache = new Map();
    const VARIANT_CACHE_TTL_MS = 5 * 60 * 1000; // 5 minutes

    // Admin view state
    let isAdminView = false;
    let adminUserStats = [];
    let isLoadingAdmin = false;
    let selectedAdminUser = null;
    let selectedUserBlobs = [];

    // Sort state
    let sortBy = "date"; // "date" or "size"
    let sortOrder = "desc"; // "asc" or "desc"

    $: canAccess = isLoggedIn && userPubkey;
    $: isAdmin = currentEffectiveRole === "admin" || currentEffectiveRole === "owner";

    // Sort and display blobs (filter out variants)
    // Note: variantHashes.size forces reactivity when Set changes
    $: rawBlobs = selectedAdminUser ? selectedUserBlobs : blobs;
    $: filteredBlobs = (variantHashes.size, rawBlobs.filter(b => !variantHashes.has(b.sha256)));
    $: displayBlobs = sortBlobs(filteredBlobs, sortBy, sortOrder);

    // Force Svelte to track selectedHashes changes for checkbox reactivity
    $: selectedHashesSize = selectedHashes.size;

    function sortBlobs(blobList, by, order) {
        if (!blobList || blobList.length === 0) return blobList;
        const sorted = [...blobList].sort((a, b) => {
            let cmp = 0;
            if (by === "date") {
                cmp = (a.uploaded || 0) - (b.uploaded || 0);
            } else if (by === "size") {
                cmp = (a.size || 0) - (b.size || 0);
            }
            return order === "desc" ? -cmp : cmp;
        });
        return sorted;
    }

    function toggleSort(newSortBy) {
        if (sortBy === newSortBy) {
            sortOrder = sortOrder === "desc" ? "asc" : "desc";
        } else {
            sortBy = newSortBy;
            sortOrder = "desc";
        }
    }

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
            // Use standard base64 encoding per BUD-01 spec
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

    async function loadResponsiveBlobs(pubkey, merge = false) {
        try {
            const relays = getRelayUrls();
            const pool = nostrClient.getPool();
            const originals = merge ? new Set(responsiveBlobs) : new Set();
            const variants = merge ? new Set(variantHashes) : new Set();

            // Paginate to fetch ALL binding events (relay caps at 256 per query)
            const PAGE_SIZE = 256;
            let until = undefined;
            let totalEvents = 0;

            while (true) {
                const filter = {
                    kinds: [30063, 1063],
                    authors: [pubkey],
                    limit: PAGE_SIZE,
                };
                if (until !== undefined) {
                    filter.until = until;
                }

                const events = await pool.querySync(relays, filter);
                if (events.length === 0) break;

                totalEvents += events.length;

                // Find the oldest event timestamp for the next page cursor
                let oldestTimestamp = Infinity;
                for (const ev of events) {
                    if (ev.created_at < oldestTimestamp) {
                        oldestTimestamp = ev.created_at;
                    }
                }

                for (const ev of events) {
                    let originalHash = null;
                    const allHashes = [];

                    if (ev.kind === 30063) {
                        // ORLY format: d tag contains original hash
                        const dTag = ev.tags.find(t => t[0] === "d");
                        originalHash = dTag?.[1];
                        // Collect all x tags as variant hashes
                        for (const tag of ev.tags) {
                            if (tag[0] === "x" && tag[1]) {
                                allHashes.push(tag[1]);
                            }
                        }
                    } else if (ev.kind === 1063) {
                        // NIP-94/smesh format: imeta tags with variant field
                        // Find the "original" variant's hash
                        for (const tag of ev.tags) {
                            if (tag[0] === "imeta") {
                                const variantField = tag.find(f => f.startsWith("variant "));
                                const xField = tag.find(f => f.startsWith("x "));
                                if (xField) {
                                    const hash = xField.substring(2);
                                    allHashes.push(hash);
                                    if (variantField === "variant original") {
                                        originalHash = hash;
                                    }
                                }
                            } else if (tag[0] === "x" && tag[1]) {
                                // Also collect standalone x tags
                                if (!allHashes.includes(tag[1])) {
                                    allHashes.push(tag[1]);
                                }
                            }
                        }
                    }

                    if (originalHash) {
                        originals.add(originalHash);
                        for (const hash of allHashes) {
                            if (hash !== originalHash) {
                                variants.add(hash);
                            }
                        }
                    }
                }

                // If we got fewer than PAGE_SIZE, we've fetched everything
                if (events.length < PAGE_SIZE) break;

                // Move cursor before the oldest event in this page
                until = oldestTimestamp - 1;
            }

            responsiveBlobs = originals;
            variantHashes = variants;
            console.log("Found responsive blobs:", originals.size, "variants to hide:", variants.size, "total binding events:", totalEvents);
        } catch (err) {
            console.warn("Failed to load responsive blob info:", err);
            if (!merge) {
                responsiveBlobs = new Set();
                variantHashes = new Set();
            }
        }
    }

    async function loadBlobs() {
        if (!userPubkey) {
            console.log("loadBlobs: no userPubkey, skipping");
            return;
        }

        console.log("loadBlobs: starting, userSigner available:", !!userSigner);
        isLoading = true;
        error = "";

        try {
            await loadResponsiveBlobs(userPubkey, true);

            const url = `${getApiBase()}/blossom/list/${userPubkey}`;
            const authHeader = await createBlossomAuth(userSigner, "list");
            const response = await fetch(url, {
                headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
            });

            if (!response.ok) {
                throw new Error(`Failed to load blobs: ${response.statusText}`);
            }

            const data = await response.json();
            const blobList = Array.isArray(data) ? data : [];
            blobs = [...blobList].sort((a, b) => (b.uploaded || 0) - (a.uploaded || 0));
            console.log("Loaded blobs:", blobs.length);
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
            case "image": return "🖼️";
            case "video": return "🎬";
            case "audio": return "🎵";
            default: return "📄";
        }
    }

    function openModal(blob) {
        selectedBlob = blob;
        zoomLevel = 1;
        showModal = true;
        blobVariants = [];
        copiedVariant = null;
        // Fetch variants for images
        if (getMimeCategory(blob.type) === "image") {
            fetchBlobVariants(blob.sha256);
        }
    }

    function closeModal() {
        showModal = false;
        selectedBlob = null;
        zoomLevel = 1;
        blobVariants = [];
        copiedVariant = null;
    }

    /**
     * Parse imeta tag fields into an object
     */
    function parseImetaTag(tag) {
        const fields = {};
        for (let i = 1; i < tag.length; i++) {
            const part = tag[i];
            const spaceIndex = part.indexOf(' ');
            if (spaceIndex > 0) {
                const key = part.substring(0, spaceIndex);
                const value = part.substring(spaceIndex + 1);
                fields[key] = value;
            }
        }
        return fields;
    }

    // Track current variant fetch to prevent race conditions
    let currentVariantFetch = null;

    /**
     * Fetch binding events (kind 30063 or 1063) for a blob hash
     */
    async function fetchBlobVariants(sha256Hex) {
        // Store this request's hash to check later
        currentVariantFetch = sha256Hex;
        isLoadingVariants = true;
        blobVariants = [];

        try {
            const relays = getRelayUrls();
            const pool = nostrClient.getPool();

            // Use the correct pubkey - if viewing another user's blobs, use their pubkey
            const targetPubkey = selectedAdminUser?.pubkey || userPubkey;

            // Query for kind 30063 (ORLY format) with #d tag
            const filter30063 = {
                kinds: [30063],
                "#d": [sha256Hex],
                limit: 10
            };
            if (targetPubkey) {
                filter30063.authors = [targetPubkey];
            }

            // Query for kind 1063 (NIP-94/smesh format) with #x tag
            const filter1063 = {
                kinds: [1063],
                "#x": [sha256Hex],
                limit: 10
            };
            if (targetPubkey) {
                filter1063.authors = [targetPubkey];
            }

            console.log("Querying for variants with filters:", JSON.stringify([filter30063, filter1063]));
            // Query both filters in parallel
            const [events30063, events1063] = await Promise.all([
                pool.querySync(relays, filter30063),
                pool.querySync(relays, filter1063)
            ]);
            let events = [...events30063, ...events1063];

            // Check if this is still the current request (user might have switched blobs)
            if (currentVariantFetch !== sha256Hex) {
                console.log("Variant fetch cancelled - different blob selected");
                return;
            }

            console.log(`Found ${events.length} binding events from relay`);

            // If no events from relay, check local cache as fallback - but respect TTL
            if (events.length === 0) {
                const cached = variantFetchCache.get(sha256Hex);
                const now = Date.now();
                const cacheIsFresh = cached && (now - cached.timestamp) < VARIANT_CACHE_TTL_MS;

                if (cacheIsFresh) {
                    // Cache is fresh - trust it over stale IndexedDB data
                    // If cached says no variants, we're done; if it says has variants but relay
                    // returned nothing, the binding event was likely deleted - trust relay
                    console.log(`Cache is fresh (${cached.hasVariants ? 'had' : 'no'} variants), trusting relay result`);
                } else {
                    // Cache is stale or missing - check IndexedDB as fallback
                    console.log("No events from relay, cache stale, checking IndexedDB...");
                    try {
                        const { queryEventsFromDB } = await import('./nostr.js');
                        const cachedEvents = await queryEventsFromDB([filter30063, filter1063]);
                        if (cachedEvents.length > 0) {
                            console.log(`Found ${cachedEvents.length} binding events in IndexedDB cache`);
                            events = cachedEvents;
                        }
                    } catch (cacheErr) {
                        console.warn("Failed to query IndexedDB cache:", cacheErr);
                    }
                }
            }

            // Update the TTL cache with current result
            variantFetchCache.set(sha256Hex, {
                timestamp: Date.now(),
                hasVariants: events.length > 0
            });

            // Check again after cache query
            if (currentVariantFetch !== sha256Hex) {
                return;
            }

            if (events.length === 0) {
                // No binding events found - remove from responsiveBlobs if present
                if (responsiveBlobs.has(sha256Hex)) {
                    console.log(`Removing ${sha256Hex} from responsiveBlobs (no binding events found)`);
                    responsiveBlobs = new Set([...responsiveBlobs].filter(h => h !== sha256Hex));
                }
                isLoadingVariants = false;
                return;
            }

            // Parse variants from the most recent event
            const latestEvent = events.reduce((a, b) =>
                a.created_at > b.created_at ? a : b
            );

            const variants = [];
            for (const tag of latestEvent.tags) {
                if (tag[0] !== "imeta") continue;

                const fields = parseImetaTag(tag);
                if (!fields.url || !fields.x || !fields.dim) continue;

                const dimMatch = fields.dim.match(/^(\d+)x(\d+)$/);
                if (!dimMatch) continue;

                variants.push({
                    variant: fields.variant || "original",
                    url: fields.url,
                    sha256: fields.x,
                    width: parseInt(dimMatch[1], 10),
                    height: parseInt(dimMatch[2], 10),
                    mimeType: fields.m || "image/jpeg",
                    size: fields.size ? parseInt(fields.size, 10) : null
                });
            }

            // Final check before updating state
            if (currentVariantFetch !== sha256Hex) {
                return;
            }

            // Sort by width (smallest first)
            variants.sort((a, b) => a.width - b.width);
            blobVariants = variants;

        } catch (err) {
            console.error("Failed to fetch variants:", err);
        }

        if (currentVariantFetch === sha256Hex) {
            isLoadingVariants = false;
        }
    }

    /**
     * Copy variant URL to clipboard
     */
    async function copyVariantUrl(variant) {
        try {
            await navigator.clipboard.writeText(variant.url);
            copiedVariant = variant.sha256;
            setTimeout(() => {
                if (copiedVariant === variant.sha256) {
                    copiedVariant = null;
                }
            }, 2000);
        } catch (err) {
            console.error("Failed to copy:", err);
        }
    }

    /**
     * Format variant label for display
     */
    function formatVariantLabel(variant) {
        const labels = {
            thumb: "Thumbnail",
            mobile: "Mobile",
            "mobile-lg": "Mobile+",
            desktop: "Desktop",
            "desktop-lg": "Desktop+",
            original: "Original"
        };
        return labels[variant.variant] || variant.variant;
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
            // Starts with / - it's a path, prepend API base
            if (blob.url.startsWith("/")) {
                return `${getApiBase()}${blob.url}`;
            }
            // No protocol - looks like host:port/path, add http://
            // This handles cases like "localhost:3334/blossom/..."
            return `http://${blob.url}`;
        }
        // Fallback: construct URL with sha256 only
        return `${getApiBase()}/blossom/${blob.sha256}`;
    }

    function getThumbnailUrl(blob) {
        // Get thumbnail URL for images (128px using Lanczos scaling)
        const baseUrl = getBlobUrl(blob);
        const sep = baseUrl.includes('?') ? '&' : '?';
        return `${baseUrl}${sep}w=128`;
    }

    function openLoginModal() {
        dispatch("openLoginModal");
    }

    async function deleteBlob(blob) {
        const hasVariants = responsiveBlobs.has(blob.sha256) || blobVariants.length > 0;
        const confirmMsg = hasVariants
            ? `Delete blob ${truncateHash(blob.sha256)} and all its responsive variants?`
            : `Delete blob ${truncateHash(blob.sha256)}?`;

        if (!confirm(confirmMsg)) return;

        try {
            // If this blob has variants, delete them first
            if (hasVariants) {
                // Get variant hashes from blobVariants if available, otherwise query
                let variantHashesToDelete = blobVariants
                    .filter(v => v.sha256 !== blob.sha256)
                    .map(v => v.sha256);

                // If we don't have variants loaded, query for them
                // Track the binding event for deletion
                let bindingEventToDelete = null;
                if (variantHashesToDelete.length === 0 && responsiveBlobs.has(blob.sha256)) {
                    try {
                        const relays = getRelayUrls();
                        const pool = nostrClient.getPool();

                        // Query both kind 30063 (ORLY legacy) and kind 1063 (NIP-94)
                        const filter30063 = {
                            kinds: [30063],
                            "#d": [blob.sha256],
                            authors: userPubkey ? [userPubkey] : undefined,
                            limit: 1
                        };
                        const filter1063 = {
                            kinds: [1063],
                            "#x": [blob.sha256],
                            authors: userPubkey ? [userPubkey] : undefined,
                            limit: 1
                        };

                        const [events30063, events1063] = await Promise.all([
                            pool.querySync(relays, filter30063),
                            pool.querySync(relays, filter1063)
                        ]);

                        const events = [...events30063, ...events1063];
                        if (events.length > 0) {
                            // Use most recent event
                            bindingEventToDelete = events.reduce((a, b) =>
                                a.created_at > b.created_at ? a : b
                            );
                            for (const tag of bindingEventToDelete.tags) {
                                if (tag[0] === "x" && tag[1] && tag[1] !== blob.sha256) {
                                    variantHashesToDelete.push(tag[1]);
                                }
                            }
                        }
                    } catch (err) {
                        console.warn("Failed to query variants for deletion:", err);
                    }
                }

                // Delete variant blobs
                for (const variantHash of variantHashesToDelete) {
                    try {
                        const variantUrl = `${getApiBase()}/blossom/${variantHash}`;
                        const variantAuth = await createBlossomAuth(userSigner, "delete", variantHash);
                        await fetch(variantUrl, {
                            method: "DELETE",
                            headers: variantAuth ? { Authorization: `Nostr ${variantAuth}` } : {},
                        });
                        console.log("Deleted variant:", variantHash);
                    } catch (err) {
                        console.warn("Failed to delete variant:", variantHash, err);
                    }
                }

                // Publish a deletion event for the binding event (kind 5)
                try {
                    const deleteTags = [];
                    if (bindingEventToDelete) {
                        if (bindingEventToDelete.kind === 30063) {
                            // Addressable event - use 'a' tag
                            deleteTags.push(["a", `30063:${userPubkey}:${blob.sha256}`]);
                        } else {
                            // Regular event (kind 1063) - use 'e' tag with event ID
                            deleteTags.push(["e", bindingEventToDelete.id]);
                        }
                    } else {
                        // Fallback to 'a' tag for legacy events
                        deleteTags.push(["a", `30063:${userPubkey}:${blob.sha256}`]);
                    }

                    const deleteEvent = {
                        kind: 5,
                        created_at: Math.floor(Date.now() / 1000),
                        content: "Deleted responsive variants",
                        tags: deleteTags,
                    };
                    const signedDelete = await userSigner.signEvent(deleteEvent);
                    const relayUrl = getRelayUrls()[0];
                    await publishEventWithAuth(relayUrl, signedDelete, userSigner, userPubkey);
                    console.log("Published deletion event for binding:", signedDelete.id);
                } catch (err) {
                    console.warn("Failed to publish deletion event:", err);
                }

                // Update local state
                responsiveBlobs = new Set([...responsiveBlobs].filter(h => h !== blob.sha256));
                variantHashes = new Set([...variantHashes].filter(h => !variantHashesToDelete.includes(h)));
            }

            // Delete the original blob
            const url = `${getApiBase()}/blossom/${blob.sha256}`;
            const authHeader = await createBlossomAuth(userSigner, "delete", blob.sha256);
            const response = await fetch(url, {
                method: "DELETE",
                headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
            });

            if (!response.ok) {
                throw new Error(`Failed to delete: ${response.statusText}`);
            }

            console.log("Delete successful, removing blob from list:", blob.sha256);
            blobs = blobs.filter(b => b.sha256 !== blob.sha256);
            console.log("Blobs after filter:", blobs.length);
            if (selectedBlob?.sha256 === blob.sha256) {
                closeModal();
            }
        } catch (err) {
            console.error("Error deleting blob:", err);
            alert(`Failed to delete blob: ${err.message}`);
        }
    }

    async function deleteVariants(blob) {
        if (!confirm(`Delete all responsive variants for this image?\n\nThis will remove all generated sizes (thumbnail, mobile, desktop) but keep the original.`)) return;

        isDeletingVariants = true;

        try {
            const url = `${getApiBase()}/blossom/delete-variants/${blob.sha256}`;
            const authHeader = await createBlossomAuth(userSigner, "delete", blob.sha256);
            const response = await fetch(url, {
                method: "DELETE",
                headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
            });

            if (!response.ok) {
                const reason = response.headers.get("X-Reason") || response.statusText;
                throw new Error(reason);
            }

            const result = await response.json();
            console.log("Delete variants result:", result);

            // Clear variants list and refresh
            blobVariants = [];
            alert(`Deleted ${result.deleted} variants.`);
        } catch (err) {
            console.error("Error deleting variants:", err);
            alert(`Failed to delete variants: ${err.message}`);
        } finally {
            isDeletingVariants = false;
        }
    }

    function handleFileSelect(event) {
        selectedFiles = Array.from(event.target.files);
    }

    function triggerFileInput() {
        fileInput?.click();
    }

    // Static image types that should get variants (excludes animated formats)
    const STATIC_IMAGE_TYPES = [
        "image/jpeg",
        "image/png",
        "image/webp",  // Can be animated but usually static
        "image/bmp",
        "image/tiff",
        "image/avif",
        "image/heic",
        "image/heif",
    ];

    function isStaticImage(mimeType) {
        return STATIC_IMAGE_TYPES.includes(mimeType);
    }

    /**
     * Generate variants for a file and upload all (original + variants).
     * Order: generate variants -> publish binding event -> upload blobs
     */
    async function uploadWithVariants(file, fileIndex, totalFiles) {
        const baseProgress = `[${fileIndex + 1}/${totalFiles}] ${file.name}`;
        const uploadUrl = `${getApiBase()}/blossom/upload`;

        // Step 1: Compute original hash
        uploadProgress = `${baseProgress}: Computing hash...`;
        const originalData = await file.arrayBuffer();
        const originalHash = await computeSHA256(originalData);

        // Step 2: Create ImageBitmap and generate all variants in memory
        uploadProgress = `${baseProgress}: Processing image...`;
        const imageBitmap = await createImageBitmap(file);
        const originalWidth = imageBitmap.width;
        const originalHeight = imageBitmap.height;

        const variantBlobs = [];
        for (const { name, maxWidth } of VARIANT_SIZES) {
            if (originalWidth <= maxWidth) continue;

            uploadProgress = `${baseProgress}: Creating ${name} variant...`;
            const resized = await resizeImage(imageBitmap, maxWidth);
            const resizedData = await resized.blob.arrayBuffer();
            const variantHash = await computeSHA256(resizedData);

            variantBlobs.push({
                name,
                blob: resized.blob,
                hash: variantHash,
                width: resized.width,
                height: resized.height,
            });
        }
        imageBitmap.close();

        // Step 3: Build variant metadata for binding event
        const variants = [
            {
                variant: "original",
                sha256: originalHash,
                url: `${getApiBase()}/blossom/${originalHash}`,
                width: originalWidth,
                height: originalHeight,
                mimeType: file.type || "image/jpeg",
                size: file.size,
            },
            ...variantBlobs.map(v => ({
                variant: v.name,
                sha256: v.hash,
                url: `${getApiBase()}/blossom/${v.hash}.jpg`,
                width: v.width,
                height: v.height,
                mimeType: "image/jpeg",
                size: v.blob.size,
            })),
        ];

        console.log("Variants created:", variants.length, variants.map(v => v.variant));

        // Step 4: Publish binding event FIRST (before uploading blobs)
        if (variants.length > 1) {
            uploadProgress = `${baseProgress}: Publishing binding event...`;

            // Use kind 1063 (NIP-94 File Metadata) for binding events
            const bindingEvent = {
                kind: 1063,
                created_at: Math.floor(Date.now() / 1000),
                content: "",
                tags: [
                    ...variants.map(v => [
                        "imeta",
                        `url ${v.url}`,
                        `x ${v.sha256}`,
                        `m ${v.mimeType}`,
                        `dim ${v.width}x${v.height}`,
                        `variant ${v.variant}`,
                        `size ${v.size}`,
                    ]),
                    ...variants.map(v => ["x", v.sha256]),
                ],
            };

            const signedEvent = await userSigner.signEvent(bindingEvent);

            // Use websocket auth for proper NIP-42 handling
            const relayUrl = getRelayUrls()[0];
            const result = await publishEventWithAuth(relayUrl, signedEvent, userSigner, userPubkey);
            console.log("Binding event published:", signedEvent.id, result);

            // Update local sets immediately
            responsiveBlobs = new Set([...responsiveBlobs, originalHash]);
            const newVariantHashes = variants
                .filter(v => v.sha256 !== originalHash)
                .map(v => v.sha256);
            variantHashes = new Set([...variantHashes, ...newVariantHashes]);
        }

        // Step 5: Upload original blob
        uploadProgress = `${baseProgress}: Uploading original...`;
        const originalAuth = await createBlossomAuth(userSigner, "upload", originalHash);

        const originalResponse = await fetch(uploadUrl, {
            method: "PUT",
            headers: {
                "Content-Type": file.type || "application/octet-stream",
                "X-SHA-256": originalHash,
                ...(originalAuth ? { Authorization: `Nostr ${originalAuth}` } : {}),
            },
            body: file,
        });

        if (!originalResponse.ok) {
            const reason = originalResponse.headers.get("X-Reason") || originalResponse.statusText;
            throw new Error(reason);
        }

        const descriptor = await originalResponse.json();

        // Step 6: Upload variant blobs
        for (const v of variantBlobs) {
            uploadProgress = `${baseProgress}: Uploading ${v.name}...`;

            // Check if variant already exists
            const checkUrl = `${getApiBase()}/blossom/${v.hash}`;
            const headResponse = await fetch(checkUrl, { method: "HEAD" });

            if (!headResponse.ok) {
                const variantAuth = await createBlossomAuth(userSigner, "upload", v.hash);
                const variantResponse = await fetch(uploadUrl, {
                    method: "PUT",
                    headers: {
                        "Content-Type": "image/jpeg",
                        "X-SHA-256": v.hash,
                        ...(variantAuth ? { Authorization: `Nostr ${variantAuth}` } : {}),
                    },
                    body: v.blob,
                });

                if (!variantResponse.ok) {
                    console.warn(`Failed to upload ${v.name} variant:`, variantResponse.statusText);
                }
            }
        }

        return descriptor;
    }

    async function uploadFiles() {
        if (selectedFiles.length === 0) return;

        isUploading = true;
        error = "";
        const uploaded = [];
        const failed = [];

        for (let i = 0; i < selectedFiles.length; i++) {
            const file = selectedFiles[i];

            try {
                if (isStaticImage(file.type)) {
                    // Upload with variant generation for static images
                    const descriptor = await uploadWithVariants(file, i, selectedFiles.length);
                    console.log("Upload with variants complete:", descriptor);
                    uploaded.push(descriptor);
                } else {
                    // Regular upload for non-images
                    uploadProgress = `Uploading ${i + 1}/${selectedFiles.length}: ${file.name}`;
                    const url = `${getApiBase()}/blossom/upload`;
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
                }
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
            console.log("Upload complete, refreshing blob list...");
            await loadBlobs();
            console.log("Blob list refresh complete, blobs count:", blobs.length);
        }

        if (failed.length > 0) {
            error = `Failed to upload: ${failed.map(f => f.name).join(", ")}`;
        }
    }

    // Admin functions
    function hexToNpub(pubkeyHex) {
        try {
            return npubEncode(pubkeyHex);
        } catch (e) {
            return truncateHash(pubkeyHex);
        }
    }

    function truncateNpub(npub) {
        if (!npub) return "";
        return `${npub.slice(0, 12)}...${npub.slice(-8)}`;
    }

    async function fetchAdminUserStats() {
        isLoadingAdmin = true;
        error = "";

        try {
            const url = `${getApiBase()}/blossom/admin/users`;
            const authHeader = await createBlossomAuth(userSigner, "admin");
            const response = await fetch(url, {
                headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
            });

            if (!response.ok) {
                throw new Error(`Failed to load user stats: ${response.statusText}`);
            }

            adminUserStats = await response.json();

            // Fetch profiles for each user (non-blocking)
            for (const stat of adminUserStats) {
                fetchUserProfile(stat.pubkey).then(profile => {
                    stat.profile = profile || { name: "", picture: "" };
                    adminUserStats = adminUserStats; // trigger reactivity
                }).catch(() => {
                    stat.profile = { name: "", picture: "" };
                });
            }
        } catch (err) {
            console.error("Error fetching admin user stats:", err);
            error = err.message || "Failed to load user stats";
        } finally {
            isLoadingAdmin = false;
        }
    }

    async function loadUserBlobs(pubkeyHex) {
        isLoading = true;
        error = "";

        try {
            // Load the user's responsive blob info (replaces current sets)
            await loadResponsiveBlobs(pubkeyHex, false);

            const url = `${getApiBase()}/blossom/list/${pubkeyHex}`;
            const authHeader = await createBlossomAuth(userSigner, "list");
            const response = await fetch(url, {
                headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
            });

            if (!response.ok) {
                throw new Error(`Failed to load user blobs: ${response.statusText}`);
            }

            const userBlobData = await response.json();
            selectedUserBlobs = [...userBlobData].sort((a, b) => (b.uploaded || 0) - (a.uploaded || 0));
        } catch (err) {
            console.error("Error loading user blobs:", err);
            error = err.message || "Failed to load user blobs";
        } finally {
            isLoading = false;
        }
    }

    function enterAdminView() {
        isAdminView = true;
        fetchAdminUserStats();
    }

    async function exitAdminView() {
        isAdminView = false;
        adminUserStats = [];
        selectedAdminUser = null;
        selectedUserBlobs = [];
        // Reload responsive blob info for the current user's own blobs
        if (userPubkey) {
            await loadResponsiveBlobs(userPubkey, false);
        }
    }

    async function selectUser(userStat) {
        selectedAdminUser = {
            pubkey: userStat.pubkey,
            profile: userStat.profile
        };
        await loadUserBlobs(userStat.pubkey);
    }

    function exitUserView() {
        selectedAdminUser = null;
        selectedUserBlobs = [];
    }

    function handleRefresh() {
        if (selectedAdminUser) {
            loadUserBlobs(selectedAdminUser.pubkey);
        } else if (isAdminView) {
            fetchAdminUserStats();
        } else {
            loadBlobs();
        }
    }

    // Generate variants state (for single image)
    let isGeneratingVariants = false;
    let generatingProgress = "";

    // Selection state for bulk delete
    let selectedHashes = new Set();
    let isDeletingSelected = false;

    function toggleSelection(hash, event) {
        event.stopPropagation();
        if (selectedHashes.has(hash)) {
            selectedHashes.delete(hash);
        } else {
            selectedHashes.add(hash);
        }
        selectedHashes = selectedHashes; // trigger reactivity
    }

    function isSelected(hash) {
        return selectedHashes.has(hash);
    }

    async function deleteSelectedBlobs() {
        if (selectedHashes.size === 0) return;
        if (!confirm(`Delete ${selectedHashes.size} selected file(s)? This cannot be undone.`)) return;

        isDeletingSelected = true;
        error = "";
        let deleted = 0;
        let failed = 0;

        const hashesToDelete = Array.from(selectedHashes);

        for (const hash of hashesToDelete) {
            try {
                const url = `${getApiBase()}/blossom/${hash}`;
                const authHeader = await createBlossomAuth(userSigner, "delete", hash);
                const response = await fetch(url, {
                    method: "DELETE",
                    headers: authHeader ? { Authorization: `Nostr ${authHeader}` } : {},
                });

                if (response.ok) {
                    deleted++;
                } else {
                    console.error(`Delete failed for ${hash}: ${response.status} ${response.statusText}`);
                    failed++;
                }
            } catch (err) {
                console.error(`Failed to delete ${hash}:`, err);
                failed++;
            }
        }

        // Clear selection
        selectedHashes = new Set();

        // Refresh the list
        try {
            if (selectedAdminUser) {
                await loadUserBlobs(selectedAdminUser.pubkey);
            } else {
                await loadBlobs();
            }
        } catch (err) {
            console.error("Failed to refresh blobs:", err);
        }

        isDeletingSelected = false;

        if (failed > 0) {
            error = `Deleted ${deleted}, failed ${failed}`;
        }
    }

    // Variant size definitions per NIP-XX
    // Selection rule: pick smallest variant >= target width (next-larger)
    const VARIANT_SIZES = [
        { name: "thumb", maxWidth: 128 },
        { name: "mobile-sm", maxWidth: 512 },
        { name: "mobile-lg", maxWidth: 1024 },
        { name: "desktop-sm", maxWidth: 1536 },
        { name: "desktop-md", maxWidth: 2048 },
        { name: "desktop-lg", maxWidth: 2560 },
    ];

    /**
     * Compute SHA256 hash of ArrayBuffer using Web Crypto API
     */
    async function computeSHA256(data) {
        const hashBuffer = await crypto.subtle.digest("SHA-256", data);
        const hashArray = Array.from(new Uint8Array(hashBuffer));
        return hashArray.map(b => b.toString(16).padStart(2, "0")).join("");
    }

    /**
     * Resize image using Canvas API (GPU accelerated)
     * Uses createImageBitmap for hardware acceleration when available
     */
    async function resizeImage(imageBitmap, maxWidth, quality = 0.85) {
        const { width, height } = imageBitmap;

        // Calculate new dimensions maintaining aspect ratio
        let newWidth = width;
        let newHeight = height;
        if (width > maxWidth) {
            newWidth = maxWidth;
            newHeight = Math.round((height * maxWidth) / width);
        }

        // Use OffscreenCanvas if available (better performance)
        let canvas;
        let ctx;
        if (typeof OffscreenCanvas !== "undefined") {
            canvas = new OffscreenCanvas(newWidth, newHeight);
            ctx = canvas.getContext("2d", { alpha: false });
        } else {
            canvas = document.createElement("canvas");
            canvas.width = newWidth;
            canvas.height = newHeight;
            ctx = canvas.getContext("2d", { alpha: false });
        }

        // Enable image smoothing for quality scaling
        ctx.imageSmoothingEnabled = true;
        ctx.imageSmoothingQuality = "high";

        // Draw scaled image
        ctx.drawImage(imageBitmap, 0, 0, newWidth, newHeight);

        // Convert to blob
        let blob;
        if (typeof OffscreenCanvas !== "undefined" && canvas instanceof OffscreenCanvas) {
            blob = await canvas.convertToBlob({ type: "image/jpeg", quality });
        } else {
            blob = await new Promise(resolve => canvas.toBlob(resolve, "image/jpeg", quality));
        }

        return {
            blob,
            width: newWidth,
            height: newHeight,
        };
    }

    /**
     * Generate responsive variants for a single image.
     * Order: generate variants -> publish binding event -> upload blobs
     */
    async function generateVariants(blob) {
        if (!confirm(`Generate responsive variants for this image?\n\nThis will create thumb (128px), mobile (512px), desktop (1280px), and original variants.`)) {
            return;
        }

        isGeneratingVariants = true;
        generatingProgress = "Loading image...";
        error = "";

        try {
            // Step 1: Fetch the original image
            const imageUrl = getBlobUrl(blob);
            const imageResponse = await fetch(imageUrl);
            if (!imageResponse.ok) {
                throw new Error("Failed to fetch original image");
            }
            const imageBlob = await imageResponse.blob();

            // Create ImageBitmap for hardware-accelerated processing
            generatingProgress = "Processing image...";
            const imageBitmap = await createImageBitmap(imageBlob);
            const originalWidth = imageBitmap.width;
            const originalHeight = imageBitmap.height;

            // Step 2: Generate all variants in memory (don't upload yet)
            const variantBlobs = [];
            for (let i = 0; i < VARIANT_SIZES.length; i++) {
                const { name, maxWidth } = VARIANT_SIZES[i];
                if (originalWidth <= maxWidth) continue;

                generatingProgress = `Creating ${name}...`;
                const resized = await resizeImage(imageBitmap, maxWidth);
                const resizedData = await resized.blob.arrayBuffer();
                const variantHash = await computeSHA256(resizedData);

                variantBlobs.push({
                    name,
                    blob: resized.blob,
                    hash: variantHash,
                    width: resized.width,
                    height: resized.height,
                });
            }
            imageBitmap.close();

            // Step 3: Build variant metadata for binding event
            const variants = [
                {
                    variant: "original",
                    sha256: blob.sha256,
                    url: imageUrl,
                    width: originalWidth,
                    height: originalHeight,
                    mimeType: blob.type || "image/jpeg",
                    size: imageBlob.size,
                },
                ...variantBlobs.map(v => ({
                    variant: v.name,
                    sha256: v.hash,
                    url: `${getApiBase()}/blossom/${v.hash}.jpg`,
                    width: v.width,
                    height: v.height,
                    mimeType: "image/jpeg",
                    size: v.blob.size,
                })),
            ];

            if (variants.length <= 1) {
                generatingProgress = "Image too small for variants";
                setTimeout(() => { generatingProgress = ""; }, 2000);
                return;
            }

            // Step 4: Publish binding event FIRST with proper auth
            // Use kind 1063 (NIP-94 File Metadata) for binding events
            generatingProgress = "Publishing binding event...";
            const bindingEvent = {
                kind: 1063,
                created_at: Math.floor(Date.now() / 1000),
                content: "",
                tags: [
                    ...variants.map(v => [
                        "imeta",
                        `url ${v.url}`,
                        `x ${v.sha256}`,
                        `m ${v.mimeType}`,
                        `dim ${v.width}x${v.height}`,
                        `variant ${v.variant}`,
                        `size ${v.size}`,
                    ]),
                    ...variants.map(v => ["x", v.sha256]),
                ],
            };

            const signedEvent = await userSigner.signEvent(bindingEvent);
            const relayUrl = getRelayUrls()[0];
            const result = await publishEventWithAuth(relayUrl, signedEvent, userSigner, userPubkey);
            console.log("Binding event published:", signedEvent.id, result);

            // Update local sets
            responsiveBlobs = new Set([...responsiveBlobs, blob.sha256]);
            const newVariantHashes = variants
                .filter(v => v.sha256 !== blob.sha256)
                .map(v => v.sha256);
            variantHashes = new Set([...variantHashes, ...newVariantHashes]);

            // Step 5: Upload variant blobs
            const uploadUrl = `${getApiBase()}/blossom/upload`;
            for (const v of variantBlobs) {
                generatingProgress = `Uploading ${v.name}...`;

                // Check if already exists
                const checkUrl = `${getApiBase()}/blossom/${v.hash}`;
                const headResponse = await fetch(checkUrl, { method: "HEAD" });

                if (!headResponse.ok) {
                    const uploadAuth = await createBlossomAuth(userSigner, "upload", v.hash);
                    const uploadResponse = await fetch(uploadUrl, {
                        method: "PUT",
                        headers: {
                            "Content-Type": "image/jpeg",
                            "X-SHA-256": v.hash,
                            ...(uploadAuth ? { Authorization: `Nostr ${uploadAuth}` } : {}),
                        },
                        body: v.blob,
                    });

                    if (!uploadResponse.ok) {
                        console.warn(`Failed to upload ${v.name}:`, uploadResponse.statusText);
                    }
                }
            }

            generatingProgress = "Done!";

            // Reload variants for the modal
            await fetchBlobVariants(blob.sha256);

            setTimeout(() => { generatingProgress = ""; }, 2000);
        } catch (err) {
            console.error("Error generating variants:", err);
            error = err.message || "Failed to generate variants";
        } finally {
            isGeneratingVariants = false;
        }
    }

</script>

<svelte:window on:keydown={handleKeydown} />

{#if canAccess}
    <div class="blossom-view">
        <div class="header-section">
            {#if selectedAdminUser}
                <button class="back-btn" on:click={exitUserView}>
                    &larr; Back
                </button>
                <h3 class="user-header">
                    {#if selectedAdminUser.profile?.picture}
                        <img src={selectedAdminUser.profile.picture} alt="" class="header-avatar" />
                    {/if}
                    {selectedAdminUser.profile?.name || truncateNpub(hexToNpub(selectedAdminUser.pubkey))}
                </h3>
            {:else if isAdminView}
                <button class="back-btn" on:click={exitAdminView}>
                    &larr; Back
                </button>
                <h3>All Users Storage</h3>
            {:else}
                <h3>Blossom Media Storage</h3>
            {/if}

            <div class="header-buttons">
                {#if !isAdminView || selectedAdminUser}
                    {#if selectedHashes.size > 0}
                        <button
                            class="delete-selected-btn"
                            on:click={deleteSelectedBlobs}
                            disabled={isDeletingSelected}
                        >
                            {isDeletingSelected ? "Deleting..." : `Delete Selected (${selectedHashes.size})`}
                        </button>
                    {/if}
                    <select class="sort-select" bind:value={sortBy}>
                        <option value="date">Date {sortBy === "date" ? (sortOrder === "desc" ? "↓" : "↑") : ""}</option>
                        <option value="size">Size {sortBy === "size" ? (sortOrder === "desc" ? "↓" : "↑") : ""}</option>
                    </select>
                    <button class="sort-order-btn" on:click={() => sortOrder = sortOrder === "desc" ? "asc" : "desc"} title="Toggle sort order">
                        {sortOrder === "desc" ? "↓" : "↑"}
                    </button>
                {/if}
                {#if isAdmin && !isAdminView && !selectedAdminUser}
                    <button class="admin-btn" on:click={enterAdminView} disabled={isLoading}>
                        Admin
                    </button>
                {/if}
                <button class="refresh-btn" on:click={handleRefresh} disabled={isLoading || isLoadingAdmin}>
                    🔄 {isLoading || isLoadingAdmin ? "Loading..." : "Refresh"}
                </button>
            </div>
        </div>

        {#if !isAdminView && !selectedAdminUser}
            <div class="upload-section">
                <span class="upload-label">Upload new files</span>
                <input
                    type="file"
                    multiple
                    bind:this={fileInput}
                    on:change={handleFileSelect}
                    class="file-input-hidden"
                />
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
                <button class="select-btn" on:click={triggerFileInput} disabled={isUploading}>
                    Select Files
                </button>
            </div>

        {/if}

        {#if error}
            <div class="error-message">
                {error}
            </div>
        {/if}

        {#if isAdminView && !selectedAdminUser}
            <!-- Admin users list view -->
            {#if isLoadingAdmin}
                <div class="loading">Loading user statistics...</div>
            {:else if adminUserStats.length === 0}
                <div class="empty-state">
                    <p>No users have uploaded files yet.</p>
                </div>
            {:else}
                <div class="admin-users-list">
                    {#each adminUserStats as userStat}
                        <div
                            class="user-stat-item"
                            on:click={() => selectUser(userStat)}
                            on:keypress={(e) => e.key === "Enter" && selectUser(userStat)}
                            role="button"
                            tabindex="0"
                        >
                            <div class="user-avatar-container">
                                {#if userStat.profile?.picture}
                                    <img src={userStat.profile.picture} alt="" class="user-avatar" />
                                {:else}
                                    <div class="user-avatar-placeholder"></div>
                                {/if}
                            </div>
                            <div class="user-info">
                                <div class="user-name">
                                    {userStat.profile?.name || truncateNpub(hexToNpub(userStat.pubkey))}
                                </div>
                                <div class="user-npub" title={userStat.pubkey}>
                                    <span class="npub-full">{hexToNpub(userStat.pubkey)}</span>
                                    <span class="npub-truncated">{truncateNpub(hexToNpub(userStat.pubkey))}</span>
                                </div>
                            </div>
                            <div class="user-stats">
                                <span class="blob-count">{userStat.blob_count} files</span>
                                <span class="total-size">{formatSize(userStat.total_size_bytes)}</span>
                            </div>
                        </div>
                    {/each}
                </div>
            {/if}
        {:else}
            <!-- Normal blob list view (own files or selected user's files) -->
            {#if isLoading && displayBlobs.length === 0}
                <div class="loading">Loading blobs...</div>
            {:else if displayBlobs.length === 0}
                <div class="empty-state">
                    <p>{selectedAdminUser ? "No files found for this user." : "No files found in your Blossom storage."}</p>
                </div>
            {:else}
                <div class="blob-list">
                    {#each displayBlobs as blob}
                        <div
                            class="blob-item"
                            class:selected={(selectedHashesSize, selectedHashes.has(blob.sha256))}
                            on:click={() => openModal(blob)}
                            on:keypress={(e) => e.key === "Enter" && openModal(blob)}
                            role="button"
                            tabindex="0"
                        >
                            <input
                                type="checkbox"
                                class="blob-checkbox"
                                checked={(selectedHashesSize, selectedHashes.has(blob.sha256))}
                                on:change={(e) => toggleSelection(blob.sha256, e)}
                                on:keypress|stopPropagation
                            />
                            <div class="blob-thumbnail">
                                {#if getMimeCategory(blob.type) === "image"}
                                    <img src={getThumbnailUrl(blob)} alt="" class="thumbnail-img" loading="lazy" />
                                {:else if getMimeCategory(blob.type) === "video"}
                                    <video src={getBlobUrl(blob)} class="thumbnail-video" muted preload="metadata"></video>
                                {:else}
                                    <span class="thumbnail-icon">{getMimeIcon(blob.type)}</span>
                                {/if}
                            </div>
                            <div class="blob-info">
                                <div class="blob-hash" title={blob.sha256}>
                                    <span class="hash-full">{blob.sha256}</span>
                                    <span class="hash-truncated">{truncateHash(blob.sha256)}</span>
                                </div>
                                <div class="blob-meta">
                                    <span class="blob-size">{formatSize(blob.size)}</span>
                                    <span class="blob-type">{blob.type || "unknown"}</span>
                                    <span class="blob-date">{formatDate(blob.uploaded)}</span>
                                    {#if responsiveBlobs.has(blob.sha256)}
                                        <span class="responsive-chip">responsive</span>
                                    {/if}
                                </div>
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

                <!-- Responsive Variants Section -->
                {#if getMimeCategory(selectedBlob.type) === "image"}
                    <div class="variants-section">
                        <div class="variants-header">
                            <span class="variants-title">Responsive Variants</span>
                            {#if isLoadingVariants}
                                <span class="variants-loading">Loading...</span>
                            {/if}
                        </div>
                        {#if blobVariants.length > 0}
                            <div class="variants-list">
                                {#each blobVariants as variant}
                                    <div class="variant-item">
                                        <span class="variant-label">{formatVariantLabel(variant)}</span>
                                        <span class="variant-dims">{variant.width}×{variant.height}</span>
                                        {#if variant.size}
                                            <span class="variant-size">{formatSize(variant.size)}</span>
                                        {/if}
                                        <button
                                            class="variant-copy-btn"
                                            class:copied={copiedVariant === variant.sha256}
                                            on:click={() => copyVariantUrl(variant)}
                                        >
                                            {copiedVariant === variant.sha256 ? "Copied!" : "Copy URL"}
                                        </button>
                                    </div>
                                {/each}
                            </div>
                        {:else if !isLoadingVariants}
                            <div class="variants-empty">
                                No responsive variants found. Use "Migrate Images" to create them.
                            </div>
                        {/if}
                    </div>
                {/if}

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
                    {#if getMimeCategory(selectedBlob.type) === "image"}
                        {#if blobVariants.length === 0}
                            <button class="action-btn primary" on:click={() => generateVariants(selectedBlob)} disabled={isGeneratingVariants}>
                                {isGeneratingVariants ? generatingProgress : "Generate Variants"}
                            </button>
                        {:else}
                            <button class="action-btn warning" on:click={() => deleteVariants(selectedBlob)} disabled={isDeletingVariants}>
                                {isDeletingVariants ? "Deleting..." : "Delete Variants"}
                            </button>
                        {/if}
                    {/if}
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
        box-sizing: border-box;
        width: 100%;
        max-width: 100%;
        overflow-x: hidden;
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
        flex: 1;
    }

    .header-buttons {
        display: flex;
        align-items: center;
        gap: 0.5em;
    }

    .sort-select {
        padding: 0.4em 0.6em;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background-color: var(--card-bg);
        color: var(--text-color);
        font-size: 0.9em;
        cursor: pointer;
    }

    .sort-select:focus {
        outline: none;
        border-color: var(--primary);
    }

    .sort-order-btn {
        padding: 0.4em 0.6em;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background-color: var(--card-bg);
        color: var(--text-color);
        font-size: 0.9em;
        cursor: pointer;
        min-width: 2em;
    }

    .sort-order-btn:hover {
        background-color: var(--sidebar-bg);
    }

    .delete-selected-btn {
        padding: 0.4em 0.8em;
        border: none;
        border-radius: 4px;
        background-color: var(--warning, #dc3545);
        color: white;
        cursor: pointer;
        font-size: 0.9em;
    }

    .delete-selected-btn:hover:not(:disabled) {
        opacity: 0.9;
    }

    .delete-selected-btn:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .back-btn {
        background: transparent;
        border: 1px solid var(--border-color);
        color: var(--text-color);
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        margin-right: 0.5em;
    }

    .back-btn:hover {
        background-color: var(--sidebar-bg);
    }

    .admin-btn {
        background-color: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
    }

    .admin-btn:hover:not(:disabled) {
        background-color: var(--accent-hover-color);
    }

    .admin-btn:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .user-header {
        display: flex;
        align-items: center;
        gap: 0.5em;
    }

    .header-avatar {
        width: 28px;
        height: 28px;
        border-radius: 50%;
        object-fit: cover;
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

    .upload-label {
        color: var(--text-color);
        font-size: 0.95em;
        flex: 1;
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
        width: 100%;
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
        min-width: 0;
        overflow: hidden;
    }

    .blob-item:hover,
    .blob-item.selected {
        background-color: var(--sidebar-bg);
    }

    .blob-checkbox {
        width: 18px;
        height: 18px;
        flex-shrink: 0;
        cursor: pointer;
        accent-color: var(--primary);
    }

    .blob-thumbnail {
        width: 48px;
        height: 48px;
        flex-shrink: 0;
        display: flex;
        align-items: center;
        justify-content: center;
        background-color: var(--bg-color);
        border-radius: 4px;
        overflow: hidden;
    }

    .thumbnail-img,
    .thumbnail-video {
        width: 100%;
        height: 100%;
        object-fit: cover;
    }

    .thumbnail-icon {
        font-size: 1.5em;
    }

    .blob-info {
        flex: 1;
        min-width: 0;
        overflow: hidden;
    }

    .blob-hash {
        font-family: monospace;
        font-size: 0.9em;
        color: var(--text-color);
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .hash-full {
        display: none;
    }

    .hash-truncated {
        display: inline;
    }

    @media (min-width: 900px) {
        .hash-full {
            display: inline;
        }
        .hash-truncated {
            display: none;
        }
    }

    .blob-meta {
        display: flex;
        gap: 1em;
        font-size: 0.8em;
        color: var(--text-color);
        opacity: 0.7;
        margin-top: 0.25em;
        flex-wrap: wrap;
    }

    .blob-meta .blob-date {
        white-space: nowrap;
    }

    .responsive-chip {
        background: var(--success, #22c55e);
        color: white;
        padding: 0.1em 0.5em;
        border-radius: 4px;
        font-size: 0.85em;
        font-weight: 500;
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

    /* Admin users list styles */
    .admin-users-list {
        display: flex;
        flex-direction: column;
        gap: 0.5em;
        width: 100%;
    }

    .user-stat-item {
        display: flex;
        align-items: center;
        gap: 1em;
        padding: 0.75em 1em;
        background-color: var(--card-bg);
        border-radius: 6px;
        cursor: pointer;
        transition: background-color 0.2s;
    }

    .user-stat-item:hover {
        background-color: var(--sidebar-bg);
    }

    .user-avatar-container {
        flex-shrink: 0;
    }

    .user-avatar {
        width: 40px;
        height: 40px;
        border-radius: 50%;
        object-fit: cover;
    }

    .user-avatar-placeholder {
        width: 40px;
        height: 40px;
        border-radius: 50%;
        background-color: var(--border-color);
    }

    .user-info {
        flex: 1;
        min-width: 0;
    }

    .user-name {
        font-weight: 500;
        color: var(--text-color);
    }

    .user-npub {
        font-family: monospace;
        font-size: 0.8em;
        color: var(--text-color);
        opacity: 0.6;
    }

    .npub-full {
        display: inline;
    }

    .npub-truncated {
        display: none;
    }

    .user-stats {
        display: flex;
        flex-direction: column;
        align-items: flex-end;
        gap: 0.25em;
    }

    .user-stats .blob-count,
    .user-stats .total-size {
        font-size: 0.85em;
        color: var(--text-color);
        opacity: 0.7;
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
        top: 3em;
        left: 200px;
        right: 0;
        bottom: 0;
        background-color: rgba(0, 0, 0, 0.8);
        display: flex;
        align-items: center;
        justify-content: center;
        z-index: 1000;
        padding: 1em;
        box-sizing: border-box;
    }

    .modal-content {
        background-color: var(--bg-color);
        border-radius: 8px;
        max-width: 100%;
        max-height: 100%;
        width: 100%;
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

    /* Responsive variants section */
    .variants-section {
        padding: 0.75em;
        background-color: var(--bg-secondary, rgba(0,0,0,0.05));
        border-radius: 6px;
        margin: 0.5em 0;
    }

    .variants-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 0.5em;
    }

    .variants-title {
        font-weight: 600;
        font-size: 0.9em;
        color: var(--text-color);
    }

    .variants-loading {
        font-size: 0.8em;
        color: var(--text-secondary, #888);
        font-style: italic;
    }

    .variants-list {
        display: flex;
        flex-direction: column;
        gap: 0.4em;
    }

    .variant-item {
        display: flex;
        align-items: center;
        gap: 0.75em;
        padding: 0.4em 0.6em;
        background-color: var(--card-bg);
        border-radius: 4px;
        font-size: 0.85em;
    }

    .variant-label {
        font-weight: 500;
        color: var(--text-color);
        min-width: 80px;
    }

    .variant-dims {
        color: var(--text-secondary, #888);
        font-family: monospace;
        font-size: 0.9em;
    }

    .variant-size {
        color: var(--text-secondary, #888);
        font-size: 0.85em;
        margin-left: auto;
    }

    .variant-copy-btn {
        padding: 0.25em 0.6em;
        background-color: var(--primary);
        color: var(--text-on-primary, #fff);
        border: none;
        border-radius: 3px;
        cursor: pointer;
        font-size: 0.8em;
        transition: background-color 0.2s, transform 0.1s;
    }

    .variant-copy-btn:hover {
        opacity: 0.9;
    }

    .variant-copy-btn.copied {
        background-color: #28a745;
    }

    .variants-empty {
        font-size: 0.85em;
        color: var(--text-secondary, #888);
        font-style: italic;
        padding: 0.5em 0;
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
        display: inline-flex;
        align-items: center;
        justify-content: center;
        height: 2.2em;
        padding: 0 1em;
        background-color: var(--primary);
        color: var(--text-color);
        border: 1px solid transparent;
        border-radius: 4px;
        cursor: pointer;
        text-decoration: none;
        font-size: 0.9em;
        box-sizing: border-box;
    }

    .action-btn:hover {
        background-color: var(--accent-hover-color);
    }

    .action-btn.danger {
        background-color: transparent;
        border-color: var(--warning);
        color: var(--warning);
    }

    .action-btn.danger:hover {
        background-color: var(--warning);
        color: var(--text-color);
    }

    .action-btn.warning {
        background-color: transparent;
        border-color: #f59e0b;
        color: #f59e0b;
    }

    .action-btn.warning:hover:not(:disabled) {
        background-color: #f59e0b;
        color: #fff;
    }

    .action-btn.warning:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    @media (max-width: 720px) {
        .hash-full {
            display: none;
        }

        .hash-truncated {
            display: inline;
        }

        .npub-full {
            display: none;
        }

        .npub-truncated {
            display: inline;
        }
    }

    @media (max-width: 600px) {
        .blob-item {
            flex-wrap: wrap;
        }

        .modal-overlay {
            left: 0;
            top: 0;
            padding: 0.5em;
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
