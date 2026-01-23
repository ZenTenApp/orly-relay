<script>
    import { onMount } from 'svelte';
    import Header from './components/Header.svelte';
    import LoginModal from './LoginModal.svelte';
    import Dashboard from './pages/Dashboard.svelte';
    import Config from './pages/Config.svelte';
    import Update from './pages/Update.svelte';
    import { isLoggedIn, userPubkey, userSigner, authMethod } from './stores.js';

    let currentPage = 'dashboard';
    let showLoginModal = false;
    let isDarkTheme = false;

    onMount(() => {
        // Check for stored auth
        const storedMethod = localStorage.getItem('launcher_auth_method');
        const storedPubkey = localStorage.getItem('launcher_pubkey');

        if (storedMethod === 'extension' && storedPubkey) {
            // Try to restore extension session
            if (window.nostr) {
                window.nostr.getPublicKey().then(pk => {
                    if (pk === storedPubkey) {
                        $isLoggedIn = true;
                        $userPubkey = pk;
                        $userSigner = window.nostr;
                        $authMethod = 'extension';
                    }
                }).catch(() => {
                    // Extension not available, clear stored auth
                    localStorage.removeItem('launcher_auth_method');
                    localStorage.removeItem('launcher_pubkey');
                });
            }
        }

        // Check for dark theme preference
        isDarkTheme = window.matchMedia('(prefers-color-scheme: dark)').matches;
    });

    function handleLogin(event) {
        const { method, pubkey, signer, privateKey } = event.detail;

        $isLoggedIn = true;
        $userPubkey = pubkey;
        $userSigner = signer;
        $authMethod = method;

        localStorage.setItem('launcher_auth_method', method);
        localStorage.setItem('launcher_pubkey', pubkey);

        if (method === 'nsec' && privateKey) {
            // Store encrypted key (handled by LoginModal)
        }

        showLoginModal = false;
    }

    function handleLogout() {
        $isLoggedIn = false;
        $userPubkey = '';
        $userSigner = null;
        $authMethod = '';

        localStorage.removeItem('launcher_auth_method');
        localStorage.removeItem('launcher_pubkey');
        localStorage.removeItem('launcher_privkey_encrypted');
    }

    function navigateTo(page) {
        currentPage = page;
    }
</script>

<main class:dark-theme={isDarkTheme}>
    <Header
        {currentPage}
        isLoggedIn={$isLoggedIn}
        userPubkey={$userPubkey}
        on:navigate={(e) => navigateTo(e.detail)}
        on:login={() => showLoginModal = true}
        on:logout={handleLogout}
    />

    <div class="content">
        {#if !$isLoggedIn}
            <div class="login-prompt">
                <h2>ORLY Launcher Admin</h2>
                <p>Please login to manage the relay services.</p>
                <button class="login-btn" on:click={() => showLoginModal = true}>
                    Login with Nostr
                </button>
            </div>
        {:else if currentPage === 'dashboard'}
            <Dashboard />
        {:else if currentPage === 'config'}
            <Config />
        {:else if currentPage === 'update'}
            <Update />
        {/if}
    </div>

    <LoginModal
        bind:showModal={showLoginModal}
        {isDarkTheme}
        on:login={handleLogin}
        on:close={() => showLoginModal = false}
    />
</main>

<style>
    :global(*) {
        box-sizing: border-box;
        margin: 0;
        padding: 0;
    }

    :global(body) {
        font-family: system-ui, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
        background: var(--bg-color);
        color: var(--text-color);
        min-height: 100vh;
    }

    main {
        --bg-color: #f5f5f5;
        --card-bg: #ffffff;
        --text-color: #333333;
        --muted-color: #666666;
        --border-color: #e0e0e0;
        --primary: #00bcd4;
        --primary-hover: #00acc1;
        --success: #4caf50;
        --error: #f44336;
        --warning: #ff9800;

        min-height: 100vh;
        background: var(--bg-color);
    }

    main.dark-theme {
        --bg-color: #1a1a1a;
        --card-bg: #2d2d2d;
        --text-color: #e0e0e0;
        --muted-color: #999999;
        --border-color: #444444;
    }

    .content {
        max-width: 1200px;
        margin: 0 auto;
        padding: 20px;
    }

    .login-prompt {
        text-align: center;
        padding: 60px 20px;
    }

    .login-prompt h2 {
        font-size: 2rem;
        margin-bottom: 16px;
        color: var(--text-color);
    }

    .login-prompt p {
        color: var(--muted-color);
        margin-bottom: 24px;
    }

    .login-btn {
        padding: 12px 32px;
        font-size: 1rem;
        background: var(--primary);
        color: white;
        border: none;
        border-radius: 6px;
        cursor: pointer;
        transition: background 0.2s;
    }

    .login-btn:hover {
        background: var(--primary-hover);
    }
</style>
