<script>
    import { onMount, onDestroy } from 'svelte';
    import { userSigner, userPubkey, statusData, isLoading, error } from '../stores.js';
    import { fetchStatus, restartServices, startServices, stopServices, startService, stopService, restartService, saveConfig } from '../api.js';
    import ProcessCard from '../components/ProcessCard.svelte';

    let refreshInterval;

    onMount(async () => {
        await loadStatus();
        // Auto-refresh every 5 seconds
        refreshInterval = setInterval(loadStatus, 5000);
    });

    onDestroy(() => {
        if (refreshInterval) {
            clearInterval(refreshInterval);
        }
    });

    async function loadStatus() {
        try {
            $statusData = await fetchStatus($userSigner, $userPubkey);
            $error = '';
        } catch (e) {
            $error = e.message;
        }
    }

    async function handleRestart() {
        if (!confirm('Are you sure you want to restart all services?')) {
            return;
        }

        $isLoading = true;
        try {
            await restartServices($userSigner, $userPubkey);
            // Wait a moment then refresh
            setTimeout(loadStatus, 2000);
        } catch (e) {
            $error = e.message;
        } finally {
            $isLoading = false;
        }
    }

    async function handleStart() {
        $isLoading = true;
        try {
            await startServices($userSigner, $userPubkey);
            // Wait a moment then refresh
            setTimeout(loadStatus, 2000);
        } catch (e) {
            $error = e.message;
        } finally {
            $isLoading = false;
        }
    }

    async function handleStop() {
        if (!confirm('Are you sure you want to stop all services?')) {
            return;
        }

        $isLoading = true;
        try {
            await stopServices($userSigner, $userPubkey);
            // Wait a moment then refresh
            setTimeout(loadStatus, 2000);
        } catch (e) {
            $error = e.message;
        } finally {
            $isLoading = false;
        }
    }

    async function handleServiceStart(event) {
        const { service } = event.detail;
        $isLoading = true;
        try {
            await startService($userSigner, $userPubkey, service);
            // Wait a moment then refresh
            setTimeout(loadStatus, 2000);
        } catch (e) {
            $error = e.message;
        } finally {
            $isLoading = false;
        }
    }

    async function handleServiceStop(event) {
        const { service } = event.detail;
        // Check if this is a critical service with dependents
        const hasDependents = ['orly-db', 'orly-acl'].includes(service);
        if (hasDependents) {
            if (!confirm(`Stopping ${service} will also stop its dependent services. Continue?`)) {
                return;
            }
        }

        $isLoading = true;
        try {
            await stopService($userSigner, $userPubkey, service);
            // Wait a moment then refresh
            setTimeout(loadStatus, 2000);
        } catch (e) {
            $error = e.message;
        } finally {
            $isLoading = false;
        }
    }

    async function handleServiceRestart(event) {
        const { service } = event.detail;
        // Check if this is a critical service with dependents
        const hasDependents = ['orly-db', 'orly-acl'].includes(service);
        if (hasDependents) {
            if (!confirm(`Restarting ${service} will also restart its dependent services. Continue?`)) {
                return;
            }
        }

        $isLoading = true;
        try {
            await restartService($userSigner, $userPubkey, service);
            // Wait a moment then refresh
            setTimeout(loadStatus, 2000);
        } catch (e) {
            $error = e.message;
        } finally {
            $isLoading = false;
        }
    }

    // Map service names to config properties
    function getConfigForService(service, enabled) {
        switch (service) {
            // Database backends (mutually exclusive)
            case 'orly-db-badger':
                return enabled ? { db_backend: 'badger' } : null;
            case 'orly-db-neo4j':
                return enabled ? { db_backend: 'neo4j' } : null;

            // ACL backends (mutually exclusive)
            case 'orly-acl-follows':
                return enabled ? { acl_enabled: true, acl_mode: 'follows' } : { acl_enabled: false };
            case 'orly-acl-managed':
                return enabled ? { acl_enabled: true, acl_mode: 'managed' } : { acl_enabled: false };
            case 'orly-acl-curation':
                return enabled ? { acl_enabled: true, acl_mode: 'curation' } : { acl_enabled: false };

            // Sync services (independent)
            case 'orly-sync-distributed':
                return { distributed_sync_enabled: enabled };
            case 'orly-sync-cluster':
                return { cluster_sync_enabled: enabled };
            case 'orly-sync-relaygroup':
                return { relay_group_enabled: enabled };
            case 'orly-sync-negentropy':
                return { negentropy_enabled: enabled };

            // Certificate service
            case 'orly-certs':
                return { certs_enabled: enabled };

            default:
                return null;
        }
    }

    async function handleToggleEnabled(event) {
        const { service, enabled, category, isExclusive } = event.detail;

        // For exclusive categories, warn if enabling will disable others
        if (enabled && isExclusive) {
            const currentlyEnabled = $statusData.processes
                .filter(p => p.category === category && p.enabled && p.name !== service)
                .map(p => p.name);

            if (currentlyEnabled.length > 0) {
                if (!confirm(`Enabling ${service} will disable ${currentlyEnabled.join(', ')}. Continue?`)) {
                    // Refresh to reset the checkbox
                    await loadStatus();
                    return;
                }
            }
        }

        const configUpdate = getConfigForService(service, enabled);
        if (!configUpdate) {
            $error = `Unknown service: ${service}`;
            return;
        }

        $isLoading = true;
        try {
            await saveConfig($userSigner, $userPubkey, configUpdate);
            // Refresh status after config change
            setTimeout(loadStatus, 1000);
        } catch (e) {
            $error = e.message;
            // Refresh to reset the checkbox
            setTimeout(loadStatus, 500);
        } finally {
            $isLoading = false;
        }
    }
