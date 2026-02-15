<script>
    export let isLoggedIn = false;
    export let userRole = "";
    export let boltConfig = null;
    export let isLoading = false;
    export let message = "";
    export let messageType = "info";

    import { createEventDispatcher } from "svelte";
    const dispatch = createEventDispatcher();

    let pendingEnabled = null;

    $: isDirty = boltConfig && pendingEnabled !== null && pendingEnabled !== boltConfig.bolt_s_enabled;

    function handleToggle() {
        if (boltConfig) {
            pendingEnabled = !currentEnabled;
        }
    }

    $: currentEnabled = pendingEnabled !== null ? pendingEnabled : (boltConfig ? boltConfig.bolt_s_enabled : false);

    function applyChanges() {
        dispatch("toggleBolt", { enabled: pendingEnabled });
    }

    function loadConfig() {
        dispatch("loadBoltConfig");
    }

    function openLoginModal() {
        dispatch("openLoginModal");
    }

    async function copyToClipboard(text) {
        try {
            await navigator.clipboard.writeText(text);
            copiedField = text;
            setTimeout(() => { copiedField = ""; }, 2000);
        } catch {
            // fallback
            const ta = document.createElement("textarea");
            ta.value = text;
            document.body.appendChild(ta);
            ta.select();
            document.execCommand("copy");
            document.body.removeChild(ta);
            copiedField = text;
            setTimeout(() => { copiedField = ""; }, 2000);
        }
    }

    let copiedField = "";
</script>

