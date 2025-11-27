<script>
    export let isLoggedIn = false;
    export let userRole = "";
    export let isPolicyAdmin = false;
    export let policyEnabled = false;
    export let policyJson = "";
    export let isLoadingPolicy = false;
    export let policyMessage = "";
    export let policyMessageType = "";
    export let validationErrors = [];
    export let policyAdmins = [];
    export let policyFollows = [];

    import { createEventDispatcher } from "svelte";
    const dispatch = createEventDispatcher();

    // New admin input
    let newAdminInput = "";

    function loadPolicy() {
        dispatch("loadPolicy");
    }

    function validatePolicy() {
        dispatch("validatePolicy");
    }

    function savePolicy() {
        dispatch("savePolicy");
    }

    function formatJson() {
        dispatch("formatJson");
    }

    function openLoginModal() {
        dispatch("openLoginModal");
    }

    function refreshFollows() {
        dispatch("refreshFollows");
    }

    function addPolicyAdmin() {
        if (newAdminInput.trim()) {
            dispatch("addPolicyAdmin", newAdminInput.trim());
            newAdminInput = "";
        }
    }

    function removePolicyAdmin(pubkey) {
        dispatch("removePolicyAdmin", pubkey);
    }

    // Parse admins from current policy JSON for display
    $: {
        try {
            if (policyJson) {
                const parsed = JSON.parse(policyJson);
                policyAdmins = parsed.policy_admins || [];
            }
        } catch (e) {
            // Ignore parse errors
        }
    }

    // Pretty-print example policy for reference
    const examplePolicy = `{
  "kind": {
    "whitelist": [0, 1, 3, 6, 7, 10002],
    "blacklist": []
  },
  "global": {
    "description": "Global rules applied to all events",
    "size_limit": 65536,
    "max_age_of_event": 86400,
    "max_age_event_in_future": 300
  },
  "rules": {
    "1": {
      "description": "Kind 1 (short text notes)",
      "content_limit": 8192,
      "write_allow_follows": true
    },
    "30023": {
      "description": "Long-form articles",
      "content_limit": 100000,
      "tag_validation": {
        "d": "^[a-z0-9-]{1,64}$",
        "t": "^[a-z0-9-]{1,32}$"
      }
    }
  },
  "default_policy": "allow",
  "policy_admins": ["<your-hex-pubkey>"],
  "policy_follow_whitelist_enabled": true
}`;
</script>

