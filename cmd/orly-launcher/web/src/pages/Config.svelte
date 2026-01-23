<script>
    import { onMount } from 'svelte';
    import { userSigner, userPubkey, configData, isLoading, error } from '../stores.js';
    import { fetchConfig, saveConfig, restartServices } from '../api.js';

    let editMode = false;
    let editedConfig = {};
    let saveMessage = '';
    let saveSuccess = false;
    let isSaving = false;

    onMount(async () => {
        await loadConfig();
    });

    async function loadConfig() {
        $isLoading = true;
        try {
            $configData = await fetchConfig($userSigner, $userPubkey);
            editedConfig = JSON.parse(JSON.stringify($configData)); // Deep copy
            $error = '';
        } catch (e) {
            $error = e.message;
        } finally {
            $isLoading = false;
        }
    }

    function startEdit() {
        editedConfig = JSON.parse(JSON.stringify($configData));
        editMode = true;
        saveMessage = '';
    }

    function cancelEdit() {
        editedConfig = JSON.parse(JSON.stringify($configData));
        editMode = false;
        saveMessage = '';
    }

    async function handleSave() {
        isSaving = true;
        saveMessage = '';
        try {
            const result = await saveConfig($userSigner, $userPubkey, editedConfig);
            saveSuccess = result.success;
            saveMessage = result.message;
            if (result.success) {
                $configData = { ...editedConfig };
                editMode = false;
            }
        } catch (e) {
            saveSuccess = false;
            saveMessage = e.message;
        } finally {
            isSaving = false;
        }
    }

    async function handleRestart() {
        if (!confirm('Restart all services? This will briefly interrupt the relay.')) {
            return;
        }
        try {
            await restartServices($userSigner, $userPubkey);
            saveMessage = 'Restart initiated. Services are restarting...';
            saveSuccess = true;
        } catch (e) {
            saveMessage = e.message;
            saveSuccess = false;
        }
    }

    function addOwner() {
        const newOwner = prompt('Enter hex pubkey for new admin owner:');
        if (newOwner && newOwner.match(/^[0-9a-fA-F]{64}$/)) {
            editedConfig.admin_owners = [...(editedConfig.admin_owners || []), newOwner.toLowerCase()];
        } else if (newOwner) {
            alert('Invalid pubkey. Must be 64 hex characters.');
        }
    }

    function removeOwner(index) {
        editedConfig.admin_owners = editedConfig.admin_owners.filter((_, i) => i !== index);
    }
</script>

