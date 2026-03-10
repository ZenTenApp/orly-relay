<script>
    import { isStandaloneMode, relayUrl, relayInfo, relayConnectionStatus, savedRelays, saveRelay, searchActive, notificationDropdownOpen } from "./stores.js";
    import { totalUnreadCount } from "./notificationStores.js";
    import { getApiBase, connectToRelay, normalizeWsUrl } from "./config.js";

    export let isDarkTheme = false;
    export let isLoggedIn = false;
    export let userRole = "";
    export let currentEffectiveRole = "";

    // Event dispatchers
    import { createEventDispatcher } from "svelte";
    const dispatch = createEventDispatcher();

    function toggleSearch() {
        searchActive.update(v => !v);
    }

    function toggleNotifications() {
        notificationDropdownOpen.update(v => !v);
    }

    // Dropdown state
    let showDropdown = false;
    let isConnecting = false;
    let connectingUrl = "";

    function toggleMobileMenu() {
        dispatch("toggleMobileMenu");
    }

    function openRelayModal() {
        dispatch("openRelayModal");
    }

    function toggleDropdown(event) {
        event.stopPropagation();
        showDropdown = !showDropdown;
    }

    function closeDropdown() {
        showDropdown = false;
    }

    async function switchToRelay(url) {
        console.log('[Header] switchToRelay called with:', url);
        if (isConnecting || url === $relayUrl) {
            console.log('[Header] Skipping - already connecting or same relay');
            return;
        }

        isConnecting = true;
        connectingUrl = url;

        try {
            console.log('[Header] Calling connectToRelay...');
            const result = await connectToRelay(url);
            console.log('[Header] connectToRelay result:', result);
            if (result.success) {
                const wsUrl = normalizeWsUrl(url);
                saveRelay(url, wsUrl);
                dispatch("relayChanged", { info: result.info });
                closeDropdown();
            } else {
                console.log('[Header] Connection failed:', result.error);
            }
        } catch (error) {
            console.error("[Header] Failed to switch relay:", error);
        } finally {
            isConnecting = false;
            connectingUrl = "";
        }
    }

    function handleManageRelays(event) {
        event.stopPropagation();
        closeDropdown();
        openRelayModal();
    }

    // Close dropdown when clicking outside
    function handleClickOutside(event) {
        if (showDropdown) {
            closeDropdown();
        }
    }

    // Get display name for relay - always show host URL
    // Explicitly reference $relayUrl in reactive statement for proper tracking
    $: relayDisplayName = getRelayHost($relayUrl);

    function getRelayHost(storeUrl) {
        try {
            // In standalone mode, use the stored relay URL
            // In embedded mode (no stored URL), use the current origin
            const url = storeUrl || getApiBase();
            console.log("[Header] getRelayHost - storeUrl:", storeUrl, "resolved url:", url);
            const parsed = new URL(url);
            return parsed.host;
        } catch {
            return storeUrl || "local";
        }
    }

    function formatRelayUrl(url) {
        try {
            const parsed = new URL(url.startsWith('http') ? url : 'https://' + url);
            return parsed.host;
        } catch {
            return url;
        }
    }

    function isCurrentRelay(url) {
        return $relayUrl === url && $relayConnectionStatus === "connected";
    }
</script>

<svelte:window on:click={handleClickOutside} />