<div class="policy-view">
    <h2>Policy Configuration</h2>
    {#if isLoggedIn && (userRole === "owner" || isPolicyAdmin)}
        <div class="policy-section">
            <div class="policy-header">
                <h3>Policy Editor</h3>
                <div class="policy-status">
                    <span class="status-badge" class:enabled={policyEnabled}>
                        {policyEnabled ? "Policy Enabled" : "Policy Disabled"}
                    </span>
                    {#if isPolicyAdmin}
                        <span class="admin-badge">Policy Admin</span>
                    {/if}
                </div>
            </div>

            <div class="policy-info">
                <p>
                    Edit the policy JSON below and click "Save & Publish" to update the relay's policy configuration.
                    Changes are applied immediately after validation.
                </p>
                <p class="info-note">
                    Policy updates are published as kind 12345 events and require policy admin permissions.
                </p>
            </div>

            <div class="editor-container">
                <textarea
                    class="policy-editor"
                    bind:value={policyJson}
                    placeholder="Loading policy configuration..."
                    disabled={isLoadingPolicy}
                    spellcheck="false"
                ></textarea>
            </div>

            {#if validationErrors.length > 0}
                <div class="validation-errors">
                    <h4>Validation Errors:</h4>
                    <ul>
                        {#each validationErrors as error}
                            <li>{error}</li>
                        {/each}
                    </ul>
                </div>
            {/if}

            <div class="policy-actions">
                <button
                    class="policy-btn load-btn"
                    on:click={loadPolicy}
                    disabled={isLoadingPolicy}
                >
                    Load Current
                </button>
                <button
                    class="policy-btn format-btn"
                    on:click={formatJson}
                    disabled={isLoadingPolicy}
                >
                    Format JSON
                </button>
                <button
                    class="policy-btn validate-btn"
                    on:click={validatePolicy}
                    disabled={isLoadingPolicy}
                >
                    Validate
                </button>
                <button
                    class="policy-btn save-btn"
                    on:click={savePolicy}
                    disabled={isLoadingPolicy}
                >
                    Save & Publish
                </button>
            </div>

            {#if policyMessage}
                <div
                    class="policy-message"
                    class:error={policyMessageType === "error"}
                    class:success={policyMessageType === "success"}
                >
                    {policyMessage}
                </div>
            {/if}
        </div>

        <!-- Policy Admins Section -->
        <div class="policy-section">
            <h3>Policy Administrators</h3>
            <div class="policy-info">
                <p>
                    Policy admins can update the relay's policy configuration via kind 12345 events.
                    Their follows get whitelisted if <code>policy_follow_whitelist_enabled</code> is true in the policy.
                </p>
                <p class="info-note">
                    <strong>Note:</strong> Policy admins are separate from relay admins (ORLY_ADMINS).
                    Changes here update the JSON editor - click "Save & Publish" to apply.
                </p>
            </div>

            <div class="admin-list">
                {#if policyAdmins.length === 0}
                    <p class="no-items">No policy admins configured</p>
                {:else}
                    {#each policyAdmins as admin}
                        <div class="admin-item">
                            <span class="admin-pubkey" title={admin}>{admin.substring(0, 16)}...{admin.substring(admin.length - 8)}</span>
                            <button
                                class="remove-btn"
                                on:click={() => removePolicyAdmin(admin)}
                                disabled={isLoadingPolicy}
                                title="Remove admin"
                            >
                                ✕
                            </button>
                        </div>
                    {/each}
                {/if}
            </div>

            <div class="add-admin">
                <input
                    type="text"
                    placeholder="npub or hex pubkey"
                    bind:value={newAdminInput}
                    disabled={isLoadingPolicy}
                    on:keydown={(e) => e.key === "Enter" && addPolicyAdmin()}
                />
                <button
                    class="policy-btn add-btn"
                    on:click={addPolicyAdmin}
                    disabled={isLoadingPolicy || !newAdminInput.trim()}
                >
                    + Add Admin
                </button>
            </div>
        </div>

        <!-- Policy Follow Whitelist Section -->
        <div class="policy-section">
            <h3>Policy Follow Whitelist</h3>
            <div class="policy-info">
                <p>
                    Pubkeys followed by policy admins (kind 3 events).
                    These get automatic read+write access when rules have <code>write_allow_follows: true</code>.
                </p>
            </div>

            <div class="follows-header">
                <span class="follows-count">{policyFollows.length} pubkey(s) in whitelist</span>
                <button
                    class="policy-btn refresh-btn"
                    on:click={refreshFollows}
                    disabled={isLoadingPolicy}
                >
                    🔄 Refresh Follows
                </button>
            </div>

            <div class="follows-list">
                {#if policyFollows.length === 0}
                    <p class="no-items">No follows loaded. Click "Refresh Follows" to load from database.</p>
                {:else}
                    <div class="follows-grid">
                        {#each policyFollows as follow}
                            <div class="follow-item" title={follow}>
                                {follow.substring(0, 12)}...{follow.substring(follow.length - 6)}
                            </div>
                        {/each}
                    </div>
                {/if}
            </div>
        </div>

        <div class="policy-section">
            <h3>Policy Reference</h3>
            <div class="reference-content">
                <h4>Structure Overview</h4>
                <ul class="field-list">
                    <li><code>kind.whitelist</code> - Only allow these event kinds (takes precedence)</li>
                    <li><code>kind.blacklist</code> - Deny these event kinds (if no whitelist)</li>
                    <li><code>global</code> - Rules applied to all events</li>
                    <li><code>rules</code> - Per-kind rules (keyed by kind number as string)</li>
                    <li><code>default_policy</code> - "allow" or "deny" when no rules match</li>
                    <li><code>policy_admins</code> - Hex pubkeys that can update policy</li>
                    <li><code>policy_follow_whitelist_enabled</code> - Enable follow-based access</li>
                </ul>

                <h4>Rule Fields</h4>
                <ul class="field-list">
                    <li><code>description</code> - Human-readable rule description</li>
                    <li><code>write_allow</code> / <code>write_deny</code> - Pubkey lists for write access</li>
                    <li><code>read_allow</code> / <code>read_deny</code> - Pubkey lists for read access</li>
                    <li><code>write_allow_follows</code> - Grant access to policy admin follows</li>
                    <li><code>size_limit</code> - Max total event size in bytes</li>
                    <li><code>content_limit</code> - Max content field size in bytes</li>
                    <li><code>max_expiry</code> - Max expiry offset in seconds</li>
                    <li><code>max_age_of_event</code> - Max age of created_at in seconds</li>
                    <li><code>max_age_event_in_future</code> - Max future offset in seconds</li>
                    <li><code>must_have_tags</code> - Required tag letters (e.g., ["d", "t"])</li>
                    <li><code>tag_validation</code> - Regex patterns for tag values</li>
                    <li><code>script</code> - Path to external validation script</li>
                </ul>

                <h4>Example Policy</h4>
                <pre class="example-json">{examplePolicy}</pre>
            </div>
        </div>
    {:else if isLoggedIn}
        <div class="permission-denied">
            <p>Policy configuration requires owner or policy admin permissions.</p>
            <p>
                To become a policy admin, ask an existing policy admin to add your pubkey
                to the <code>policy_admins</code> list.
            </p>
            <p>
                Current user role: <strong>{userRole || "none"}</strong>
            </p>
        </div>
    {:else}
        <div class="login-prompt">
            <p>Please log in to access policy configuration.</p>
            <button class="login-btn" on:click={openLoginModal}>Log In</button>
        </div>
    {/if}
</div>

<style>
    .policy-view {
        width: 100%;
        max-width: 1200px;
        margin: 0;
        padding: 20px;
        background: var(--header-bg);
        color: var(--text-color);
        border-radius: 8px;
    }

    .policy-view h2 {
        margin: 0 0 1.5rem 0;
        color: var(--text-color);
        font-size: 1.8rem;
        font-weight: 600;
    }

    .policy-section {
        background-color: var(--card-bg);
        border-radius: 8px;
        padding: 1.5em;
        margin-bottom: 1.5rem;
        border: 1px solid var(--border-color);
    }

    .policy-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 1rem;
    }

    .policy-header h3 {
        margin: 0;
        color: var(--text-color);
        font-size: 1.2rem;
        font-weight: 600;
    }

    .policy-status {
        display: flex;
        gap: 0.5rem;
    }

    .status-badge {
        padding: 0.25em 0.75em;
        border-radius: 1rem;
        font-size: 0.8em;
        font-weight: 600;
        background: var(--danger);
        color: white;
    }

    .status-badge.enabled {
        background: var(--success);
    }

    .admin-badge {
        padding: 0.25em 0.75em;
        border-radius: 1rem;
        font-size: 0.8em;
        font-weight: 600;
        background: var(--primary);
        color: white;
    }

    .policy-info {
        margin-bottom: 1rem;
        padding: 1rem;
        background: var(--bg-color);
        border-radius: 4px;
        border: 1px solid var(--border-color);
    }

    .policy-info p {
        margin: 0 0 0.5rem 0;
        line-height: 1.5;
    }

    .policy-info p:last-child {
        margin-bottom: 0;
    }

    .info-note {
        font-size: 0.9em;
        opacity: 0.8;
    }

    .editor-container {
        margin-bottom: 1rem;
    }

    .policy-editor {
        width: 100%;
        height: 400px;
        padding: 1em;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--input-bg);
        color: var(--input-text-color);
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        font-size: 0.85em;
        line-height: 1.5;
        resize: vertical;
        tab-size: 2;
    }

    .policy-editor:disabled {
        opacity: 0.6;
        cursor: not-allowed;
    }

    .validation-errors {
        margin-bottom: 1rem;
        padding: 1rem;
        background: var(--danger-bg, rgba(220, 53, 69, 0.1));
        border: 1px solid var(--danger);
        border-radius: 4px;
    }

    .validation-errors h4 {
        margin: 0 0 0.5rem 0;
        color: var(--danger);
        font-size: 1rem;
    }

    .validation-errors ul {
        margin: 0;
        padding-left: 1.5rem;
    }

    .validation-errors li {
        color: var(--danger);
        margin-bottom: 0.25rem;
    }

    .policy-actions {
        display: flex;
        gap: 0.5rem;
        flex-wrap: wrap;
    }

    .policy-btn {
        background: var(--primary);
        color: white;
        border: none;
        padding: 0.5em 1em;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9em;
        transition: background-color 0.2s, filter 0.2s;
        display: flex;
        align-items: center;
        gap: 0.25em;
    }

    .policy-btn:hover:not(:disabled) {
        filter: brightness(1.1);
    }

    .policy-btn:disabled {
        background: var(--secondary);
        cursor: not-allowed;
    }

    .load-btn {
        background: var(--info);
    }

    .format-btn {
        background: var(--secondary);
    }

    .validate-btn {
        background: var(--warning);
    }

    .save-btn {
        background: var(--success);
    }

    .policy-message {
        padding: 1rem;
        border-radius: 4px;
        margin-top: 1rem;
        background: var(--info-bg, rgba(23, 162, 184, 0.1));
        color: var(--info-text, var(--text-color));
        border: 1px solid var(--info);
    }

    .policy-message.error {
        background: var(--danger-bg, rgba(220, 53, 69, 0.1));
        color: var(--danger-text, var(--danger));
        border: 1px solid var(--danger);
    }

    .policy-message.success {
        background: var(--success-bg, rgba(40, 167, 69, 0.1));
        color: var(--success-text, var(--success));
        border: 1px solid var(--success);
    }

    .reference-content h4 {
        margin: 1rem 0 0.5rem 0;
        color: var(--text-color);
        font-size: 1rem;
    }

    .reference-content h4:first-child {
        margin-top: 0;
    }

    .field-list {
        margin: 0 0 1rem 0;
        padding-left: 1.5rem;
    }

    .field-list li {
        margin-bottom: 0.25rem;
        line-height: 1.5;
    }

    .field-list code {
        background: var(--code-bg, rgba(0, 0, 0, 0.1));
        padding: 0.1em 0.4em;
        border-radius: 3px;
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        font-size: 0.9em;
    }

    .example-json {
        background: var(--input-bg);
        color: var(--input-text-color);
        padding: 1rem;
        border-radius: 4px;
        border: 1px solid var(--border-color);
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        font-size: 0.8em;
        line-height: 1.4;
        overflow-x: auto;
        white-space: pre;
        margin: 0;
    }

    .permission-denied,
    .login-prompt {
        text-align: center;
        padding: 2em;
        background-color: var(--card-bg);
        border-radius: 8px;
        border: 1px solid var(--border-color);
        color: var(--text-color);
    }

    .permission-denied p,
    .login-prompt p {
        margin: 0 0 1rem 0;
        line-height: 1.4;
    }

    .permission-denied code {
        background: var(--code-bg, rgba(0, 0, 0, 0.1));
        padding: 0.2em 0.4em;
        border-radius: 0.25rem;
        font-family: monospace;
        font-size: 0.9em;
    }

    .login-btn {
        background: var(--primary);
        color: white;
        border: none;
        padding: 0.75em 1.5em;
        border-radius: 4px;
        cursor: pointer;
        font-weight: bold;
        font-size: 0.9em;
        transition: background-color 0.2s;
    }

    .login-btn:hover {
        filter: brightness(1.1);
    }

    /* Admin list styles */
    .admin-list {
        margin-bottom: 1rem;
    }

    .admin-item {
        display: flex;
        justify-content: space-between;
        align-items: center;
        padding: 0.5em 0.75em;
        background: var(--bg-color);
        border: 1px solid var(--border-color);
        border-radius: 4px;
        margin-bottom: 0.5rem;
    }

    .admin-pubkey {
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        font-size: 0.85em;
        color: var(--text-color);
    }

    .remove-btn {
        background: var(--danger);
        color: white;
        border: none;
        width: 24px;
        height: 24px;
        border-radius: 50%;
        cursor: pointer;
        font-size: 0.8em;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: filter 0.2s;
    }

    .remove-btn:hover:not(:disabled) {
        filter: brightness(0.9);
    }

    .remove-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .add-admin {
        display: flex;
        gap: 0.5rem;
    }

    .add-admin input {
        flex: 1;
        padding: 0.5em 0.75em;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--input-bg);
        color: var(--input-text-color);
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        font-size: 0.85em;
    }

    .add-btn {
        background: var(--success);
        white-space: nowrap;
    }

    .no-items {
        color: var(--text-color);
        opacity: 0.6;
        font-style: italic;
        padding: 1rem;
        text-align: center;
    }

    /* Follow list styles */
    .follows-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 1rem;
    }

    .follows-count {
        font-weight: 600;
        color: var(--text-color);
    }

    .refresh-btn {
        background: var(--info);
    }

    .follows-list {
        max-height: 300px;
        overflow-y: auto;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--bg-color);
    }

    .follows-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
        gap: 0.5rem;
        padding: 0.75rem;
    }

    .follow-item {
        padding: 0.4em 0.6em;
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 4px;
        font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        font-size: 0.75em;
        color: var(--text-color);
        text-overflow: ellipsis;
        overflow: hidden;
        white-space: nowrap;
    }
</style>
