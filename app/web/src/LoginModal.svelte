<script>
    import { createEventDispatcher, onMount, onDestroy } from "svelte";
    import { PrivateKeySigner } from "./nostr.js";
    import { generateSecretKey, getPublicKey } from "nostr-tools/pure";
    import { nsecEncode, npubEncode, decode as nip19Decode } from "nostr-tools/nip19";
    import { encryptNsec, decryptNsec, isValidNsec } from "./nsec-crypto.js";

    const dispatch = createEventDispatcher();

    export let showModal = false;
    export let isDarkTheme = false;

    let activeTab = "extension";
    let nsecInput = "";
    let encryptionPassword = "";
    let confirmPassword = "";
    let unlockPassword = "";
    let isLoading = false;
    let isGenerating = false;
    let isDeriving = false;
    let errorMessage = "";
    let successMessage = "";
    let generatedNsec = "";
    let generatedNpub = "";
    let npubInput = "";

    // Deriving modal timer
    let derivingElapsed = 0;
    let derivingStartTime = null;
    let derivingAnimationFrame = null;

    function startDerivingTimer() {
        derivingElapsed = 0;
        derivingStartTime = performance.now();
        updateDerivingTimer();
    }

    function updateDerivingTimer() {
        if (derivingStartTime !== null) {
            derivingElapsed = (performance.now() - derivingStartTime) / 1000;
            derivingAnimationFrame = requestAnimationFrame(updateDerivingTimer);
        }
    }

    function stopDerivingTimer() {
        derivingStartTime = null;
        if (derivingAnimationFrame) {
            cancelAnimationFrame(derivingAnimationFrame);
            derivingAnimationFrame = null;
        }
    }

    onDestroy(() => {
        stopDerivingTimer();
    });

    // Check if there's an encrypted key stored
    let hasEncryptedKey = false;
    let storedPubkey = "";

    onMount(() => {
        checkStoredCredentials();
    });

    function checkStoredCredentials() {
        hasEncryptedKey = !!localStorage.getItem("nostr_privkey_encrypted");
        storedPubkey = localStorage.getItem("nostr_pubkey") || "";
    }

    // Reset to show the nsec input form
    function clearStoredCredentials() {
        localStorage.removeItem("nostr_privkey_encrypted");
        localStorage.removeItem("nostr_privkey");
        localStorage.removeItem("nostr_pubkey");
        localStorage.removeItem("nostr_auth_method");
        hasEncryptedKey = false;
        storedPubkey = "";
        unlockPassword = "";
        errorMessage = "";
        successMessage = "";
    }

    function closeModal() {
        showModal = false;
        nsecInput = "";
        npubInput = "";
        encryptionPassword = "";
        confirmPassword = "";
        unlockPassword = "";
        errorMessage = "";
        successMessage = "";
        generatedNsec = "";
        generatedNpub = "";
        dispatch("close");
    }

    // Re-check stored credentials when modal opens
    $: if (showModal) {
        checkStoredCredentials();
    }

    // Unlock with stored encrypted key
    async function unlockWithPassword() {
        isLoading = true;
        isDeriving = true;
        startDerivingTimer();
        errorMessage = "";
        successMessage = "";

        try {
            if (!unlockPassword) {
                throw new Error("Please enter your password");
            }

            const encryptedData = localStorage.getItem("nostr_privkey_encrypted");
            if (!encryptedData) {
                throw new Error("No encrypted key found");
            }

            // Decrypt the nsec (library validates bech32 checksum)
            const nsec = await decryptNsec(encryptedData, unlockPassword);

            stopDerivingTimer();
            isDeriving = false;

            // Create signer and login
            const signer = PrivateKeySigner.fromKey(nsec);
            const publicKey = await signer.getPublicKey();

            dispatch("login", {
                method: "nsec",
                pubkey: publicKey,
                privateKey: nsec,
                signer: signer,
            });

            closeModal();
        } catch (error) {
            stopDerivingTimer();
            if (error.message.includes("decrypt") || error.message.includes("tag")) {
                errorMessage = "Invalid password";
            } else {
                errorMessage = error.message;
            }
        } finally {
            isLoading = false;
            isDeriving = false;
            stopDerivingTimer();
        }
    }

    function switchTab(tab) {
        activeTab = tab;
        errorMessage = "";
        successMessage = "";
        generatedNsec = "";
        generatedNpub = "";
    }

    // Generate a new nsec using cryptographically secure random bytes
    async function generateNewKey() {
        isGenerating = true;
        errorMessage = "";
        successMessage = "";

        try {
            // Generate a new secret key using system entropy (crypto.getRandomValues)
            const secretKey = generateSecretKey();

            // Encode as nsec (bech32)
            const nsec = nsecEncode(secretKey);

            // Get the corresponding public key and encode as npub
            const pubkey = getPublicKey(secretKey);
            const npub = npubEncode(pubkey);

            generatedNsec = nsec;
            generatedNpub = npub;
            nsecInput = nsec;

            successMessage = "New key generated! Set an encryption password below to secure it.";
        } catch (error) {
            errorMessage = "Failed to generate key: " + error.message;
        } finally {
            isGenerating = false;
        }
    }

    async function loginWithExtension() {
        isLoading = true;
        errorMessage = "";
        successMessage = "";

        try {
            // Check if window.nostr is available
            if (!window.nostr) {
                throw new Error(
                    "No Nostr extension found. Please install a NIP-07 compatible extension like nos2x or Alby.",
                );
            }

            // Get public key from extension
            const pubkey = await window.nostr.getPublicKey();

            if (pubkey) {
                // Store authentication info
                localStorage.setItem("nostr_auth_method", "extension");
                localStorage.setItem("nostr_pubkey", pubkey);

                successMessage = "Successfully logged in with extension!";
                dispatch("login", {
                    method: "extension",
                    pubkey: pubkey,
                    signer: window.nostr,
                });

                setTimeout(() => {
                    closeModal();
                }, 1500);
            }
        } catch (error) {
            errorMessage = error.message;
        } finally {
            isLoading = false;
        }
    }

    async function loginWithNpub() {
        isLoading = true;
        errorMessage = "";

        try {
            const input = npubInput.trim();
            if (!input) {
                throw new Error("Please enter an npub");
            }

            let pubkey;
            if (/^[0-9a-f]{64}$/i.test(input)) {
                pubkey = input.toLowerCase();
            } else {
                const decoded = nip19Decode(input);
                if (decoded.type !== "npub") {
                    throw new Error("Invalid npub — expected an npub1... string or 64-char hex pubkey");
                }
                pubkey = decoded.data;
            }

            localStorage.setItem("nostr_auth_method", "npub");
            localStorage.setItem("nostr_pubkey", pubkey);

            dispatch("login", {
                method: "npub",
                pubkey: pubkey,
                signer: null,
            });

            closeModal();
        } catch (error) {
            errorMessage = error.message;
        } finally {
            isLoading = false;
        }
    }

    async function loginWithNsec() {
        isLoading = true;
        errorMessage = "";
        successMessage = "";

        try {
            if (!nsecInput.trim()) {
                throw new Error("Please enter your nsec");
            }

            // Validate nsec format and bech32 checksum
            if (!isValidNsec(nsecInput.trim())) {
                throw new Error('Invalid nsec format or checksum');
            }

            // Validate password if provided
            if (encryptionPassword) {
                if (encryptionPassword.length < 8) {
                    throw new Error("Password must be at least 8 characters");
                }
                if (encryptionPassword !== confirmPassword) {
                    throw new Error("Passwords do not match");
                }
            }

            // Create PrivateKeySigner from nsec
            const signer = PrivateKeySigner.fromKey(nsecInput.trim());

            // Get the public key from the signer
            const publicKey = await signer.getPublicKey();

            // Store with encryption if password provided
            localStorage.setItem("nostr_auth_method", "nsec");
            localStorage.setItem("nostr_pubkey", publicKey);

            if (encryptionPassword) {
                // Encrypt the nsec before storing
                isDeriving = true;
                startDerivingTimer();
                const encryptedNsec = await encryptNsec(nsecInput.trim(), encryptionPassword);
                stopDerivingTimer();
                isDeriving = false;
                localStorage.setItem("nostr_privkey_encrypted", encryptedNsec);
                localStorage.removeItem("nostr_privkey"); // Remove any plaintext key
            } else {
                // Store plaintext (less secure)
                localStorage.setItem("nostr_privkey", nsecInput.trim());
                localStorage.removeItem("nostr_privkey_encrypted");
                successMessage = "Successfully logged in with nsec!";
            }

            dispatch("login", {
                method: "nsec",
                pubkey: publicKey,
                privateKey: nsecInput.trim(),
                signer: signer,
            });

            setTimeout(() => {
                closeModal();
            }, 1500);
        } catch (error) {
            errorMessage = error.message;
        } finally {
            isLoading = false;
        }
    }

    function handleKeydown(event) {
        if (event.key === "Escape") {
            closeModal();
        }
        if (event.key === "Enter" && activeTab === "nsec") {
            loginWithNsec();
        }
        if (event.key === "Enter" && activeTab === "npub") {
            loginWithNpub();
        }
    }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if showModal}
    <div
        class="modal-overlay"
        on:click={closeModal}
        on:keydown={(e) => e.key === "Escape" && closeModal()}
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
                <h2>Login to Nostr</h2>
                <button class="close-btn" on:click={closeModal}>&times;</button>
            </div>

            <div class="tab-container">
                <div class="tabs">
                    <button
                        class="tab-btn"
                        class:active={activeTab === "extension"}
                        on:click={() => switchTab("extension")}
                    >
                        Extension
                    </button>
                    <button
                        class="tab-btn"
                        class:active={activeTab === "nsec"}
                        on:click={() => switchTab("nsec")}
                    >
                        Nsec
                    </button>
                    <button
                        class="tab-btn"
                        class:active={activeTab === "npub"}
                        on:click={() => switchTab("npub")}
                    >
                        Read-only
                    </button>
                </div>

                <div class="tab-content">
                    {#if activeTab === "extension"}
                        <div class="extension-login">
                            <p>
                                Login using a NIP-07 compatible browser
                                extension like nos2x or Alby.
                            </p>
                            <button
                                class="login-extension-btn"
                                on:click={loginWithExtension}
                                disabled={isLoading}
                            >
                                {isLoading
                                    ? "Connecting..."
                                    : "Log in using extension"}
                            </button>
                        </div>
                    {:else if activeTab === "npub"}
                        <div class="extension-login">
                            <p>
                                Enter an npub to browse in read-only mode.
                                You won't be able to post or sign events.
                            </p>
                            <input
                                type="text"
                                placeholder="npub1... or hex pubkey"
                                bind:value={npubInput}
                                disabled={isLoading}
                                class="nsec-input"
                            />
                            <button
                                class="login-nsec-btn"
                                on:click={loginWithNpub}
                                disabled={isLoading || !npubInput.trim()}
                            >
                                {isLoading ? "Logging in..." : "Browse read-only"}
                            </button>
                        </div>
                    {:else}
                        <div class="nsec-login">
                            {#if hasEncryptedKey}
                                <!-- Unlock existing encrypted key -->
                                <p>
                                    You have a stored encrypted key. Enter your
                                    password to unlock it.
                                </p>

                                {#if storedPubkey}
                                    <div class="stored-info">
                                        <label>Stored public key:</label>
                                        <code class="npub-display">{storedPubkey.slice(0, 16)}...{storedPubkey.slice(-8)}</code>
                                    </div>
                                {/if}

                                <input
                                    type="password"
                                    placeholder="Enter your password"
                                    bind:value={unlockPassword}
                                    disabled={isLoading || isDeriving}
                                    class="password-input"
                                />

                                <button
                                    class="login-nsec-btn"
                                    on:click={unlockWithPassword}
                                    disabled={isLoading || isDeriving || !unlockPassword}
                                >
                                    {#if isDeriving}
                                        Deriving key...
                                    {:else if isLoading}
                                        Unlocking...
                                    {:else}
                                        Unlock
                                    {/if}
                                </button>

                                <button
                                    class="clear-btn"
                                    on:click={clearStoredCredentials}
                                    disabled={isLoading || isDeriving}
                                >
                                    Clear stored key &amp; start fresh
                                </button>
                            {:else}
                                <!-- Normal nsec entry / generation -->
                                <p>
                                    Enter your nsec or generate a new one. Optionally
                                    set a password to encrypt it securely.
                                </p>

                                <button
                                    class="generate-btn"
                                    on:click={generateNewKey}
                                    disabled={isLoading || isGenerating}
                                >
                                    {isGenerating
                                        ? "Generating..."
                                        : "Generate New Key"}
                                </button>

                                {#if generatedNpub}
                                    <div class="generated-info">
                                        <label>Your new public key (npub):</label>
                                        <code class="npub-display">{generatedNpub}</code>
                                    </div>
                                {/if}

                                <input
                                    type="password"
                                    placeholder="nsec1..."
                                    bind:value={nsecInput}
                                    disabled={isLoading || isDeriving}
                                    class="nsec-input"
                                />

                                <div class="password-section">
                                    <label>Encryption Password (optional but recommended):</label>
                                    <input
                                        type="password"
                                        placeholder="Enter password (min 8 chars)"
                                        bind:value={encryptionPassword}
                                        disabled={isLoading || isDeriving}
                                        class="password-input"
                                    />
                                    {#if encryptionPassword}
                                        <input
                                            type="password"
                                            placeholder="Confirm password"
                                            bind:value={confirmPassword}
                                            disabled={isLoading || isDeriving}
                                            class="password-input"
                                        />
                                {/if}
                                <small class="password-hint">
                                    Password uses Argon2id with ~3 second derivation time for security.
                                </small>
                                </div>

                                <button
                                    class="login-nsec-btn"
                                    on:click={loginWithNsec}
                                    disabled={isLoading || isDeriving || !nsecInput.trim()}
                                >
                                    {#if isDeriving}
                                        Deriving key...
                                    {:else if isLoading}
                                        Logging in...
                                    {:else}
                                        Log in with nsec
                                    {/if}
                                </button>
                            {/if}
                        </div>
                    {/if}

                    {#if errorMessage}
                        <div class="message error-message">{errorMessage}</div>
                    {/if}

                    {#if successMessage}
                        <div class="message success-message">
                            {successMessage}
                        </div>
                    {/if}
                </div>
            </div>
        </div>
    </div>
{/if}

{#if isDeriving}
    <div class="deriving-overlay">
        <div class="deriving-modal" class:dark-theme={isDarkTheme}>
            <div class="deriving-spinner"></div>
            <h3>Deriving encryption key</h3>
            <div class="deriving-timer">{derivingElapsed.toFixed(1)}s</div>
            <p class="deriving-note">This may take 3-6 seconds for security</p>
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
        background: var(--bg-color);
        border-radius: 8px;
        box-shadow: 0 4px 20px rgba(0, 0, 0, 0.3);
        width: 90%;
        max-width: 500px;
        max-height: 90vh;
        overflow-y: auto;
        border: 1px solid var(--border-color);
    }

    .modal-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 20px;
        border-bottom: 1px solid var(--border-color);
    }

    .modal-header h2 {
        margin: 0;
        color: var(--text-color);
        font-size: 1.5rem;
    }

    .close-btn {
        background: none;
        border: none;
        font-size: 1.5rem;
        cursor: pointer;
        color: var(--text-color);
        padding: 0;
        width: 30px;
        height: 30px;
        display: flex;
        align-items: center;
        justify-content: center;
        border-radius: 50%;
        transition: background-color 0.2s;
    }

    .close-btn:hover {
        background-color: var(--tab-hover-bg);
    }

    .tab-container {
        padding: 20px;
    }

    .tabs {
        display: flex;
        border-bottom: 1px solid var(--border-color);
        margin-bottom: 20px;
    }

    .tab-btn {
        flex: 1;
        padding: 12px 16px;
        background: none;
        border: none;
        cursor: pointer;
        color: var(--text-color);
        font-size: 1rem;
        transition: all 0.2s;
        border-bottom: 2px solid transparent;
    }

    .tab-btn:hover {
        background-color: var(--tab-hover-bg);
    }

    .tab-btn.active {
        border-bottom-color: var(--primary);
        color: var(--primary);
    }

    .tab-content {
        min-height: 200px;
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
        color: var(--text-color);
        line-height: 1.5;
    }

    .login-extension-btn,
    .login-nsec-btn {
        padding: 12px 24px;
        background: var(--primary);
        color: var(--text-color);
        border: none;
        border-radius: 6px;
        cursor: pointer;
        font-size: 1rem;
        transition: background-color 0.2s;
    }

    .login-extension-btn:hover:not(:disabled),
    .login-nsec-btn:hover:not(:disabled) {
        background: #00acc1;
    }

    .login-extension-btn:disabled,
    .login-nsec-btn:disabled {
        background: #ccc;
        cursor: not-allowed;
    }

    .nsec-input {
        padding: 12px;
        border: 1px solid var(--input-border);
        border-radius: 6px;
        font-size: 1rem;
        background: var(--bg-color);
        color: var(--text-color);
    }

    .nsec-input:focus {
        outline: none;
        border-color: var(--primary);
    }

    .generate-btn {
        padding: 10px 20px;
        background: var(--success);
        color: white;
        border: none;
        border-radius: 6px;
        cursor: pointer;
        font-size: 0.95rem;
        transition: background-color 0.2s;
    }

    .generate-btn:hover:not(:disabled) {
        background: var(--success);
        filter: brightness(0.9);
    }

    .generate-btn:disabled {
        background: #ccc;
        cursor: not-allowed;
    }

    .generated-info {
        background: var(--card-bg, #f5f5f5);
        padding: 12px;
        border-radius: 6px;
        border: 1px solid var(--border-color);
    }

    .generated-info label {
        display: block;
        font-size: 0.85rem;
        color: var(--muted-foreground, #666);
        margin-bottom: 6px;
    }

    .npub-display {
        display: block;
        word-break: break-all;
        font-size: 0.85rem;
        background: var(--bg-color);
        padding: 8px;
        border-radius: 4px;
        color: var(--text-color);
    }

    .password-section {
        display: flex;
        flex-direction: column;
        gap: 8px;
    }

    .password-section label {
        font-size: 0.9rem;
        color: var(--text-color);
        font-weight: 500;
    }

    .password-input {
        padding: 10px 12px;
        border: 1px solid var(--input-border);
        border-radius: 6px;
        font-size: 0.95rem;
        background: var(--bg-color);
        color: var(--text-color);
    }

    .password-input:focus {
        outline: none;
        border-color: var(--primary);
    }

    .password-hint {
        font-size: 0.8rem;
        color: var(--muted-foreground, #888);
        font-style: italic;
    }

    .stored-info {
        background: var(--card-bg, #f5f5f5);
        padding: 12px;
        border-radius: 6px;
        border: 1px solid var(--border-color);
    }

    .stored-info label {
        display: block;
        font-size: 0.85rem;
        color: var(--muted-foreground, #666);
        margin-bottom: 6px;
    }

    .clear-btn {
        padding: 10px 20px;
        background: transparent;
        color: var(--error, #dc3545);
        border: 1px solid var(--error, #dc3545);
        border-radius: 6px;
        cursor: pointer;
        font-size: 0.9rem;
        transition: all 0.2s;
    }

    .clear-btn:hover:not(:disabled) {
        background: var(--error, #dc3545);
        color: white;
    }

    .clear-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
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
        background: #e8f5e8;
        color: #2e7d32;
        border: 1px solid #c8e6c9;
    }

    .modal.dark-theme .error-message {
        background: #4a2c2a;
        color: #ffcdd2;
        border: 1px solid #6d4c41;
    }

    .modal.dark-theme .success-message {
        background: #2e4a2e;
        color: #a5d6a7;
        border: 1px solid var(--success);
    }

    /* Deriving modal overlay */
    .deriving-overlay {
        position: fixed;
        top: 0;
        left: 0;
        width: 100%;
        height: 100%;
        background-color: rgba(0, 0, 0, 0.7);
        display: flex;
        justify-content: center;
        align-items: center;
        z-index: 2000;
    }

    .deriving-modal {
        background: var(--bg-color, #fff);
        border-radius: 12px;
        padding: 2rem;
        text-align: center;
        box-shadow: 0 8px 32px rgba(0, 0, 0, 0.3);
        min-width: 280px;
    }

    .deriving-modal h3 {
        margin: 1rem 0 0.5rem;
        color: var(--text-color, #333);
        font-size: 1.2rem;
    }

    .deriving-timer {
        font-size: 2.5rem;
        font-weight: bold;
        color: var(--primary);
        font-family: monospace;
        margin: 0.5rem 0;
    }

    .deriving-note {
        margin: 0.5rem 0 0;
        color: var(--muted-foreground, #666);
        font-size: 0.9rem;
    }

    .deriving-spinner {
        width: 48px;
        height: 48px;
        border: 4px solid var(--border-color, #e0e0e0);
        border-top-color: var(--primary);
        border-radius: 50%;
        margin: 0 auto;
        animation: spin 1s linear infinite;
    }

    @keyframes spin {
        to {
            transform: rotate(360deg);
        }
    }

    .deriving-modal.dark-theme {
        background: #1a1a1a;
    }

    .deriving-modal.dark-theme h3 {
        color: #fff;
    }

    .deriving-modal.dark-theme .deriving-note {
        color: #aaa;
    }
</style>