<div class="neo4j-view">
    <h2>Neo4j Database</h2>
    {#if isLoggedIn && userRole === "owner"}
        <div class="neo4j-section">
            <div class="neo4j-header">
                <h3>Bolt+S External Access</h3>
                {#if !boltConfig && !isLoading}
                    <button class="neo4j-btn" on:click={loadConfig}>Load Configuration</button>
                {/if}
            </div>

            {#if isLoading}
                <div class="neo4j-loading">
                    <div class="spinner"></div>
                    <span>Loading configuration...</span>
                </div>
            {:else if boltConfig}
                <div class="neo4j-status">
                    <div class="status-row">
                        <span class="status-label">Status</span>
                        <span class="status-badge" class:enabled={boltConfig.bolt_s_enabled}>
                            {boltConfig.bolt_s_enabled ? "Enabled" : "Disabled"}
                        </span>
                    </div>
                    <div class="status-row">
                        <span class="status-label">Config Path</span>
                        <span class="status-value">{boltConfig.conf_path}</span>
                    </div>
                    <div class="status-row">
                        <span class="status-label">Bolt Port</span>
                        <span class="status-value">{boltConfig.bolt_port}</span>
                    </div>
                    {#if boltConfig.tls_cert_dir}
                        <div class="status-row">
                            <span class="status-label">TLS Cert Dir</span>
                            <span class="status-value">{boltConfig.tls_cert_dir}</span>
                        </div>
                    {/if}
                </div>

                {#if !boltConfig.has_cert_dir}
                    <div class="neo4j-warning">
                        <strong>Setup required before enabling bolt+s:</strong>
                        <ol class="setup-steps">
                            <li>Set the TLS certificate directory:<br/>
                                <code>ORLY_NEO4J_TLS_CERT_DIR=/etc/letsencrypt/live/example.com</code>
                            </li>
                            <li>Grant the relay user permission to restart Neo4j:<br/>
                                <code>echo 'mleku ALL=(ALL) NOPASSWD: /usr/bin/systemctl restart neo4j' | sudo tee /etc/sudoers.d/orly-neo4j</code>
                            </li>
                            <li>Ensure Neo4j can read the TLS certificates:<br/>
                                <code>sudo chmod -R 755 /etc/letsencrypt/live/ /etc/letsencrypt/archive/</code>
                            </li>
                            <li>Open bolt port in firewall if needed:<br/>
                                <code>sudo ufw allow {boltConfig.bolt_port}/tcp</code>
                            </li>
                            <li>Restart the relay to pick up the new env var, then return here to enable bolt+s.</li>
                        </ol>
                    </div>
                {/if}

                <div class="toggle-section">
                    <label class="toggle-container" class:disabled={!boltConfig.has_cert_dir}>
                        <input
                            type="checkbox"
                            checked={currentEnabled}
                            on:change={handleToggle}
                            disabled={!boltConfig.has_cert_dir}
                        />
                        <span class="toggle-slider"></span>
                        <span class="toggle-label">
                            {currentEnabled ? "Bolt+S Enabled" : "Bolt+S Disabled"}
                        </span>
                    </label>
                </div>

                {#if isDirty}
                    <div class="apply-section">
                        <p class="apply-warning">
                            Configuration has changed. Click below to update <code>neo4j.conf</code> and restart Neo4j.
                        </p>
                        <button
                            class="neo4j-btn apply-btn"
                            on:click={applyChanges}
                            disabled={isLoading}
                        >
                            Apply &amp; Restart Neo4j
                        </button>
                    </div>
                {/if}

                {#if boltConfig.bolt_s_enabled && boltConfig.bolt_uri}
                    <div class="connection-info">
                        <h4>Connection Info</h4>
                        <div class="connection-row">
                            <span class="connection-label">Bolt+S URI</span>
                            <div class="connection-value-group">
                                <code class="connection-value">{boltConfig.bolt_uri}</code>
                                <button class="copy-btn" on:click={() => copyToClipboard(boltConfig.bolt_uri)}>
                                    {copiedField === boltConfig.bolt_uri ? "Copied" : "Copy"}
                                </button>
                            </div>
                        </div>
                    </div>
                {/if}

                {#if message}
                    <div class="neo4j-message" class:error={messageType === "error"} class:success={messageType === "success"}>
                        {message}
                    </div>
                {/if}
            {/if}
        </div>
    {:else if isLoggedIn}
        <div class="permission-denied">
            <p>Owner permission required to manage Neo4j configuration.</p>
        </div>
    {:else}
        <div class="login-prompt">
            <p>Please log in with owner permissions to access Neo4j management.</p>
            <button class="neo4j-btn" on:click={openLoginModal}>Log In</button>
        </div>
    {/if}
</div>

<style>
    .neo4j-view {
        padding: 1.5rem;
        max-width: 40em;
    }

    .neo4j-view h2 {
        margin: 0 0 1.5rem 0;
        color: var(--text-color);
    }

    .neo4j-section {
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 8px;
        padding: 1.5rem;
    }

    .neo4j-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 1rem;
    }

    .neo4j-header h3 {
        margin: 0;
        color: var(--text-color);
    }

    .neo4j-loading {
        display: flex;
        align-items: center;
        gap: 0.75rem;
        padding: 1rem 0;
        color: var(--text-color);
    }

    .spinner {
        width: 20px;
        height: 20px;
        border: 2px solid var(--border-color);
        border-top-color: var(--primary);
        border-radius: 50%;
        animation: spin 0.6s linear infinite;
    }

    @keyframes spin {
        to { transform: rotate(360deg); }
    }

    .neo4j-status {
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
        margin-bottom: 1rem;
        padding: 0.75rem;
        background: var(--bg-color);
        border-radius: 6px;
    }

    .status-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 1rem;
    }

    .status-label {
        font-weight: 600;
        font-size: 0.9em;
        color: var(--text-color);
        opacity: 0.8;
    }

    .status-value {
        font-family: monospace;
        font-size: 0.85em;
        color: var(--text-color);
    }

    .status-badge {
        background: var(--secondary);
        color: var(--text-color);
        padding: 0.2em 0.6em;
        border-radius: 4px;
        font-weight: 600;
        font-size: 0.85em;
    }

    .status-badge.enabled {
        background: var(--success);
        color: var(--success-text);
    }

    .neo4j-warning {
        background: var(--warning, #fff3cd);
        color: var(--warning-text, #856404);
        border: 1px solid var(--warning, #ffc107);
        border-radius: 6px;
        padding: 0.75rem 1rem;
        margin-bottom: 1rem;
        font-size: 0.9em;
    }

    .neo4j-warning code {
        background: rgba(0, 0, 0, 0.1);
        padding: 0.1em 0.3em;
        border-radius: 3px;
        font-size: 0.85em;
    }

    .setup-steps {
        margin: 0.5rem 0 0 0;
        padding-left: 1.5rem;
    }

    .setup-steps li {
        margin-bottom: 0.5rem;
        line-height: 1.5;
    }

    .toggle-section {
        margin: 1rem 0;
    }

    .toggle-container {
        display: flex;
        align-items: center;
        gap: 0.75rem;
        cursor: pointer;
        user-select: none;
    }

    .toggle-container.disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .toggle-container input[type="checkbox"] {
        display: none;
    }

    .toggle-slider {
        width: 44px;
        height: 22px;
        background: var(--border-color);
        border-radius: 11px;
        position: relative;
        transition: background 0.2s;
        flex-shrink: 0;
    }

    .toggle-slider::before {
        content: "";
        position: absolute;
        width: 18px;
        height: 18px;
        background: var(--text-color);
        border-radius: 50%;
        top: 2px;
        left: 2px;
        transition: transform 0.2s;
    }

    .toggle-container input:checked + .toggle-slider {
        background: var(--primary);
    }

    .toggle-container input:checked + .toggle-slider::before {
        transform: translateX(22px);
    }

    .toggle-label {
        font-size: 0.95em;
        font-weight: 500;
        color: var(--text-color);
    }

    .apply-section {
        margin: 1rem 0;
        padding: 0.75rem;
        background: var(--bg-color);
        border: 1px dashed var(--border-color);
        border-radius: 6px;
    }

    .apply-warning {
        margin: 0 0 0.75rem 0;
        font-size: 0.9em;
        color: var(--text-color);
        opacity: 0.8;
    }

    .apply-warning code {
        background: rgba(0, 0, 0, 0.1);
        padding: 0.1em 0.3em;
        border-radius: 3px;
    }

    .connection-info {
        margin-top: 1.5rem;
        padding: 1rem;
        background: var(--bg-color);
        border: 1px solid var(--border-color);
        border-radius: 6px;
    }

    .connection-info h4 {
        margin: 0 0 0.75rem 0;
        color: var(--text-color);
    }

    .connection-row {
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 1rem;
    }

    .connection-label {
        font-weight: 600;
        font-size: 0.9em;
        color: var(--text-color);
        opacity: 0.8;
        flex-shrink: 0;
    }

    .connection-value-group {
        display: flex;
        align-items: center;
        gap: 0.5rem;
    }

    .connection-value {
        background: var(--code-bg, var(--input-bg));
        padding: 0.3em 0.6em;
        border-radius: 4px;
        font-size: 0.9em;
        color: var(--text-color);
        word-break: break-all;
    }

    .neo4j-btn {
        background: var(--primary);
        color: var(--text-color);
        border: none;
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        transition: background-color 0.2s;
    }

    .neo4j-btn:hover:not(:disabled) {
        background: var(--accent-hover-color);
    }

    .neo4j-btn:disabled {
        background: var(--secondary);
        cursor: not-allowed;
        opacity: 0.6;
    }

    .apply-btn {
        background: var(--warning, #ffc107);
        color: var(--warning-text, #000);
        font-weight: 600;
    }

    .apply-btn:hover:not(:disabled) {
        opacity: 0.9;
    }

    .copy-btn {
        background: var(--secondary);
        color: var(--text-color);
        border: none;
        padding: 0.25em 0.5em;
        border-radius: 3px;
        cursor: pointer;
        font-size: 0.8em;
        transition: background-color 0.2s;
    }

    .copy-btn:hover {
        background: var(--accent-hover-color);
    }

    .neo4j-message {
        padding: 0.75rem 1rem;
        border-radius: 4px;
        margin-top: 1rem;
        background: var(--success-bg);
        color: var(--success-text);
        border: 1px solid var(--success);
        font-size: 0.9em;
    }

    .neo4j-message.error {
        background: var(--danger-bg);
        color: var(--danger-text);
        border: 1px solid var(--danger);
    }

    .neo4j-message.success {
        background: var(--success-bg);
        color: var(--success-text);
        border: 1px solid var(--success);
    }

    .permission-denied,
    .login-prompt {
        text-align: center;
        padding: 2rem;
        color: var(--text-color);
    }

    .login-prompt .neo4j-btn {
        margin-top: 1rem;
    }
</style>
