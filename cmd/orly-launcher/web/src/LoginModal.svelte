<script>
    import { createEventDispatcher, onMount } from 'svelte';
    import { generateSecretKey, getPublicKey } from 'nostr-tools/pure';
    import { nsecEncode, npubEncode, decode } from 'nostr-tools/nip19';
    import { finalizeEvent } from 'nostr-tools/pure';

    const dispatch = createEventDispatcher();

    export let showModal = false;
    export let isDarkTheme = false;

    let activeTab = 'extension';
    let nsecInput = '';
    let isLoading = false;
    let errorMessage = '';
    let successMessage = '';
    let generatedNsec = '';
    let generatedNpub = '';

    function closeModal() {
        showModal = false;
        nsecInput = '';
        errorMessage = '';
        successMessage = '';
        generatedNsec = '';
        generatedNpub = '';
        dispatch('close');
    }

    function switchTab(tab) {
        activeTab = tab;
        errorMessage = '';
        successMessage = '';
        generatedNsec = '';
        generatedNpub = '';
    }

    async function generateNewKey() {
        errorMessage = '';
        successMessage = '';

        try {
            const secretKey = generateSecretKey();
            const nsec = nsecEncode(secretKey);
            const pubkey = getPublicKey(secretKey);
            const npub = npubEncode(pubkey);

            generatedNsec = nsec;
            generatedNpub = npub;
            nsecInput = nsec;

            successMessage = 'New key generated!';
        } catch (error) {
            errorMessage = 'Failed to generate key: ' + error.message;
        }
    }

    async function loginWithExtension() {
        isLoading = true;
        errorMessage = '';
        successMessage = '';

        try {
            if (!window.nostr) {
                throw new Error('No Nostr extension found. Please install nos2x or Alby.');
            }

            const pubkey = await window.nostr.getPublicKey();

            if (pubkey) {
                successMessage = 'Successfully logged in with extension!';
                dispatch('login', {
                    method: 'extension',
                    pubkey: pubkey,
                    signer: window.nostr,
                });

                setTimeout(closeModal, 500);
            }
        } catch (error) {
            errorMessage = error.message;
        } finally {
            isLoading = false;
        }
    }

    async function loginWithNsec() {
        isLoading = true;
        errorMessage = '';
        successMessage = '';

        try {
            if (!nsecInput.trim()) {
                throw new Error('Please enter your nsec');
            }

            const trimmed = nsecInput.trim();

            // Decode nsec
            let decoded;
            try {
                decoded = decode(trimmed);
            } catch {
                throw new Error('Invalid nsec format');
            }

            if (decoded.type !== 'nsec') {
                throw new Error('Please enter an nsec (private key)');
            }

            const secretKey = decoded.data;
            const publicKey = getPublicKey(secretKey);

            // Create a signer that uses the secret key
            const signer = {
                getPublicKey: async () => publicKey,
                signEvent: async (event) => {
                    return finalizeEvent(event, secretKey);
                }
            };

            successMessage = 'Successfully logged in!';
            dispatch('login', {
                method: 'nsec',
                pubkey: publicKey,
                privateKey: trimmed,
                signer: signer,
            });

            setTimeout(closeModal, 500);
        } catch (error) {
            errorMessage = error.message;
        } finally {
            isLoading = false;
        }
    }

    function handleKeydown(event) {
        if (event.key === 'Escape') {
            closeModal();
        }
        if (event.key === 'Enter' && activeTab === 'nsec') {
            loginWithNsec();
        }
    }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if showModal}
    <div
        class="modal-overlay"
        on:click={closeModal}
        on:keydown={(e) => e.key === 'Escape' && closeModal()}
        role="button"
        tabindex="0"
    >
        <div
            class="modal"
            class:dark-theme={isDarkTheme}
            on:click|stopPropagation
            on:keydown|stopPropagation
        >
            <div class="modal-header">
                <h2>Login to Launcher Admin</h2>
                <button class="close-btn" on:click={closeModal}>&times;</button>
            </div>

            <div class="tab-container">
                <div class="tabs">
                    <button
                        class="tab-btn"
                        class:active={activeTab === 'extension'}
                        on:click={() => switchTab('extension')}
                    >
                        Extension
                    </button>
                    <button
                        class="tab-btn"
                        class:active={activeTab === 'nsec'}
                        on:click={() => switchTab('nsec')}
                    >
                        Nsec
                    </button>
                </div>

                <div class="tab-content">
                    {#if activeTab === 'extension'}
                        <div class="extension-login">
                            <p>Login using a NIP-07 browser extension like nos2x or Alby.</p>
                            <button
                                class="login-btn"
                                on:click={loginWithExtension}
                                disabled={isLoading}
                            >
                                {isLoading ? 'Connecting...' : 'Login with Extension'}
                            </button>
                        </div>
                    {:else}
                        <div class="nsec-login">
                            <p>Enter your nsec or generate a new key pair.</p>

                            <button
                                class="generate-btn"
                                on:click={generateNewKey}
                                disabled={isLoading}
                            >
                                Generate New Key
                            </button>

                            {#if generatedNpub}
                                <div class="generated-info">
                                    <label>Your new public key (npub):</label>
                                    <code>{generatedNpub}</code>
                                </div>
                            {/if}

                            <input
                                type="password"
                                placeholder="nsec1..."
                                bind:value={nsecInput}
                                disabled={isLoading}
                                class="nsec-input"
                            />

                            <button
                                class="login-btn"
                                on:click={loginWithNsec}
                                disabled={isLoading || !nsecInput.trim()}
                            >
                                {isLoading ? 'Logging in...' : 'Login with Nsec'}
                            </button>
                        </div>
                    {/if}

                    {#if errorMessage}
                        <div class="message error-message">{errorMessage}</div>
                    {/if}

                    {#if successMessage}
                        <div class="message success-message">{successMessage}</div>
                    {/if}
                </div>
            </div>
        </div>
    </div>
{/if}

<style>
    .modal-overlay {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background-color: rgba(0, 0, 0, 0.5);
        display: flex;
        justify-content: center;
        align-items: center;
        z-index: 1000;
    }

    .modal {
        background: var(--card-bg, #fff);
        border-radius: 8px;
        box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
        width: 90%;
        max-width: 450px;
        border: 1px solid var(--border-color, #e0e0e0);
    }

    .modal-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 20px;
        border-bottom: 1px solid var(--border-color, #e0e0e0);
    }

    .modal-header h2 {
        margin: 0;
        color: var(--text-color, #333);
        font-size: 1.25rem;
    }

    .close-btn {
        background: none;
        border: none;
        font-size: 1.5rem;
        cursor: pointer;
        color: var(--text-color, #333);
        padding: 0;
        width: 30px;
        height: 30px;
        display: flex;
        align-items: center;
        justify-content: center;
        border-radius: 50%;
    }

    .close-btn:hover {
        background-color: var(--border-color, #e0e0e0);
    }

    .tab-container {
        padding: 20px;
    }

    .tabs {
        display: flex;
        border-bottom: 1px solid var(--border-color, #e0e0e0);
        margin-bottom: 20px;
    }

    .tab-btn {
        flex: 1;
        padding: 12px 16px;
        background: none;
        border: none;
        cursor: pointer;
        color: var(--text-color, #333);
        font-size: 1rem;
        border-bottom: 2px solid transparent;
    }

    .tab-btn:hover {
        background-color: var(--border-color, #e0e0e0);
    }

    .tab-btn.active {
        border-bottom-color: var(--primary, #00bcd4);
        color: var(--primary, #00bcd4);
    }

    .tab-content {
        min-height: 180px;
    }

    .extension-login,
    .nsec-login {
        display: flex;
        flex-direction: column;
        gap: 16px;
    }

    .extension-login p,
    .nsec-login p {
        margin: 0;
        color: var(--muted-color, #666);
        line-height: 1.5;
    }

    .login-btn {
        padding: 12px 24px;
        background: var(--primary, #00bcd4);
        color: white;
        border: none;
        border-radius: 6px;
        cursor: pointer;
        font-size: 1rem;
    }

    .login-btn:hover:not(:disabled) {
        background: var(--primary-hover, #00acc1);
    }

    .login-btn:disabled {
        background: #ccc;
        cursor: not-allowed;
    }

    .nsec-input {
        padding: 12px;
        border: 1px solid var(--border-color, #e0e0e0);
        border-radius: 6px;
        font-size: 1rem;
        background: var(--card-bg, #fff);
        color: var(--text-color, #333);
    }

    .nsec-input:focus {
        outline: none;
        border-color: var(--primary, #00bcd4);
    }

    .generate-btn {
        padding: 10px 20px;
        background: var(--success, #4caf50);
        color: white;
        border: none;
        border-radius: 6px;
        cursor: pointer;
        font-size: 0.95rem;
    }

    .generate-btn:hover:not(:disabled) {
        opacity: 0.9;
    }

    .generate-btn:disabled {
        background: #ccc;
        cursor: not-allowed;
    }

    .generated-info {
        background: var(--bg-color, #f5f5f5);
        padding: 12px;
        border-radius: 6px;
        border: 1px solid var(--border-color, #e0e0e0);
    }

    .generated-info label {
        display: block;
        font-size: 0.85rem;
        color: var(--muted-color, #666);
        margin-bottom: 6px;
    }

    .generated-info code {
        display: block;
        word-break: break-all;
        font-size: 0.8rem;
        color: var(--text-color, #333);
    }

    .message {
        padding: 10px;
        border-radius: 4px;
        margin-top: 16px;
        text-align: center;
    }

    .error-message {
        background: #ffebee;
        color: #c62828;
        border: 1px solid #ffcdd2;
    }

    .success-message {
        background: #e8f5e9;
        color: #2e7d32;
        border: 1px solid #c8e6c9;
    }

    .dark-theme .error-message {
        background: #4a2c2a;
        color: #ffcdd2;
    }

    .dark-theme .success-message {
        background: #2e4a2e;
        color: #a5d6a7;
    }
</style>
