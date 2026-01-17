<script>
    import { onMount } from "svelte";
    import { getApiBase } from "./config.js";

    // Props
    export let userSigner;
    export let userPubkey;

    // State management
    let activeTab = "pubkeys";
    let isLoading = false;
    let message = "";
    let messageType = "info";

    // Relay configuration state
    let relayName = "";
    let relayDescription = "";
    let relayIcon = "";

    // Banned pubkeys
    let bannedPubkeys = [];
    let newBannedPubkey = "";
    let newBannedPubkeyReason = "";

    // Allowed pubkeys
    let allowedPubkeys = [];
    let newAllowedPubkey = "";
    let newAllowedPubkeyReason = "";

    // Banned events
    let bannedEvents = [];
    let newBannedEvent = "";
    let newBannedEventReason = "";

    // Allowed events
    let allowedEvents = [];
    let newAllowedEvent = "";
    let newAllowedEventReason = "";

    // Blocked IPs
    let blockedIPs = [];
    let newBlockedIP = "";
    let newBlockedIPReason = "";

    // Allowed kinds
    let allowedKinds = [];
    let newAllowedKind = "";

    // Events needing moderation
    let eventsNeedingModeration = [];

    // Relay config
    let relayConfig = {
        relay_name: "",
        relay_description: "",
        relay_icon: "",
    };

    // Supported methods
    const supportedMethods = [
        "supportedmethods",
        "banpubkey",
        "listbannedpubkeys",
        "allowpubkey",
        "listallowedpubkeys",
        "listeventsneedingmoderation",
        "allowevent",
        "banevent",
        "listbannedevents",
        "changerelayname",
        "changerelaydescription",
        "changerelayicon",
        "allowkind",
        "disallowkind",
        "listallowedkinds",
        "blockip",
        "unblockip",
        "listblockedips",
    ];

    // Load relay info on component mount
    onMount(() => {
        // Small delay to ensure component is fully rendered
        setTimeout(() => {
            fetchRelayInfo();
        }, 100);
    });

    // Reactive statement to ensure form updates when relayConfig changes
    $: console.log("relayConfig changed:", relayConfig);

    // Fetch current relay information
    async function fetchRelayInfo() {
        try {
            isLoading = true;
            console.log("Fetching relay info from /");
            const response = await fetch(getApiBase() +"/", {
                headers: {
                    Accept: "application/nostr+json",
                },
            });
            console.log("Response status:", response.status);
            console.log("Response headers:", response.headers);

            if (response.ok) {
                const relayInfo = await response.json();
                console.log("Raw relay info:", relayInfo);

                // Reassign the entire object to trigger Svelte reactivity
                relayConfig = {
                    relay_name: relayInfo.name || "",
                    relay_description: relayInfo.description || "",
                    relay_icon: relayInfo.icon || "",
                };

                console.log("Updated relayConfig:", relayConfig);
                console.log("Loaded relay info:", relayInfo);

                message = "Relay configuration loaded successfully";
                messageType = "success";
            } else {
                console.error(
                    "Failed to fetch relay info, status:",
                    response.status,
                );
                message = `Failed to fetch relay info: ${response.status}`;
                messageType = "error";
            }
        } catch (error) {
            console.error("Failed to fetch relay info:", error);
            message = `Failed to fetch relay info: ${error.message}`;
            messageType = "error";
        } finally {
            isLoading = false;
        }
    }

    // Create NIP-98 authentication event for HTTP requests
    async function createNIP98AuthEvent(method, url) {
        if (!userSigner) {
            throw new Error(
                "No signer available for authentication. Please log in with a Nostr extension.",
            );
        }

        if (!userPubkey) {
            throw new Error("No user pubkey available for authentication.");
        }

        // Get the full URL
        const fullUrl = getApiBase() +url;

        // Create NIP-98 authentication event
        const authEvent = {
            kind: 27235, // HTTPAuth kind
            created_at: Math.floor(Date.now() / 1000),
            tags: [
                ["u", fullUrl],
                ["method", method],
            ],
            content: "",
            pubkey: userPubkey,
        };

        // Sign the authentication event
        const signedAuthEvent = await userSigner.signEvent(authEvent);

        // Encode the signed event as base64
        const eventJson = JSON.stringify(signedAuthEvent);
        const eventBase64 = btoa(eventJson);

        return `Nostr ${eventBase64}`;
    }

    // Make NIP-86 API call with NIP-98 authentication
    async function callNIP86API(method, params = []) {
        try {
            isLoading = true;
            message = "";

            const request = {
                method: method,
                params: params,
            };

            // Create NIP-98 authentication header
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
                throw new Error(
                    `HTTP ${response.status}: ${response.statusText}`,
                );
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

    // Load data functions
    async function loadBannedPubkeys() {
        try {
            bannedPubkeys = await callNIP86API("listbannedpubkeys");
        } catch (error) {
            console.error("Failed to load banned pubkeys:", error);
        }
    }

    async function loadAllowedPubkeys() {
        try {
            allowedPubkeys = await callNIP86API("listallowedpubkeys");
        } catch (error) {
            console.error("Failed to load allowed pubkeys:", error);
        }
    }

    async function loadBannedEvents() {
        try {
            bannedEvents = await callNIP86API("listbannedevents");
        } catch (error) {
            console.error("Failed to load banned events:", error);
        }
    }

    // Removed loadAllowedEvents - method doesn't exist in NIP-86 API

    async function loadBlockedIPs() {
        try {
            blockedIPs = await callNIP86API("listblockedips");
        } catch (error) {
            console.error("Failed to load blocked IPs:", error);
        }
    }

    async function loadAllowedKinds() {
        try {
            allowedKinds = await callNIP86API("listallowedkinds");
        } catch (error) {
            console.error("Failed to load allowed kinds:", error);
        }
    }

    async function loadEventsNeedingModeration() {
        try {
            isLoading = true;
            eventsNeedingModeration = await callNIP86API(
                "listeventsneedingmoderation",
            );
            console.log(
                "Loaded events needing moderation:",
                eventsNeedingModeration,
            );
        } catch (error) {
            console.error("Failed to load events needing moderation:", error);
            message = `Failed to load moderation events: ${error.message}`;
            messageType = "error";
            // Set empty array to prevent further issues
            eventsNeedingModeration = [];
        } finally {
            isLoading = false;
        }
    }

    // Action functions
    async function banPubkey() {
        if (!newBannedPubkey) return;

        try {
            await callNIP86API("banpubkey", [
                newBannedPubkey,
                newBannedPubkeyReason,
            ]);
            message = "Pubkey banned successfully";
            messageType = "success";
            newBannedPubkey = "";
            newBannedPubkeyReason = "";
            await loadBannedPubkeys();
        } catch (error) {
            console.error("Failed to ban pubkey:", error);
        }
    }

    async function allowPubkey() {
        if (!newAllowedPubkey) return;

        try {
            await callNIP86API("allowpubkey", [
                newAllowedPubkey,
                newAllowedPubkeyReason,
            ]);
            message = "Pubkey allowed successfully";
            messageType = "success";
            newAllowedPubkey = "";
            newAllowedPubkeyReason = "";
            await loadAllowedPubkeys();
        } catch (error) {
            console.error("Failed to allow pubkey:", error);
        }
    }

    async function banEvent() {
        if (!newBannedEvent) return;

        try {
            await callNIP86API("banevent", [
                newBannedEvent,
                newBannedEventReason,
            ]);
            message = "Event banned successfully";
            messageType = "success";
            newBannedEvent = "";
            newBannedEventReason = "";
            await loadBannedEvents();
        } catch (error) {
            console.error("Failed to ban event:", error);
        }
    }

    async function allowEvent() {
        if (!newAllowedEvent) return;

        try {
            await callNIP86API("allowevent", [
                newAllowedEvent,
                newAllowedEventReason,
            ]);
            message = "Event allowed successfully";
            messageType = "success";
            newAllowedEvent = "";
            newAllowedEventReason = "";
            // Note: No need to reload allowed events list as method doesn't exist
        } catch (error) {
            console.error("Failed to allow event:", error);
        }
    }

    async function blockIP() {
        if (!newBlockedIP) return;

        try {
            await callNIP86API("blockip", [newBlockedIP, newBlockedIPReason]);
            message = "IP blocked successfully";
            messageType = "success";
            newBlockedIP = "";
            newBlockedIPReason = "";
            await loadBlockedIPs();
        } catch (error) {
            console.error("Failed to block IP:", error);
        }
    }

    async function allowKind() {
        if (!newAllowedKind) return;

        const kindNum = parseInt(newAllowedKind);
        if (isNaN(kindNum)) {
            message = "Invalid kind number";
            messageType = "error";
            return;
        }

        try {
            await callNIP86API("allowkind", [kindNum]);
            message = "Kind allowed successfully";
            messageType = "success";
            newAllowedKind = "";
            await loadAllowedKinds();
        } catch (error) {
            console.error("Failed to allow kind:", error);
        }
    }

    async function disallowKind(kind) {
        try {
            await callNIP86API("disallowkind", [kind]);
            message = "Kind disallowed successfully";
            messageType = "success";
            await loadAllowedKinds();
        } catch (error) {
            console.error("Failed to disallow kind:", error);
        }
    }

    async function updateRelayName() {
        if (!relayConfig.relay_name) return;

        try {
            await callNIP86API("changerelayname", [relayConfig.relay_name]);
            message = "Relay name updated successfully";
            messageType = "success";
            // Refresh relay info to show updated values
            await fetchRelayInfo();
        } catch (error) {
            console.error("Failed to update relay name:", error);
        }
    }

    async function updateRelayDescription() {
        if (!relayConfig.relay_description) return;

        try {
            await callNIP86API("changerelaydescription", [
                relayConfig.relay_description,
            ]);
            message = "Relay description updated successfully";
            messageType = "success";
            // Refresh relay info to show updated values
            await fetchRelayInfo();
        } catch (error) {
            console.error("Failed to update relay description:", error);
        }
    }

    async function updateRelayIcon() {
        if (!relayConfig.relay_icon) return;

        try {
            await callNIP86API("changerelayicon", [relayConfig.relay_icon]);
            message = "Relay icon updated successfully";
            messageType = "success";
            // Refresh relay info to show updated values
            await fetchRelayInfo();
        } catch (error) {
            console.error("Failed to update relay icon:", error);
        }
    }

    // Update all relay configuration at once
    async function updateRelayConfiguration() {
        try {
            isLoading = true;
            message = "";

            const updates = [];

            // Update relay name if provided
            if (relayConfig.relay_name) {
                updates.push(
                    callNIP86API("changerelayname", [relayConfig.relay_name]),
                );
            }

            // Update relay description if provided
            if (relayConfig.relay_description) {
                updates.push(
                    callNIP86API("changerelaydescription", [
                        relayConfig.relay_description,
                    ]),
                );
            }

            // Update relay icon if provided
            if (relayConfig.relay_icon) {
                updates.push(
                    callNIP86API("changerelayicon", [relayConfig.relay_icon]),
                );
            }

            if (updates.length === 0) {
                message = "No changes to update";
                messageType = "info";
                return;
            }

            // Execute all updates in parallel
            await Promise.all(updates);

            message = "Relay configuration updated successfully";
            messageType = "success";

            // Refresh relay info to show updated values
            await fetchRelayInfo();
        } catch (error) {
            console.error("Failed to update relay configuration:", error);
            message = `Failed to update relay configuration: ${error.message}`;
            messageType = "error";
        } finally {
            isLoading = false;
        }
    }

    async function allowEventFromModeration(eventId) {
        try {
            await callNIP86API("allowevent", [
                eventId,
                "Approved from moderation queue",
            ]);
            message = "Event allowed successfully";
            messageType = "success";
            await loadEventsNeedingModeration();
        } catch (error) {
            console.error("Failed to allow event from moderation:", error);
        }
    }

    async function banEventFromModeration(eventId) {
        try {
            await callNIP86API("banevent", [
                eventId,
                "Banned from moderation queue",
            ]);
            message = "Event banned successfully";
            messageType = "success";
            await loadEventsNeedingModeration();
        } catch (error) {
            console.error("Failed to ban event from moderation:", error);
        }
    }

    // Load data when component mounts
    async function loadAllData() {
        await Promise.all([
            loadBannedPubkeys(),
            loadAllowedPubkeys(),
            loadBannedEvents(),
            // loadAllowedEvents(), // Removed - method doesn't exist
            loadBlockedIPs(),
            loadAllowedKinds(),
            // Note: loadEventsNeedingModeration() removed to prevent freezing
        ]);
    }

    // Initialize - only load basic data, not moderation
    loadAllData();
</script>

<div>
    <div class="header">
        <h2>Managed ACL Configuration</h2>
        <p>Configure access control using NIP-86 management API</p>
        <div class="owner-only-notice">
            <strong>Owner Only:</strong> This interface is restricted to relay owners
            only.
        </div>
    </div>

    {#if message}
        <div class="message {messageType}">
            {message}
        </div>
    {/if}

    <div class="tabs">
        <button
            class="tab {activeTab === 'pubkeys' ? 'active' : ''}"
            on:click={() => (activeTab = "pubkeys")}
        >
            Pubkeys
        </button>
        <button
            class="tab {activeTab === 'events' ? 'active' : ''}"
            on:click={() => (activeTab = "events")}
        >
            Events
        </button>
        <button
            class="tab {activeTab === 'ips' ? 'active' : ''}"
            on:click={() => (activeTab = "ips")}
        >
            IPs
        </button>
        <button
            class="tab {activeTab === 'kinds' ? 'active' : ''}"
            on:click={() => (activeTab = "kinds")}
        >
            Kinds
        </button>
        <button
            class="tab {activeTab === 'moderation' ? 'active' : ''}"
            on:click={() => {
                activeTab = "moderation";
                // Load moderation data only when tab is opened
                if (
                    !eventsNeedingModeration ||
                    eventsNeedingModeration.length === 0
                ) {
                    loadEventsNeedingModeration();
                }
            }}
        >
            Moderation
        </button>
        <button
            class="tab {activeTab === 'relay' ? 'active' : ''}"
            on:click={() => (activeTab = "relay")}
        >
            Relay Config
        </button>
    </div>

    <div class="tab-content">
        {#if activeTab === "pubkeys"}
            <div class="pubkeys-section">
                <div class="section">
                    <h3>Banned Pubkeys</h3>
                    <div class="add-form">
                        <input
                            type="text"
                            placeholder="Pubkey (64 hex chars)"
                            bind:value={newBannedPubkey}
                        />
                        <input
                            type="text"
                            placeholder="Reason (optional)"
                            bind:value={newBannedPubkeyReason}
                        />
                        <button on:click={banPubkey} disabled={isLoading}
                            >Ban Pubkey</button
                        >
                    </div>
                    <div class="list">
                        {#if bannedPubkeys && bannedPubkeys.length > 0}
                            {#each bannedPubkeys as item}
                                <div class="list-item">
                                    <span class="pubkey">{item.pubkey}</span>
                                    {#if item.reason}
                                        <span class="reason">{item.reason}</span
                                        >
                                    {/if}
                                </div>
                            {/each}
                        {:else}
                            <div class="no-items">
                                <p>No banned pubkeys configured.</p>
                            </div>
                        {/if}
                    </div>
                </div>

                <div class="section">
                    <h3>Allowed Pubkeys</h3>
                    <div class="add-form">
                        <input
                            type="text"
                            placeholder="Pubkey (64 hex chars)"
                            bind:value={newAllowedPubkey}
                        />
                        <input
                            type="text"
                            placeholder="Reason (optional)"
                            bind:value={newAllowedPubkeyReason}
                        />
                        <button on:click={allowPubkey} disabled={isLoading}
                            >Allow Pubkey</button
                        >
                    </div>
                    <div class="list">
                        {#if allowedPubkeys && allowedPubkeys.length > 0}
                            {#each allowedPubkeys as item}
                                <div class="list-item">
                                    <span class="pubkey">{item.pubkey}</span>
                                    {#if item.reason}
                                        <span class="reason">{item.reason}</span
                                        >
                                    {/if}
                                </div>
                            {/each}
                        {:else}
                            <div class="no-items">
                                <p>No allowed pubkeys configured.</p>
                            </div>
                        {/if}
                    </div>
                </div>
            </div>
        {/if}

        {#if activeTab === "events"}
            <div class="events-section">
                <div class="section">
                    <h3>Banned Events</h3>
                    <div class="add-form">
                        <input
                            type="text"
                            placeholder="Event ID (64 hex chars)"
                            bind:value={newBannedEvent}
                        />
                        <input
                            type="text"
                            placeholder="Reason (optional)"
                            bind:value={newBannedEventReason}
                        />
                        <button on:click={banEvent} disabled={isLoading}
                            >Ban Event</button
                        >
                    </div>
                    <div class="list">
                        {#if bannedEvents && bannedEvents.length > 0}
                            {#each bannedEvents as item}
                                <div class="list-item">
                                    <span class="event-id">{item.id}</span>
                                    {#if item.reason}
                                        <span class="reason">{item.reason}</span
                                        >
                                    {/if}
                                </div>
                            {/each}
                        {:else}
                            <div class="no-items">
                                <p>No banned events configured.</p>
                            </div>
                        {/if}
                    </div>
                </div>

                <div class="section">
                    <h3>Allowed Events</h3>
                    <div class="add-form">
                        <input
                            type="text"
                            placeholder="Event ID (64 hex chars)"
                            bind:value={newAllowedEvent}
                        />
                        <input
                            type="text"
                            placeholder="Reason (optional)"
                            bind:value={newAllowedEventReason}
                        />
                        <button on:click={allowEvent} disabled={isLoading}
                            >Allow Event</button
                        >
                    </div>
                    <div class="list">
                        {#if allowedEvents && allowedEvents.length > 0}
                            {#each allowedEvents as item}
                                <div class="list-item">
                                    <span class="event-id">{item.id}</span>
                                    {#if item.reason}
                                        <span class="reason">{item.reason}</span
                                        >
                                    {/if}
                                </div>
                            {/each}
                        {:else}
                            <div class="no-items">
                                <p>No allowed events configured.</p>
                            </div>
                        {/if}
                    </div>
                </div>
            </div>
        {/if}

        {#if activeTab === "ips"}
            <div class="ips-section">
                <div class="section">
                    <h3>Blocked IPs</h3>
                    <div class="add-form">
                        <input
                            type="text"
                            placeholder="IP Address"
                            bind:value={newBlockedIP}
                        />
                        <input
                            type="text"
                            placeholder="Reason (optional)"
                            bind:value={newBlockedIPReason}
                        />
                        <button on:click={blockIP} disabled={isLoading}
                            >Block IP</button
                        >
                    </div>
                    <div class="list">
                        {#if blockedIPs && blockedIPs.length > 0}
                            {#each blockedIPs as item}
                                <div class="list-item">
                                    <span class="ip">{item.ip}</span>
                                    {#if item.reason}
                                        <span class="reason">{item.reason}</span
                                        >
                                    {/if}
                                </div>
                            {/each}
                        {:else}
                            <div class="no-items">
                                <p>No blocked IPs configured.</p>
                            </div>
                        {/if}
                    </div>
                </div>
            </div>
        {/if}

        {#if activeTab === "kinds"}
            <div class="kinds-section">
                <div class="section">
                    <h3>Allowed Event Kinds</h3>
                    <div class="add-form">
                        <input
                            type="number"
                            placeholder="Kind number"
                            bind:value={newAllowedKind}
                        />
                        <button on:click={allowKind} disabled={isLoading}
                            >Allow Kind</button
                        >
                    </div>
                    <div class="list">
                        {#if allowedKinds && allowedKinds.length > 0}
                            {#each allowedKinds as kind}
                                <div class="list-item">
                                    <span class="kind">Kind {kind}</span>
                                    <button
                                        class="remove-btn"
                                        on:click={() => disallowKind(kind)}
                                        >Remove</button
                                    >
                                </div>
                            {/each}
                        {:else}
                            <div class="no-items">
                                <p>
                                    No allowed kinds configured. All kinds are
                                    allowed by default.
                                </p>
                            </div>
                        {/if}
                    </div>
                </div>
            </div>
        {/if}

        {#if activeTab === "moderation"}
            <div class="moderation-section">
                <div class="section">
                    <h3>Events Needing Moderation</h3>
                    <button
                        on:click={loadEventsNeedingModeration}
                        disabled={isLoading}>Refresh</button
                    >
                    <div class="list">
                        {#if eventsNeedingModeration && eventsNeedingModeration.length > 0}
                            {#each eventsNeedingModeration as item}
                                <div class="list-item">
                                    <span class="event-id">{item.id}</span>
                                    {#if item.reason}
                                        <span class="reason">{item.reason}</span
                                        >
                                    {/if}
                                    <div class="actions">
                                        <button
                                            on:click={() =>
                                                allowEventFromModeration(
                                                    item.id,
                                                )}>Allow</button
                                        >
                                        <button
                                            on:click={() =>
                                                banEventFromModeration(item.id)}
                                            >Ban</button
                                        >
                                    </div>
                                </div>
                            {/each}
                        {:else}
                            <div class="no-items">
                                <p>No events need moderation at this time.</p>
                            </div>
                        {/if}
                    </div>
                </div>
            </div>
        {/if}

        {#if activeTab === "relay"}
            <div class="relay-section">
                <div class="section">
                    <h3>Relay Configuration</h3>
                    <div class="config-actions">
                        <button
                            on:click={fetchRelayInfo}
                            disabled={isLoading}
                            class="refresh-btn"
                        >
                            🔄 Refresh from Relay Info
                        </button>
                    </div>
                    <div class="config-form">
                        <div class="form-group">
                            <label for="relay-name">Relay Name</label>
                            <input
                                id="relay-name"
                                type="text"
                                bind:value={relayConfig.relay_name}
                                placeholder="Enter relay name"
                            />
                        </div>
                        <div class="form-group">
                            <label for="relay-description"
                                >Relay Description</label
                            >
                            <textarea
                                id="relay-description"
                                bind:value={relayConfig.relay_description}
                                placeholder="Enter relay description"
                            ></textarea>
                        </div>
                        <div class="form-group">
                            <label for="relay-icon">Relay Icon URL</label>
                            <input
                                id="relay-icon"
                                type="url"
                                bind:value={relayConfig.relay_icon}
                                placeholder="Enter icon URL"
                            />
                        </div>
                        <div class="config-update-section">
                            <button
                                on:click={updateRelayConfiguration}
                                disabled={isLoading}
                                class="update-all-btn"
                            >
                                {#if isLoading}
                                    ⏳ Updating...
                                {:else}
                                    💾 Update Configuration
                                {/if}
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        {/if}
    </div>
</div>

<style>
    .header {
        margin-bottom: 30px;
    }

    .header h2 {
        margin: 0 0 10px 0;
        color: var(--text-color);
    }

    .header p {
        margin: 0;
        color: var(--text-color);
        opacity: 0.8;
    }

    .owner-only-notice {
        margin-top: 10px;
        padding: 8px 12px;
        background-color: var(--warning-bg);
        border: 1px solid var(--warning);
        border-radius: 4px;
        color: var(--text-color);
        font-size: 0.9em;
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

    .tabs {
        display: flex;
        border-bottom: 1px solid var(--border-color);
        margin-bottom: 20px;
    }

    .tab {
        padding: 10px 20px;
        border: none;
        background: none;
        cursor: pointer;
        border-bottom: 2px solid transparent;
        transition: all 0.2s;
        color: var(--text-color);
    }

    .tab:hover {
        background-color: var(--button-hover-bg);
    }

    .tab.active {
        border-bottom-color: var(--accent-color);
        color: var(--accent-color);
    }

    .tab-content {
        min-height: 400px;
    }

    .section {
        margin-bottom: 30px;
    }

    .section h3 {
        margin: 0 0 15px 0;
        color: var(--text-color);
    }

    .add-form {
        display: flex;
        gap: 10px;
        margin-bottom: 20px;
        flex-wrap: wrap;
    }

    .add-form input {
        padding: 8px 12px;
        border: 1px solid var(--input-border);
        border-radius: 4px;
        background: var(--bg-color);
        color: var(--text-color);
        flex: 1;
        min-width: 200px;
    }

    .add-form button {
        padding: 8px 16px;
        background-color: var(--accent-color);
        color: var(--text-color);
        border: none;
        border-radius: 4px;
        cursor: pointer;
    }

    .add-form button:disabled {
        background-color: var(--secondary);
        cursor: not-allowed;
    }

    .list {
        border: 1px solid var(--border-color);
        border-radius: 4px;
        max-height: 300px;
        overflow-y: auto;
        background: var(--bg-color);
    }

    .list-item {
        padding: 10px 15px;
        border-bottom: 1px solid var(--border-color);
        display: flex;
        align-items: center;
        gap: 15px;
        color: var(--text-color);
    }

    .list-item:last-child {
        border-bottom: none;
    }

    .pubkey,
    .event-id,
    .ip,
    .kind {
        font-family: monospace;
        font-size: 0.9em;
        color: var(--text-color);
    }

    .reason {
        color: var(--text-color);
        opacity: 0.7;
        font-style: italic;
    }

    .remove-btn {
        padding: 4px 8px;
        background-color: var(--danger);
        color: var(--text-color);
        border: none;
        border-radius: 3px;
        cursor: pointer;
        font-size: 0.8em;
    }

    .actions {
        display: flex;
        gap: 5px;
        margin-left: auto;
    }

    .actions button {
        padding: 4px 8px;
        border: none;
        border-radius: 3px;
        cursor: pointer;
        font-size: 0.8em;
    }

    .actions button:first-child {
        background-color: var(--success);
        color: var(--text-color);
    }

    .actions button:last-child {
        background-color: var(--danger);
        color: var(--text-color);
    }

    .config-form {
        display: flex;
        flex-direction: column;
        gap: 20px;
    }

    .form-group {
        display: flex;
        flex-direction: column;
        gap: 10px;
    }

    .form-group label {
        font-weight: bold;
        color: var(--text-color);
    }

    .form-group input,
    .form-group textarea {
        padding: 8px 12px;
        border: 1px solid var(--input-border);
        border-radius: 4px;
        background: var(--bg-color);
        color: var(--text-color);
    }

    .form-group textarea {
        min-height: 80px;
        resize: vertical;
    }

    .config-actions {
        margin-bottom: 20px;
        padding: 10px;
        background-color: var(--button-bg);
        border-radius: 4px;
    }

    .refresh-btn {
        padding: 8px 16px;
        background-color: var(--success);
        color: var(--text-color);
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
    }

    .refresh-btn:hover:not(:disabled) {
        background-color: var(--success);
        filter: brightness(0.9);
    }

    .refresh-btn:disabled {
        background-color: var(--secondary);
        cursor: not-allowed;
    }

    .config-update-section {
        margin-top: 20px;
        padding: 15px;
        background-color: var(--button-bg);
        border-radius: 6px;
        text-align: center;
    }

    .update-all-btn {
        padding: 12px 24px;
        background-color: var(--success);
        color: var(--text-color);
        border: none;
        border-radius: 6px;
        cursor: pointer;
        font-size: 1em;
        font-weight: 600;
        min-width: 200px;
    }

    .update-all-btn:hover:not(:disabled) {
        background-color: var(--success);
        filter: brightness(0.9);
        transform: translateY(-1px);
        box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
    }

    .update-all-btn:disabled {
        background-color: var(--secondary);
        cursor: not-allowed;
        transform: none;
        box-shadow: none;
    }

    .no-items {
        padding: 20px;
        text-align: center;
        color: var(--text-color);
        opacity: 0.7;
        font-style: italic;
    }
</style>
