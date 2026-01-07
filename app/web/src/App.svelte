<script>
    // Svelte component imports
    import LoginModal from "./LoginModal.svelte";
    import ManagedACL from "./ManagedACL.svelte";
    import Header from "./Header.svelte";
    import Sidebar from "./Sidebar.svelte";
    import ExportView from "./ExportView.svelte";
    import ImportView from "./ImportView.svelte";
    import EventsView from "./EventsView.svelte";
    import ComposeView from "./ComposeView.svelte";
    import RecoveryView from "./RecoveryView.svelte";
    import SprocketView from "./SprocketView.svelte";
    import PolicyView from "./PolicyView.svelte";
    import CurationView from "./CurationView.svelte";
    import BlossomView from "./BlossomView.svelte";
    import LogView from "./LogView.svelte";
    import RelayConnectView from "./RelayConnectView.svelte";
    import SearchResultsView from "./SearchResultsView.svelte";
    import FilterDisplay from "./FilterDisplay.svelte";

    // Utility imports
    import { buildFilter } from "./helpers.tsx";
    import { replaceableKinds, kindNames, CACHE_DURATION } from "./constants.js";
    import { getKindName, truncatePubkey, truncateContent, formatTimestamp, escapeHtml, aboutToHtml, copyToClipboard, showCopyFeedback } from "./utils.js";
    import * as api from "./api.js";

    // Nostr library imports
    import {
        initializeNostrClient,
        fetchUserProfile,
        fetchAllEvents,
        fetchUserEvents,
        searchEvents,
        fetchEvents,
        fetchEventById,
        fetchDeleteEventsByTarget,
        queryEvents,
        queryEventsFromDB,
        debugIndexedDB,
        nostrClient,
        NostrClient,
        Nip07Signer,
        PrivateKeySigner,
    } from "./nostr.js";
    import { publishEventWithAuth } from "./websocket-auth.js";

    // Expose debug function globally for console access
    if (typeof window !== "undefined") {
        window.debugIndexedDB = debugIndexedDB;
    }

    let isDarkTheme = false;
    let showLoginModal = false;
    let isLoggedIn = false;
    let userPubkey = "";
    let authMethod = "";
    let userProfile = null;
    let userRole = "";
    let userSigner = null;
    let showSettingsDrawer = false;
    let selectedTab = localStorage.getItem("selectedTab") || "export";
    let showFilterBuilder = false; // Show filter builder in events view
    let eventsViewFilter = {}; // Active filter for events view
    let searchTabs = [];
    let allEvents = [];
    let selectedFile = null;
    let importMessage = ""; // Message shown after import completes
    let expandedEvents = new Set();
    let isLoadingEvents = false;
    let hasMoreEvents = true;
    let eventsPerPage = 100;
    let oldestEventTimestamp = null; // For timestamp-based pagination
    let newestEventTimestamp = null; // For loading newer events
    let showPermissionMenu = false;
    let viewAsRole = "";

    // Search results state
    let searchResults = new Map(); // Map of searchTabId -> { filter, events, isLoading, hasMore, oldestTimestamp }
    let isLoadingSearch = false;

    // Screen-filling events view state
    let eventsPerScreen = 20; // Default, will be calculated based on screen size

    // Global events cache system
    let globalEventsCache = []; // All events cache
    let globalCacheTimestamp = 0;
    // CACHE_DURATION is imported from constants.js

    // Events filter toggle
    let showOnlyMyEvents = false;

    // My Events state
    let myEvents = [];
    let isLoadingMyEvents = false;
    let hasMoreMyEvents = true;
    let oldestMyEventTimestamp = null;
    let newestMyEventTimestamp = null;

    // Sprocket management state
    let sprocketScript = "";
    let sprocketStatus = null;
    let sprocketVersions = [];
    let isLoadingSprocket = false;
    let sprocketMessage = "";
    let sprocketMessageType = "info";
    let sprocketEnabled = false;
    let sprocketUploadFile = null;

    // Policy management state
    let policyJson = "";
    let policyEnabled = false;
    let isPolicyAdmin = false;
    let isLoadingPolicy = false;
    let policyMessage = "";
    let policyMessageType = "info";
    let policyValidationErrors = [];
    let policyFollows = [];

    // NRC (Nostr Relay Connect) state
    let nrcEnabled = false;

    // ACL mode
    let aclMode = "";

    // Relay version
    let relayVersion = "";

    // Compose tab state
    let composeEventJson = "";
    let composePublishError = "";

    // Recovery tab state
    let recoverySelectedKind = null;
    let recoveryCustomKind = "";
    let recoveryEvents = [];
    let isLoadingRecovery = false;
    let recoveryHasMore = true;
    let recoveryOldestTimestamp = null;
    let recoveryNewestTimestamp = null;

    // replaceableKinds is now imported from constants.js

    // Helper functions imported from utils.js:
    // - getKindName, truncatePubkey, truncateContent, formatTimestamp, escapeHtml, copyToClipboard, showCopyFeedback

    function toggleEventExpansion(eventId) {
        if (expandedEvents.has(eventId)) {
            expandedEvents.delete(eventId);
        } else {
            expandedEvents.add(eventId);
        }
        expandedEvents = expandedEvents; // Trigger reactivity
    }

    async function copyEventToClipboard(eventData, clickEvent) {
        const minifiedJson = JSON.stringify(eventData);
        const success = await copyToClipboard(minifiedJson);
        const button = clickEvent.target.closest(".copy-json-btn");
        showCopyFeedback(button, success);
        if (!success) {
            alert("Failed to copy to clipboard. Please copy manually.");
        }
    }

    async function handleToggleChange() {
        // Toggle state is already updated by bind:checked
        console.log("Toggle changed, showOnlyMyEvents:", showOnlyMyEvents);

        // Reset the attempt flag to allow reloading with new filter
        hasAttemptedEventLoad = false;

        // Reload events with the new filter
        const authors =
            showOnlyMyEvents && isLoggedIn && userPubkey ? [userPubkey] : null;
        await loadAllEvents(true, authors);
    }

    // Events are filtered server-side, but add client-side filtering as backup
    // Sort events by created_at timestamp (newest first)
    $: filteredEvents = (
        showOnlyMyEvents && isLoggedIn && userPubkey
            ? allEvents.filter(
                  (event) => event.pubkey && event.pubkey === userPubkey,
              )
            : allEvents
    ).sort((a, b) => b.created_at - a.created_at);

    async function deleteEvent(eventId) {
        if (!isLoggedIn) {
            alert("Please log in first");
            return;
        }

        // Find the event to check if user can delete it
        const event = allEvents.find((e) => e.id === eventId);
        if (!event) {
            alert("Event not found");
            return;
        }

        // Check permissions: admin/owner can delete any event, write users can only delete their own events
        const canDelete =
            userRole === "admin" ||
            userRole === "owner" ||
            (userRole === "write" &&
                event.pubkey &&
                event.pubkey === userPubkey);

        if (!canDelete) {
            alert("You do not have permission to delete this event");
            return;
        }

        if (!confirm("Are you sure you want to delete this event?")) {
            return;
        }

        try {
            // Check if signer is available
            if (!userSigner) {
                throw new Error("Signer not available for signing");
            }

            // Create the delete event template (unsigned)
            const deleteEventTemplate = {
                kind: 5,
                created_at: Math.floor(Date.now() / 1000),
                tags: [["e", eventId]], // e-tag referencing the event to delete
                content: "",
                // Don't set pubkey - let the signer set it
            };

            console.log("Created delete event template:", deleteEventTemplate);
            console.log("User pubkey:", userPubkey);
            console.log("Target event:", event);
            console.log("Target event pubkey:", event.pubkey);

            // Sign the event using the signer
            const signedDeleteEvent =
                await userSigner.signEvent(deleteEventTemplate);
            console.log("Signed delete event:", signedDeleteEvent);
            console.log(
                "Signed delete event pubkey:",
                signedDeleteEvent.pubkey,
            );
            console.log("Delete event tags:", signedDeleteEvent.tags);

            // Publish to the ORLY relay using WebSocket authentication
            const wsProtocol = window.location.protocol.startsWith('https') ? 'wss:' : 'ws:';
            const relayUrl = `${wsProtocol}//${window.location.host}/`;

            try {
                const result = await publishEventWithAuth(
                    relayUrl,
                    signedDeleteEvent,
                    userSigner,
                    userPubkey,
                );

                if (result.success) {
                    console.log(
                        "Delete event published successfully to ORLY relay",
                    );
                } else {
                    console.error(
                        "Failed to publish delete event:",
                        result.reason,
                    );
                }
            } catch (error) {
                console.error("Error publishing delete event:", error);
            }

            // Determine if we should publish to external relays
            // Only publish to external relays if:
            // 1. User is deleting their own event, OR
            // 2. User is admin/owner AND deleting their own event
            const isDeletingOwnEvent =
                event.pubkey && event.pubkey === userPubkey;
            const isAdminOrOwner = userRole === "admin" || userRole === "owner";
            const shouldPublishToExternalRelays = isDeletingOwnEvent;

            if (shouldPublishToExternalRelays) {
                // Publish the delete event to all relays (including external ones)
                const result = await nostrClient.publish(signedDeleteEvent);
                console.log("Delete event published:", result);

                if (result.success && result.okCount > 0) {
                    // Wait a moment for the deletion to propagate
                    await new Promise((resolve) => setTimeout(resolve, 2000));

                    // Verify the event was actually deleted by trying to fetch it
                    try {
                        const deletedEvent = await fetchEventById(eventId, {
                            timeout: 5000,
                        });
                        if (deletedEvent) {
                            console.warn(
                                "Event still exists after deletion attempt:",
                                deletedEvent,
                            );
                            alert(
                                `Warning: Delete event was accepted by ${result.okCount} relay(s), but the event still exists on the relay. This may indicate the relay does not properly handle delete events.`,
                            );
                        } else {
                            console.log(
                                "Event successfully deleted and verified",
                            );
                        }
                    } catch (fetchError) {
                        console.log(
                            "Could not fetch event after deletion (likely deleted):",
                            fetchError.message,
                        );
                    }

                    // Also verify that the delete event has been saved
                    try {
                        const deleteEvents = await fetchDeleteEventsByTarget(
                            eventId,
                            { timeout: 5000 },
                        );
                        if (deleteEvents.length > 0) {
                            console.log(
                                `Delete event verification: Found ${deleteEvents.length} delete event(s) targeting ${eventId}`,
                            );
                            // Check if our delete event is among them
                            const ourDeleteEvent = deleteEvents.find(
                                (de) => de.pubkey && de.pubkey === userPubkey,
                            );
                            if (ourDeleteEvent) {
                                console.log(
                                    "Our delete event found in database:",
                                    ourDeleteEvent.id,
                                );
                            } else {
                                console.warn(
                                    "Our delete event not found in database, but other delete events exist",
                                );
                            }
                        } else {
                            console.warn(
                                "No delete events found in database for target event:",
                                eventId,
                            );
                        }
                    } catch (deleteFetchError) {
                        console.log(
                            "Could not verify delete event in database:",
                            deleteFetchError.message,
                        );
                    }

                    // Remove from local lists
                    allEvents = allEvents.filter(
                        (event) => event.id !== eventId,
                    );
                    myEvents = myEvents.filter((event) => event.id !== eventId);

                    // Remove from global cache
                    globalEventsCache = globalEventsCache.filter(
                        (event) => event.id !== eventId,
                    );

                    // Remove from search results cache
                    for (const [tabId, searchResult] of searchResults) {
                        if (searchResult.events) {
                            searchResult.events = searchResult.events.filter(
                                (event) => event.id !== eventId,
                            );
                            searchResults.set(tabId, searchResult);
                        }
                    }

                    // Update persistent state
                    savePersistentState();

                    // Reload events to show the new delete event at the top
                    console.log("Reloading events to show delete event...");
                    const authors =
                        showOnlyMyEvents && isLoggedIn && userPubkey
                            ? [userPubkey]
                            : null;
                    await loadAllEvents(true, authors);

                    alert(
                        `Event deleted successfully (accepted by ${result.okCount} relay(s))`,
                    );
                } else {
                    throw new Error("No relays accepted the delete event");
                }
            } else {
                // Admin/owner deleting someone else's event - only publish to local relay
                // We need to publish only to the local relay, not external ones
                const wsProto = window.location.protocol.startsWith('https') ? 'wss:' : 'ws:';
                const localRelayUrl = `${wsProto}//${window.location.host}/`;

                // Create a modified client that only connects to the local relay
                const localClient = new NostrClient();
                await localClient.connectToRelay(localRelayUrl);

                const result = await localClient.publish(signedDeleteEvent);
                console.log(
                    "Delete event published to local relay only:",
                    result,
                );

                if (result.success && result.okCount > 0) {
                    // Wait a moment for the deletion to propagate
                    await new Promise((resolve) => setTimeout(resolve, 2000));

                    // Verify the event was actually deleted by trying to fetch it
                    try {
                        const deletedEvent = await fetchEventById(eventId, {
                            timeout: 5000,
                        });
                        if (deletedEvent) {
                            console.warn(
                                "Event still exists after deletion attempt:",
                                deletedEvent,
                            );
                            alert(
                                `Warning: Delete event was accepted by ${result.okCount} relay(s), but the event still exists on the relay. This may indicate the relay does not properly handle delete events.`,
                            );
                        } else {
                            console.log(
                                "Event successfully deleted and verified",
                            );
                        }
                    } catch (fetchError) {
                        console.log(
                            "Could not fetch event after deletion (likely deleted):",
                            fetchError.message,
                        );
                    }

                    // Also verify that the delete event has been saved
                    try {
                        const deleteEvents = await fetchDeleteEventsByTarget(
                            eventId,
                            { timeout: 5000 },
                        );
                        if (deleteEvents.length > 0) {
                            console.log(
                                `Delete event verification: Found ${deleteEvents.length} delete event(s) targeting ${eventId}`,
                            );
                            // Check if our delete event is among them
                            const ourDeleteEvent = deleteEvents.find(
                                (de) => de.pubkey && de.pubkey === userPubkey,
                            );
                            if (ourDeleteEvent) {
                                console.log(
                                    "Our delete event found in database:",
                                    ourDeleteEvent.id,
                                );
                            } else {
                                console.warn(
                                    "Our delete event not found in database, but other delete events exist",
                                );
                            }
                        } else {
                            console.warn(
                                "No delete events found in database for target event:",
                                eventId,
                            );
                        }
                    } catch (deleteFetchError) {
                        console.log(
                            "Could not verify delete event in database:",
                            deleteFetchError.message,
                        );
                    }

                    // Remove from local lists
                    allEvents = allEvents.filter(
                        (event) => event.id !== eventId,
                    );
                    myEvents = myEvents.filter((event) => event.id !== eventId);

                    // Remove from global cache
                    globalEventsCache = globalEventsCache.filter(
                        (event) => event.id !== eventId,
                    );

                    // Remove from search results cache
                    for (const [tabId, searchResult] of searchResults) {
                        if (searchResult.events) {
                            searchResult.events = searchResult.events.filter(
                                (event) => event.id !== eventId,
                            );
                            searchResults.set(tabId, searchResult);
                        }
                    }

                    // Update persistent state
                    savePersistentState();

                    // Reload events to show the new delete event at the top
                    console.log("Reloading events to show delete event...");
                    const authors =
                        showOnlyMyEvents && isLoggedIn && userPubkey
                            ? [userPubkey]
                            : null;
                    await loadAllEvents(true, authors);

                    alert(
                        `Event deleted successfully (local relay only - admin/owner deleting other user's event)`,
                    );
                } else {
                    throw new Error(
                        "Local relay did not accept the delete event",
                    );
                }
            }
        } catch (error) {
            console.error("Failed to delete event:", error);
            alert("Failed to delete event: " + error.message);
        }
    }

    // escapeHtml is imported from utils.js

    // Recovery tab functions
    async function loadRecoveryEvents() {
        const kindToUse = recoveryCustomKind
            ? parseInt(recoveryCustomKind)
            : recoverySelectedKind;

        if (kindToUse === null || kindToUse === undefined || isNaN(kindToUse)) {
            console.log("No valid kind to load, kindToUse:", kindToUse);
            return;
        }

        if (!isLoggedIn) {
            console.log("Not logged in, cannot load recovery events");
            return;
        }

        console.log(
            "Loading recovery events for kind:",
            kindToUse,
            "user:",
            userPubkey,
        );
        isLoadingRecovery = true;
        try {
            // Fetch multiple versions using limit parameter
            // For replaceable events, limit > 1 returns multiple versions
            const filters = [
                {
                    kinds: [kindToUse],
                    authors: [userPubkey],
                    limit: 100, // Get up to 100 versions
                },
            ];

            if (recoveryOldestTimestamp) {
                filters[0].until = recoveryOldestTimestamp;
            }

            console.log("Recovery filters:", filters);

            // Use queryEvents which checks IndexedDB cache first, then relay
            const events = await queryEvents(filters, {
                timeout: 30000,
                cacheFirst: true, // Check cache first
            });

            console.log("Recovery events received:", events.length);
            console.log(
                "Recovery events kinds:",
                events.map((e) => e.kind),
            );

            if (recoveryOldestTimestamp) {
                // Append to existing events
                recoveryEvents = [...recoveryEvents, ...events];
            } else {
                // Replace events
                recoveryEvents = events;
            }

            if (events.length > 0) {
                recoveryOldestTimestamp = Math.min(
                    ...events.map((e) => e.created_at),
                );
                recoveryHasMore = events.length === 100;
            } else {
                recoveryHasMore = false;
            }
        } catch (error) {
            console.error("Failed to load recovery events:", error);
        } finally {
            isLoadingRecovery = false;
        }
    }

    // Get user's write relays from relay list event (kind 10002)
    async function getUserWriteRelays() {
        if (!userPubkey) {
            return [];
        }

        try {
            // Query for the user's relay list event (kind 10002)
            const relayListEvents = await queryEventsFromDB([
                {
                    kinds: [10002],
                    authors: [userPubkey],
                    limit: 1,
                },
            ]);

            if (relayListEvents.length === 0) {
                console.log("No relay list event found for user");
                return [];
            }

            const relayListEvent = relayListEvents[0];
            console.log("Found relay list event:", relayListEvent);

            const writeRelays = [];

            // Parse r tags to extract write relays
            for (const tag of relayListEvent.tags) {
                if (tag[0] === "r" && tag.length >= 2) {
                    const relayUrl = tag[1];
                    const permission = tag.length >= 3 ? tag[2] : null;

                    // Include relay if it's explicitly marked for write or has no permission specified (default is read+write)
                    if (!permission || permission === "write") {
                        writeRelays.push(relayUrl);
                    }
                }
            }

            console.log("Found write relays:", writeRelays);
            return writeRelays;
        } catch (error) {
            console.error("Error fetching user write relays:", error);
            return [];
        }
    }

    async function repostEvent(event) {
        if (!confirm("Are you sure you want to repost this event?")) {
            return;
        }

        try {
            const wsProto = window.location.protocol.startsWith('https') ? 'wss:' : 'ws:';
            const localRelayUrl = `${wsProto}//${window.location.host}/`;
            console.log(
                "Reposting event to local relay:",
                localRelayUrl,
                event,
            );

            // Create a new event with updated timestamp
            const newEvent = { ...event };
            newEvent.created_at = Math.floor(Date.now() / 1000);
            newEvent.id = ""; // Clear the old ID so it gets recalculated
            newEvent.sig = ""; // Clear the old signature

            // For addressable events, ensure the d tag matches
            if (event.kind >= 30000 && event.kind <= 39999) {
                const dTag = event.tags.find((tag) => tag[0] === "d");
                if (dTag) {
                    newEvent.tags = newEvent.tags.filter(
                        (tag) => tag[0] !== "d",
                    );
                    newEvent.tags.push(dTag);
                }
            }

            // Sign the event before publishing
            if (userSigner) {
                const signedEvent = await userSigner.signEvent(newEvent);
                console.log("Signed event for repost:", signedEvent);

                const result = await nostrClient.publish(signedEvent, [
                    localRelayUrl,
                ]);
                console.log("Repost publish result:", result);

                if (result.success && result.okCount > 0) {
                    alert("Event reposted successfully!");
                    recoveryHasMore = false; // Reset to allow reloading
                    await loadRecoveryEvents(); // Reload the events to show the new version
                } else {
                    alert("Failed to repost event. Check console for details.");
                }
            } else {
                alert("No signer available. Please log in.");
            }
        } catch (error) {
            console.error("Error reposting event:", error);
            alert("Error reposting event: " + error.message);
        }
    }

    async function repostEventToAll(event) {
        if (
            !confirm(
                "Are you sure you want to repost this event to all your write relays?",
            )
        ) {
            return;
        }

        try {
            // Get user's write relays
            const writeRelays = await getUserWriteRelays();
            const wsProto = window.location.protocol.startsWith('https') ? 'wss:' : 'ws:';
            const localRelayUrl = `${wsProto}//${window.location.host}/`;

            // Always include local relay
            const allRelays = [
                localRelayUrl,
                ...writeRelays.filter((url) => url !== localRelayUrl),
            ];

            if (allRelays.length === 1) {
                alert(
                    "No write relays found in your relay list. Only posting to local relay.",
                );
            }

            console.log("Reposting event to all relays:", allRelays, event);

            // Create a new event with updated timestamp
            const newEvent = { ...event };
            newEvent.created_at = Math.floor(Date.now() / 1000);
            newEvent.id = ""; // Clear the old ID so it gets recalculated
            newEvent.sig = ""; // Clear the old signature

            // For addressable events, ensure the d tag matches
            if (event.kind >= 30000 && event.kind <= 39999) {
                const dTag = event.tags.find((tag) => tag[0] === "d");
                if (dTag) {
                    newEvent.tags = newEvent.tags.filter(
                        (tag) => tag[0] !== "d",
                    );
                    newEvent.tags.push(dTag);
                }
            }

            // Sign the event before publishing
            if (userSigner) {
                const signedEvent = await userSigner.signEvent(newEvent);
                console.log("Signed event for repost to all:", signedEvent);

                const result = await nostrClient.publish(
                    signedEvent,
                    allRelays,
                );
                console.log("Repost to all publish result:", result);

                if (result.success && result.okCount > 0) {
                    alert(
                        `Event reposted successfully to ${allRelays.length} relays!`,
                    );
                    recoveryHasMore = false; // Reset to allow reloading
                    await loadRecoveryEvents(); // Reload the events to show the new version
                } else {
                    alert("Failed to repost event. Check console for details.");
                }
            } else {
                alert("No signer available. Please log in.");
            }
        } catch (error) {
            console.error("Error reposting event to all:", error);
            alert("Error reposting event to all: " + error.message);
        }
    }

    function selectRecoveryKind() {
        console.log(
            "selectRecoveryKind called, recoverySelectedKind:",
            recoverySelectedKind,
        );
        if (
            recoverySelectedKind === null ||
            recoverySelectedKind === undefined
        ) {
            console.log("No kind selected, skipping load");
            return;
        }
        recoveryCustomKind = ""; // Clear custom kind when selecting from dropdown
        recoveryEvents = [];
        recoveryOldestTimestamp = null;
        recoveryHasMore = true;
        loadRecoveryEvents();
    }

    function handleCustomKindInput() {
        console.log(
            "handleCustomKindInput called, recoveryCustomKind:",
            recoveryCustomKind,
        );
        // Check if a valid number was entered (including 0)
        const kindNum = parseInt(recoveryCustomKind);
        if (recoveryCustomKind !== "" && !isNaN(kindNum) && kindNum >= 0) {
            recoverySelectedKind = null; // Clear dropdown selection when using custom
            recoveryEvents = [];
            recoveryOldestTimestamp = null;
            recoveryHasMore = true;
            loadRecoveryEvents();
        }
    }

    function isCurrentVersion(event) {
        // Find all events with the same kind and pubkey
        const sameKindEvents = recoveryEvents.filter(
            (e) => e.kind === event.kind && e.pubkey === event.pubkey,
        );

        // Check if this event has the highest timestamp
        const maxTimestamp = Math.max(
            ...sameKindEvents.map((e) => e.created_at),
        );
        return event.created_at === maxTimestamp;
    }

    // Always show all versions - no filtering needed

    $: aboutHtml = userProfile?.about
        ? escapeHtml(userProfile.about).replace(/\n{2,}/g, "<br>")
        : "";

    // Detect system theme preference and listen for changes
    if (typeof window !== "undefined" && window.matchMedia) {
        const darkModeQuery = window.matchMedia("(prefers-color-scheme: dark)");
        isDarkTheme = darkModeQuery.matches;

        // Listen for system theme changes
        darkModeQuery.addEventListener("change", (e) => {
            isDarkTheme = e.matches;
        });
    }

    // Load state from localStorage
    if (typeof localStorage !== "undefined") {
        // Check for existing authentication
        const storedAuthMethod = localStorage.getItem("nostr_auth_method");
        const storedPubkey = localStorage.getItem("nostr_pubkey");

        if (storedAuthMethod && storedPubkey) {
            isLoggedIn = true;
            userPubkey = storedPubkey;
            authMethod = storedAuthMethod;

            // Restore signer for extension method
            if (storedAuthMethod === "extension" && window.nostr) {
                userSigner = window.nostr;
            }

            // Fetch user role for already logged in users
            fetchUserRole();
            fetchACLMode();
        }

        // Load persistent app state
        loadPersistentState();

        // Load sprocket configuration
        loadSprocketConfig();

        // Load NRC configuration
        loadNRCConfig();

        // Load policy configuration
        loadPolicyConfig();

        // Load relay version
        fetchRelayVersion();
    }

    function savePersistentState() {
        if (typeof localStorage === "undefined") return;

        const state = {
            selectedTab,
            expandedEvents: Array.from(expandedEvents),
            globalEventsCache,
            globalCacheTimestamp,
            hasMoreEvents,
            oldestEventTimestamp,
        };

        localStorage.setItem("app_state", JSON.stringify(state));
    }

    function loadPersistentState() {
        if (typeof localStorage === "undefined") return;

        try {
            const savedState = localStorage.getItem("app_state");
            if (savedState) {
                const state = JSON.parse(savedState);

                // Restore tab state
                if (
                    state.selectedTab &&
                    baseTabs.some((tab) => tab.id === state.selectedTab)
                ) {
                    selectedTab = state.selectedTab;
                }

                // Restore expanded events
                if (state.expandedEvents) {
                    expandedEvents = new Set(state.expandedEvents);
                }

                // Restore cache data

                if (state.globalEventsCache) {
                    globalEventsCache = state.globalEventsCache;
                }

                if (state.globalCacheTimestamp) {
                    globalCacheTimestamp = state.globalCacheTimestamp;
                }

                if (state.hasMoreEvents !== undefined) {
                    hasMoreEvents = state.hasMoreEvents;
                }

                if (state.oldestEventTimestamp) {
                    oldestEventTimestamp = state.oldestEventTimestamp;
                }

                if (state.hasMoreMyEvents !== undefined) {
                    hasMoreMyEvents = state.hasMoreMyEvents;
                }

                if (state.oldestMyEventTimestamp) {
                    oldestMyEventTimestamp = state.oldestMyEventTimestamp;
                }

                // Restore events from cache
                restoreEventsFromCache();
            }
        } catch (error) {
            console.error("Failed to load persistent state:", error);
        }
    }

    function restoreEventsFromCache() {
        // Restore global events cache
        if (
            globalEventsCache.length > 0 &&
            isCacheValid(globalCacheTimestamp)
        ) {
            allEvents = globalEventsCache;
        }
    }

    function isCacheValid(timestamp) {
        if (!timestamp) return false;
        return Date.now() - timestamp < CACHE_DURATION;
    }

    function updateGlobalCache(events) {
        globalEventsCache = events.sort((a, b) => b.created_at - a.created_at);
        globalCacheTimestamp = Date.now();
        savePersistentState();
    }

    function clearCache() {
        globalEventsCache = [];
        globalCacheTimestamp = 0;
        savePersistentState();
    }

    // Sprocket management functions
    async function loadSprocketConfig() {
        try {
            const response = await fetch("/api/sprocket/config", {
                method: "GET",
                headers: {
                    "Content-Type": "application/json",
                },
            });

            if (response.ok) {
                const config = await response.json();
                sprocketEnabled = config.enabled;
            }
        } catch (error) {
            console.error("Error loading sprocket config:", error);
        }
    }

    async function loadNRCConfig() {
        try {
            const config = await api.fetchNRCConfig();
            nrcEnabled = config.enabled;
        } catch (error) {
            console.error("Error loading NRC config:", error);
        }
    }

    async function loadPolicyConfig() {
        try {
            const response = await fetch("/api/policy/config", {
                method: "GET",
                headers: {
                    "Content-Type": "application/json",
                },
            });

            if (response.ok) {
                const config = await response.json();
                policyEnabled = config.enabled || false;
            }
        } catch (error) {
            console.error("Error loading policy config:", error);
            policyEnabled = false;
        }
    }

    async function loadSprocketStatus() {
        if (!isLoggedIn || userRole !== "owner" || !sprocketEnabled) return;

        try {
            isLoadingSprocket = true;
            const response = await fetch("/api/sprocket/status", {
                method: "GET",
                headers: {
                    Authorization: `Nostr ${await createNIP98Auth("GET", "/api/sprocket/status")}`,
                    "Content-Type": "application/json",
                },
            });

            if (response.ok) {
                sprocketStatus = await response.json();
            } else {
                showSprocketMessage("Failed to load sprocket status", "error");
            }
        } catch (error) {
            showSprocketMessage(
                `Error loading sprocket status: ${error.message}`,
                "error",
            );
        } finally {
            isLoadingSprocket = false;
        }
    }

    async function loadSprocket() {
        if (!isLoggedIn || userRole !== "owner") return;

        try {
            isLoadingSprocket = true;
            const response = await fetch("/api/sprocket/status", {
                method: "GET",
                headers: {
                    Authorization: `Nostr ${await createNIP98Auth("GET", "/api/sprocket/status")}`,
                    "Content-Type": "application/json",
                },
            });

            if (response.ok) {
                const status = await response.json();
                sprocketScript = status.script_content || "";
                sprocketStatus = status;
                showSprocketMessage("Script loaded successfully", "success");
            } else {
                showSprocketMessage("Failed to load script", "error");
            }
        } catch (error) {
            showSprocketMessage(
                `Error loading script: ${error.message}`,
                "error",
            );
        } finally {
            isLoadingSprocket = false;
        }
    }

    async function saveSprocket() {
        if (!isLoggedIn || userRole !== "owner") return;

        try {
            isLoadingSprocket = true;
            const response = await fetch("/api/sprocket/update", {
                method: "POST",
                headers: {
                    Authorization: `Nostr ${await createNIP98Auth("POST", "/api/sprocket/update")}`,
                    "Content-Type": "text/plain",
                },
                body: sprocketScript,
            });

            if (response.ok) {
                showSprocketMessage(
                    "Script saved and updated successfully",
                    "success",
                );
                await loadSprocketStatus();
                await loadVersions();
            } else {
                const errorText = await response.text();
                showSprocketMessage(
                    `Failed to save script: ${errorText}`,
                    "error",
                );
            }
        } catch (error) {
            showSprocketMessage(
                `Error saving script: ${error.message}`,
                "error",
            );
        } finally {
            isLoadingSprocket = false;
        }
    }

    async function restartSprocket() {
        if (!isLoggedIn || userRole !== "owner") return;

        try {
            isLoadingSprocket = true;
            const response = await fetch("/api/sprocket/restart", {
                method: "POST",
                headers: {
                    Authorization: `Nostr ${await createNIP98Auth("POST", "/api/sprocket/restart")}`,
                    "Content-Type": "application/json",
                },
            });

            if (response.ok) {
                showSprocketMessage(
                    "Sprocket restarted successfully",
                    "success",
                );
                await loadSprocketStatus();
            } else {
                const errorText = await response.text();
                showSprocketMessage(
                    `Failed to restart sprocket: ${errorText}`,
                    "error",
                );
            }
        } catch (error) {
            showSprocketMessage(
                `Error restarting sprocket: ${error.message}`,
                "error",
            );
        } finally {
            isLoadingSprocket = false;
        }
    }

    async function deleteSprocket() {
        if (!isLoggedIn || userRole !== "owner") return;

        if (
            !confirm(
                "Are you sure you want to delete the sprocket script? This will stop the current process.",
            )
        ) {
            return;
        }

        try {
            isLoadingSprocket = true;
            const response = await fetch("/api/sprocket/update", {
                method: "POST",
                headers: {
                    Authorization: `Nostr ${await createNIP98Auth("POST", "/api/sprocket/update")}`,
                    "Content-Type": "text/plain",
                },
                body: "", // Empty body deletes the script
            });

            if (response.ok) {
                sprocketScript = "";
                showSprocketMessage(
                    "Sprocket script deleted successfully",
                    "success",
                );
                await loadSprocketStatus();
                await loadVersions();
            } else {
                const errorText = await response.text();
                showSprocketMessage(
                    `Failed to delete script: ${errorText}`,
                    "error",
                );
            }
        } catch (error) {
            showSprocketMessage(
                `Error deleting script: ${error.message}`,
                "error",
            );
        } finally {
            isLoadingSprocket = false;
        }
    }

    async function loadVersions() {
        if (!isLoggedIn || userRole !== "owner") return;

        try {
            isLoadingSprocket = true;
            const response = await fetch("/api/sprocket/versions", {
                method: "GET",
                headers: {
                    Authorization: `Nostr ${await createNIP98Auth("GET", "/api/sprocket/versions")}`,
                    "Content-Type": "application/json",
                },
            });

            if (response.ok) {
                sprocketVersions = await response.json();
            } else {
                showSprocketMessage("Failed to load versions", "error");
            }
        } catch (error) {
            showSprocketMessage(
                `Error loading versions: ${error.message}`,
                "error",
            );
        } finally {
            isLoadingSprocket = false;
        }
    }

    async function loadVersion(version) {
        if (!isLoggedIn || userRole !== "owner") return;

        sprocketScript = version.content;
        showSprocketMessage(`Loaded version: ${version.name}`, "success");
    }

    async function deleteVersion(filename) {
        if (!isLoggedIn || userRole !== "owner") return;

        if (!confirm(`Are you sure you want to delete version ${filename}?`)) {
            return;
        }

        try {
            isLoadingSprocket = true;
            const response = await fetch("/api/sprocket/delete-version", {
                method: "POST",
                headers: {
                    Authorization: `Nostr ${await createNIP98Auth("POST", "/api/sprocket/delete-version")}`,
                    "Content-Type": "application/json",
                },
                body: JSON.stringify({ filename }),
            });

            if (response.ok) {
                showSprocketMessage(
                    `Version ${filename} deleted successfully`,
                    "success",
                );
                await loadVersions();
            } else {
                const errorText = await response.text();
                showSprocketMessage(
                    `Failed to delete version: ${errorText}`,
                    "error",
                );
            }
        } catch (error) {
            showSprocketMessage(
                `Error deleting version: ${error.message}`,
                "error",
            );
        } finally {
            isLoadingSprocket = false;
        }
    }

    function showSprocketMessage(message, type = "info") {
        sprocketMessage = message;
        sprocketMessageType = type;

        // Auto-hide message after 5 seconds
        setTimeout(() => {
            sprocketMessage = "";
        }, 5000);
    }

    // Policy management functions
    function showPolicyMessage(message, type = "info") {
        policyMessage = message;
        policyMessageType = type;

        // Auto-hide message after 5 seconds for non-errors
        if (type !== "error") {
            setTimeout(() => {
                policyMessage = "";
            }, 5000);
        }
    }

    async function loadPolicy() {
        if (!isLoggedIn || (userRole !== "owner" && !isPolicyAdmin)) return;

        try {
            isLoadingPolicy = true;
            policyValidationErrors = [];

            // Query for the most recent kind 12345 event (policy config)
            const filter = { kinds: [12345], limit: 1 };
            const events = await queryEvents(filter);

            if (events && events.length > 0) {
                policyJson = events[0].content;
                // Try to format it nicely
                try {
                    policyJson = JSON.stringify(JSON.parse(policyJson), null, 2);
                } catch (e) {
                    // Keep as-is if not valid JSON
                }
                showPolicyMessage("Policy loaded successfully", "success");
            } else {
                // No policy event found, try to load from file via API
                const response = await fetch("/api/policy", {
                    method: "GET",
                    headers: {
                        Authorization: `Nostr ${await createNIP98Auth("GET", "/api/policy")}`,
                        "Content-Type": "application/json",
                    },
                });

                if (response.ok) {
                    const data = await response.json();
                    policyJson = JSON.stringify(data, null, 2);
                    showPolicyMessage("Policy loaded from file", "success");
                } else {
                    showPolicyMessage("No policy configuration found", "info");
                    policyJson = "";
                }
            }
        } catch (error) {
            showPolicyMessage(`Error loading policy: ${error.message}`, "error");
        } finally {
            isLoadingPolicy = false;
        }
    }

    async function validatePolicy() {
        policyValidationErrors = [];

        if (!policyJson.trim()) {
            policyValidationErrors = ["Policy JSON is empty"];
            showPolicyMessage("Validation failed", "error");
            return false;
        }

        try {
            const parsed = JSON.parse(policyJson);

            // Basic structure validation
            if (typeof parsed !== "object" || parsed === null) {
                policyValidationErrors = ["Policy must be a JSON object"];
                showPolicyMessage("Validation failed", "error");
                return false;
            }

            // Validate policy_admins if present
            if (parsed.policy_admins) {
                if (!Array.isArray(parsed.policy_admins)) {
                    policyValidationErrors.push("policy_admins must be an array");
                } else {
                    for (const admin of parsed.policy_admins) {
                        if (typeof admin !== "string" || !/^[0-9a-fA-F]{64}$/.test(admin)) {
                            policyValidationErrors.push(`Invalid policy_admin pubkey: ${admin}`);
                        }
                    }
                }
            }

            // Validate rules if present
            if (parsed.rules) {
                if (typeof parsed.rules !== "object") {
                    policyValidationErrors.push("rules must be an object");
                } else {
                    for (const [kindStr, rule] of Object.entries(parsed.rules)) {
                        if (!/^\d+$/.test(kindStr)) {
                            policyValidationErrors.push(`Invalid kind number: ${kindStr}`);
                        }
                        if (rule.tag_validation && typeof rule.tag_validation === "object") {
                            for (const [tag, pattern] of Object.entries(rule.tag_validation)) {
                                try {
                                    new RegExp(pattern);
                                } catch (e) {
                                    policyValidationErrors.push(`Invalid regex for tag '${tag}': ${pattern}`);
                                }
                            }
                        }
                    }
                }
            }

            // Validate default_policy if present
            if (parsed.default_policy && !["allow", "deny"].includes(parsed.default_policy)) {
                policyValidationErrors.push("default_policy must be 'allow' or 'deny'");
            }

            if (policyValidationErrors.length > 0) {
                showPolicyMessage("Validation failed - see errors below", "error");
                return false;
            }

            showPolicyMessage("Validation passed", "success");
            return true;
        } catch (error) {
            policyValidationErrors = [`JSON parse error: ${error.message}`];
            showPolicyMessage("Invalid JSON syntax", "error");
            return false;
        }
    }

    async function savePolicy() {
        if (!isLoggedIn || (userRole !== "owner" && !isPolicyAdmin)) return;

        // Validate first
        const isValid = await validatePolicy();
        if (!isValid) return;

        try {
            isLoadingPolicy = true;

            // Create and publish kind 12345 event
            const policyEvent = {
                kind: 12345,
                created_at: Math.floor(Date.now() / 1000),
                tags: [],
                content: policyJson,
            };

            // Sign and publish the event
            const result = await publishEventWithAuth(policyEvent, userSigner);

            if (result.success) {
                showPolicyMessage("Policy updated successfully", "success");
            } else {
                showPolicyMessage(`Failed to publish policy: ${result.error || "Unknown error"}`, "error");
            }
        } catch (error) {
            showPolicyMessage(`Error saving policy: ${error.message}`, "error");
        } finally {
            isLoadingPolicy = false;
        }
    }

    function formatPolicyJson() {
        try {
            const parsed = JSON.parse(policyJson);
            policyJson = JSON.stringify(parsed, null, 2);
            showPolicyMessage("JSON formatted", "success");
        } catch (error) {
            showPolicyMessage(`Cannot format: ${error.message}`, "error");
        }
    }

    // Convert npub to hex pubkey
    function npubToHex(input) {
        if (!input) return null;

        // If already hex (64 characters)
        if (/^[0-9a-fA-F]{64}$/.test(input)) {
            return input.toLowerCase();
        }

        // If npub, decode it
        if (input.startsWith("npub1")) {
            try {
                // Bech32 decode - simplified implementation
                const ALPHABET = "qpzry9x8gf2tvdw0s3jn54khce6mua7l";
                const data = input.slice(5); // Remove "npub1" prefix

                let bits = [];
                for (const char of data) {
                    const value = ALPHABET.indexOf(char.toLowerCase());
                    if (value === -1) throw new Error("Invalid character in npub");
                    bits.push(...[...Array(5)].map((_, i) => (value >> (4 - i)) & 1));
                }

                // Remove checksum (last 30 bits = 6 characters * 5 bits)
                bits = bits.slice(0, -30);

                // Convert 5-bit groups to 8-bit bytes
                const bytes = [];
                for (let i = 0; i + 8 <= bits.length; i += 8) {
                    let byte = 0;
                    for (let j = 0; j < 8; j++) {
                        byte = (byte << 1) | bits[i + j];
                    }
                    bytes.push(byte);
                }

                // Convert to hex
                return bytes.map(b => b.toString(16).padStart(2, '0')).join('');
            } catch (e) {
                console.error("Failed to decode npub:", e);
                return null;
            }
        }

        return null;
    }

    function addPolicyAdmin(event) {
        const input = event.detail;
        if (!input) {
            showPolicyMessage("Please enter a pubkey", "error");
            return;
        }

        const hexPubkey = npubToHex(input);
        if (!hexPubkey || hexPubkey.length !== 64) {
            showPolicyMessage("Invalid pubkey format. Use hex (64 chars) or npub", "error");
            return;
        }

        try {
            const config = JSON.parse(policyJson || "{}");
            if (!config.policy_admins) {
                config.policy_admins = [];
            }

            if (config.policy_admins.includes(hexPubkey)) {
                showPolicyMessage("Admin already in list", "warning");
                return;
            }

            config.policy_admins.push(hexPubkey);
            policyJson = JSON.stringify(config, null, 2);
            showPolicyMessage("Admin added - click 'Save & Publish' to apply", "info");
        } catch (error) {
            showPolicyMessage(`Error adding admin: ${error.message}`, "error");
        }
    }

    function removePolicyAdmin(event) {
        const pubkey = event.detail;

        try {
            const config = JSON.parse(policyJson || "{}");
            if (config.policy_admins) {
                config.policy_admins = config.policy_admins.filter(p => p !== pubkey);
                policyJson = JSON.stringify(config, null, 2);
                showPolicyMessage("Admin removed - click 'Save & Publish' to apply", "info");
            }
        } catch (error) {
            showPolicyMessage(`Error removing admin: ${error.message}`, "error");
        }
    }

    async function refreshFollows() {
        if (!isLoggedIn || (userRole !== "owner" && !isPolicyAdmin)) return;

        try {
            isLoadingPolicy = true;
            policyFollows = [];

            // Parse current policy to get admin list
            let admins = [];
            try {
                const config = JSON.parse(policyJson || "{}");
                admins = config.policy_admins || [];
            } catch (e) {
                showPolicyMessage("Cannot parse policy JSON to get admins", "error");
                return;
            }

            if (admins.length === 0) {
                showPolicyMessage("No policy admins configured", "warning");
                return;
            }

            // Query kind 3 events from policy admins
            const filter = {
                kinds: [3],
                authors: admins,
                limit: admins.length
            };

            const events = await queryEvents(filter);

            // Extract p-tags from all follow lists
            const followsSet = new Set();
            for (const event of events) {
                if (event.tags) {
                    for (const tag of event.tags) {
                        if (tag[0] === 'p' && tag[1] && tag[1].length === 64) {
                            followsSet.add(tag[1]);
                        }
                    }
                }
            }

            policyFollows = Array.from(followsSet);
            showPolicyMessage(`Loaded ${policyFollows.length} follows from ${events.length} admin(s)`, "success");
        } catch (error) {
            showPolicyMessage(`Error loading follows: ${error.message}`, "error");
        } finally {
            isLoadingPolicy = false;
        }
    }

    function handleSprocketFileSelect(event) {
        sprocketUploadFile = event.target.files[0];
    }

    async function uploadSprocketScript() {
        if (!isLoggedIn || userRole !== "owner" || !sprocketUploadFile) return;

        try {
            isLoadingSprocket = true;

            // Read the file content
            const fileContent = await sprocketUploadFile.text();

            // Upload the script
            const response = await fetch("/api/sprocket/update", {
                method: "POST",
                headers: {
                    Authorization: `Nostr ${await createNIP98Auth("POST", "/api/sprocket/update")}`,
                    "Content-Type": "text/plain",
                },
                body: fileContent,
            });

            if (response.ok) {
                sprocketScript = fileContent;
                showSprocketMessage(
                    "Script uploaded and updated successfully",
                    "success",
                );
                await loadSprocketStatus();
                await loadVersions();
            } else {
                const errorText = await response.text();
                showSprocketMessage(
                    `Failed to upload script: ${errorText}`,
                    "error",
                );
            }
        } catch (error) {
            showSprocketMessage(
                `Error uploading script: ${error.message}`,
                "error",
            );
        } finally {
            isLoadingSprocket = false;
            sprocketUploadFile = null;
            // Clear the file input
            const fileInput = document.getElementById("sprocket-upload-file");
            if (fileInput) {
                fileInput.value = "";
            }
        }
    }

    const baseTabs = [
        { id: "export", icon: "📤", label: "Export" },
        { id: "import", icon: "💾", label: "Import", requiresAdmin: true },
        { id: "events", icon: "📡", label: "Events" },
        { id: "blossom", icon: "🌸", label: "Blossom" },
        { id: "compose", icon: "✏️", label: "Compose", requiresWrite: true },
        { id: "recovery", icon: "🔄", label: "Recovery" },
        {
            id: "managed-acl",
            icon: "🛡️",
            label: "Managed ACL",
            requiresOwner: true,
        },
        {
            id: "curation",
            icon: "📋",
            label: "Curation",
            requiresOwner: true,
        },
        { id: "sprocket", icon: "⚙️", label: "Sprocket", requiresOwner: true },
        { id: "policy", icon: "📜", label: "Policy", requiresOwner: true },
        { id: "relay-connect", icon: "🔗", label: "Relay Connect", requiresOwner: true },
        { id: "logs", icon: "📋", label: "Logs", requiresOwner: true },
    ];

    // Filter tabs based on current effective role (including view-as setting)
    $: filteredBaseTabs = baseTabs.filter((tab) => {
        const currentRole = currentEffectiveRole;

        if (
            tab.requiresAdmin &&
            (!isLoggedIn ||
                (currentRole !== "admin" && currentRole !== "owner"))
        ) {
            return false;
        }
        if (tab.requiresOwner && (!isLoggedIn || currentRole !== "owner")) {
            return false;
        }
        if (tab.requiresWrite && (!isLoggedIn || currentRole === "read")) {
            return false;
        }
        // Hide sprocket tab if not enabled
        if (tab.id === "sprocket" && !sprocketEnabled) {
            return false;
        }
        // Hide policy tab if not enabled
        if (tab.id === "policy" && !policyEnabled) {
            return false;
        }
        // Hide relay-connect tab if NRC is not enabled
        if (tab.id === "relay-connect" && !nrcEnabled) {
            return false;
        }
        // Hide managed ACL tab if not in managed mode
        if (tab.id === "managed-acl" && aclMode !== "managed") {
            return false;
        }
        // Hide curation tab if not in curating mode
        if (tab.id === "curation" && aclMode !== "curating") {
            return false;
        }
        // Debug logging for tab filtering
        console.log(`Tab ${tab.id} filter check:`, {
            isLoggedIn,
            userRole,
            viewAsRole,
            currentRole,
            requiresAdmin: tab.requiresAdmin,
            requiresOwner: tab.requiresOwner,
            requiresWrite: tab.requiresWrite,
            visible: true,
        });
        return true;
    });

    $: tabs = [...filteredBaseTabs, ...searchTabs];

    // Debug logging for tabs
    $: console.log("Tabs debug:", {
        isLoggedIn,
        userRole,
        aclMode,
        filteredBaseTabs: filteredBaseTabs.map((t) => t.id),
        allTabs: tabs.map((t) => t.id),
    });

    function selectTab(tabId) {
        selectedTab = tabId;

        // Load sprocket data when switching to sprocket tab
        if (
            tabId === "sprocket" &&
            isLoggedIn &&
            userRole === "owner" &&
            sprocketEnabled
        ) {
            loadSprocketStatus();
            loadVersions();
        }

        savePersistentState();
    }

    function openLoginModal() {
        if (!isLoggedIn) {
            showLoginModal = true;
        }
    }

    async function handleLogin(event) {
        const { method, pubkey, privateKey, signer } = event.detail;
        isLoggedIn = true;
        userPubkey = pubkey;
        authMethod = method;
        userSigner = signer;
        showLoginModal = false;

        // Initialize Nostr client and fetch profile
        try {
            await initializeNostrClient();

            // Set up NDK signer based on authentication method
            if (method === "extension" && signer) {
                // Extension signer (NIP-07 compatible)
                nostrClient.setSigner(signer);
            } else if (method === "nsec" && privateKey) {
                // Private key signer for nsec
                const keySigner = new PrivateKeySigner(privateKey);
                nostrClient.setSigner(keySigner);
            }

            userProfile = await fetchUserProfile(pubkey);
            console.log("Profile loaded:", userProfile);
        } catch (error) {
            console.error("Failed to load profile:", error);
        }

        // Fetch user role/permissions
        await fetchUserRole();
        await fetchACLMode();
    }

    function handleLogout() {
        isLoggedIn = false;
        userPubkey = "";
        authMethod = "";
        userProfile = null;
        userRole = "";
        userSigner = null;
        userPrivkey = null;
        showSettingsDrawer = false;

        // Clear events
        myEvents = [];
        allEvents = [];

        // Clear cache
        clearCache();

        // Clear stored authentication
        if (typeof localStorage !== "undefined") {
            localStorage.removeItem("nostr_auth_method");
            localStorage.removeItem("nostr_pubkey");
            localStorage.removeItem("nostr_privkey");
        }
    }

    function closeLoginModal() {
        showLoginModal = false;
    }

    function openSettingsDrawer() {
        showSettingsDrawer = true;
    }

    function closeSettingsDrawer() {
        showSettingsDrawer = false;
    }

    function toggleFilterBuilder() {
        showFilterBuilder = !showFilterBuilder;
    }

    function createSearchTab(filter, label) {
        const searchTabId = `search-${Date.now()}`;
        const newSearchTab = {
            id: searchTabId,
            icon: "🔍",
            label: label,
            isSearchTab: true,
            filter: filter,
        };
        searchTabs = [...searchTabs, newSearchTab];
        selectedTab = searchTabId;

        // Initialize search results for this tab
        searchResults.set(searchTabId, {
            filter: filter,
            events: [],
            isLoading: false,
            hasMore: true,
            oldestTimestamp: null,
        });

        // Start loading search results
        loadSearchResults(searchTabId, true);
    }

    function handleFilterApply(event) {
        const { searchText, selectedKinds, pubkeys, eventIds, tags, sinceTimestamp, untilTimestamp, limit } = event.detail;

        // Build the filter for the events view
        const filter = buildFilter({
            searchText,
            kinds: selectedKinds,
            authors: pubkeys,
            ids: eventIds,
            tags,
            since: sinceTimestamp,
            until: untilTimestamp,
            limit: limit || 100,
        });

        // Store the active filter and reload events with it
        eventsViewFilter = filter;
        loadAllEvents(true, null);
    }

    function handleFilterClear() {
        // Clear the filter and reload all events
        eventsViewFilter = {};
        loadAllEvents(true, null);
    }

    function handleFilterSweep(searchTabId) {
        // Close the search tab
        closeSearchTab(searchTabId);
    }

    function closeSearchTab(tabId) {
        searchTabs = searchTabs.filter((tab) => tab.id !== tabId);
        searchResults.delete(tabId); // Clean up search results
        if (selectedTab === tabId) {
            selectedTab = "export"; // Fall back to export tab
        }
    }

    async function loadSearchResults(searchTabId, reset = true) {
        const searchResult = searchResults.get(searchTabId);
        if (!searchResult || searchResult.isLoading) return;

        // Update loading state
        searchResult.isLoading = true;
        searchResults.set(searchTabId, searchResult);

        try {
            const filter = { ...searchResult.filter };
            
            // Apply timestamp-based pagination
            if (!reset && searchResult.oldestTimestamp) {
                filter.until = searchResult.oldestTimestamp;
            }
            
            // Override limit for pagination
            if (!reset) {
                filter.limit = 200;
            }

            console.log(
                "Loading search results with filter:",
                filter,
            );
            
            // Use fetchEvents with the filter array
            const events = await fetchEvents([filter], { timeout: 30000 });
            console.log("Received search results:", events.length, "events");

            if (reset) {
                searchResult.events = events.sort(
                    (a, b) => b.created_at - a.created_at,
                );
            } else {
                searchResult.events = [...searchResult.events, ...events].sort(
                    (a, b) => b.created_at - a.created_at,
                );
            }

            // Update oldest timestamp for next pagination
            if (events.length > 0) {
                const oldestInBatch = Math.min(
                    ...events.map((e) => e.created_at),
                );
                if (
                    !searchResult.oldestTimestamp ||
                    oldestInBatch < searchResult.oldestTimestamp
                ) {
                    searchResult.oldestTimestamp = oldestInBatch;
                }
            }

            searchResult.hasMore = events.length === (reset ? filter.limit || 100 : 200);
            searchResult.isLoading = false;
            searchResults.set(searchTabId, searchResult);
        } catch (error) {
            console.error("Failed to load search results:", error);
            searchResult.isLoading = false;
            searchResults.set(searchTabId, searchResult);
            alert("Failed to load search results: " + error.message);
        }
    }

    async function loadMoreSearchResults(searchTabId) {
        await loadSearchResults(searchTabId, false);
    }

    function handleSearchScroll(event, searchTabId) {
        const { scrollTop, scrollHeight, clientHeight } = event.target;
        const threshold = 100; // Load more when 100px from bottom

        if (scrollHeight - scrollTop - clientHeight < threshold) {
            const searchResult = searchResults.get(searchTabId);
            if (
                searchResult &&
                !searchResult.isLoading &&
                searchResult.hasMore
            ) {
                loadMoreSearchResults(searchTabId);
            }
        }
    }

    $: if (typeof document !== "undefined") {
        if (isDarkTheme) {
            document.body.classList.add("dark-theme");
        } else {
            document.body.classList.remove("dark-theme");
        }
    }

    // Auto-fetch profile when user is logged in but profile is missing
    $: if (isLoggedIn && userPubkey && !userProfile) {
        fetchProfileIfMissing();
    }

    async function fetchProfileIfMissing() {
        if (!isLoggedIn || !userPubkey || userProfile) {
            return; // Don't fetch if not logged in, no pubkey, or profile already exists
        }

        try {
            console.log("Auto-fetching profile for:", userPubkey);
            await initializeNostrClient();
            userProfile = await fetchUserProfile(userPubkey);
            console.log("Profile auto-loaded:", userProfile);
        } catch (error) {
            console.error("Failed to auto-load profile:", error);
        }
    }

    async function fetchUserRole() {
        if (!isLoggedIn || !userPubkey) {
            userRole = "";
            return;
        }

        try {
            const response = await fetch(`/api/permissions/${userPubkey}`);
            if (response.ok) {
                const data = await response.json();
                userRole = data.permission || "";
                console.log("User role loaded:", userRole);
                console.log("Is owner?", userRole === "owner");
            } else {
                console.error("Failed to fetch user role:", response.status);
                userRole = "";
            }
        } catch (error) {
            console.error("Error fetching user role:", error);
            userRole = "";
        }
    }

    async function fetchACLMode() {
        try {
            const response = await fetch("/api/acl-mode");
            if (response.ok) {
                const data = await response.json();
                aclMode = data.acl_mode || "";
                console.log("ACL mode loaded:", aclMode);
            } else {
                console.error("Failed to fetch ACL mode:", response.status);
                aclMode = "";
            }
        } catch (error) {
            console.error("Error fetching ACL mode:", error);
            aclMode = "";
        }
    }

    async function fetchRelayVersion() {
        try {
            const info = await api.fetchRelayInfo();
            if (info && info.version) {
                relayVersion = info.version;
            }
        } catch (error) {
            console.error("Error fetching relay version:", error);
        }
    }

    // Export functionality
    async function exportEvents(pubkeys = []) {
        // Skip login check when ACL is "none" (open relay mode)
        if (aclMode !== "none" && !isLoggedIn) {
            alert("Please log in first");
            return;
        }

        // Check permissions for exporting all events using current effective role
        // Skip permission check when ACL is "none"
        if (
            aclMode !== "none" &&
            pubkeys.length === 0 &&
            currentEffectiveRole !== "admin" &&
            currentEffectiveRole !== "owner"
        ) {
            alert("Admin or owner permission required to export all events");
            return;
        }

        try {
            // Build headers - only include auth when ACL is not "none"
            const headers = {
                "Content-Type": "application/json",
            };
            if (aclMode !== "none" && isLoggedIn) {
                headers.Authorization = await createNIP98AuthHeader(
                    "/api/export",
                    "POST",
                );
            }
            const response = await fetch("/api/export", {
                method: "POST",
                headers,
                body: JSON.stringify({ pubkeys }),
            });

            if (!response.ok) {
                throw new Error(
                    `Export failed: ${response.status} ${response.statusText}`,
                );
            }

            const blob = await response.blob();
            const url = window.URL.createObjectURL(blob);
            const a = document.createElement("a");
            a.href = url;

            // Get filename from response headers or use default
            const contentDisposition = response.headers.get(
                "Content-Disposition",
            );
            let filename = "events.jsonl";
            if (contentDisposition) {
                const filenameMatch =
                    contentDisposition.match(/filename="([^"]+)"/);
                if (filenameMatch) {
                    filename = filenameMatch[1];
                }
            }

            a.download = filename;
            document.body.appendChild(a);
            a.click();
            document.body.removeChild(a);
            window.URL.revokeObjectURL(url);
        } catch (error) {
            console.error("Export failed:", error);
            alert("Export failed: " + error.message);
        }
    }

    async function exportAllEvents() {
        await exportEvents([]); // Empty array means export all events
    }

    async function exportMyEvents() {
        await exportEvents([userPubkey]); // Export only current user's events
    }

    // Import functionality
    function handleFileSelect(event) {
        // event.detail contains the original DOM event from the child component
        selectedFile = event.detail.target.files[0];
    }

    async function importEvents() {
        // Skip login/permission check when ACL is "none" (open relay mode)
        if (aclMode !== "none" && (!isLoggedIn || (userRole !== "admin" && userRole !== "owner"))) {
            importMessage = "Admin or owner permission required";
            setTimeout(() => { importMessage = ""; }, 5000);
            return;
        }

        if (!selectedFile) {
            importMessage = "Please select a file";
            setTimeout(() => { importMessage = ""; }, 5000);
            return;
        }

        try {
            // Show uploading message
            importMessage = "Uploading...";

            // Build headers - only include auth when ACL is not "none"
            const headers = {};
            if (aclMode !== "none" && isLoggedIn) {
                headers.Authorization = await createNIP98AuthHeader(
                    "/api/import",
                    "POST",
                );
            }
            const formData = new FormData();
            formData.append("file", selectedFile);

            const response = await fetch("/api/import", {
                method: "POST",
                headers,
                body: formData,
            });

            if (!response.ok) {
                throw new Error(
                    `Import failed: ${response.status} ${response.statusText}`,
                );
            }

            const result = await response.json();
            importMessage = "Upload complete";
            selectedFile = null;
            document.getElementById("import-file").value = "";
            // Clear message after 5 seconds
            setTimeout(() => { importMessage = ""; }, 5000);
        } catch (error) {
            console.error("Import failed:", error);
            importMessage = "Import failed: " + error.message;
            setTimeout(() => { importMessage = ""; }, 5000);
        }
    }

    // Events loading functionality
    async function loadMyEvents(reset = false) {
        if (!isLoggedIn) {
            alert("Please log in first");
            return;
        }

        if (isLoadingMyEvents) return;

        // Always load fresh data when feed becomes visible (reset = true)
        // Skip cache check to ensure fresh data every time

        isLoadingMyEvents = true;

        // Reset timestamps when doing a fresh load
        if (reset) {
            oldestMyEventTimestamp = null;
            newestMyEventTimestamp = null;
        }

        try {
            // Use WebSocket REQ to fetch user events with timestamp-based pagination
            // Load 1000 events on initial load, otherwise use 200 for pagination
            const events = await fetchUserEvents(userPubkey, {
                limit: reset ? 1000 : 200,
                until: reset ? null : oldestMyEventTimestamp,
            });

            if (reset) {
                myEvents = events.sort((a, b) => b.created_at - a.created_at);
            } else {
                myEvents = [...myEvents, ...events].sort(
                    (a, b) => b.created_at - a.created_at,
                );
            }

            // Update oldest timestamp for next pagination
            if (events.length > 0) {
                const oldestInBatch = Math.min(
                    ...events.map((e) => e.created_at),
                );
                if (
                    !oldestMyEventTimestamp ||
                    oldestInBatch < oldestMyEventTimestamp
                ) {
                    oldestMyEventTimestamp = oldestInBatch;
                }
            }

            hasMoreMyEvents = events.length === (reset ? 1000 : 200);

            // Auto-load more events if content doesn't fill viewport and more events are available
            // Only do this on initial load (reset = true) to avoid interfering with scroll-based loading
            if (reset && hasMoreMyEvents) {
                setTimeout(() => {
                    // Only check viewport if we're currently on the My Events tab
                    if (selectedTab === "myevents") {
                        const eventsContainers = document.querySelectorAll(
                            ".events-view-content",
                        );
                        // The My Events container should be the first one (before All Events)
                        const myEventsContainer = eventsContainers[0];
                        if (
                            myEventsContainer &&
                            myEventsContainer.scrollHeight <=
                                myEventsContainer.clientHeight
                        ) {
                            // Content doesn't fill viewport, load more automatically
                            loadMoreMyEvents();
                        }
                    }
                }, 100); // Small delay to ensure DOM is updated
            }
        } catch (error) {
            console.error("Failed to load events:", error);
            alert("Failed to load events: " + error.message);
        } finally {
            isLoadingMyEvents = false;
        }
    }

    async function loadMoreMyEvents() {
        if (!isLoadingMyEvents && hasMoreMyEvents) {
            await loadMyEvents(false);
        }
    }

    function handleMyEventsScroll(event) {
        const { scrollTop, scrollHeight, clientHeight } = event.target;
        const threshold = 100; // Load more when 100px from bottom

        if (scrollHeight - scrollTop - clientHeight < threshold) {
            loadMoreMyEvents();
        }
    }

    async function loadAllEvents(reset = false, authors = null) {
        if (
            !isLoggedIn ||
            (userRole !== "read" &&
                userRole !== "write" &&
                userRole !== "admin" &&
                userRole !== "owner")
        ) {
            alert("Read, write, admin, or owner permission required");
            return;
        }

        if (isLoadingEvents) return;

        // Always load fresh data when feed becomes visible (reset = true)
        // Skip cache check to ensure fresh data every time

        isLoadingEvents = true;

        // Reset timestamps when doing a fresh load
        if (reset) {
            oldestEventTimestamp = null;
            newestEventTimestamp = null;
        }

        try {
            // Use Nostr WebSocket to fetch events with timestamp-based pagination
            // Load 100 events on initial load, otherwise use 200 for pagination
            console.log(
                "Loading events with authors filter:",
                authors,
                "including delete events",
            );

            // For reset, use current timestamp to get the most recent events
            const untilTimestamp = reset
                ? Math.floor(Date.now() / 1000)
                : oldestEventTimestamp;

            // Merge eventsViewFilter with pagination params
            // eventsViewFilter takes precedence for authors if set, otherwise use the authors param
            const filterAuthors = eventsViewFilter.authors || authors;
            const events = await fetchAllEvents({
                ...eventsViewFilter,
                limit: reset ? 100 : 200,
                until: eventsViewFilter.until || untilTimestamp,
                authors: filterAuthors,
            });
            console.log("Received events:", events.length, "events");
            if (authors && events.length > 0) {
                const nonUserEvents = events.filter(
                    (event) => event.pubkey && event.pubkey !== userPubkey,
                );
                if (nonUserEvents.length > 0) {
                    console.warn(
                        "Server returned non-user events:",
                        nonUserEvents.length,
                        "out of",
                        events.length,
                    );
                }
            }

            if (reset) {
                allEvents = events.sort((a, b) => b.created_at - a.created_at);
                // Update global cache
                updateGlobalCache(events);
            } else {
                allEvents = [...allEvents, ...events].sort(
                    (a, b) => b.created_at - a.created_at,
                );
                // Update global cache with all events
                updateGlobalCache(allEvents);
            }

            // Update oldest timestamp for next pagination
            if (events.length > 0) {
                const oldestInBatch = Math.min(
                    ...events.map((e) => e.created_at),
                );
                if (
                    !oldestEventTimestamp ||
                    oldestInBatch < oldestEventTimestamp
                ) {
                    oldestEventTimestamp = oldestInBatch;
                }
            }

            hasMoreEvents = events.length === (reset ? 1000 : 200);

            // Auto-load more events if content doesn't fill viewport and more events are available
            // Only do this on initial load (reset = true) to avoid interfering with scroll-based loading
            if (reset && hasMoreEvents) {
                setTimeout(() => {
                    // Only check viewport if we're currently on the All Events tab
                    if (selectedTab === "events") {
                        const eventsContainers = document.querySelectorAll(
                            ".events-view-content",
                        );
                        // The All Events container should be the first one (only container now)
                        const allEventsContainer = eventsContainers[0];
                        if (
                            allEventsContainer &&
                            allEventsContainer.scrollHeight <=
                                allEventsContainer.clientHeight
                        ) {
                            // Content doesn't fill viewport, load more automatically
                            loadMoreEvents();
                        }
                    }
                }, 100); // Small delay to ensure DOM is updated
            }
        } catch (error) {
            console.error("Failed to load events:", error);
            alert("Failed to load events: " + error.message);
        } finally {
            isLoadingEvents = false;
        }
    }

    async function loadMoreEvents() {
        await loadAllEvents(false);
    }

    function handleScroll(event) {
        const { scrollTop, scrollHeight, clientHeight } = event.target;
        const threshold = 100; // Load more when 100px from bottom

        if (scrollHeight - scrollTop - clientHeight < threshold) {
            loadMoreEvents();
        }
    }

    // Load events when events tab is selected (only if no events loaded yet)
    // Track if we've already attempted to load events to prevent infinite loops
    let hasAttemptedEventLoad = false;

    $: if (
        selectedTab === "events" &&
        isLoggedIn &&
        (userRole === "read" ||
            userRole === "write" ||
            userRole === "admin" ||
            userRole === "owner") &&
        allEvents.length === 0 &&
        !hasAttemptedEventLoad &&
        !isLoadingEvents
    ) {
        hasAttemptedEventLoad = true;
        const authors = showOnlyMyEvents && userPubkey ? [userPubkey] : null;
        loadAllEvents(true, authors);
    }

    // Reset the attempt flag when switching tabs or when showOnlyMyEvents changes
    $: if (
        selectedTab !== "events" ||
        (selectedTab === "events" && allEvents.length > 0)
    ) {
        hasAttemptedEventLoad = false;
    }

    // NIP-98 authentication helper
    async function createNIP98AuthHeader(url, method) {
        if (!isLoggedIn || !userPubkey) {
            throw new Error("Not logged in");
        }

        // Create NIP-98 auth event
        const authEvent = {
            kind: 27235,
            created_at: Math.floor(Date.now() / 1000),
            tags: [
                ["u", window.location.origin + url],
                ["method", method.toUpperCase()],
            ],
            content: "",
            pubkey: userPubkey,
        };

        let signedEvent;

        if (userSigner && authMethod === "extension") {
            // Use the signer from the extension
            try {
                signedEvent = await userSigner.signEvent(authEvent);
            } catch (error) {
                throw new Error(
                    "Failed to sign with extension: " + error.message,
                );
            }
        } else if (authMethod === "nsec") {
            // For nsec method, we need to implement proper signing
            // For now, create a mock signature (in production, use proper crypto)
            authEvent.id = "mock-id-" + Date.now();
            authEvent.sig = "mock-signature-" + Date.now();
            signedEvent = authEvent;
        } else {
            throw new Error("No valid signer available");
        }

        // Encode as base64
        const eventJson = JSON.stringify(signedEvent);
        const base64Event = btoa(eventJson);

        return `Nostr ${base64Event}`;
    }

    // NIP-98 authentication helper (for sprocket functions)
    async function createNIP98Auth(method, url) {
        if (!isLoggedIn || !userPubkey) {
            throw new Error("Not logged in");
        }

        // Create NIP-98 auth event
        const authEvent = {
            kind: 27235,
            created_at: Math.floor(Date.now() / 1000),
            tags: [
                ["u", window.location.origin + url],
                ["method", method.toUpperCase()],
            ],
            content: "",
            pubkey: userPubkey,
        };

        let signedEvent;

        if (userSigner && authMethod === "extension") {
            // Use the signer from the extension
            try {
                signedEvent = await userSigner.signEvent(authEvent);
            } catch (error) {
                throw new Error(
                    "Failed to sign with extension: " + error.message,
                );
            }
        } else if (authMethod === "nsec") {
            // For nsec method, we need to implement proper signing
            // For now, create a mock signature (in production, use proper crypto)
            authEvent.id = "mock-id-" + Date.now();
            authEvent.sig = "mock-signature-" + Date.now();
            signedEvent = authEvent;
        } else {
            throw new Error("No valid signer available");
        }

        // Encode as base64
        const eventJson = JSON.stringify(signedEvent);
        const base64Event = btoa(eventJson);

        return base64Event;
    }

    // Compose tab functions
    function reformatJson() {
        try {
            if (!composeEventJson.trim()) {
                alert("Please enter some JSON to reformat");
                return;
            }
            const parsed = JSON.parse(composeEventJson);
            composeEventJson = JSON.stringify(parsed, null, 2);
        } catch (error) {
            alert("Invalid JSON: " + error.message);
        }
    }

    async function signEvent() {
        try {
            if (!composeEventJson.trim()) {
                alert("Please enter an event to sign");
                return;
            }

            if (!isLoggedIn || !userPubkey) {
                alert("Please log in to sign events");
                return;
            }

            if (!userSigner) {
                alert(
                    "No signer available. Please log in with a valid authentication method.",
                );
                return;
            }

            const event = JSON.parse(composeEventJson);

            // Update event with current user's pubkey and timestamp
            event.pubkey = userPubkey;
            event.created_at = Math.floor(Date.now() / 1000);

            // Remove any existing id and sig to ensure fresh signing
            delete event.id;
            delete event.sig;

            // Sign the event using the real signer
            const signedEvent = await userSigner.signEvent(event);

            // Update the compose area with the signed event
            composeEventJson = JSON.stringify(signedEvent, null, 2);

            // Show success feedback
            alert("Event signed successfully!");
        } catch (error) {
            console.error("Error signing event:", error);
            alert("Error signing event: " + error.message);
        }
    }

    async function publishEvent() {
        // Clear any previous errors
        composePublishError = "";

        try {
            if (!composeEventJson.trim()) {
                composePublishError = "Please enter an event to publish";
                return;
            }

            if (!isLoggedIn) {
                composePublishError = "Please log in to publish events";
                return;
            }

            if (!userSigner) {
                composePublishError = "No signer available. Please log in with a valid authentication method.";
                return;
            }

            let event;
            try {
                event = JSON.parse(composeEventJson);
            } catch (parseError) {
                composePublishError = `Invalid JSON: ${parseError.message}`;
                return;
            }

            // Validate that the event has required fields
            if (!event.id || !event.sig) {
                composePublishError = 'Event must be signed before publishing. Please click "Sign" first.';
                return;
            }

            // Pre-check: validate user has write permission
            if (userRole === "read") {
                composePublishError = `Permission denied: Your current role is "${userRole}" which does not allow publishing events. Contact a relay administrator to upgrade your permissions.`;
                return;
            }

            // Publish to the ORLY relay using WebSocket (same address as current page)
            const wsProtocol = window.location.protocol.startsWith('https') ? 'wss:' : 'ws:';
            const relayUrl = `${wsProtocol}//${window.location.host}/`;

            // Use the authentication module to publish the event
            const result = await publishEventWithAuth(
                relayUrl,
                event,
                userSigner,
                userPubkey,
            );

            if (result.success) {
                composePublishError = "";
                alert("Event published successfully to ORLY relay!");
                // Optionally clear the editor after successful publish
                // composeEventJson = '';
            } else {
                // Parse the error reason and provide helpful guidance
                const reason = result.reason || "Unknown error";
                composePublishError = formatPublishError(reason, event.kind);
            }
        } catch (error) {
            console.error("Error publishing event:", error);
            const errorMsg = error.message || "Unknown error";
            composePublishError = formatPublishError(errorMsg, null);
        }
    }

    // Helper function to format publish errors with helpful guidance
    function formatPublishError(reason, eventKind) {
        const lowerReason = reason.toLowerCase();

        // Check for policy-related errors
        if (lowerReason.includes("policy") || lowerReason.includes("blocked") || lowerReason.includes("denied")) {
            let msg = `Policy Error: ${reason}`;
            if (eventKind !== null) {
                msg += `\n\nKind ${eventKind} may be restricted by the relay's policy configuration.`;
            }
            if (policyEnabled) {
                msg += "\n\nThe relay has policy enforcement enabled. Contact a relay administrator to allow this event kind or adjust your permissions.";
            }
            return msg;
        }

        // Check for permission/auth errors
        if (lowerReason.includes("auth") || lowerReason.includes("permission") || lowerReason.includes("unauthorized")) {
            return `Permission Error: ${reason}\n\nYour current permissions may not allow publishing this type of event. Current role: ${userRole || "unknown"}. Contact a relay administrator to upgrade your permissions.`;
        }

        // Check for kind-specific restrictions
        if (lowerReason.includes("kind") || lowerReason.includes("not allowed") || lowerReason.includes("restricted")) {
            let msg = `Event Type Error: ${reason}`;
            if (eventKind !== null) {
                msg += `\n\nKind ${eventKind} is not currently allowed on this relay.`;
            }
            msg += "\n\nThe relay administrator may need to update the policy configuration to allow this event kind.";
            return msg;
        }

        // Check for rate limiting
        if (lowerReason.includes("rate") || lowerReason.includes("limit") || lowerReason.includes("too many")) {
            return `Rate Limit Error: ${reason}\n\nPlease wait a moment before trying again.`;
        }

        // Check for size limits
        if (lowerReason.includes("size") || lowerReason.includes("too large") || lowerReason.includes("content")) {
            return `Size Limit Error: ${reason}\n\nThe event may exceed the relay's size limits. Try reducing the content length.`;
        }

        // Default error message
        return `Publishing failed: ${reason}`;
    }

    // Clear the compose publish error
    function clearComposeError() {
        composePublishError = "";
    }

    // Persist selected tab to local storage
    $: {
        localStorage.setItem("selectedTab", selectedTab);
    }

    // Handle permission view switching
    function setViewAsRole(role) {
        viewAsRole = role;
        localStorage.setItem("viewAsRole", role);
        console.log(
            "View as role changed to:",
            role,
            "Current effective role:",
            currentEffectiveRole,
        );
    }

    // Reactive statement for current effective role
    $: currentEffectiveRole =
        viewAsRole && viewAsRole !== "" ? viewAsRole : userRole;

    // Initialize viewAsRole from local storage if available
    viewAsRole = localStorage.getItem("viewAsRole") || "";

    // Get available roles based on user's actual role
    function getAvailableRoles() {
        const allRoles = ["owner", "admin", "write", "read"];
        const userRoleIndex = allRoles.indexOf(userRole);
        if (userRoleIndex === -1) return ["read"]; // Default to read if role not found
        return allRoles.slice(userRoleIndex); // Return current role and all lower roles
    }
