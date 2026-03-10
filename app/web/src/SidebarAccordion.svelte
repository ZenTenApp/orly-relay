<script>
    import { createEventDispatcher } from 'svelte';
    import { activeView, expandedSection, userMenuOpen } from './stores.js';
    import { totalUnreadDMs, totalUnreadChannels } from './chatStores.js';
    import { totalUnreadCount } from './notificationStores.js';

    export let isLoggedIn = false;
    export let userProfile = null;
    export let userPubkey = "";
    export let currentEffectiveRole = "";
    export let version = "";
    export let mobileOpen = false;

    // Admin feature flags
    export let aclMode = "";
    export let sprocketEnabled = false;
    export let policyEnabled = false;
    export let nrcEnabled = false;
    export let blossomEnabled = true;
    export let isOrlyRelay = true;

    const dispatch = createEventDispatcher();

    // Navigation structure
    const sections = [
        {
            id: "feed",
            icon: "⚡",
            label: "Feed",
            children: null, // no children = direct nav
        },
        {
            id: "chat",
            icon: "💬",
            label: "Chat",
            children: [
                { id: "chat-inbox", label: "Inbox" },
                { id: "chat-channels", label: "Channels" },
            ],
        },
        {
            id: "library",
            icon: "📚",
            label: "Library",
            children: [
                { id: "library-my", label: "My Library" },
                { id: "library-bookmarks", label: "Bookmarks" },
                { id: "library-new", label: "New" },
            ],
        },
    ];

    // Build admin section dynamically based on permissions
    $: adminChildren = buildAdminChildren(isLoggedIn, currentEffectiveRole, aclMode,
        sprocketEnabled, policyEnabled, nrcEnabled, blossomEnabled, isOrlyRelay);

    function buildAdminChildren(loggedIn, role, acl, sprocket, policy, nrc, blossom, orly) {
        if (!loggedIn) return [];
        const items = [];
        items.push({ id: "admin-export", label: "Export" });
        if (role === "admin" || role === "owner") {
            items.push({ id: "admin-import", label: "Import" });
        }
        if (role === "read" || role === "write" || role === "admin" || role === "owner") {
            items.push({ id: "admin-events", label: "Events" });
        }
        if (blossom) {
            items.push({ id: "admin-blossom", label: "Blossom" });
        }
        if (role !== "read") {
            items.push({ id: "admin-compose", label: "Compose" });
        }
        items.push({ id: "admin-recovery", label: "Recovery" });
        if (role === "owner" && orly) {
            if (acl === "managed") items.push({ id: "admin-managed-acl", label: "Managed ACL" });
            if (acl === "curating") items.push({ id: "admin-curation", label: "Curation" });
            if (sprocket) items.push({ id: "admin-sprocket", label: "Sprocket" });
            if (policy) items.push({ id: "admin-policy", label: "Policy" });
            if (nrc) items.push({ id: "admin-relay-connect", label: "Relay Connect" });
            items.push({ id: "admin-logs", label: "Logs" });
        }
        return items;
    }

    $: allSections = [
        ...sections,
        ...(adminChildren.length > 0 ? [{
            id: "admin",
            icon: "⚙️",
            label: "Admin",
            children: adminChildren,
        }] : []),
    ];

    function toggleSection(sectionId) {
        expandedSection.update(current => current === sectionId ? null : sectionId);
    }

    function navigate(viewId) {
        activeView.set(viewId);
        dispatch('navigate', viewId);
        // Close mobile menu on navigation
        if (mobileOpen) dispatch('closeMobileMenu');
    }

    function handleSectionClick(section) {
        if (section.children) {
            toggleSection(section.id);
        } else {
            navigate(section.id);
        }
    }

    function handleChildClick(childId) {
        navigate(childId);
    }

    function getUnreadBadge(sectionId) {
        if (sectionId === "chat") return $totalUnreadDMs + $totalUnreadChannels;
        return 0;
    }

    function getChildUnreadBadge(childId) {
        if (childId === "chat-inbox") return $totalUnreadDMs;
        if (childId === "chat-channels") return $totalUnreadChannels;
        return 0;
    }

    function handleUserClick() {
        userMenuOpen.update(v => !v);
        dispatch('toggleUserMenu');
    }

    function handleLogoClick() {
        dispatch('showAbout');
    }

    // Truncate display name
    function displayName(profile, pubkey) {
        if (profile?.name) return profile.name;
        if (profile?.display_name) return profile.display_name;
        if (pubkey) return pubkey.slice(0, 8) + '...';
        return 'Anonymous';
    }
