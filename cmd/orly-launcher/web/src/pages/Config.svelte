<script>
    import { onMount } from 'svelte';
    import { userSigner, userPubkey, configData, isLoading, error } from '../stores.js';
    import { fetchConfig } from '../api.js';

    onMount(async () => {
        await loadConfig();
    });

    async function loadConfig() {
        $isLoading = true;
        try {
            $configData = await fetchConfig($userSigner, $userPubkey);
            $error = '';
        } catch (e) {
            $error = e.message;
        } finally {
            $isLoading = false;
        }
    }
</script>

<div class="config-page">
    <div class="page-header">
        <h2>Configuration</h2>
        <button class="refresh-btn" on:click={loadConfig} disabled={$isLoading}>
            Refresh
        </button>
    </div>

    {#if $error}
        <div class="error-banner">{$error}</div>
    {/if}

    {#if $configData}
        <div class="config-sections">
            <section class="config-section">
                <h3>Database</h3>
                <div class="config-grid">
                    <div class="config-item">
                        <span class="label">Backend</span>
                        <span class="value">{$configData.db_backend}</span>
                    </div>
                    <div class="config-item">
                        <span class="label">Binary</span>
                        <span class="value mono">{$configData.db_binary}</span>
                    </div>
                    <div class="config-item">
                        <span class="label">Listen Address</span>
                        <span class="value mono">{$configData.db_listen}</span>
                    </div>
                    <div class="config-item">
                        <span class="label">Data Directory</span>
                        <span class="value mono">{$configData.data_dir}</span>
                    </div>
                </div>
            </section>

            <section class="config-section">
                <h3>ACL</h3>
                <div class="config-grid">
                    <div class="config-item">
                        <span class="label">Enabled</span>
                        <span class="value bool" class:enabled={$configData.acl_enabled}>
                            {$configData.acl_enabled ? 'Yes' : 'No'}
                        </span>
                    </div>
                    <div class="config-item">
                        <span class="label">Mode</span>
                        <span class="value">{$configData.acl_mode}</span>
                    </div>
                    <div class="config-item">
                        <span class="label">Binary</span>
                        <span class="value mono">{$configData.acl_binary}</span>
                    </div>
                    <div class="config-item">
                        <span class="label">Listen Address</span>
                        <span class="value mono">{$configData.acl_listen}</span>
                    </div>
                </div>
            </section>

            <section class="config-section">
                <h3>Relay</h3>
                <div class="config-grid">
                    <div class="config-item">
                        <span class="label">Binary</span>
                        <span class="value mono">{$configData.relay_binary}</span>
                    </div>
                    <div class="config-item">
                        <span class="label">Log Level</span>
                        <span class="value">{$configData.log_level}</span>
                    </div>
                </div>
            </section>

            <section class="config-section">
                <h3>Sync Services</h3>
                <div class="config-grid">
                    <div class="config-item">
                        <span class="label">Distributed Sync</span>
                        <span class="value bool" class:enabled={$configData.distributed_sync_enabled}>
                            {$configData.distributed_sync_enabled ? 'Enabled' : 'Disabled'}
                        </span>
                    </div>
                    <div class="config-item">
                        <span class="label">Cluster Sync</span>
                        <span class="value bool" class:enabled={$configData.cluster_sync_enabled}>
                            {$configData.cluster_sync_enabled ? 'Enabled' : 'Disabled'}
                        </span>
                    </div>
                    <div class="config-item">
                        <span class="label">Relay Group</span>
                        <span class="value bool" class:enabled={$configData.relay_group_enabled}>
                            {$configData.relay_group_enabled ? 'Enabled' : 'Disabled'}
                        </span>
                    </div>
                    <div class="config-item">
                        <span class="label">Negentropy</span>
                        <span class="value bool" class:enabled={$configData.negentropy_enabled}>
                            {$configData.negentropy_enabled ? 'Enabled' : 'Disabled'}
                        </span>
                    </div>
                </div>
            </section>

            <section class="config-section">
                <h3>Admin</h3>
                <div class="config-grid">
                    <div class="config-item">
                        <span class="label">Binary Directory</span>
                        <span class="value mono">{$configData.bin_dir}</span>
                    </div>
                    <div class="config-item full-width">
                        <span class="label">Admin Owners</span>
                        <div class="owners-list">
                            {#each $configData.admin_owners || [] as owner}
                                <code class="owner">{owner}</code>
                            {:else}
                                <span class="no-owners">No owners configured</span>
                            {/each}
                        </div>
                    </div>
                </div>
            </section>
        </div>

        <div class="config-note">
            <p>Configuration is loaded from environment variables. To change settings, update the environment and restart the launcher.</p>
        </div>
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

    .refresh-btn {
        padding: 8px 16px;
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        color: var(--text-color);
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9rem;
    }

    .refresh-btn:hover:not(:disabled) {
        background: var(--border-color);
    }

    .refresh-btn:disabled {
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

    .owners-list {
        display: flex;
        flex-wrap: wrap;
        gap: 8px;
        margin-top: 4px;
    }

    .owner {
        font-size: 0.75rem;
        background: var(--bg-color);
        padding: 4px 8px;
        border-radius: 4px;
        word-break: break-all;
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

    .loading {
        text-align: center;
        color: var(--muted-color);
        padding: 40px;
    }
</style>