</script>

<!-- Header -->
<Header
    {isDarkTheme}
    {isLoggedIn}
    {userRole}
    {currentEffectiveRole}
    {userProfile}
    {userPubkey}
    on:openSettingsDrawer={openSettingsDrawer}
    on:openLoginModal={openLoginModal}
/>

<!-- Main Content Area -->
<div class="app-container" class:dark-theme={isDarkTheme}>
    <!-- Sidebar -->
    <Sidebar
        {isDarkTheme}
        {tabs}
        {selectedTab}
        version={relayVersion}
        on:selectTab={(e) => selectTab(e.detail)}
        on:closeSearchTab={(e) => closeSearchTab(e.detail)}
    />

    <!-- Main Content -->
    <main class="main-content">
        {#if selectedTab === "export"}
            <ExportView
                {isLoggedIn}
                {currentEffectiveRole}
                {aclMode}
                on:exportMyEvents={exportMyEvents}
                on:exportAllEvents={exportAllEvents}
                on:openLoginModal={openLoginModal}
            />
        {:else if selectedTab === "import"}
            <ImportView
                {isLoggedIn}
                {currentEffectiveRole}
                {selectedFile}
                {aclMode}
                {importMessage}
                on:fileSelect={handleFileSelect}
                on:importEvents={importEvents}
                on:openLoginModal={openLoginModal}
            />
        {:else if selectedTab === "events"}
            <EventsView
                {isLoggedIn}
                {userRole}
                {userPubkey}
                {filteredEvents}
                {expandedEvents}
                {isLoadingEvents}
                {showOnlyMyEvents}
                {showFilterBuilder}
                on:scroll={handleScroll}
                on:toggleEventExpansion={(e) => toggleEventExpansion(e.detail)}
                on:deleteEvent={(e) => deleteEvent(e.detail)}
                on:copyEventToClipboard={(e) =>
                    copyEventToClipboard(e.detail.event, e.detail.e)}
                on:toggleChange={handleToggleChange}
                on:loadAllEvents={(e) =>
                    loadAllEvents(e.detail.refresh, e.detail.authors)}
                on:toggleFilterBuilder={toggleFilterBuilder}
                on:filterApply={handleFilterApply}
                on:filterClear={handleFilterClear}
            />
        {:else if selectedTab === "blossom"}
            <BlossomView
                {isLoggedIn}
                {userPubkey}
                {userSigner}
                {currentEffectiveRole}
                on:openLoginModal={openLoginModal}
            />
        {:else if selectedTab === "compose"}
            <ComposeView
                bind:composeEventJson
                {userPubkey}
                {userRole}
                {policyEnabled}
                publishError={composePublishError}
                on:reformatJson={reformatJson}
                on:signEvent={signEvent}
                on:publishEvent={publishEvent}
                on:clearError={clearComposeError}
            />
        {:else if selectedTab === "managed-acl"}
            <div class="managed-acl-view">
                {#if aclMode !== "managed"}
                    <div class="acl-mode-warning">
                        <h3>⚠️ Managed ACL Mode Not Active</h3>
                        <p>
                            To use the Managed ACL interface, you need to set
                            the ACL mode to "managed" in your relay
                            configuration.
                        </p>
                        <p>
                            Current ACL mode: <strong
                                >{aclMode || "unknown"}</strong
                            >
                        </p>
                        <p>
                            Please set <code>ORLY_ACL_MODE=managed</code> in your
                            environment variables and restart the relay.
                        </p>
                    </div>
                {:else if isLoggedIn && userRole === "owner"}
                    <ManagedACL {userSigner} {userPubkey} />
                {:else}
                    <div class="access-denied">
                        <p>
                            Please log in with owner permissions to access
                            managed ACL configuration.
                        </p>
                        <button class="login-btn" on:click={openLoginModal}
                            >Log In</button
                        >
                    </div>
                {/if}
            </div>
        {:else if selectedTab === "curation"}
            <div class="curation-view-container">
                {#if aclMode !== "curating"}
                    <div class="acl-mode-warning">
                        <h3>Curating Mode Not Active</h3>
                        <p>
                            To use the Curation interface, you need to set
                            the ACL mode to "curating" in your relay
                            configuration.
                        </p>
                        <p>
                            Current ACL mode: <strong
                                >{aclMode || "unknown"}</strong
                            >
                        </p>
                        <p>
                            Please set <code>ORLY_ACL_MODE=curating</code> in your
                            environment variables and restart the relay.
                        </p>
                    </div>
                {:else if isLoggedIn && userRole === "owner"}
                    <CurationView {userSigner} {userPubkey} />
                {:else}
                    <div class="access-denied">
                        <p>
                            Please log in with owner permissions to access
                            curation configuration.
                        </p>
                        <button class="login-btn" on:click={openLoginModal}
                            >Log In</button
                        >
                    </div>
                {/if}
            </div>
        {:else if selectedTab === "sprocket"}
            <SprocketView
                {isLoggedIn}
                {userRole}
                {sprocketStatus}
                {isLoadingSprocket}
                {sprocketUploadFile}
                bind:sprocketScript
                {sprocketMessage}
                {sprocketMessageType}
                {sprocketVersions}
                on:restartSprocket={restartSprocket}
                on:deleteSprocket={deleteSprocket}
                on:sprocketFileSelect={handleSprocketFileSelect}
                on:uploadSprocketScript={uploadSprocketScript}
                on:saveSprocket={saveSprocket}
                on:loadSprocket={loadSprocket}
                on:loadVersions={loadVersions}
                on:loadVersion={(e) => loadVersion(e.detail)}
                on:deleteVersion={(e) => deleteVersion(e.detail)}
                on:openLoginModal={openLoginModal}
            />
        {:else if selectedTab === "policy"}
            <PolicyView
                {isLoggedIn}
                {userRole}
                {isPolicyAdmin}
                {policyEnabled}
                bind:policyJson
                {isLoadingPolicy}
                {policyMessage}
                {policyMessageType}
                validationErrors={policyValidationErrors}
                {policyFollows}
                on:loadPolicy={loadPolicy}
                on:validatePolicy={validatePolicy}
                on:savePolicy={savePolicy}
                on:formatJson={formatPolicyJson}
                on:addPolicyAdmin={addPolicyAdmin}
                on:removePolicyAdmin={removePolicyAdmin}
                on:refreshFollows={refreshFollows}
                on:openLoginModal={openLoginModal}
            />
        {:else if selectedTab === "relay-connect"}
            <RelayConnectView
                {isLoggedIn}
                {userRole}
                {userSigner}
                {userPubkey}
                on:openLoginModal={openLoginModal}
            />
        {:else if selectedTab === "logs"}
            <LogView
                {isLoggedIn}
                {userRole}
                {userSigner}
                on:openLoginModal={openLoginModal}
            />
        {:else if selectedTab === "recovery"}
            <div class="recovery-tab">
                <div>
                    <h3>Event Recovery</h3>
                    <p>Search and recover old versions of replaceable events</p>
                </div>

                <div class="recovery-controls-card">
                    <div class="recovery-controls">
                        <div class="kind-selector">
                            <label for="recovery-kind">Select Event Kind:</label
                            >
                            <select
                                id="recovery-kind"
                                bind:value={recoverySelectedKind}
                                on:change={selectRecoveryKind}
                            >
                                <option value={null}
                                    >Choose a replaceable kind...</option
                                >
                                {#each replaceableKinds as kind}
                                    <option value={kind.value}
                                        >{kind.label}</option
                                    >
                                {/each}
                            </select>
                        </div>

                        <div class="custom-kind-input">
                            <label for="custom-kind"
                                >Or enter custom kind number:</label
                            >
                            <input
                                id="custom-kind"
                                type="number"
                                bind:value={recoveryCustomKind}
                                on:input={handleCustomKindInput}
                                placeholder="e.g., 10001"
                                min="0"
                            />
                        </div>
                    </div>
                </div>

                {#if (recoverySelectedKind !== null && recoverySelectedKind !== undefined && recoverySelectedKind >= 0) || (recoveryCustomKind !== "" && parseInt(recoveryCustomKind) >= 0)}
                    <div class="recovery-results">
                        {#if isLoadingRecovery}
                            <div class="loading">Loading events...</div>
                        {:else if recoveryEvents.length === 0}
                            <div class="no-events">
                                No events found for this kind
                            </div>
                        {:else}
                            <div class="events-list">
                                {#each recoveryEvents as event}
                                    {@const isCurrent = isCurrentVersion(event)}
                                    <div
                                        class="event-item"
                                        class:old-version={!isCurrent}
                                    >
                                        <div class="event-header">
                                            <div class="event-header-left">
                                                <span class="event-kind">
                                                    {#if isCurrent}
                                                        Current Version{/if}</span
                                                >
                                                <span class="event-timestamp">
                                                    {new Date(
                                                        event.created_at * 1000,
                                                    ).toLocaleString()}
                                                </span>
                                            </div>
                                            <div class="event-header-actions">
                                                {#if !isCurrent}
                                                    <button
                                                        class="repost-all-button"
                                                        on:click={() =>
                                                            repostEventToAll(
                                                                event,
                                                            )}
                                                    >
                                                        🌐 Repost to All
                                                    </button>
                                                    {#if currentEffectiveRole !== "read"}
                                                        <button
                                                            class="repost-button"
                                                            on:click={() =>
                                                                repostEvent(
                                                                    event,
                                                                )}
                                                        >
                                                            🔄 Repost
                                                        </button>
                                                    {/if}
                                                {/if}
                                                <button
                                                    class="copy-json-btn"
                                                    on:click|stopPropagation={(
                                                        e,
                                                    ) =>
                                                        copyEventToClipboard(
                                                            event,
                                                            e,
                                                        )}
                                                >
                                                    📋 Copy JSON
                                                </button>
                                            </div>
                                        </div>

                                        <div class="event-content">
                                            <pre
                                                class="event-json">{JSON.stringify(
                                                    event,
                                                    null,
                                                    2,
                                                )}</pre>
                                        </div>
                                    </div>
                                {/each}
                            </div>

                            {#if recoveryHasMore}
                                <button
                                    class="load-more"
                                    on:click={loadRecoveryEvents}
                                    disabled={isLoadingRecovery}
                                >
                                    Load More Events
                                </button>
                            {/if}
                        {/if}
                    </div>
                {/if}
            </div>
        {:else if searchTabs.some((tab) => tab.id === selectedTab)}
            {#each searchTabs as searchTab}
                {#if searchTab.id === selectedTab}
                    <div class="search-results-view">
                        <div class="search-results-header">
                            <h2>🔍 {searchTab.label}</h2>
                            <button
                                class="refresh-btn"
                                on:click={() =>
                                    loadSearchResults(
                                        searchTab.id,
                                        true,
                                    )}
                                disabled={searchResults.get(searchTab.id)
                                    ?.isLoading}
                            >
                                🔄 Refresh
                            </button>
                        </div>
                        
                        <!-- FilterDisplay - show active filter -->
                        <FilterDisplay
                            filter={searchResults.get(searchTab.id)?.filter || {}}
                            on:sweep={() => handleFilterSweep(searchTab.id)}
                        />
                        
                        <div
                            class="search-results-content"
                            on:scroll={(e) =>
                                handleSearchScroll(e, searchTab.id)}
                        >
                            {#if searchResults.get(searchTab.id)?.events?.length > 0}
                                {#each searchResults.get(searchTab.id).events as event}
                                    <div
                                        class="search-result-item"
                                        class:expanded={expandedEvents.has(
                                            event.id,
                                        )}
                                    >
                                        <div
                                            class="search-result-row"
                                            on:click={() =>
                                                toggleEventExpansion(event.id)}
                                            on:keydown={(e) =>
                                                e.key === "Enter" &&
                                                toggleEventExpansion(event.id)}
                                            role="button"
                                            tabindex="0"
                                        >
                                            <div class="search-result-avatar">
                                                <div class="avatar-placeholder">
                                                    👤
                                                </div>
                                            </div>
                                            <div class="search-result-info">
                                                <div
                                                    class="search-result-author"
                                                >
                                                    {truncatePubkey(
                                                        event.pubkey,
                                                    )}
                                                </div>
                                                <div class="search-result-kind">
                                                    <span class="kind-number"
                                                        >{event.kind}</span
                                                    >
                                                    <span class="kind-name"
                                                        >{getKindName(
                                                            event.kind,
                                                        )}</span
                                                    >
                                                </div>
                                            </div>
                                            <div class="search-result-content">
                                                <div class="event-timestamp">
                                                    {formatTimestamp(
                                                        event.created_at,
                                                    )}
                                                </div>
                                                <div
                                                    class="event-content-single-line"
                                                >
                                                    {truncateContent(
                                                        event.content,
                                                    )}
                                                </div>
                                            </div>
                                            {#if event.kind !== 5 && (userRole === "admin" || userRole === "owner" || (userRole === "write" && event.pubkey && event.pubkey === userPubkey))}
                                                <button
                                                    class="delete-btn"
                                                    on:click|stopPropagation={() =>
                                                        deleteEvent(event.id)}
                                                >
                                                    🗑️
                                                </button>
                                            {/if}
                                        </div>
                                        {#if expandedEvents.has(event.id)}
                                            <div class="search-result-details">
                                                <div class="json-container">
                                                    <pre
                                                        class="event-json">{JSON.stringify(
                                                            event,
                                                            null,
                                                            2,
                                                        )}</pre>
                                                    <button
                                                        class="copy-json-btn"
                                                        on:click|stopPropagation={(
                                                            e,
                                                        ) =>
                                                            copyEventToClipboard(
                                                                event,
                                                                e,
                                                            )}
                                                        title="Copy minified JSON to clipboard"
                                                    >
                                                        📋
                                                    </button>
                                                </div>
                                            </div>
                                        {/if}
                                    </div>
                                {/each}
                            {:else if !searchResults.get(searchTab.id)?.isLoading}
                                <div class="no-search-results">
                                    <p>
                                        No search results found.
                                    </p>
                                </div>
                            {/if}

                            {#if searchResults.get(searchTab.id)?.isLoading}
                                <div class="loading-search-results">
                                    <div class="loading-spinner"></div>
                                    <p>Searching...</p>
                                </div>
                            {/if}

                            {#if !searchResults.get(searchTab.id)?.hasMore && searchResults.get(searchTab.id)?.events?.length > 0}
                                <div class="end-of-search-results">
                                    <p>No more search results to load.</p>
                                </div>
                            {/if}
                        </div>
                    </div>
                {/if}
            {/each}
        {:else}
            <div class="welcome-message">
                {#if isLoggedIn}
                    <p>
                        Welcome {userProfile?.name ||
                            userPubkey.slice(0, 8) + "..."}
                    </p>
                {:else}
                    <p>Log in to access your user dashboard</p>
                {/if}
            </div>
        {/if}
    </main>
</div>

<!-- Settings Drawer -->
{#if showSettingsDrawer}
    <div
        class="drawer-overlay"
        on:click={closeSettingsDrawer}
        on:keydown={(e) => e.key === "Escape" && closeSettingsDrawer()}
        role="button"
        tabindex="0"
    >
        <div
            class="settings-drawer"
            class:dark-theme={isDarkTheme}
            on:click|stopPropagation
            on:keydown|stopPropagation
        >
            <div class="drawer-header">
                <h2>Settings</h2>
                <button class="close-btn" on:click={closeSettingsDrawer}
                    >✕</button
                >
            </div>
            <div class="drawer-content">
                {#if userProfile}
                    <div class="profile-section">
                        <div class="profile-hero">
                            {#if userProfile.banner}
                                <img
                                    src={userProfile.banner}
                                    alt="Profile banner"
                                    class="profile-banner"
                                />
                            {/if}
                            <!-- Logout button floating in top-right corner of banner -->
                            <button
                                class="logout-btn floating"
                                on:click={handleLogout}>Log out</button
                            >
                            <!-- Avatar overlaps the bottom edge of the banner by 50% -->
                            {#if userProfile.picture}
                                <img
                                    src={userProfile.picture}
                                    alt="User avatar"
                                    class="profile-avatar overlap"
                                />
                            {:else}
                                <div class="profile-avatar-placeholder overlap">
                                    👤
                                </div>
                            {/if}
                            <!-- Username and nip05 to the right of the avatar, above the bottom edge -->
                            <div class="name-row">
                                <h3 class="profile-username">
                                    {userProfile.name || "Unknown User"}
                                </h3>
                                {#if userProfile.nip05}
                                    <span class="profile-nip05-inline"
                                        >{userProfile.nip05}</span
                                    >
                                {/if}
                            </div>
                        </div>

                        <!-- About text in a box underneath, with avatar overlapping its top edge -->
                        {#if userProfile.about}
                            <div class="about-card">
                                <p class="profile-about">{@html aboutHtml}</p>
                            </div>
                        {/if}
                    </div>

                    <!-- View as section -->
                    {#if userRole && userRole !== "read"}
                        <div class="view-as-section">
                            <h3>View as Role</h3>
                            <p>
                                See the interface as it appears for different
                                permission levels:
                            </p>
                            <div class="radio-group">
                                {#each getAvailableRoles() as role}
                                    <label class="radio-label">
                                        <input
                                            type="radio"
                                            name="viewAsRole"
                                            value={role}
                                            checked={currentEffectiveRole ===
                                                role}
                                            on:change={() =>
                                                setViewAsRole(
                                                    role === userRole
                                                        ? ""
                                                        : role,
                                                )}
                                        />
                                        {role.charAt(0).toUpperCase() +
                                            role.slice(1)}{role === userRole
                                            ? " (Default)"
                                            : ""}
                                    </label>
                                {/each}
                            </div>
                        </div>
                    {/if}
                {:else if isLoggedIn && userPubkey}
                    <div class="profile-loading-section">
                        <!-- Logout button in top-right corner -->
                        <button
                            class="logout-btn floating"
                            on:click={handleLogout}>Log out</button
                        >
                        <h3>Profile Loading</h3>
                        <p>Your profile metadata is being loaded...</p>
                        <button
                            class="retry-profile-btn"
                            on:click={fetchProfileIfMissing}
                        >
                            Retry Loading Profile
                        </button>
                        <div class="user-pubkey-display">
                            <strong>Public Key:</strong>
                            {userPubkey.slice(0, 16)}...{userPubkey.slice(-8)}
                        </div>
                    </div>
                {/if}
                <!-- Additional settings can be added here -->
            </div>
        </div>
    </div>
{/if}

<!-- Login Modal -->
<LoginModal
    bind:showModal={showLoginModal}
    {isDarkTheme}
    on:login={handleLogin}
    on:close={closeLoginModal}
/>

<style>
    :global(html),
    :global(body) {
        margin: 0;
        padding: 0;
        overflow: hidden;
        height: 100%;
        /* Base colors */
        --bg-color: #ddd;
        --header-bg: #eee;
        --sidebar-bg: #eee;
        --card-bg: #f8f9fa;
        --panel-bg: #f8f9fa;
        --border-color: #dee2e6;
        --text-color: #444444;
        --text-muted: #6c757d;
        --input-border: #ccc;
        --input-bg: #ffffff;
        --input-text-color: #495057;
        --button-bg: #ddd;
        --button-hover-bg: #eee;
        --button-text: #444444;
        --button-hover-border: #adb5bd;

        /* Theme colors */
        --primary: #00bcd4;
        --primary-bg: rgba(0, 188, 212, 0.1);
        --secondary: #6c757d;
        --success: #28a745;
        --success-bg: #d4edda;
        --success-text: #155724;
        --info: #17a2b8;
        --warning: #ff3e00;
        --warning-bg: #fff3cd;
        --danger: #dc3545;
        --danger-bg: #f8d7da;
        --danger-text: #721c24;
        --error-bg: #f8d7da;
        --error-text: #721c24;

        /* Code colors */
        --code-bg: #f8f9fa;
        --code-text: #495057;

        /* Tab colors */
        --tab-inactive-bg: #bbb;

        /* Accent colors */
        --accent-color: #007bff;
        --accent-hover-color: #0056b3;
    }

    :global(body.dark-theme) {
        /* Base colors */
        --bg-color: #263238;
        --header-bg: #1e272c;
        --sidebar-bg: #1e272c;
        --card-bg: #37474f;
        --panel-bg: #37474f;
        --border-color: #404040;
        --text-color: #ffffff;
        --text-muted: #adb5bd;
        --input-border: #555;
        --input-bg: #37474f;
        --input-text-color: #ffffff;
        --button-bg: #263238;
        --button-hover-bg: #1e272c;
        --button-text: #ffffff;
        --button-hover-border: #6c757d;

        /* Theme colors */
        --primary: #00bcd4;
        --primary-bg: rgba(0, 188, 212, 0.2);
        --secondary: #6c757d;
        --success: #28a745;
        --success-bg: #1e4620;
        --success-text: #d4edda;
        --info: #17a2b8;
        --warning: #ff3e00;
        --warning-bg: #4d1f00;
        --danger: #dc3545;
        --danger-bg: #4d1319;
        --danger-text: #f8d7da;
        --error-bg: #4d1319;
        --error-text: #f8d7da;

        /* Code colors */
        --code-bg: #1e272c;
        --code-text: #ffffff;

        /* Tab colors */
        --tab-inactive-bg: #1a1a1a;

        /* Accent colors */
        --accent-color: #007bff;
        --accent-hover-color: #0056b3;
    }

    .login-btn {
        padding: 0.5em 1em;
        border: none;
        border-radius: 6px;
        background-color: #4caf50;
        color: var(--text-color);
        cursor: pointer;
        font-size: 1rem;
        font-weight: 500;
        transition: background-color 0.2s;
        display: inline-flex;
        align-items: center;
        justify-content: center;
        vertical-align: middle;
        margin: 0 auto;
        padding: 0.5em 1em;
    }

    .login-btn:hover {
        background-color: #45a049;
    }

    .acl-mode-warning {
        padding: 1em;
        background-color: #fff3cd;
        border: 1px solid #ffeaa7;
        border-radius: 8px;
        color: #856404;
        margin: 20px 0;
    }

    .acl-mode-warning h3 {
        margin: 0 0 15px 0;
        color: #856404;
    }

    .acl-mode-warning p {
        margin: 10px 0;
        line-height: 1.5;
    }

    .acl-mode-warning code {
        background-color: #f8f9fa;
        padding: 2px 6px;
        border-radius: 4px;
        font-family: monospace;
        color: #495057;
    }

    /* App Container */
    .app-container {
        display: flex;
        margin-top: 3em;
        height: calc(100vh - 3em);
    }

    /* Main Content */
    .main-content {
        position: fixed;
        left: 200px;
        top: 3em;
        right: 0;
        bottom: 0;
        padding: 0;
        overflow-y: auto;
        background-color: var(--bg-color);
        color: var(--text-color);
        display: flex;
        align-items: flex-start;
        justify-content: flex-start;
        flex-direction: column;
        display: flex;
    }

    .welcome-message {
        text-align: center;
    }

    .welcome-message p {
        font-size: 1.2rem;
    }

    @media (max-width: 640px) {
        .main-content {
            left: 160px;
            padding: 1rem;
        }
    }

    .logout-btn {
        padding: 0.5rem 1rem;
        border: none;
        border-radius: 6px;
        background-color: var(--warning);
        color: var(--text-color);
        cursor: pointer;
        font-size: 1rem;
        font-weight: 500;
        transition: background-color 0.2s;
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 0.5rem;
    }

    .logout-btn:hover {
        background-color: #e53935;
    }

    .logout-btn.floating {
        position: absolute;
        top: 0.5em;
        right: 0.5em;
        z-index: 10;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.3);
    }

    /* Settings Drawer */
    .drawer-overlay {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background: rgba(0, 0, 0, 0.5);
        z-index: 1000;
        display: flex;
        justify-content: flex-end;
    }

    .settings-drawer {
        width: 640px;
        height: 100%;
        background: var(--bg-color);
        /*border-left: 1px solid var(--border-color);*/
        overflow-y: auto;
        animation: slideIn 0.3s ease;
    }

    @keyframes slideIn {
        from {
            transform: translateX(100%);
        }
        to {
            transform: translateX(0);
        }
    }

    .drawer-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        background: var(--header-bg);
    }

    .drawer-header h2 {
        margin: 0;
        color: var(--text-color);
        font-size: 1em;
        padding: 1rem;
    }

    .close-btn {
        background: none;
        border: none;
        font-size: 1em;
        cursor: pointer;
        color: var(--text-color);
        padding: 0.5em;
        transition: background-color 0.2s;
        align-items: center;
    }

    .close-btn:hover {
        background: var(--button-hover-bg);
    }

    .profile-section {
        margin-bottom: 2rem;
    }

    .profile-hero {
        position: relative;
    }

    .profile-banner {
        width: 100%;
        height: 160px;
        object-fit: cover;
        border-radius: 0;
        display: block;
    }

    /* Avatar sits half over the bottom edge of the banner */
    .profile-avatar,
    .profile-avatar-placeholder {
        width: 72px;
        height: 72px;
        border-radius: 50%;
        object-fit: cover;
        flex-shrink: 0;
        box-shadow: 0 2px 8px rgba(0, 0, 0, 0.25);
        border: 2px solid var(--bg-color);
    }

    .overlap {
        position: absolute;
        left: 12px;
        bottom: -36px; /* half out of the banner */
        z-index: 2;
        background: var(--button-hover-bg);
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 1.5rem;
    }

    /* Username and nip05 on the banner, to the right of avatar */
    .name-row {
        position: absolute;
        left: calc(12px + 72px + 12px);
        bottom: 8px;
        right: 12px;
        display: flex;
        align-items: baseline;
        gap: 8px;
        z-index: 1;
        background: var(--bg-color);
        padding: 0.2em 0.5em;
        border-radius: 0.5em;
        width: fit-content;
    }

    .profile-username {
        margin: 0;
        font-size: 1.1rem;
        color: var(--text-color);
    }

    .profile-nip05-inline {
        font-size: 0.85rem;
        color: var(--text-color);
        font-family: monospace;
        opacity: 0.95;
    }

    /* About box below with overlap space for avatar */
    .about-card {
        background: var(--header-bg);
        padding: 12px 12px 12px 96px; /* offset text from overlapping avatar */
        position: relative;
        word-break: auto-phrase;
    }

    .profile-about {
        margin: 0;
        color: var(--text-color);
        font-size: 0.9rem;
        line-height: 1.4;
    }

    .profile-loading-section {
        padding: 1rem;
        text-align: center;
        position: relative; /* Allow absolute positioning of floating logout button */
    }

    .profile-loading-section h3 {
        margin: 0 0 1rem 0;
        color: var(--text-color);
        font-size: 1.1rem;
    }

    .profile-loading-section p {
        margin: 0 0 1rem 0;
        color: var(--text-color);
        opacity: 0.8;
    }

    .retry-profile-btn {
        padding: 0.5rem 1rem;
        background: var(--primary);
        color: var(--text-color);
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9rem;
        margin-bottom: 1rem;
        transition: background-color 0.2s;
    }

    .retry-profile-btn:hover {
        background: #00acc1;
    }

    .user-pubkey-display {
        font-family: monospace;
        font-size: 0.8rem;
        color: var(--text-color);
        opacity: 0.7;
        background: var(--button-bg);
        padding: 0.5rem;
        border-radius: 4px;
        word-break: break-all;
    }

    /* Managed ACL View */
    .managed-acl-view {
        padding: 20px;
        max-width: 1200px;
        margin: 0;
        background: var(--header-bg);
        color: var(--text-color);
        border-radius: 8px;
    }

    .refresh-btn {
        padding: 0.5rem 1rem;
        background: var(--primary);
        color: var(--text-color);
        border: none;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.875rem;
        font-weight: 500;
        transition: background-color 0.2s;
        display: inline-flex;
        align-items: center;
        gap: 0.25rem;
        height: 2em;
        margin: 1em;
    }

    .refresh-btn:hover {
        background: #00acc1;
    }

    @keyframes spin {
        0% {
            transform: rotate(0deg);
        }
        100% {
            transform: rotate(360deg);
        }
    }

    /* View as Section */
    .view-as-section {
        color: var(--text-color);
        padding: 1rem;
        border-radius: 0.5em;
        margin-bottom: 1rem;
    }

    .view-as-section h3 {
        margin-top: 0;
        margin-bottom: 0.5rem;
        font-size: 1rem;
        color: var(--primary);
    }

    .view-as-section p {
        margin: 0.5rem 0;
        font-size: 0.9rem;
        opacity: 0.8;
    }

    .radio-group {
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
    }

    .radio-label {
        display: flex;
        align-items: center;
        gap: 0.5rem;
        cursor: pointer;
        padding: 0.25rem;
        border-radius: 0.5em;
        transition: background 0.2s;
    }

    .radio-label:hover {
        background: rgba(255, 255, 255, 0.1);
    }

    .radio-label input {
        margin: 0;
    }

    .avatar-placeholder {
        width: 1.5rem;
        height: 1.5rem;
        border-radius: 50%;
        background: var(--button-bg);
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 0.7rem;
    }

    .kind-number {
        background: var(--primary);
        color: var(--text-color);
        padding: 0.125rem 0.375rem;
        border-radius: 0.5em;
        font-size: 0.7rem;
        font-weight: 500;
        font-family: monospace;
    }

    .kind-name {
        font-size: 0.75rem;
        color: var(--text-color);
        opacity: 0.7;
        font-weight: 500;
    }

    .event-timestamp {
        font-size: 0.75rem;
        color: var(--text-color);
        opacity: 0.7;
        margin-bottom: 0.25rem;
        font-weight: 500;
    }

    .event-content-single-line {
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        line-height: 1.2;
    }

    .delete-btn {
        flex-shrink: 0;
        background: none;
        border: none;
        cursor: pointer;
        padding: 0.2rem;
        border-radius: 0.5em;
        transition: background-color 0.2s;
        font-size: 1.6rem;
        display: flex;
        align-items: center;
        justify-content: center;
        width: 1.5rem;
        height: 1.5rem;
    }

    .delete-btn:hover {
        background: var(--warning);
        color: var(--text-color);
    }

    .json-container {
        position: relative;
    }

    .copy-json-btn {
        color: var(--text-color);
        background: var(--accent-color);
        border: 0;
        border-radius: 0.5rem;
        padding: 0.5rem;
        font-size: 1rem;
        cursor: pointer;
        width: auto;
        height: auto;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .copy-json-btn:hover {
        background: var(--accent-hover-color);
    }

    .event-json {
        background: var(--bg-color);
        padding: 1rem;
        margin: 0;
        font-family: "Courier New", monospace;
        font-size: 0.8rem;
        line-height: 1.4;
        color: var(--text-color);
        white-space: pre-wrap;
        word-break: break-word;
        overflow-x: auto;
    }

    .no-events {
        padding: 2rem;
        text-align: center;
        color: var(--text-color);
        opacity: 0.7;
    }

    .loading-spinner {
        width: 2rem;
        height: 2rem;
        border: 3px solid var(--border-color);
        border-top: 3px solid var(--primary);
        border-radius: 50%;
        animation: spin 1s linear infinite;
        margin: 0 auto 1rem auto;
    }

    @keyframes spin {
        0% {
            transform: rotate(0deg);
        }
        100% {
            transform: rotate(360deg);
        }
    }

    /* Search Results Styles */
    .search-results-view {
        position: fixed;
        top: 3em;
        left: 200px;
        right: 0;
        bottom: 0;
        background: var(--bg-color);
        color: var(--text-color);
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    .search-results-header {
        padding: 0.5rem 1rem;
        background: var(--header-bg);
        border-bottom: 1px solid var(--border-color);
        flex-shrink: 0;
        display: flex;
        justify-content: space-between;
        align-items: center;
        height: 2.5em;
    }

    .search-results-header h2 {
        margin: 0;
        font-size: 1rem;
        font-weight: 600;
        color: var(--text-color);
    }

    .search-results-content {
        flex: 1;
        overflow-y: auto;
        padding: 0;
    }

    .search-result-item {
        border-bottom: 1px solid var(--border-color);
        transition: background-color 0.2s;
    }

    .search-result-item:hover {
        background: var(--button-hover-bg);
    }

    .search-result-item.expanded {
        background: var(--button-hover-bg);
    }

    .search-result-row {
        display: flex;
        align-items: center;
        padding: 0.75rem 1rem;
        cursor: pointer;
        gap: 0.75rem;
        min-height: 3rem;
    }

    .search-result-avatar {
        flex-shrink: 0;
        width: 2rem;
        height: 2rem;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .search-result-info {
        flex-shrink: 0;
        width: 12rem;
        display: flex;
        flex-direction: column;
        gap: 0.25rem;
    }

    .search-result-author {
        font-family: monospace;
        font-size: 0.8rem;
        color: var(--text-color);
        opacity: 0.8;
    }

    .search-result-kind {
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }

    .search-result-content {
        flex: 1;
        color: var(--text-color);
        font-size: 0.9rem;
        line-height: 1.3;
        word-break: break-word;
    }

    .search-result-content .event-timestamp {
        font-size: 0.75rem;
        color: var(--text-color);
        opacity: 0.7;
        margin-bottom: 0.25rem;
        font-weight: 500;
    }

    .search-result-content .event-content-single-line {
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        line-height: 1.2;
    }

    .search-result-details {
        border-top: 1px solid var(--border-color);
        background: var(--header-bg);
        padding: 1rem;
    }

    .no-search-results {
        padding: 2rem;
        text-align: center;
        color: var(--text-color);
        opacity: 0.7;
    }

    .no-search-results p {
        margin: 0;
        font-size: 1rem;
    }

    .loading-search-results {
        padding: 2rem;
        text-align: center;
        color: var(--text-color);
        opacity: 0.7;
    }

    .loading-search-results p {
        margin: 0;
        font-size: 0.9rem;
    }

    .end-of-search-results {
        padding: 1rem;
        text-align: center;
        color: var(--text-color);
        opacity: 0.5;
        font-size: 0.8rem;
        border-top: 1px solid var(--border-color);
    }

    .end-of-search-results p {
        margin: 0;
    }

    @media (max-width: 1280px) {
        .main-content {
            left: 60px;
        }
        .search-results-view {
            left: 60px;
        }
    }

    @media (max-width: 640px) {
        .settings-drawer {
            width: 100%;
        }

        .name-row {
            left: calc(8px + 56px + 8px);
            bottom: 6px;
            right: 8px;
            gap: 6px;
            background: var(--bg-color);
            padding: 0.2em 0.5em;
            border-radius: 0.5em;
            width: fit-content;
        }

        .profile-username {
            font-size: 1rem;
            color: var(--text-color);
        }
        .profile-nip05-inline {
            font-size: 0.8rem;
            color: var(--text-color);
        }

        .managed-acl-view {
            padding: 1rem;
        }

        .kind-name {
            font-size: 0.7rem;
        }

        .search-results-view {
            left: 160px;
        }

        .search-result-info {
            width: 8rem;
        }

        .search-result-author {
            font-size: 0.7rem;
        }

        .search-result-content {
            font-size: 0.8rem;
        }
    }

    /* Recovery Tab Styles */
    .recovery-tab {
        padding: 20px;
        width: 100%;
        max-width: 1200px;
        margin: 0;
        box-sizing: border-box;
    }

    .recovery-tab h3 {
        margin: 0 0 10px 0;
        color: var(--text-color);
    }

    .recovery-tab p {
        margin: 0;
        color: var(--text-color);
        opacity: 0.7;
        padding: 0.5em;
    }

    .recovery-controls-card {
        background-color: transparent;
        border: none;
        border-radius: 0.5em;
        padding: 0;
    }

    .recovery-controls {
        display: flex;
        gap: 20px;
        align-items: center;
        flex-wrap: wrap;
    }

    .kind-selector {
        display: flex;
        flex-direction: column;
        gap: 5px;
    }

    .kind-selector label {
        font-weight: 500;
        color: var(--text-color);
    }

    .kind-selector select {
        padding: 8px 12px;
        border: 1px solid var(--border-color);
        border-radius: 0.5em;
        background: var(--bg-color);
        color: var(--text-color);
        min-width: 300px;
    }

    .custom-kind-input {
        display: flex;
        flex-direction: column;
        gap: 5px;
    }

    .custom-kind-input label {
        font-weight: 500;
        color: var(--text-color);
    }

    .custom-kind-input input {
        padding: 8px 12px;
        border: 1px solid var(--border-color);
        border-radius: 0.5em;
        background: var(--bg-color);
        color: var(--text-color);
        min-width: 200px;
    }

    .custom-kind-input input::placeholder {
        color: var(--text-color);
        opacity: 0.6;
    }

    .recovery-results {
        margin-top: 20px;
    }

    .loading,
    .no-events {
        text-align: left;
        padding: 40px 20px;
        color: var(--text-color);
        opacity: 0.7;
    }

    .events-list {
        display: flex;
        flex-direction: column;
        gap: 15px;
    }

    .event-item {
        background: var(--surface-bg);
        border: 2px solid var(--primary);
        border-radius: 0.5em;
        padding: 20px;
        transition: all 0.2s ease;
        background: var(--header-bg);
    }

    .event-item.old-version {
        opacity: 0.85;
        border: none;
        background: var(--header-bg);
    }

    .event-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 15px;
        flex-wrap: wrap;
        gap: 10px;
    }

    .event-header-left {
        display: flex;
        align-items: center;
        gap: 15px;
        flex-wrap: wrap;
    }

    .event-header-actions {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .event-kind {
        font-weight: 600;
        color: var(--primary);
    }

    .event-timestamp {
        color: var(--text-color);
        font-size: 0.9em;
        opacity: 0.7;
    }

    .repost-all-button {
        background: #059669;
        color: var(--text-color);
        border: none;
        padding: 6px 12px;
        border-radius: 0.5em;
        cursor: pointer;
        font-size: 0.9em;
        transition: background 0.2s ease;
        margin-right: 8px;
    }

    .repost-all-button:hover {
        background: #047857;
    }

    .repost-button {
        background: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 6px 12px;
        border-radius: 0.5em;
        cursor: pointer;
        font-size: 0.9em;
        transition: background 0.2s ease;
    }

    .repost-button:hover {
        background: #00acc1;
    }

    .event-content {
        margin-bottom: 15px;
    }

    .load-more {
        width: 100%;
        padding: 12px;
        background: var(--primary);
        color: var(--text-color);
        border: none;
        border-radius: 0.5em;
        cursor: pointer;
        font-size: 1em;
        margin-top: 20px;
        transition: background 0.2s ease;
    }

    .load-more:hover:not(:disabled) {
        background: #00acc1;
    }

    .load-more:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    /* Dark theme adjustments for recovery tab */
    :global(body.dark-theme) .event-item.old-version {
        background: var(--header-bg);
        border: none;
    }
</style>