</script>

<nav class="sidebar-accordion" class:mobile-open={mobileOpen}>
    {#if mobileOpen}
        <div class="mobile-overlay" on:click={() => dispatch('closeMobileMenu')}></div>
    {/if}

    <div class="sidebar-content">
        <!-- User avatar / login button at top -->
        <div class="sidebar-user">
            {#if isLoggedIn}
                <button class="user-button" on:click={handleUserClick}>
                    {#if userProfile?.picture}
                        <img src={userProfile.picture} alt="avatar" class="user-avatar" />
                    {:else}
                        <div class="user-avatar-placeholder">
                            {displayName(userProfile, userPubkey).charAt(0).toUpperCase()}
                        </div>
                    {/if}
                    <span class="user-name">{displayName(userProfile, userPubkey)}</span>
                </button>
            {:else}
                <button class="login-button" on:click={() => dispatch('openLoginModal')}>
                    Log in
                </button>
            {/if}
        </div>

        <!-- Navigation sections -->
        <div class="nav-sections">
            {#each allSections as section}
                <div class="nav-section" class:expanded={$expandedSection === section.id}>
                    <button
                        class="section-header"
                        class:active={$activeView === section.id || (section.children && section.children.some(c => $activeView === c.id))}
                        on:click={() => handleSectionClick(section)}
                    >
                        <span class="section-icon">{section.icon}</span>
                        <span class="section-label">{section.label}</span>
                        {#if section.children}
                            <span class="section-chevron">{$expandedSection === section.id ? '▾' : '▸'}</span>
                        {/if}
                        {#if getUnreadBadge(section.id) > 0}
                            <span class="unread-badge">{getUnreadBadge(section.id)}</span>
                        {/if}
                    </button>

                    {#if section.children && $expandedSection === section.id}
                        <div class="section-children">
                            {#each section.children as child}
                                <button
                                    class="child-item"
                                    class:active={$activeView === child.id}
                                    on:click={() => handleChildClick(child.id)}
                                >
                                    <span class="child-label">{child.label}</span>
                                    {#if getChildUnreadBadge(child.id) > 0}
                                        <span class="unread-badge small">{getChildUnreadBadge(child.id)}</span>
                                    {/if}
                                </button>
                            {/each}
                            <div class="section-boundary"></div>
                        </div>
                    {/if}
                </div>
            {/each}
        </div>

        <!-- Logo at bottom -->
        <div class="sidebar-footer">
            <button class="logo-button" on:click={handleLogoClick} title="About smesh">
                <span class="logo-text">smesh</span>
                {#if version}
                    <span class="version-text">{version}</span>
                {/if}
            </button>
        </div>
    </div>
</nav>

<style>
    .sidebar-accordion {
        position: fixed;
        top: 3em;
        left: 0;
        bottom: 0;
        width: 200px;
        background: var(--sidebar-bg);
        border-right: 1px solid var(--border-color);
        display: flex;
        flex-direction: column;
        z-index: 100;
        transition: transform 0.2s ease;
    }

    .sidebar-content {
        display: flex;
        flex-direction: column;
        height: 100%;
        overflow-y: auto;
    }

    /* User section */
    .sidebar-user {
        padding: 0.75em;
        border-bottom: 1px solid var(--border-color);
    }

    .user-button {
        display: flex;
        align-items: center;
        gap: 0.5em;
        width: 100%;
        padding: 0.5em;
        background: none;
        border: none;
        border-radius: 8px;
        cursor: pointer;
        color: var(--text-color);
        transition: background 0.15s;
    }

    .user-button:hover {
        background: var(--button-hover-bg);
    }

    .user-avatar {
        width: 32px;
        height: 32px;
        border-radius: 50%;
        object-fit: cover;
    }

    .user-avatar-placeholder {
        width: 32px;
        height: 32px;
        border-radius: 50%;
        background: var(--primary);
        color: #000;
        display: flex;
        align-items: center;
        justify-content: center;
        font-weight: bold;
        font-size: 0.9rem;
    }

    .user-name {
        font-size: 0.85rem;
        font-weight: 500;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .login-button {
        width: 100%;
        padding: 0.6em;
        background: var(--primary);
        color: #000;
        border: none;
        border-radius: 6px;
        cursor: pointer;
        font-weight: 600;
        font-size: 0.85rem;
        transition: filter 0.15s;
    }

    .login-button:hover {
        filter: brightness(0.9);
    }

    /* Navigation sections */
    .nav-sections {
        flex: 1;
        padding: 0.5em 0;
    }

    .nav-section {
        margin: 0;
    }

    .section-header {
        display: flex;
        align-items: center;
        gap: 0.5em;
        width: 100%;
        padding: 0.6em 0.75em;
        background: none;
        border: none;
        cursor: pointer;
        color: var(--text-color);
        font-size: 0.85rem;
        font-weight: 500;
        transition: background 0.15s;
        text-align: left;
    }

    .section-header:hover {
        background: var(--button-hover-bg);
    }

    .section-header.active {
        color: var(--primary);
        background: var(--primary-bg);
    }

    .section-icon {
        font-size: 1rem;
        width: 1.5em;
        text-align: center;
    }

    .section-label {
        flex: 1;
    }

    .section-chevron {
        font-size: 0.7rem;
        color: var(--text-muted);
    }

    .unread-badge {
        background: var(--primary);
        color: #000;
        font-size: 0.65rem;
        font-weight: 700;
        padding: 0.1em 0.4em;
        border-radius: 10px;
        min-width: 1.2em;
        text-align: center;
    }

    .unread-badge.small {
        font-size: 0.6rem;
        padding: 0.05em 0.35em;
    }

    /* Children */
    .section-children {
        padding-left: 0;
        background: var(--primary-bg);
    }

    .child-item {
        display: flex;
        align-items: center;
        gap: 0.5em;
        width: 100%;
        padding: 0.45em 0.75em 0.45em 2.75em;
        background: none;
        border: none;
        cursor: pointer;
        color: var(--text-color);
        font-size: 0.8rem;
        transition: background 0.15s;
        text-align: left;
    }

    .child-item:hover {
        background: var(--button-hover-bg);
    }

    .child-item.active {
        color: var(--primary);
        font-weight: 600;
    }

    .child-label {
        flex: 1;
    }

    .section-boundary {
        height: 1px;
        background: var(--border-color);
        margin: 0.25em 0.75em;
    }

    /* Footer */
    .sidebar-footer {
        padding: 0.5em;
        border-top: 1px solid var(--border-color);
        text-align: right;
    }

    .logo-button {
        background: none;
        border: none;
        cursor: pointer;
        padding: 0.4em 0.6em;
        border-radius: 6px;
        transition: background 0.15s;
    }

    .logo-button:hover {
        background: var(--button-hover-bg);
    }

    .logo-text {
        font-size: 0.85rem;
        font-weight: 700;
        color: var(--primary);
        letter-spacing: 0.05em;
    }

    .version-text {
        display: block;
        font-size: 0.6rem;
        color: var(--text-muted);
        margin-top: 0.1em;
    }

    /* Mobile */
    .mobile-overlay {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background: rgba(0, 0, 0, 0.5);
        z-index: -1;
    }

    @media (max-width: 1280px) {
        .sidebar-accordion {
            width: 60px;
        }

        .section-label,
        .section-chevron,
        .user-name,
        .child-label,
        .version-text,
        .logo-text {
            display: none;
        }

        .section-header {
            justify-content: center;
            padding: 0.6em;
        }

        .section-icon {
            width: auto;
        }

        .section-children {
            display: none;
        }

        .sidebar-user {
            padding: 0.5em;
        }

        .user-button {
            justify-content: center;
            padding: 0.5em;
        }

        .user-avatar-placeholder,
        .user-avatar {
            width: 28px;
            height: 28px;
        }

        .sidebar-footer {
            text-align: center;
        }
    }

    @media (max-width: 640px) {
        .sidebar-accordion {
            width: 250px;
            transform: translateX(-100%);
            top: 0;
            z-index: 1000;
        }

        .sidebar-accordion.mobile-open {
            transform: translateX(0);
        }

        .section-label,
        .section-chevron,
        .user-name,
        .child-label,
        .version-text,
        .logo-text {
            display: inline;
        }

        .section-header {
            justify-content: flex-start;
            padding: 0.6em 0.75em;
        }

        .section-children {
            display: block;
        }

        .sidebar-footer {
            text-align: right;
        }
    }
</style>