<div class="config-page">
    <div class="page-header">
        <h2>Configuration</h2>
        <div class="header-buttons">
            {#if editMode}
                <button class="cancel-btn" on:click={cancelEdit} disabled={isSaving}>Cancel</button>
                <button class="save-btn" on:click={handleSave} disabled={isSaving}>
                    {isSaving ? 'Saving...' : 'Save'}
                </button>
            {:else}
                <button class="refresh-btn" on:click={loadConfig} disabled={$isLoading}>Refresh</button>
                <button class="edit-btn" on:click={startEdit} disabled={$isLoading || !$configData}>Edit</button>
            {/if}
        </div>
    </div>

    {#if $error}
        <div class="error-banner">{$error}</div>
    {/if}

    {#if saveMessage}
        <div class="message-banner" class:success={saveSuccess} class:error={!saveSuccess}>
            {saveMessage}
            {#if saveSuccess && saveMessage.includes('Restart required')}
                <button class="restart-btn-inline" on:click={handleRestart}>Restart Now</button>
            {/if}
        </div>
    {/if}

    {#if $configData}
        <div class="config-sections">
            <section class="config-section">
                <h3>Database</h3>
                <div class="config-grid">
                    <div class="config-item">
                        <label class="label">Backend</label>
                        {#if editMode}
                            <select bind:value={editedConfig.db_backend}>
                                <option value="badger">Badger</option>
                                <option value="neo4j">Neo4j</option>
                            </select>
                        {:else}
                            <span class="value">{$configData.db_backend}</span>
                        {/if}
                    </div>
                    <div class="config-item">
                        <label class="label">Binary</label>
                        {#if editMode}
                            <input type="text" bind:value={editedConfig.db_binary} placeholder="orly-db-badger" />
                        {:else}
                            <span class="value mono">{$configData.db_binary}</span>
                        {/if}
                    </div>
                    <div class="config-item">
                        <label class="label">Listen Address</label>
                        {#if editMode}
                            <input type="text" bind:value={editedConfig.db_listen} placeholder="127.0.0.1:50051" />
                        {:else}
                            <span class="value mono">{$configData.db_listen}</span>
                        {/if}
                    </div>
                    <div class="config-item">
                        <label class="label">Data Directory</label>
                        {#if editMode}
                            <input type="text" bind:value={editedConfig.data_dir} />
                        {:else}
                            <span class="value mono">{$configData.data_dir}</span>
                        {/if}
                    </div>
                </div>
            </section>

            <section class="config-section">
                <h3>ACL</h3>
                <div class="config-grid">
                    <div class="config-item">
                        <label class="label">Enabled</label>
                        {#if editMode}
                            <label class="toggle">
                                <input type="checkbox" bind:checked={editedConfig.acl_enabled} />
                                <span>{editedConfig.acl_enabled ? 'Enabled' : 'Disabled'}</span>
                            </label>
                        {:else}
                            <span class="value bool" class:enabled={$configData.acl_enabled}>
                                {$configData.acl_enabled ? 'Yes' : 'No'}
                            </span>
                        {/if}
                    </div>
                    <div class="config-item">
                        <label class="label">Mode</label>
                        {#if editMode}
                            <select bind:value={editedConfig.acl_mode}>
                                <option value="follows">Follows</option>
                                <option value="managed">Managed</option>
                                <option value="curation">Curation</option>
                            </select>
                        {:else}
                            <span class="value">{$configData.acl_mode}</span>
                        {/if}
                    </div>
                    <div class="config-item">
                        <label class="label">Binary</label>
                        {#if editMode}
                            <input type="text" bind:value={editedConfig.acl_binary} />
                        {:else}
                            <span class="value mono">{$configData.acl_binary}</span>
                        {/if}
                    </div>
                    <div class="config-item">
                        <label class="label">Listen Address</label>
                        {#if editMode}
                            <input type="text" bind:value={editedConfig.acl_listen} placeholder="127.0.0.1:50052" />
                        {:else}
                            <span class="value mono">{$configData.acl_listen}</span>
                        {/if}
                    </div>
                </div>
            </section>

            <section class="config-section">
                <h3>Relay</h3>
                <div class="config-grid">
                    <div class="config-item">
                        <label class="label">Binary</label>
                        {#if editMode}
                            <input type="text" bind:value={editedConfig.relay_binary} placeholder="orly" />
                        {:else}
                            <span class="value mono">{$configData.relay_binary}</span>
                        {/if}
                    </div>
                    <div class="config-item">
                        <label class="label">Log Level</label>
                        {#if editMode}
                            <select bind:value={editedConfig.log_level}>
                                <option value="trace">Trace</option>
                                <option value="debug">Debug</option>
                                <option value="info">Info</option>
                                <option value="warn">Warn</option>
                                <option value="error">Error</option>
                            </select>
                        {:else}
                            <span class="value">{$configData.log_level}</span>
                        {/if}
                    </div>
                </div>
            </section>

            <section class="config-section">
                <h3>Sync Services</h3>
                <div class="config-grid">
                    <div class="config-item">
                        <label class="label">Distributed Sync</label>
                        {#if editMode}
                            <label class="toggle">
                                <input type="checkbox" bind:checked={editedConfig.distributed_sync_enabled} />
                                <span>{editedConfig.distributed_sync_enabled ? 'Enabled' : 'Disabled'}</span>
                            </label>
                        {:else}
                            <span class="value bool" class:enabled={$configData.distributed_sync_enabled}>
                                {$configData.distributed_sync_enabled ? 'Enabled' : 'Disabled'}
                            </span>
                        {/if}
                    </div>
                    <div class="config-item">
                        <label class="label">Cluster Sync</label>
                        {#if editMode}
                            <label class="toggle">
                                <input type="checkbox" bind:checked={editedConfig.cluster_sync_enabled} />
                                <span>{editedConfig.cluster_sync_enabled ? 'Enabled' : 'Disabled'}</span>
                            </label>
                        {:else}
                            <span class="value bool" class:enabled={$configData.cluster_sync_enabled}>
                                {$configData.cluster_sync_enabled ? 'Enabled' : 'Disabled'}
                            </span>
                        {/if}
                    </div>
                    <div class="config-item">
                        <label class="label">Relay Group</label>
                        {#if editMode}
                            <label class="toggle">
                                <input type="checkbox" bind:checked={editedConfig.relay_group_enabled} />
                                <span>{editedConfig.relay_group_enabled ? 'Enabled' : 'Disabled'}</span>
                            </label>
                        {:else}
                            <span class="value bool" class:enabled={$configData.relay_group_enabled}>
                                {$configData.relay_group_enabled ? 'Enabled' : 'Disabled'}
                            </span>
                        {/if}
                    </div>
                    <div class="config-item">
                        <label class="label">Negentropy</label>
                        {#if editMode}
                            <label class="toggle">
                                <input type="checkbox" bind:checked={editedConfig.negentropy_enabled} />
                                <span>{editedConfig.negentropy_enabled ? 'Enabled' : 'Disabled'}</span>
                            </label>
                        {:else}
                            <span class="value bool" class:enabled={$configData.negentropy_enabled}>
                                {$configData.negentropy_enabled ? 'Enabled' : 'Disabled'}
                            </span>
                        {/if}
                    </div>
                </div>
            </section>

            <section class="config-section">
                <h3>Admin</h3>
                <div class="config-grid">
                    <div class="config-item">
                        <label class="label">Binary Directory</label>
                        {#if editMode}
                            <input type="text" bind:value={editedConfig.bin_dir} />
                        {:else}
                            <span class="value mono">{$configData.bin_dir}</span>
                        {/if}
                    </div>
                    <div class="config-item full-width">
                        <label class="label">
                            Admin Owners
                            {#if editMode}
                                <button class="add-owner-btn" on:click={addOwner}>+ Add</button>
                            {/if}
                        </label>
                        <div class="owners-list">
                            {#each (editMode ? editedConfig.admin_owners : $configData.admin_owners) || [] as owner, index}
                                <div class="owner-item">
                                    <code class="owner">{owner}</code>
                                    {#if editMode}
                                        <button class="remove-owner-btn" on:click={() => removeOwner(index)}>x</button>
                                    {/if}
                                </div>
                            {:else}
                                <span class="no-owners">No owners configured</span>
                            {/each}
                        </div>
                    </div>
                </div>
            </section>
        </div>

        {#if !editMode}
            <div class="config-note">
                <p>Configuration is saved to <code>{$configData.bin_dir?.replace(/\/bin$/, '')}/launcher.json</code>. Environment variables override file settings.</p>
            </div>
        {/if}
    {:else if !$error}
        <div class="loading">Loading configuration...</div>
    {/if}
</div>

<style>
    .config-page {
        padding: 20px 0;
    }

    .page-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 24px;
    }

    .page-header h2 {
        font-size: 1.5rem;
        color: var(--text-color);
    }

    .header-buttons {
        display: flex;
        gap: 8px;
    }

    .refresh-btn, .edit-btn, .cancel-btn, .save-btn {
        padding: 8px 16px;
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        color: var(--text-color);
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9rem;
    }

    .edit-btn {
        background: var(--primary);
        border-color: var(--primary);
        color: white;
    }

    .save-btn {
        background: var(--success);
        border-color: var(--success);
        color: white;
    }

    .cancel-btn:hover:not(:disabled) {
        background: var(--border-color);
    }

    .edit-btn:hover:not(:disabled), .save-btn:hover:not(:disabled) {
        opacity: 0.9;
    }

    button:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .error-banner {
        background: #ffebee;
        color: #c62828;
        padding: 12px 16px;
        border-radius: 6px;
        margin-bottom: 20px;
        border: 1px solid #ffcdd2;
    }

    .message-banner {
        padding: 12px 16px;
        border-radius: 6px;
        margin-bottom: 20px;
        display: flex;
        align-items: center;
        gap: 12px;
    }

    .message-banner.success {
        background: #e8f5e9;
        color: #2e7d32;
        border: 1px solid #c8e6c9;
    }

    .message-banner.error {
        background: #ffebee;
        color: #c62828;
        border: 1px solid #ffcdd2;
    }

    .restart-btn-inline {
        padding: 4px 12px;
        background: var(--primary);
        border: none;
        color: white;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.85rem;
    }

    .config-sections {
        display: flex;
        flex-direction: column;
        gap: 24px;
    }

    .config-section {
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 8px;
        padding: 20px;
    }

    .config-section h3 {
        font-size: 1.1rem;
        color: var(--text-color);
        margin-bottom: 16px;
        padding-bottom: 8px;
        border-bottom: 1px solid var(--border-color);
    }

    .config-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(250px, 1fr));
        gap: 16px;
    }

    .config-item {
        display: flex;
        flex-direction: column;
        gap: 4px;
    }

    .config-item.full-width {
        grid-column: 1 / -1;
    }

    .config-item .label {
        font-size: 0.85rem;
        color: var(--muted-color);
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .config-item .value {
        font-size: 0.95rem;
        color: var(--text-color);
    }

    .config-item .value.mono {
        font-family: monospace;
        font-size: 0.85rem;
    }

    .config-item .value.bool {
        font-weight: 500;
    }

    .config-item .value.bool.enabled {
        color: var(--success);
    }

    .config-item input[type="text"],
    .config-item select {
        padding: 8px 12px;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        background: var(--bg-color);
        color: var(--text-color);
        font-size: 0.9rem;
    }

    .config-item input[type="text"]:focus,
    .config-item select:focus {
        outline: none;
        border-color: var(--primary);
    }

    .toggle {
        display: flex;
        align-items: center;
        gap: 8px;
        cursor: pointer;
    }

    .toggle input[type="checkbox"] {
        width: 18px;
        height: 18px;
    }

    .owners-list {
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
        margin-top: 4px;
    }

    .owner-item {
        display: flex;
        align-items: center;
        gap: 4px;
    }

    .owner {
        font-size: 0.75rem;
        background: var(--bg-color);
        padding: 4px 8px;
        border-radius: 4px;
        word-break: break-all;
    }

    .remove-owner-btn {
        padding: 2px 6px;
        background: #ffebee;
        border: none;
        color: #c62828;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.8rem;
    }

    .add-owner-btn {
        padding: 2px 8px;
        background: var(--primary);
        border: none;
        color: white;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.75rem;
    }

    .no-owners {
        color: var(--muted-color);
        font-style: italic;
    }

    .config-note {
        margin-top: 24px;
        padding: 16px;
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 8px;
    }

    .config-note p {
        color: var(--muted-color);
        font-size: 0.9rem;
        margin: 0;
    }

    .config-note code {
        background: var(--bg-color);
        padding: 2px 6px;
        border-radius: 4px;
        font-size: 0.85rem;
    }

    .loading {
        text-align: center;
        color: var(--muted-color);
        padding: 40px;
    }
</style>
