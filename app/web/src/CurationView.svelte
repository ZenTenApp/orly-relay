<script>
    import { onMount } from "svelte";
    import { curationKindCategories, parseCustomKinds, formatKindsCompact } from "./kindCategories.js";
    import { getApiBase, getWsUrl } from "./config.js";

    // Props
    export let userSigner;
    export let userPubkey;

    // State management
    let activeTab = "trusted";
    let isLoading = false;
    let message = "";
    let messageType = "info";
    let isConfigured = false;

    // User detail view state
    let selectedUser = null;
    let selectedUserType = null; // "trusted", "blacklisted", or "unclassified"
    let userEvents = [];
    let userEventsTotal = 0;
    let userEventsOffset = 0;
    let loadingEvents = false;
    let expandedEvents = {}; // Track which events are expanded

    // Configuration state
    let config = {
        daily_limit: 50,
        first_ban_hours: 1,
        second_ban_hours: 168,
        categories: [],
        custom_kinds: "",
        kind_ranges: []
    };

    // Trusted pubkeys
    let trustedPubkeys = [];
    let newTrustedPubkey = "";
    let newTrustedNote = "";

    // Blacklisted pubkeys
    let blacklistedPubkeys = [];
    let newBlacklistedPubkey = "";
    let newBlacklistedReason = "";

    // Unclassified users
    let unclassifiedUsers = [];

    // Spam events
    let spamEvents = [];

    // Blocked IPs
    let blockedIPs = [];

    // Check configuration on mount
    onMount(async () => {
        await checkConfiguration();
    });

    // Create NIP-98 authentication event
    async function createNIP98AuthEvent(method, url) {
        if (!userSigner) {
            throw new Error("No signer available. Please log in with a Nostr extension.");
        }
        if (!userPubkey) {
            throw new Error("No user pubkey available.");
        }

        const fullUrl = getApiBase() + url;
        const authEvent = {
            kind: 27235,
            created_at: Math.floor(Date.now() / 1000),
            tags: [
                ["u", fullUrl],
                ["method", method],
            ],
            content: "",
            pubkey: userPubkey,
        };

        const signedAuthEvent = await userSigner.signEvent(authEvent);
        return `Nostr ${btoa(JSON.stringify(signedAuthEvent))}`;
    }

    // Make NIP-86 API call
    async function callNIP86API(method, params = []) {
        try {
            isLoading = true;
            message = "";

            const request = { method, params };
            const authHeader = await createNIP98AuthEvent("POST", "/api/nip86");

            const response = await fetch("/api/nip86", {
                method: "POST",
                headers: {
                    "Content-Type": "application/nostr+json+rpc",
                    Authorization: authHeader,
                },
                body: JSON.stringify(request),
            });

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const result = await response.json();
            if (result.error) {
                throw new Error(result.error);
            }

            return result.result;
        } catch (error) {
            console.error("NIP-86 API error:", error);
            message = error.message;
            messageType = "error";
            throw error;
        } finally {
            isLoading = false;
        }
    }

    // Check if curating mode is configured
    async function checkConfiguration() {
        try {
            const result = await callNIP86API("isconfigured");
            isConfigured = result === true;

            if (isConfigured) {
                await loadConfig();
                await loadAllData();
            }
        } catch (error) {
            console.error("Failed to check configuration:", error);
            isConfigured = false;
        }
    }

    // Load current configuration
    async function loadConfig() {
        try {
            const result = await callNIP86API("getcuratingconfig");
            if (result) {
                config = {
                    daily_limit: result.daily_limit || 50,
                    first_ban_hours: result.first_ban_hours || 1,
                    second_ban_hours: result.second_ban_hours || 168,
                    categories: result.categories || [],
                    custom_kinds: result.custom_kinds ? result.custom_kinds.join(", ") : "",
                    kind_ranges: result.kind_ranges || []
                };
            }
        } catch (error) {
            console.error("Failed to load config:", error);
        }
    }

    // Load all data
    async function loadAllData() {
        await Promise.all([
            loadTrustedPubkeys(),
            loadBlacklistedPubkeys(),
            loadUnclassifiedUsers(),
            loadSpamEvents(),
            loadBlockedIPs(),
        ]);
    }

    // Load trusted pubkeys
    async function loadTrustedPubkeys() {
        try {
            trustedPubkeys = await callNIP86API("listtrustedpubkeys");
        } catch (error) {
            console.error("Failed to load trusted pubkeys:", error);
            trustedPubkeys = [];
        }
    }

    // Load blacklisted pubkeys
    async function loadBlacklistedPubkeys() {
        try {
            blacklistedPubkeys = await callNIP86API("listblacklistedpubkeys");
        } catch (error) {
            console.error("Failed to load blacklisted pubkeys:", error);
            blacklistedPubkeys = [];
        }
    }

    // Load unclassified users
    async function loadUnclassifiedUsers() {
        try {
            unclassifiedUsers = await callNIP86API("listunclassifiedusers");
        } catch (error) {
            console.error("Failed to load unclassified users:", error);
            unclassifiedUsers = [];
        }
    }

    // Scan database for all pubkeys
    async function scanDatabase() {
        try {
            const result = await callNIP86API("scanpubkeys");
            showMessage(`Database scanned: ${result.total_pubkeys} pubkeys, ${result.total_events} events (${result.skipped} skipped)`, "success");
            // Refresh the unclassified users list
            await loadUnclassifiedUsers();
        } catch (error) {
            console.error("Failed to scan database:", error);
            showMessage("Failed to scan database: " + error.message, "error");
        }
    }

    // Load spam events
    async function loadSpamEvents() {
        try {
            spamEvents = await callNIP86API("listspamevents");
        } catch (error) {
            console.error("Failed to load spam events:", error);
            spamEvents = [];
        }
    }

    // Load blocked IPs
    async function loadBlockedIPs() {
        try {
            blockedIPs = await callNIP86API("listblockedips");
        } catch (error) {
            console.error("Failed to load blocked IPs:", error);
            blockedIPs = [];
        }
    }

    // Trust a pubkey
    async function trustPubkey(pubkey = null, note = "") {
        const pk = pubkey || newTrustedPubkey;
        const n = pubkey ? note : newTrustedNote;

        if (!pk) return;

        try {
            await callNIP86API("trustpubkey", [pk, n]);
            message = "Pubkey trusted successfully";
            messageType = "success";
            newTrustedPubkey = "";
            newTrustedNote = "";
            await loadTrustedPubkeys();
            await loadUnclassifiedUsers();
        } catch (error) {
            console.error("Failed to trust pubkey:", error);
        }
    }

    // Untrust a pubkey
    async function untrustPubkey(pubkey) {
        try {
            await callNIP86API("untrustpubkey", [pubkey]);
            message = "Pubkey untrusted";
            messageType = "success";
            await loadTrustedPubkeys();
        } catch (error) {
            console.error("Failed to untrust pubkey:", error);
        }
    }

    // Blacklist a pubkey
    async function blacklistPubkey(pubkey = null, reason = "") {
        const pk = pubkey || newBlacklistedPubkey;
        const r = pubkey ? reason : newBlacklistedReason;

        if (!pk) return;

        try {
            await callNIP86API("blacklistpubkey", [pk, r]);
            message = "Pubkey blacklisted";
            messageType = "success";
            newBlacklistedPubkey = "";
            newBlacklistedReason = "";
            await loadBlacklistedPubkeys();
            await loadUnclassifiedUsers();
        } catch (error) {
            console.error("Failed to blacklist pubkey:", error);
        }
    }

    // Remove from blacklist
    async function unblacklistPubkey(pubkey) {
        try {
            await callNIP86API("unblacklistpubkey", [pubkey]);
            message = "Pubkey removed from blacklist";
            messageType = "success";
            await loadBlacklistedPubkeys();
        } catch (error) {
            console.error("Failed to remove from blacklist:", error);
        }
    }

    // Mark event as spam
    async function markSpam(eventId, reason) {
        try {
            await callNIP86API("markspam", [eventId, reason]);
            message = "Event marked as spam";
            messageType = "success";
            await loadSpamEvents();
        } catch (error) {
            console.error("Failed to mark spam:", error);
        }
    }

    // Unmark spam
    async function unmarkSpam(eventId) {
        try {
            await callNIP86API("unmarkspam", [eventId]);
            message = "Spam mark removed";
            messageType = "success";
            await loadSpamEvents();
        } catch (error) {
            console.error("Failed to unmark spam:", error);
        }
    }

    // Delete event
    async function deleteEvent(eventId) {
        if (!confirm("Permanently delete this event?")) return;

        try {
            await callNIP86API("deleteevent", [eventId]);
            message = "Event deleted";
            messageType = "success";
            await loadSpamEvents();
        } catch (error) {
            console.error("Failed to delete event:", error);
        }
    }

    // Unblock IP
    async function unblockIP(ip) {
        try {
            await callNIP86API("unblockip", [ip]);
            message = "IP unblocked";
            messageType = "success";
            await loadBlockedIPs();
        } catch (error) {
            console.error("Failed to unblock IP:", error);
        }
    }

    // Toggle category selection
    function toggleCategory(categoryId) {
        if (config.categories.includes(categoryId)) {
            config.categories = config.categories.filter(c => c !== categoryId);
        } else {
            config.categories = [...config.categories, categoryId];
        }
    }

    // Publish configuration event
    async function publishConfiguration() {
        if (!userSigner || !userPubkey) {
            message = "Please log in with a Nostr extension to publish configuration";
            messageType = "error";
            return;
        }

        if (config.categories.length === 0 && !config.custom_kinds.trim()) {
            message = "Please select at least one kind category or enter custom kinds";
            messageType = "error";
            return;
        }

        try {
            isLoading = true;
            message = "";

            // Build tags
            const tags = [
                ["d", "curating-config"],
                ["daily_limit", String(config.daily_limit)],
                ["first_ban_hours", String(config.first_ban_hours)],
                ["second_ban_hours", String(config.second_ban_hours)],
            ];

            // Add category tags
            for (const cat of config.categories) {
                tags.push(["kind_category", cat]);
            }

            // Parse and add custom kinds
            const customKinds = parseCustomKinds(config.custom_kinds);
            for (const kind of customKinds) {
                tags.push(["kind", String(kind)]);
            }

            // Create the configuration event
            const configEvent = {
                kind: 30078,
                created_at: Math.floor(Date.now() / 1000),
                tags: tags,
                content: "Curating relay configuration",
                pubkey: userPubkey,
            };

            // Sign the event
            const signedEvent = await userSigner.signEvent(configEvent);

            // Submit to relay via WebSocket
            const ws = new WebSocket(getWsUrl());

            await new Promise((resolve, reject) => {
                ws.onopen = () => {
                    ws.send(JSON.stringify(["EVENT", signedEvent]));
                };
                ws.onmessage = (e) => {
                    const msg = JSON.parse(e.data);
                    if (msg[0] === "OK") {
                        if (msg[2] === true) {
                            resolve();
                        } else {
                            reject(new Error(msg[3] || "Event rejected"));
                        }
                    }
                };
                ws.onerror = (e) => reject(new Error("WebSocket error"));
                setTimeout(() => reject(new Error("Timeout")), 10000);
            });

            ws.close();

            message = "Configuration published successfully";
            messageType = "success";
            isConfigured = true;
            await loadAllData();
        } catch (error) {
            console.error("Failed to publish configuration:", error);
            message = `Failed to publish: ${error.message}`;
            messageType = "error";
        } finally {
            isLoading = false;
        }
    }

    // Update configuration (re-publish)
    async function updateConfiguration() {
        await publishConfiguration();
    }

    // Format pubkey for display
    function formatPubkey(pubkey) {
        if (!pubkey || pubkey.length < 16) return pubkey;
        return `${pubkey.slice(0, 8)}...${pubkey.slice(-8)}`;
    }

    // Format date
    function formatDate(timestamp) {
        if (!timestamp) return "";
        return new Date(timestamp).toLocaleString();
    }

    // Show message helper
    function showMessage(msg, type = "info") {
        message = msg;
        messageType = type;
    }

    // Open user detail view
    async function openUserDetail(pubkey, type) {
        console.log("openUserDetail called:", pubkey, type);
        selectedUser = pubkey;
        selectedUserType = type;
        userEvents = [];
        userEventsTotal = 0;
        userEventsOffset = 0;
        expandedEvents = {};
        console.log("selectedUser set to:", selectedUser);
        await loadUserEvents();
    }

    // Close user detail view
    function closeUserDetail() {
        selectedUser = null;
        selectedUserType = null;
        userEvents = [];
        userEventsTotal = 0;
        userEventsOffset = 0;
        expandedEvents = {};
    }

    // Load events for selected user
    async function loadUserEvents() {
        console.log("loadUserEvents called, selectedUser:", selectedUser, "loadingEvents:", loadingEvents);
        if (!selectedUser || loadingEvents) return;

        try {
            loadingEvents = true;
            console.log("Calling geteventsforpubkey API...");
            const result = await callNIP86API("geteventsforpubkey", [selectedUser, 100, userEventsOffset]);
            console.log("API result:", result);
            if (result) {
                if (userEventsOffset === 0) {
                    userEvents = result.events || [];
                } else {
                    userEvents = [...userEvents, ...(result.events || [])];
                }
                userEventsTotal = result.total || 0;
            }
        } catch (error) {
            console.error("Failed to load user events:", error);
            showMessage("Failed to load events: " + error.message, "error");
        } finally {
            loadingEvents = false;
        }
    }

    // Load more events
    async function loadMoreEvents() {
        userEventsOffset = userEvents.length;
        await loadUserEvents();
    }

    // Toggle event expansion
    function toggleEventExpansion(eventId) {
        expandedEvents = {
            ...expandedEvents,
            [eventId]: !expandedEvents[eventId]
        };
    }

    // Truncate content to 6 lines (approximately 300 chars per line)
    function truncateContent(content, maxLines = 6) {
        if (!content) return "";
        const lines = content.split('\n');
        if (lines.length <= maxLines && content.length <= maxLines * 100) {
            return content;
        }
        // Truncate by lines or characters, whichever is smaller
        let truncated = lines.slice(0, maxLines).join('\n');
        if (truncated.length > maxLines * 100) {
            truncated = truncated.substring(0, maxLines * 100);
        }
        return truncated;
    }

    // Check if content is truncated
    function isContentTruncated(content, maxLines = 6) {
        if (!content) return false;
        const lines = content.split('\n');
        return lines.length > maxLines || content.length > maxLines * 100;
    }

    // Trust user from detail view and refresh
    async function trustUserFromDetail() {
        await trustPubkey(selectedUser, "");
        // Refresh list and go back
        await loadAllData();
        closeUserDetail();
    }

    // Blacklist user from detail view and refresh
    async function blacklistUserFromDetail() {
        await blacklistPubkey(selectedUser, "");
        // Refresh list and go back
        await loadAllData();
        closeUserDetail();
    }

    // Untrust user from detail view and refresh
    async function untrustUserFromDetail() {
        await untrustPubkey(selectedUser);
        await loadAllData();
        closeUserDetail();
    }

    // Unblacklist user from detail view and refresh
    async function unblacklistUserFromDetail() {
        await unblacklistPubkey(selectedUser);
        await loadAllData();
        closeUserDetail();
    }

    // Delete all events for a blacklisted user
    async function deleteAllEventsForUser() {
        if (!confirm(`Delete ALL ${userEventsTotal} events from this user? This cannot be undone.`)) {
            return;
        }

        try {
            isLoading = true;
            const result = await callNIP86API("deleteeventsforpubkey", [selectedUser]);
            showMessage(`Deleted ${result.deleted} events`, "success");
            // Refresh the events list
            userEvents = [];
            userEventsTotal = 0;
            userEventsOffset = 0;
            await loadUserEvents();
        } catch (error) {
            console.error("Failed to delete events:", error);
            showMessage("Failed to delete events: " + error.message, "error");
        } finally {
            isLoading = false;
        }
    }

    // Get kind name
    function getKindName(kind) {
        const kindNames = {
            0: "Metadata",
            1: "Text Note",
            3: "Follow List",
            4: "Encrypted DM",
            6: "Repost",
            7: "Reaction",
            14: "Chat Message",
            16: "Order Message",
            17: "Payment Receipt",
            1063: "File Metadata",
            10002: "Relay List",
            30017: "Stall",
            30018: "Product (NIP-15)",
            30023: "Long-form",
            30078: "App Data",
            30402: "Product (NIP-99)",
            30405: "Collection",
            30406: "Shipping",
            31555: "Review",
        };
        return kindNames[kind] || `Kind ${kind}`;
    }
