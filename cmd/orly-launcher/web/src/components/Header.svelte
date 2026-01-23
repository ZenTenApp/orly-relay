<script>
    import { createEventDispatcher } from 'svelte';

    const dispatch = createEventDispatcher();

    export let currentPage = 'dashboard';
    export let isLoggedIn = false;
    export let userPubkey = '';

    function navigate(page) {
        dispatch('navigate', page);
    }

    function formatPubkey(pk) {
        if (!pk) return '';
        return pk.slice(0, 8) + '...' + pk.slice(-4);
    }
</script>

<header>
    <div class="header-content">
        <h1>ORLY Launcher</h1>

        {#if isLoggedIn}
            <nav>
                <button
                    class="nav-btn"
                    class:active={currentPage === 'dashboard'}
                    on:click={() => navigate('dashboard')}
                >
                    Dashboard
                </button>
                <button
                    class="nav-btn"
                    class:active={currentPage === 'config'}
                    on:click={() => navigate('config')}
                >
                    Config
                </button>
                <button
                    class="nav-btn"
                    class:active={currentPage === 'update'}
                    on:click={() => navigate('update')}
                >
                    Update
                </button>
            </nav>

            <div class="user-section">
                <span class="pubkey">{formatPubkey(userPubkey)}</span>
                <button class="logout-btn" on:click={() => dispatch('logout')}>
                    Logout
                </button>
            </div>
        {:else}
            <button class="login-header-btn" on:click={() => dispatch('login')}>
                Login
            </button>
        {/if}
    </div>
</header>

<style>
    header {
        background: var(--card-bg);
        border-bottom: 1px solid var(--border-color);
        padding: 0 20px;
    }

    .header-content {
        max-width: 1200px;
        margin: 0 auto;
        display: flex;
        align-items: center;
        justify-content: space-between;
        height: 60px;
    }

    h1 {
        font-size: 1.25rem;
        font-weight: 600;
        color: var(--text-color);
    }

    nav {
        display: flex;
        gap: 4px;
    }

    .nav-btn {
        padding: 8px 16px;
        background: none;
        border: none;
        border-radius: 4px;
        color: var(--muted-color);
        cursor: pointer;
        font-size: 0.9rem;
    }

    .nav-btn:hover {
        background: var(--border-color);
        color: var(--text-color);
    }

    .nav-btn.active {
        background: var(--primary);
        color: white;
    }

    .user-section {
        display: flex;
        align-items: center;
        gap: 12px;
    }

    .pubkey {
        font-family: monospace;
        font-size: 0.85rem;
        color: var(--muted-color);
    }

    .logout-btn,
    .login-header-btn {
        padding: 6px 14px;
        font-size: 0.85rem;
        border-radius: 4px;
        cursor: pointer;
    }

    .logout-btn {
        background: none;
        border: 1px solid var(--border-color);
        color: var(--text-color);
    }

    .logout-btn:hover {
        background: var(--border-color);
    }

    .login-header-btn {
        background: var(--primary);
        border: none;
        color: white;
    }

    .login-header-btn:hover {
        background: var(--primary-hover);
    }
</style>