</script>

<div class="dashboard">
    <div class="page-header">
        <h2>Dashboard</h2>
        <div class="actions">
            <button class="refresh-btn" on:click={loadStatus} disabled={$isLoading}>
                Refresh
            </button>
            {#if $statusData?.services_running}
                <button class="stop-btn" on:click={handleStop} disabled={$isLoading}>
                    Stop Services
                </button>
                <button class="restart-btn" on:click={handleRestart} disabled={$isLoading}>
                    Restart All
                </button>
            {:else}
                <button class="start-btn" on:click={handleStart} disabled={$isLoading}>
                    Start Services
                </button>
            {/if}
        </div>
    </div>

    {#if $error}
        <div class="error-banner">{$error}</div>
    {/if}

    {#if $statusData}
        <div class="status-summary">
            <div class="summary-card">
                <span class="label">Status</span>
                <span class="value status-indicator" class:running={$statusData.services_running} class:stopped={!$statusData.services_running}>
                    {$statusData.services_running ? 'Running' : 'Stopped'}
                </span>
            </div>
            <div class="summary-card">
                <span class="label">Version</span>
                <span class="value">{$statusData.version || 'unknown'}</span>
            </div>
            <div class="summary-card">
                <span class="label">Uptime</span>
                <span class="value">{$statusData.uptime}</span>
            </div>
            <div class="summary-card">
                <span class="label">Running</span>
                <span class="value">{$statusData.processes?.filter(p => p.status === 'running').length || 0} / {$statusData.processes?.filter(p => p.enabled).length || 0}</span>
            </div>
        </div>

        <h3>Available Modules</h3>
        <div class="processes-grid">
            {#each $statusData.processes || [] as process}
                <ProcessCard
                    {process}
                    isLoading={$isLoading}
                    on:start={handleServiceStart}
                    on:stop={handleServiceStop}
                    on:restart={handleServiceRestart}
                    on:toggle-enabled={handleToggleEnabled}
                />
            {/each}
        </div>
    {:else if !$error}
        <div class="loading">Loading status...</div>
    {/if}
</div>

<style>
    .dashboard {
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

    .actions {
        display: flex;
        gap: 8px;
    }

    .refresh-btn,
    .restart-btn,
    .start-btn,
    .stop-btn {
        padding: 8px 16px;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.9rem;
    }

    .refresh-btn {
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        color: var(--text-color);
    }

    .refresh-btn:hover:not(:disabled) {
        background: var(--border-color);
    }

    .restart-btn {
        background: var(--warning);
        border: none;
        color: white;
    }

    .restart-btn:hover:not(:disabled) {
        opacity: 0.9;
    }

    .start-btn {
        background: var(--success, #4caf50);
        border: none;
        color: white;
    }

    .start-btn:hover:not(:disabled) {
        opacity: 0.9;
    }

    .stop-btn {
        background: var(--error, #f44336);
        border: none;
        color: white;
    }

    .stop-btn:hover:not(:disabled) {
        opacity: 0.9;
    }

    .restart-btn:disabled,
    .refresh-btn:disabled,
    .start-btn:disabled,
    .stop-btn:disabled {
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

    .status-summary {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
        gap: 16px;
        margin-bottom: 32px;
    }

    .summary-card {
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 8px;
        padding: 16px;
        display: flex;
        flex-direction: column;
        gap: 4px;
    }

    .summary-card .label {
        font-size: 0.85rem;
        color: var(--muted-color);
    }

    .summary-card .value {
        font-size: 1.25rem;
        font-weight: 600;
        color: var(--text-color);
    }

    .status-indicator.running {
        color: var(--success, #4caf50);
    }

    .status-indicator.stopped {
        color: var(--error, #f44336);
    }

    h3 {
        font-size: 1.1rem;
        color: var(--text-color);
        margin-bottom: 16px;
    }

    .processes-grid {
        display: grid;
        grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
        gap: 16px;
    }

    .loading {
        text-align: center;
        color: var(--muted-color);
        padding: 40px;
    }
</style>