</script>

<div class="curation-view">
    <h2>Curation Mode</h2>

    {#if message}
        <div class="message {messageType}">{message}</div>
    {/if}

    {#if !isConfigured}
        <!-- Setup Mode -->
        <div class="setup-section">
            <div class="setup-header">
                <h3>Initial Configuration</h3>
                <p>Configure curating mode before the relay will accept events. Select which event kinds to allow and set rate limiting parameters.</p>
            </div>

            <div class="config-section">
                <h4>Allowed Event Kinds</h4>
                <p class="help-text">Select categories of events to allow. At least one must be selected.</p>

                <div class="category-grid">
                    {#each curationKindCategories as category}
                        <label class="category-item" class:selected={config.categories.includes(category.id)}>
                            <input
                                type="checkbox"
                                checked={config.categories.includes(category.id)}
                                on:change={() => toggleCategory(category.id)}
                            />
                            <div class="category-info">
                                <span class="category-name">{category.name}</span>
                                <span class="category-desc">{category.description}</span>
                                <span class="category-kinds">Kinds: {category.kinds.join(", ")}</span>
                            </div>
                        </label>
                    {/each}
                </div>

                <div class="custom-kinds">
                    <label for="custom-kinds">Custom Kinds (comma-separated, ranges allowed e.g., "100, 200-300")</label>
                    <input
                        id="custom-kinds"
                        type="text"
                        bind:value={config.custom_kinds}
                        placeholder="e.g., 100, 200-250, 500"
                    />
                </div>
            </div>

            <div class="config-section">
                <h4>Rate Limiting</h4>

                <div class="form-row">
                    <div class="form-group">
                        <label for="daily-limit">Daily Event Limit (unclassified users)</label>
                        <input
                            id="daily-limit"
                            type="number"
                            min="1"
                            bind:value={config.daily_limit}
                        />
                    </div>
                </div>

                <div class="form-row">
                    <div class="form-group">
                        <label for="first-ban">First IP Ban Duration (hours)</label>
                        <input
                            id="first-ban"
                            type="number"
                            min="1"
                            bind:value={config.first_ban_hours}
                        />
                    </div>
                    <div class="form-group">
                        <label for="second-ban">Second+ IP Ban Duration (hours)</label>
                        <input
                            id="second-ban"
                            type="number"
                            min="1"
                            bind:value={config.second_ban_hours}
                        />
                    </div>
                </div>
            </div>

            <div class="publish-section">
                <button
                    class="publish-btn"
                    on:click={publishConfiguration}
                    disabled={isLoading}
                >
                    {#if isLoading}
                        Publishing...
                    {:else}
                        Publish Configuration
                    {/if}
                </button>
                <p class="publish-note">This will publish a kind 30078 event to activate curating mode.</p>
            </div>
        </div>
    {:else}
        <!-- User Detail View -->
        {#if selectedUser}
            <div class="user-detail-view">
                <div class="detail-header">
                    <div class="detail-header-left">
                        <button class="back-btn" on:click={closeUserDetail}>
                            &larr; Back
                        </button>
                        <h3>User Events</h3>
                        <span class="detail-pubkey" title={selectedUser}>{formatPubkey(selectedUser)}</span>
                        <span class="detail-count">{userEventsTotal} events</span>
                    </div>
                    <div class="detail-header-right">
                        {#if selectedUserType === "trusted"}
                            <button class="btn-danger" on:click={untrustUserFromDetail}>Remove Trust</button>
                            <button class="btn-danger" on:click={blacklistUserFromDetail}>Blacklist</button>
                        {:else if selectedUserType === "blacklisted"}
                            <button class="btn-delete-all" on:click={deleteAllEventsForUser} disabled={isLoading || userEventsTotal === 0}>
                                Delete All Events
                            </button>
                            <button class="btn-success" on:click={unblacklistUserFromDetail}>Remove from Blacklist</button>
                            <button class="btn-success" on:click={trustUserFromDetail}>Trust</button>
                        {:else}
                            <button class="btn-success" on:click={trustUserFromDetail}>Trust</button>
                            <button class="btn-danger" on:click={blacklistUserFromDetail}>Blacklist</button>
                        {/if}
                    </div>
                </div>

                <div class="events-list">
                    {#if loadingEvents && userEvents.length === 0}
                        <div class="loading">Loading events...</div>
                    {:else if userEvents.length === 0}
                        <div class="empty">No events found for this user.</div>
                    {:else}
                        {#each userEvents as event}
                            <div class="event-item">
                                <div class="event-header">
                                    <span class="event-kind">{getKindName(event.kind)}</span>
                                    <span class="event-id" title={event.id}>{formatPubkey(event.id)}</span>
                                    <span class="event-time">{formatDate(event.created_at * 1000)}</span>
                                </div>
                                <div class="event-content" class:expanded={expandedEvents[event.id]}>
                                    {#if expandedEvents[event.id] || !isContentTruncated(event.content)}
                                        <pre>{event.content || "(empty)"}</pre>
                                    {:else}
                                        <pre>{truncateContent(event.content)}...</pre>
                                    {/if}
                                </div>
                                {#if isContentTruncated(event.content)}
                                    <button class="expand-btn" on:click={() => toggleEventExpansion(event.id)}>
                                        {expandedEvents[event.id] ? "Show less" : "Show more"}
                                    </button>
                                {/if}
                            </div>
                        {/each}

                        {#if userEvents.length < userEventsTotal}
                            <div class="load-more">
                                <button on:click={loadMoreEvents} disabled={loadingEvents}>
                                    {loadingEvents ? "Loading..." : `Load more (${userEvents.length} of ${userEventsTotal})`}
                                </button>
                            </div>
                        {/if}
                    {/if}
                </div>
            </div>
        {:else}
            <!-- Active Mode -->
            <div class="tabs">
                <button class="tab" class:active={activeTab === "trusted"} on:click={() => activeTab = "trusted"}>
                    Trusted ({trustedPubkeys.length})
                </button>
                <button class="tab" class:active={activeTab === "blacklist"} on:click={() => activeTab = "blacklist"}>
                    Blacklist ({blacklistedPubkeys.length})
                </button>
                <button class="tab" class:active={activeTab === "unclassified"} on:click={() => activeTab = "unclassified"}>
                    Unclassified ({unclassifiedUsers.length})
                </button>
                <button class="tab" class:active={activeTab === "spam"} on:click={() => activeTab = "spam"}>
                    Spam ({spamEvents.length})
                </button>
                <button class="tab" class:active={activeTab === "ips"} on:click={() => activeTab = "ips"}>
                    Blocked IPs ({blockedIPs.length})
                </button>
                <button class="tab" class:active={activeTab === "settings"} on:click={() => activeTab = "settings"}>
                    Settings
                </button>
            </div>

            <div class="tab-content">
            {#if activeTab === "trusted"}
                <div class="section">
                    <h3>Trusted Publishers</h3>
                    <p class="help-text">Trusted users can publish unlimited events without rate limiting.</p>

                    <div class="add-form">
                        <input
                            type="text"
                            placeholder="Pubkey (64 hex chars)"
                            bind:value={newTrustedPubkey}
                        />
                        <input
                            type="text"
                            placeholder="Note (optional)"
                            bind:value={newTrustedNote}
                        />
                        <button on:click={() => trustPubkey()} disabled={isLoading}>
                            Trust
                        </button>
                    </div>

                    <div class="list">
                        {#if trustedPubkeys.length > 0}
                            {#each trustedPubkeys as item}
                                <div class="list-item clickable" on:click={() => openUserDetail(item.pubkey, "trusted")}>
                                    <div class="item-main">
                                        <span class="pubkey" title={item.pubkey}>{formatPubkey(item.pubkey)}</span>
                                        {#if item.note}
                                            <span class="note">{item.note}</span>
                                        {/if}
                                    </div>
                                    <div class="item-actions">
                                        <button class="btn-danger" on:click|stopPropagation={() => untrustPubkey(item.pubkey)}>
                                            Remove
                                        </button>
                                    </div>
                                </div>
                            {/each}
                        {:else}
                            <div class="empty">No trusted pubkeys yet.</div>
                        {/if}
                    </div>
                </div>
            {/if}

            {#if activeTab === "blacklist"}
                <div class="section">
                    <h3>Blacklisted Publishers</h3>
                    <p class="help-text">Blacklisted users cannot publish any events.</p>

                    <div class="add-form">
                        <input
                            type="text"
                            placeholder="Pubkey (64 hex chars)"
                            bind:value={newBlacklistedPubkey}
                        />
                        <input
                            type="text"
                            placeholder="Reason (optional)"
                            bind:value={newBlacklistedReason}
                        />
                        <button on:click={() => blacklistPubkey()} disabled={isLoading}>
                            Blacklist
                        </button>
                    </div>

                    <div class="list">
                        {#if blacklistedPubkeys.length > 0}
                            {#each blacklistedPubkeys as item}
                                <div class="list-item clickable" on:click={() => openUserDetail(item.pubkey, "blacklisted")}>
                                    <div class="item-main">
                                        <span class="pubkey" title={item.pubkey}>{formatPubkey(item.pubkey)}</span>
                                        {#if item.reason}
                                            <span class="reason">{item.reason}</span>
                                        {/if}
                                    </div>
                                    <div class="item-actions">
                                        <button class="btn-success" on:click|stopPropagation={() => unblacklistPubkey(item.pubkey)}>
                                            Remove
                                        </button>
                                    </div>
                                </div>
                            {/each}
                        {:else}
                            <div class="empty">No blacklisted pubkeys.</div>
                        {/if}
                    </div>
                </div>
            {/if}

            {#if activeTab === "unclassified"}
                <div class="section">
                    <h3>Unclassified Users</h3>
                    <p class="help-text">Users who have posted events but haven't been classified. Sorted by event count.</p>

                    <div class="button-row">
                        <button class="refresh-btn" on:click={loadUnclassifiedUsers} disabled={isLoading}>
                            Refresh
                        </button>
                        <button class="scan-btn" on:click={scanDatabase} disabled={isLoading}>
                            Scan Database
                        </button>
                    </div>

                    <div class="list">
                        {#if unclassifiedUsers.length > 0}
                            {#each unclassifiedUsers as user}
                                <div class="list-item clickable" on:click={() => openUserDetail(user.pubkey, "unclassified")}>
                                    <div class="item-main">
                                        <span class="pubkey" title={user.pubkey}>{formatPubkey(user.pubkey)}</span>
                                        <span class="event-count">{user.event_count} events</span>
                                    </div>
                                    <div class="item-actions">
                                        <button class="btn-success" on:click|stopPropagation={() => trustPubkey(user.pubkey, "")}>
                                            Trust
                                        </button>
                                        <button class="btn-danger" on:click|stopPropagation={() => blacklistPubkey(user.pubkey, "")}>
                                            Blacklist
                                        </button>
                                    </div>
                                </div>
                            {/each}
                        {:else}
                            <div class="empty">No unclassified users.</div>
                        {/if}
                    </div>
                </div>
            {/if}

            {#if activeTab === "spam"}
                <div class="section">
                    <h3>Spam Events</h3>
                    <p class="help-text">Events flagged as spam are hidden from query results but remain in the database.</p>

                    <button class="refresh-btn" on:click={loadSpamEvents} disabled={isLoading}>
                        Refresh
                    </button>

                    <div class="list">
                        {#if spamEvents.length > 0}
                            {#each spamEvents as event}
                                <div class="list-item">
                                    <div class="item-main">
                                        <span class="event-id" title={event.event_id}>{formatPubkey(event.event_id)}</span>
                                        <span class="pubkey" title={event.pubkey}>by {formatPubkey(event.pubkey)}</span>
                                        {#if event.reason}
                                            <span class="reason">{event.reason}</span>
                                        {/if}
                                    </div>
                                    <div class="item-actions">
                                        <button class="btn-success" on:click={() => unmarkSpam(event.event_id)}>
                                            Unmark
                                        </button>
                                        <button class="btn-danger" on:click={() => deleteEvent(event.event_id)}>
                                            Delete
                                        </button>
                                    </div>
                                </div>
                            {/each}
                        {:else}
                            <div class="empty">No spam events flagged.</div>
                        {/if}
                    </div>
                </div>
            {/if}

            {#if activeTab === "ips"}
                <div class="section">
                    <h3>Blocked IP Addresses</h3>
                    <p class="help-text">IP addresses blocked due to rate limit violations.</p>

                    <button class="refresh-btn" on:click={loadBlockedIPs} disabled={isLoading}>
                        Refresh
                    </button>

                    <div class="list">
                        {#if blockedIPs.length > 0}
                            {#each blockedIPs as ip}
                                <div class="list-item">
                                    <div class="item-main">
                                        <span class="ip">{ip.ip}</span>
                                        {#if ip.reason}
                                            <span class="reason">{ip.reason}</span>
                                        {/if}
                                        {#if ip.expires_at}
                                            <span class="expires">Expires: {formatDate(ip.expires_at)}</span>
                                        {/if}
                                    </div>
                                    <div class="item-actions">
                                        <button class="btn-success" on:click={() => unblockIP(ip.ip)}>
                                            Unblock
                                        </button>
                                    </div>
                                </div>
                            {/each}
                        {:else}
                            <div class="empty">No blocked IPs.</div>
                        {/if}
                    </div>
                </div>
            {/if}

            {#if activeTab === "settings"}
                <div class="section">
                    <h3>Curating Configuration</h3>
                    <p class="help-text">Update curating mode settings. Changes will publish a new configuration event.</p>

                    <div class="config-section">
                        <h4>Allowed Event Kinds</h4>

                        <div class="category-grid">
                            {#each curationKindCategories as category}
                                <label class="category-item" class:selected={config.categories.includes(category.id)}>
                                    <input
                                        type="checkbox"
                                        checked={config.categories.includes(category.id)}
                                        on:change={() => toggleCategory(category.id)}
                                    />
                                    <div class="category-info">
                                        <span class="category-name">{category.name}</span>
                                        <span class="category-kinds">Kinds: {category.kinds.join(", ")}</span>
                                    </div>
                                </label>
                            {/each}
                        </div>

                        <div class="custom-kinds">
                            <label for="custom-kinds-edit">Custom Kinds</label>
                            <input
                                id="custom-kinds-edit"
                                type="text"
                                bind:value={config.custom_kinds}
                                placeholder="e.g., 100, 200-250, 500"
                            />
                        </div>
                    </div>

                    <div class="config-section">
                        <h4>Rate Limiting</h4>

                        <div class="form-row">
                            <div class="form-group">
                                <label for="daily-limit-edit">Daily Event Limit</label>
                                <input
                                    id="daily-limit-edit"
                                    type="number"
                                    min="1"
                                    bind:value={config.daily_limit}
                                />
                            </div>
                        </div>

                        <div class="form-row">
                            <div class="form-group">
                                <label for="first-ban-edit">First Ban (hours)</label>
                                <input
                                    id="first-ban-edit"
                                    type="number"
                                    min="1"
                                    bind:value={config.first_ban_hours}
                                />
                            </div>
                            <div class="form-group">
                                <label for="second-ban-edit">Second+ Ban (hours)</label>
                                <input
                                    id="second-ban-edit"
                                    type="number"
                                    min="1"
                                    bind:value={config.second_ban_hours}
                                />
                            </div>
                        </div>
                    </div>

                    <div class="publish-section">
                        <button
                            class="publish-btn"
                            on:click={updateConfiguration}
                            disabled={isLoading}
                        >
                            {#if isLoading}
                                Updating...
                            {:else}
                                Update Configuration
                            {/if}
                        </button>
                    </div>
                </div>
            {/if}
        </div>
        {/if}
    {/if}
</div>

<style>
    .curation-view {
        width: 100%;
        max-width: 900px;
        margin: 0;
        padding: 20px;
        background: var(--header-bg);
        color: var(--text-color);
        border-radius: 8px;
    }

    .curation-view h2 {
        margin: 0 0 1.5rem 0;
        color: var(--text-color);
        font-size: 1.8rem;
        font-weight: 600;
    }

    .message {
        padding: 10px 15px;
        border-radius: 4px;
        margin-bottom: 20px;
    }

    .message.success {
        background-color: var(--success-bg);
        color: var(--success-text);
        border: 1px solid var(--success);
    }

    .message.error {
        background-color: var(--error-bg);
        color: var(--error-text);
        border: 1px solid var(--danger);
    }

    .message.info {
        background-color: var(--primary-bg);
        color: var(--text-color);
        border: 1px solid var(--info);
    }

    /* Setup Mode */
    .setup-section {
        background: var(--card-bg);
        border-radius: 8px;
        padding: 1.5em;
        border: 1px solid var(--border-color);
    }

    .setup-header h3 {
        margin: 0 0 0.5rem 0;
        color: var(--text-color);
    }

    .setup-header p {
        margin: 0 0 1.5rem 0;
        color: var(--text-color);
        opacity: 0.8;
    }

    .config-section {
        margin-bottom: 1.5rem;
        padding: 1rem;
        background: var(--bg-color);
        border-radius: 6px;
        border: 1px solid var(--border-color);
    }

    .config-section h4 {
        margin: 0 0 0.5rem 0;
        color: var(--text-color);
    }

    .help-text {
        margin: 0 0 1rem 0;
        color: var(--text-color);
        opacity: 0.7;
        font-size: 0.9em;
    }

    .category-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
        gap: 0.75rem;
    }

    .category-item {
        display: flex;
        align-items: flex-start;
        gap: 0.75rem;
        padding: 0.75rem;
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 6px;
        cursor: pointer;
        transition: all 0.2s;
    }

    .category-item:hover {
        border-color: var(--accent-color);
    }

    .category-item.selected {
        border-color: var(--success);
        background: var(--success-bg);
    }

    .category-item input[type="checkbox"] {
        margin-top: 0.25rem;
    }

    .category-info {
        display: flex;
        flex-direction: column;
        gap: 0.25rem;
    }

    .category-name {
        font-weight: 600;
        color: var(--text-color);
    }

    .category-desc {
        font-size: 0.85em;
        color: var(--text-color);
        opacity: 0.7;
    }

    .category-kinds {
        font-size: 0.8em;
        font-family: monospace;
        color: var(--text-color);
        opacity: 0.6;
    }

    .custom-kinds {
        margin-top: 1rem;
    }

    .custom-kinds label {
        display: block;
        margin-bottom: 0.5rem;
        color: var(--text-color);
        font-weight: 500;
    }

    .custom-kinds input {
        width: 100%;
        padding: 0.5rem;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--input-bg);
        color: var(--input-text-color);
    }

    .form-row {
        display: flex;
        gap: 1rem;
        flex-wrap: wrap;
    }

    .form-group {
        flex: 1;
        min-width: 150px;
    }

    .form-group label {
        display: block;
        margin-bottom: 0.5rem;
        color: var(--text-color);
        font-weight: 500;
    }

    .form-group input {
        width: 100%;
        padding: 0.5rem;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--input-bg);
        color: var(--input-text-color);
    }

    .publish-section {
        text-align: center;
        padding: 1rem;
    }

    .publish-btn {
        padding: 0.75rem 2rem;
        font-size: 1rem;
        font-weight: 600;
        background: var(--success);
        color: var(--text-color);
        border: none;
        border-radius: 6px;
        cursor: pointer;
        transition: all 0.2s;
    }

    .publish-btn:hover:not(:disabled) {
        filter: brightness(0.9);
    }

    .publish-btn:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .publish-note {
        margin-top: 0.75rem;
        font-size: 0.85em;
        color: var(--text-color);
        opacity: 0.7;
    }

    /* Active Mode */
    .tabs {
        display: flex;
        border-bottom: 1px solid var(--border-color);
        margin-bottom: 1rem;
        flex-wrap: wrap;
    }

    .tab {
        padding: 0.75rem 1rem;
        border: none;
        background: none;
        cursor: pointer;
        border-bottom: 2px solid transparent;
        color: var(--text-color);
        font-size: 0.9rem;
        transition: all 0.2s;
    }

    .tab:hover {
        background: var(--button-hover-bg);
    }

    .tab.active {
        border-bottom-color: var(--accent-color);
        color: var(--accent-color);
    }

    .tab-content {
        min-height: 300px;
    }

    .section {
        background: var(--card-bg);
        border-radius: 8px;
        padding: 1.5em;
        border: 1px solid var(--border-color);
    }

    .section h3 {
        margin: 0 0 0.5rem 0;
        color: var(--text-color);
    }

    .add-form {
        display: flex;
        gap: 0.5rem;
        margin-bottom: 1rem;
        flex-wrap: wrap;
    }

    .add-form input {
        flex: 1;
        min-width: 150px;
        padding: 0.5rem;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--input-bg);
        color: var(--input-text-color);
    }

    .add-form button {
        padding: 0.5rem 1rem;
        background: var(--success);
        color: var(--text-color);
        border: none;
        border-radius: 4px;
        cursor: pointer;
    }

    .add-form button:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .refresh-btn {
        margin-bottom: 1rem;
        padding: 0.5rem 1rem;
        background: var(--info);
        color: var(--text-color);
        border: none;
        border-radius: 4px;
        cursor: pointer;
    }

    .refresh-btn:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .button-row {
        display: flex;
        gap: 0.5rem;
        margin-bottom: 1rem;
    }

    .scan-btn {
        padding: 0.5rem 1rem;
        background: var(--warning, #f0ad4e);
        color: var(--text-color);
        border: none;
        border-radius: 4px;
        cursor: pointer;
    }

    .scan-btn:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .list {
        border: 1px solid var(--border-color);
        border-radius: 4px;
        max-height: 400px;
        overflow-y: auto;
        background: var(--bg-color);
    }

    .list-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.75rem 1rem;
        border-bottom: 1px solid var(--border-color);
        gap: 1rem;
    }

    .list-item:last-child {
        border-bottom: none;
    }

    .item-main {
        display: flex;
        flex-direction: column;
        gap: 0.25rem;
        flex: 1;
        min-width: 0;
    }

    .pubkey, .event-id, .ip {
        font-family: monospace;
        font-size: 0.9em;
        color: var(--text-color);
    }

    .note, .reason, .expires {
        font-size: 0.85em;
        color: var(--text-color);
        opacity: 0.7;
    }

    .event-count {
        font-size: 0.85em;
        color: var(--success);
        font-weight: 500;
    }

    .item-actions {
        display: flex;
        gap: 0.5rem;
        flex-shrink: 0;
    }

    .btn-success {
        padding: 0.35rem 0.75rem;
        background: var(--success);
        color: var(--text-color);
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.85em;
    }

    .btn-danger {
        padding: 0.35rem 0.75rem;
        background: var(--danger);
        color: var(--text-color);
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.85em;
    }

    .btn-delete-all {
        padding: 0.35rem 0.75rem;
        background: #8B0000;
        color: white;
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.85em;
        font-weight: 600;
    }

    .btn-delete-all:hover:not(:disabled) {
        background: #660000;
    }

    .btn-delete-all:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .empty {
        padding: 2rem;
        text-align: center;
        color: var(--text-color);
        opacity: 0.6;
        font-style: italic;
    }

    /* Clickable list items */
    .list-item.clickable {
        cursor: pointer;
        transition: background-color 0.2s;
    }

    .list-item.clickable:hover {
        background-color: var(--button-hover-bg);
    }

    /* User Detail View */
    .user-detail-view {
        background: var(--card-bg);
        border-radius: 8px;
        padding: 1.5em;
        border: 1px solid var(--border-color);
    }

    .detail-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 1.5rem;
        padding-bottom: 1rem;
        border-bottom: 1px solid var(--border-color);
        flex-wrap: wrap;
        gap: 1rem;
    }

    .detail-header-left {
        display: flex;
        align-items: center;
        gap: 1rem;
        flex-wrap: wrap;
    }

    .detail-header-left h3 {
        margin: 0;
        color: var(--text-color);
    }

    .detail-header-right {
        display: flex;
        gap: 0.5rem;
    }

    .back-btn {
        padding: 0.5rem 1rem;
        background: var(--bg-color);
        color: var(--text-color);
        border: 1px solid var(--border-color);
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
    }

    .back-btn:hover {
        background: var(--button-hover-bg);
    }

    .detail-pubkey {
        font-family: monospace;
        font-size: 0.9em;
        color: var(--text-color);
        background: var(--bg-color);
        padding: 0.25rem 0.5rem;
        border-radius: 4px;
    }

    .detail-count {
        font-size: 0.85em;
        color: var(--success);
        font-weight: 500;
    }

    /* Events List */
    .events-list {
        max-height: 600px;
        overflow-y: auto;
    }

    .event-item {
        background: var(--bg-color);
        border: 1px solid var(--border-color);
        border-radius: 6px;
        padding: 1rem;
        margin-bottom: 0.75rem;
    }

    .event-header {
        display: flex;
        gap: 1rem;
        margin-bottom: 0.5rem;
        flex-wrap: wrap;
        align-items: center;
    }

    .event-kind {
        background: var(--accent-color);
        color: var(--text-color);
        padding: 0.2rem 0.5rem;
        border-radius: 4px;
        font-size: 0.8em;
        font-weight: 500;
    }

    .event-id {
        font-family: monospace;
        font-size: 0.8em;
        color: var(--text-color);
        opacity: 0.7;
    }

    .event-time {
        font-size: 0.8em;
        color: var(--text-color);
        opacity: 0.6;
    }

    .event-content {
        background: var(--card-bg);
        border-radius: 4px;
        padding: 0.75rem;
        overflow: hidden;
    }

    .event-content pre {
        margin: 0;
        white-space: pre-wrap;
        word-break: break-word;
        font-family: inherit;
        font-size: 0.9em;
        color: var(--text-color);
        max-height: 150px;
        overflow: hidden;
    }

    .event-content.expanded pre {
        max-height: none;
    }

    .expand-btn {
        margin-top: 0.5rem;
        padding: 0.25rem 0.5rem;
        background: transparent;
        color: var(--accent-color);
        border: 1px solid var(--accent-color);
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.8em;
    }

    .expand-btn:hover {
        background: var(--accent-color);
        color: var(--text-color);
    }

    .load-more {
        text-align: center;
        padding: 1rem;
    }

    .load-more button {
        padding: 0.5rem 1.5rem;
        background: var(--info);
        color: var(--text-color);
        border: none;
        border-radius: 4px;
        cursor: pointer;
    }

    .load-more button:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .loading {
        padding: 2rem;
        text-align: center;
        color: var(--text-color);
        opacity: 0.6;
    }
</style>
