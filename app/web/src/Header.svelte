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
                        {userProfile?.name || userPubkey.slice(0, 8) + "..."}
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
        background: var(--header-bg);
        border: 0;
        z-index: 1000;
        display: flex;
        align-items: space-between;
        padding: 0.25em;
    }

    .header-content {
        display: flex;
        align-items: center;
        width: 100%;
        padding: 0;
        margin: 0;
    }

    .logo {
        height: 2em;
        width: auto;
        flex-shrink: 0;
    }

    .header-title {
        flex: 1;
        display: flex;
        align-items: center;
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
        align-items: center;
        height: 100%;
        margin-left: auto;
    }

    .login-btn,
    .user-profile-btn {
        background: transparent;
        color: var(--button-text);
        border: 0;
        height: 100%;
        cursor: pointer;
        font-size: 1em;
        transition: background-color 0.2s;
        flex-shrink: 0;
        padding: 0.5em;
        margin: 0;
        display: block;
        align-items: center;
        justify-content: center;
    }

    .login-btn:hover,
    .user-profile-btn:hover {
        background: var(--card-bg);
    }

    .user-profile-btn {
        gap: 0.5em;
        justify-content: flex-start;
        padding: 0 0.75em;
    }

    .user-avatar {
        width: 1.5em;
        height: 1.5em;
        border-radius: 50%;
        object-fit: cover;
    }

    .user-avatar-placeholder {
        width: 1.5em;
        height: 1.5em;
        border-radius: 50%;
        background: var(--bg-color);
        display: flex;
        align-items: center;
        justify-content: center;
        font-size: 0.8em;
    }

    .user-name {
        font-weight: 500;
        max-width: 120px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }
</style>