<header class="main-header" class:dark-theme={isDarkTheme}>
    <div class="header-content">
        <button class="mobile-menu-btn" on:click={toggleMobileMenu} aria-label="Toggle menu">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M3 12h18M3 6h18M3 18h18" />
            </svg>
        </button>

        {#if isLoggedIn && userRole}
            <span class="permission-badge">{currentEffectiveRole}</span>
        {/if}

        <!-- Spacer to push right-side items -->
        <div class="header-spacer"></div>

        <!-- Relay indicator - dropdown only in standalone mode -->
        <div class="relay-dropdown-container">
            {#if $isStandaloneMode}
                <button
                    class="relay-indicator"
                    on:click={toggleDropdown}
                    title="Click to switch relays"
                >
                    <span class="relay-status" class:connected={$relayConnectionStatus === "connected"} class:error={$relayConnectionStatus === "error"}></span>
                    <span class="relay-name">{relayDisplayName}</span>
                    <span class="dropdown-arrow" class:open={showDropdown}>&#9662;</span>
                </button>

                {#if showDropdown}
                    <!-- svelte-ignore a11y-click-events-have-key-events -->
                    <!-- svelte-ignore a11y-no-static-element-interactions -->
                    <div class="relay-dropdown" on:click|stopPropagation>
                        {#if $savedRelays.length > 0}
                            <div class="dropdown-section">
                                <div class="dropdown-label">Saved Relays</div>
                                {#each $savedRelays as relay}
                                    <button
                                        class="dropdown-item"
                                        class:current={isCurrentRelay(relay.url)}
                                        class:connecting={connectingUrl === relay.url}
                                        on:click={() => switchToRelay(relay.url)}
                                        disabled={isConnecting}
                                    >
                                        <span class="item-status" class:connected={isCurrentRelay(relay.url)}></span>
                                        <span class="item-url-label">{relay.name}</span>
                                        {#if connectingUrl === relay.url}
                                            <span class="connecting-indicator">...</span>
                                        {/if}
                                    </button>
                                {/each}
                            </div>
                            <div class="dropdown-divider"></div>
                        {/if}
                        <button class="dropdown-item manage-btn" on:click={handleManageRelays}>
                            Manage Relays...
                        </button>
                    </div>
                {/if}
            {:else}
                <!-- Embedded mode: static indicator, no dropdown -->
                <div class="relay-indicator static" title="Connected to {relayDisplayName}">
                    <span class="relay-status" class:connected={$relayConnectionStatus === "connected"} class:error={$relayConnectionStatus === "error"}></span>
                    <span class="relay-name">{relayDisplayName}</span>
                </div>
            {/if}
        </div>

        <!-- Search button -->
        <button class="header-icon-btn" on:click={toggleSearch} title="Search" aria-label="Search">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <circle cx="11" cy="11" r="8" />
                <path d="M21 21l-4.35-4.35" />
            </svg>
        </button>

        <!-- Notification bell -->
        <button class="header-icon-btn notification-btn" on:click={toggleNotifications} title="Notifications" aria-label="Notifications">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                <path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9" />
                <path d="M13.73 21a2 2 0 0 1-3.46 0" />
            </svg>
            {#if $totalUnreadCount > 0}
                <span class="notification-badge">{$totalUnreadCount > 99 ? '99+' : $totalUnreadCount}</span>
            {/if}
        </button>
    </div>
</header>

<style>
    .main-header {
        color: var(--text-color);
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        height: 3em;
        background: var(--header-bg);
        border: 0;
        z-index: 1000;
        display: flex;
        align-items: stretch;
        padding: 0 0.25em;
    }

    .header-content {
        display: flex;
        align-items: stretch;
        width: 100%;
        padding: 0;
        margin: 0;
    }

    .mobile-menu-btn {
        display: none;
        align-items: center;
        justify-content: center;
        background: transparent;
        border: none;
        color: var(--text-color);
        cursor: pointer;
        padding: 0.5em;
        margin-right: 0.25em;
    }

    .mobile-menu-btn svg {
        width: 1.5em;
        height: 1.5em;
    }

    .mobile-menu-btn:hover {
        background: var(--card-bg);
        border-radius: 4px;
    }

    @media (max-width: 640px) {
        .mobile-menu-btn {
            display: flex;
        }
    }

    .permission-badge {
        background: var(--primary);
        color: #000;
        padding: 0.2em 0.5em;
        border-radius: 0.5em;
        font-size: 0.7em;
        font-weight: 500;
        text-transform: uppercase;
        letter-spacing: 0.5px;
        align-self: center;
        margin-left: 0.5em;
    }

    .header-spacer {
        flex: 1;
    }

    .header-icon-btn {
        display: flex;
        align-items: center;
        justify-content: center;
        background: transparent;
        border: none;
        color: var(--text-color);
        cursor: pointer;
        padding: 0 0.6em;
        align-self: stretch;
        transition: background 0.15s;
        position: relative;
    }

    .header-icon-btn:hover {
        background: var(--button-hover-bg);
    }

    .header-icon-btn svg {
        width: 1.25em;
        height: 1.25em;
    }

    .notification-badge {
        position: absolute;
        top: 0.4em;
        right: 0.3em;
        background: var(--primary);
        color: #000;
        font-size: 0.55rem;
        font-weight: 700;
        padding: 0.1em 0.35em;
        border-radius: 10px;
        min-width: 1em;
        text-align: center;
        line-height: 1.3;
    }

    /* Relay dropdown container */
    .relay-dropdown-container {
        position: relative;
        align-self: center;
    }

    /* Relay indicator */
    .relay-indicator {
        display: flex;
        align-items: center;
        gap: 6px;
        padding: 4px 10px;
        margin: 0 8px;
        background: var(--muted);
        border: 1px solid var(--border-color);
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.85em;
        color: var(--text-color);
        transition: background-color 0.2s, border-color 0.2s;
    }

    .relay-indicator:hover:not(.static) {
        background: var(--card-bg);
        border-color: var(--primary);
    }

    .relay-indicator.static {
        cursor: default;
    }

    .relay-status {
        width: 8px;
        height: 8px;
        border-radius: 50%;
        background: var(--warning);
        flex-shrink: 0;
    }

    .relay-status.connected {
        background: var(--success);
    }

    .relay-status.error {
        background: var(--danger);
    }

    .relay-name {
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
        max-width: 200px;
    }

    .dropdown-arrow {
        font-size: 0.7em;
        transition: transform 0.2s;
        margin-left: 2px;
    }

    .dropdown-arrow.open {
        transform: rotate(180deg);
    }

    /* Dropdown menu */
    .relay-dropdown {
        position: absolute;
        top: calc(100% + 4px);
        left: 0;
        min-width: 250px;
        background: var(--bg-color);
        border: 1px solid var(--border-color);
        border-radius: 6px;
        box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
        z-index: 1001;
        overflow: hidden;
    }

    .dropdown-section {
        padding: 4px 0;
    }

    .dropdown-label {
        padding: 6px 12px;
        font-size: 0.75em;
        color: var(--muted-foreground);
        text-transform: uppercase;
        letter-spacing: 0.5px;
        font-weight: 500;
    }

    .dropdown-item {
        display: flex;
        align-items: center;
        gap: 8px;
        width: 100%;
        padding: 8px 12px;
        background: transparent;
        border: none;
        cursor: pointer;
        text-align: left;
        color: var(--text-color);
        font-size: 0.9em;
        transition: background-color 0.15s;
    }

    .dropdown-item:hover:not(:disabled) {
        background: var(--tab-hover-bg);
    }

    .dropdown-item:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .dropdown-item.current {
        background: rgba(16, 185, 129, 0.1);
    }

    .dropdown-item.connecting {
        background: rgba(234, 179, 8, 0.1);
    }

    .item-status {
        width: 6px;
        height: 6px;
        border-radius: 50%;
        background: var(--muted-foreground);
        flex-shrink: 0;
    }

    .item-status.connected {
        background: var(--success);
    }

    .item-url-label {
        flex: 1;
        font-family: monospace;
        font-size: 0.85em;
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .connecting-indicator {
        color: var(--warning);
        font-weight: bold;
    }

    .dropdown-divider {
        height: 1px;
        background: var(--border-color);
        margin: 4px 0;
    }

    .manage-btn {
        color: var(--primary);
        font-weight: 500;
    }
</style>
