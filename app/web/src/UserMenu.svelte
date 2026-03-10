<script>
    import { createEventDispatcher } from 'svelte';
    import { isStandaloneMode, relayUrl, relayInfo as relayInfoStore, userMenuOpen } from './stores.js';

    export let isLoggedIn = false;
    export let userProfile = null;
    export let userPubkey = "";
    export let userRole = "";
    export let currentEffectiveRole = "";
    export let isDarkTheme = false;

    const dispatch = createEventDispatcher();

    function getAvailableRoles() {
        const allRoles = ["owner", "admin", "write", "read"];
        const idx = allRoles.indexOf(userRole);
        if (idx === -1) return ["read"];
        return allRoles.slice(idx);
    }

    function handleLogout() {
        userMenuOpen.set(false);
        dispatch('logout');
    }

    function handleToggleTheme() {
        dispatch('toggleTheme');
    }

    function handleViewAsRole(role) {
        dispatch('setViewAsRole', role === userRole ? "" : role);
    }

    function handleChangeRelay() {
        userMenuOpen.set(false);
        dispatch('openRelayModal');
    }

    function closeMenu() {
        userMenuOpen.set(false);
    }

    function displayName(profile, pubkey) {
        if (profile?.name) return profile.name;
        if (profile?.display_name) return profile.display_name;
        if (pubkey) return pubkey.slice(0, 8) + '...';
        return 'Anonymous';
    }
</script>

{#if $userMenuOpen && isLoggedIn}
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="menu-overlay" on:click={closeMenu}></div>
    <div class="user-menu">
        <!-- Profile section -->
        <div class="menu-profile">
            {#if userProfile?.picture}
                <img src={userProfile.picture} alt="avatar" class="menu-avatar" />
            {:else}
                <div class="menu-avatar-placeholder">
                    {displayName(userProfile, userPubkey).charAt(0).toUpperCase()}
                </div>
            {/if}
            <div class="menu-profile-info">
                <div class="menu-display-name">{displayName(userProfile, userPubkey)}</div>
                {#if userProfile?.nip05}
                    <div class="menu-nip05">{userProfile.nip05}</div>
                {/if}
                {#if userRole}
                    <div class="menu-role">{currentEffectiveRole}</div>
                {/if}
            </div>
        </div>

        <div class="menu-divider"></div>

        <!-- Theme toggle -->
        <button class="menu-item" on:click={handleToggleTheme}>
            <span class="menu-item-icon">{isDarkTheme ? '☀️' : '🌙'}</span>
            <span>{isDarkTheme ? 'Light Mode' : 'Dark Mode'}</span>
        </button>

        <!-- View as role (admin/owner only) -->
        {#if userRole && userRole !== "read"}
            <div class="menu-section-label">View as Role</div>
            {#each getAvailableRoles() as role}
                <button
                    class="menu-item"
                    class:active={currentEffectiveRole === role}
                    on:click={() => handleViewAsRole(role)}
                >
                    <span class="menu-item-icon">{currentEffectiveRole === role ? '●' : '○'}</span>
                    <span>{role.charAt(0).toUpperCase() + role.slice(1)}{role === userRole ? ' (Default)' : ''}</span>
                </button>
            {/each}
        {/if}

        <!-- Relay (standalone mode) -->
        {#if $isStandaloneMode}
            <div class="menu-divider"></div>
            <button class="menu-item" on:click={handleChangeRelay}>
                <span class="menu-item-icon">🔗</span>
                <span>Change Relay</span>
            </button>
        {/if}

        <div class="menu-divider"></div>

        <!-- Logout -->
        <button class="menu-item danger" on:click={handleLogout}>
            <span class="menu-item-icon">⏻</span>
            <span>Log out</span>
        </button>
    </div>
{/if}

<style>
    .menu-overlay {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        z-index: 999;
    }

    .user-menu {
        position: fixed;
        top: 3em;
        left: 200px;
        width: 240px;
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 8px;
        box-shadow: 0 4px 16px rgba(0, 0, 0, 0.2);
        z-index: 1000;
        overflow: hidden;
    }

    .menu-profile {
        display: flex;
        align-items: center;
        gap: 0.6em;
        padding: 0.75em;
    }

    .menu-avatar {
        width: 40px;
        height: 40px;
        border-radius: 50%;
        object-fit: cover;
        flex-shrink: 0;
    }

    .menu-avatar-placeholder {
        width: 40px;
        height: 40px;
        border-radius: 50%;
        background: var(--primary);
        color: #000;
        display: flex;
        align-items: center;
        justify-content: center;
        font-weight: bold;
        font-size: 1rem;
        flex-shrink: 0;
    }

    .menu-profile-info {
        overflow: hidden;
    }

    .menu-display-name {
        font-weight: 600;
        font-size: 0.85rem;
        color: var(--text-color);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .menu-nip05 {
        font-size: 0.7rem;
        color: var(--text-muted);
        white-space: nowrap;
        overflow: hidden;
        text-overflow: ellipsis;
    }

    .menu-role {
        font-size: 0.65rem;
        color: var(--primary);
        text-transform: uppercase;
        letter-spacing: 0.5px;
        font-weight: 600;
        margin-top: 0.15em;
    }

    .menu-divider {
        height: 1px;
        background: var(--border-color);
        margin: 0.25em 0;
    }

    .menu-section-label {
        padding: 0.4em 0.75em 0.2em;
        font-size: 0.65rem;
        color: var(--text-muted);
        text-transform: uppercase;
        letter-spacing: 0.5px;
        font-weight: 500;
    }

    .menu-item {
        display: flex;
        align-items: center;
        gap: 0.5em;
        width: 100%;
        padding: 0.5em 0.75em;
        background: none;
        border: none;
        cursor: pointer;
        color: var(--text-color);
        font-size: 0.8rem;
        text-align: left;
        transition: background 0.15s;
    }

    .menu-item:hover {
        background: var(--button-hover-bg);
    }

    .menu-item.active {
        color: var(--primary);
    }

    .menu-item.danger {
        color: var(--danger);
    }

    .menu-item.danger:hover {
        background: var(--danger-bg);
    }

    .menu-item-icon {
        width: 1.2em;
        text-align: center;
        flex-shrink: 0;
        font-size: 0.85em;
    }

    @media (max-width: 1280px) {
        .user-menu {
            left: 60px;
        }
    }

    @media (max-width: 640px) {
        .user-menu {
            left: 0;
            right: 0;
            width: auto;
            top: auto;
            bottom: 0;
            border-radius: 12px 12px 0 0;
        }
    }
</style>
