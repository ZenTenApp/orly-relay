<script>
    export let isDarkTheme = false;
    export let isLoggedIn = false;
    export let userRole = "";
    export let currentEffectiveRole = "";
    export let userProfile = null;
    export let userPubkey = "";

    // Event dispatchers
    import { createEventDispatcher } from "svelte";
    const dispatch = createEventDispatcher();

    function openSettingsDrawer() {
        dispatch("openSettingsDrawer");
    }

    function openLoginModal() {
        dispatch("openLoginModal");
    }
</script>

<header class="main-header" class:dark-theme={isDarkTheme}>
    <div class="header-content">
        <img src="/orly.png" alt="ORLY Logo" class="logo" />
        <div class="header-title">
            <span class="app-title">
                ORLY? dashboard
                {#if isLoggedIn && userRole}
                    <span class="permission-badge"
                        >{currentEffectiveRole}</span
                    >
                {/if}
            </span>
        </div>
        <div class="header-buttons">
            {#if isLoggedIn}
                <button class="user-profile-btn" on:click={openSettingsDrawer}>
                    {#if userProfile?.picture}
                        <img
                            src={userProfile.picture}
                            alt="User avatar"
                            class="user-avatar"
                        />
                    {:else}
                        <div class="user-avatar-placeholder">👤</div>
                    {/if}
                    <span class="user-name">
                        {userProfile?.name || userPubkey}
                    </span>
                </button>
            {:else}
                <button class="login-btn" on:click={openLoginModal}
                    >Log in</button
                >
            {/if}
        </div>
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

    .logo {
        height: 2.5em;
        width: auto;
        flex-shrink: 0;
        align-self: center;
    }

    .header-title {
        flex: 1;
        display: flex;
        align-items: center;
        align-self: center;
    }

    .app-title {
        font-size: 1.2em;
        font-weight: 600;
        color: var(--text-color);
        display: flex;
        align-items: center;
        gap: 0.5em;
    }

    .permission-badge {
        background: var(--primary);
        color: var(--text-color);
        padding: 0.2em 0.5em;
        border-radius: 0.5em;
        font-size: 0.7em;
        font-weight: 500;
        text-transform: uppercase;
        letter-spacing: 0.5px;
    }

    .header-buttons {
        display: flex;
        align-items: stretch;
        align-self: stretch;
        margin-left: auto;
    }

    .login-btn,
    .user-profile-btn {
        background: transparent;
        color: var(--button-text);
        border: 0;
        cursor: pointer;
        font-size: 1em;
        transition: background-color 0.2s;
        flex-shrink: 0;
        padding: 0.5em;
        margin: 0;
        display: flex !important;
        align-items: center !important;
        justify-content: center;
    }

    .login-btn:hover,
    .user-profile-btn:hover {
        background: var(--card-bg);
    }

    .user-profile-btn {
        gap: 0.5em;
        justify-content: flex-start;
        padding: 0 0.5em;
    }

    .user-avatar {
        height: 2.5em;
        width: 2.5em;
        border-radius: 50%;
        object-fit: cover;
        flex-shrink: 0;
        align-self: center;
        vertical-align: middle;
    }

    .user-avatar-placeholder {
        height: 2.5em;
        width: 2.5em;
        border-radius: 50%;
        background: var(--bg-color);
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 1.2em;
        flex-shrink: 0;
        align-self: center;
    }

    .user-name {
        font-weight: 500;
        white-space: nowrap;
        line-height: 1;
        align-self: center;
        max-width: none !important;
        overflow: visible !important;
        text-overflow: unset !important;
        width: auto !important;
    }
</style>
