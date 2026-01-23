<script>
    import { onMount } from 'svelte';
    import { userSigner, userPubkey, binariesData, isLoading, error } from '../stores.js';
    import { fetchBinaries, updateBinaries, rollbackVersion } from '../api.js';

    let version = '';
    let urls = {
        'orly': '',
        'orly-db-badger': '',
        'orly-acl-follows': '',
        'orly-launcher': '',
    };
    let updateResult = null;
    let isUpdating = false;

    onMount(async () => {
        await loadBinaries();
    });

    async function loadBinaries() {
        $isLoading = true;
        try {
            $binariesData = await fetchBinaries($userSigner, $userPubkey);
            $error = '';
        } catch (e) {
            $error = e.message;
        } finally {
            $isLoading = false;
        }
    }

    async function handleUpdate() {
        // Filter out empty URLs
        const filteredUrls = {};
        for (const [name, url] of Object.entries(urls)) {
            if (url.trim()) {
                filteredUrls[name] = url.trim();
            }
        }

        if (!version.trim()) {
            $error = 'Version is required';
            return;
        }

        if (Object.keys(filteredUrls).length === 0) {
            $error = 'At least one binary URL is required';
            return;
        }

        isUpdating = true;
        updateResult = null;
        $error = '';

        try {
            updateResult = await updateBinaries($userSigner, $userPubkey, version.trim(), filteredUrls);
            await loadBinaries();
        } catch (e) {
            $error = e.message;
        } finally {
            isUpdating = false;
        }
    }

    async function handleRollback() {
        if (!confirm('Are you sure you want to rollback to the previous version?')) {
            return;
        }

        isUpdating = true;
        $error = '';

        try {
            const result = await rollbackVersion($userSigner, $userPubkey);
            updateResult = {
                success: true,
                message: `Rolled back from ${result.previous_version} to ${result.current_version}. Restart services to apply.`
            };
            await loadBinaries();
        } catch (e) {
            $error = e.message;
        } finally {
            isUpdating = false;
        }
    }

    function setReleaseUrls() {
        // Helper to fill in URLs from a release base
        let inputUrl = prompt('Enter release URL (e.g., https://git.mleku.dev/mleku/next.orly.dev/releases/tag/v0.56.0):');
        if (!inputUrl) return;

        // Normalize the URL - convert /releases/tag/ to /releases/download/
        let cleanBase = inputUrl.replace(/\/$/, '');
        if (cleanBase.includes('/releases/tag/')) {
            cleanBase = cleanBase.replace('/releases/tag/', '/releases/download/');
        } else if (!cleanBase.includes('/releases/download/')) {
            // If it's just a repo URL, construct the download path
            const ver = version.trim() || 'v0.56.0';
            cleanBase = cleanBase.replace(/\/$/, '') + '/releases/download/' + ver;
        }

        const arch = prompt('Enter architecture (amd64 or arm64):', 'amd64');
        if (!arch) return;

        // Extract version from URL
        const urlParts = cleanBase.split('/');
        const ver = urlParts[urlParts.length - 1];
        const verNum = ver.replace('v', '');

        urls['orly'] = `${cleanBase}/orly-${verNum}-linux-${arch}`;
        urls['orly-db-badger'] = `${cleanBase}/orly-db-badger-${verNum}-linux-${arch}`;
        urls['orly-acl-follows'] = `${cleanBase}/orly-acl-follows-${verNum}-linux-${arch}`;
        urls['orly-launcher'] = `${cleanBase}/orly-launcher-${verNum}-linux-${arch}`;

        if (!version.trim()) {
            version = ver;
        }
    }
</script>

<div class="update-page">
    <div class="page-header">
        <h2>Update Binaries</h2>
    </div>

    {#if $error}
        <div class="error-banner">{$error}</div>
    {/if}

    {#if updateResult?.success}
        <div class="success-banner">
            {updateResult.message}
            {#if updateResult.downloaded_files?.length}
                <br>Downloaded: {updateResult.downloaded_files.join(', ')}
            {/if}
        </div>
    {/if}

    <div class="current-version">
        <h3>Current Version</h3>
        <div class="version-info">
            <span class="version">{$binariesData?.current_version || 'unknown'}</span>
            <button
                class="rollback-btn"
                on:click={handleRollback}
                disabled={isUpdating || ($binariesData?.available_versions?.length || 0) < 2}
            >
                Rollback
            </button>
        </div>
    </div>

    <div class="update-form">
        <h3>Install New Version</h3>

        <div class="form-group">
            <label for="version">Version</label>
            <input
                type="text"
                id="version"
                bind:value={version}
                placeholder="v0.55.11"
                disabled={isUpdating}
            />
        </div>

        <div class="form-group">
            <div class="url-header">
                <label>Binary URLs</label>
                <button class="helper-btn" on:click={setReleaseUrls} disabled={isUpdating}>
                    Fill from Release
                </button>
            </div>

            {#each Object.keys(urls) as name}
                <div class="url-input">
                    <span class="binary-name">{name}</span>
                    <input
                        type="text"
                        bind:value={urls[name]}
                        placeholder="https://..."
                        disabled={isUpdating}
                    />
                </div>
            {/each}
        </div>

        <button
            class="update-btn"
            on:click={handleUpdate}
            disabled={isUpdating}
        >
            {isUpdating ? 'Updating...' : 'Download & Install'}
        </button>
    </div>

    {#if $binariesData?.available_versions?.length}
        <div class="versions-list">
            <h3>Installed Versions</h3>
            <table>
                <thead>
                    <tr>
                        <th>Version</th>
                        <th>Installed</th>
                        <th>Binaries</th>
                        <th>Status</th>
                    </tr>
                </thead>
                <tbody>
                    {#each $binariesData.available_versions as ver}
                        <tr class:current={ver.is_current}>
                            <td class="version-cell">{ver.version}</td>
                            <td>{new Date(ver.installed_at).toLocaleString()}</td>
                            <td>{ver.binaries?.length || 0} files</td>
                            <td>
                                {#if ver.is_current}
                                    <span class="current-badge">Current</span>
                                {/if}
                            </td>
                        </tr>
                    {/each}
                </tbody>
            </table>
        </div>
    {/if}
</div>

<style>
    .update-page {
        padding: 20px 0;
    }

    .page-header {
        margin-bottom: 24px;
    }

    .page-header h2 {
        font-size: 1.5rem;
        color: var(--text-color);
    }

    .error-banner {
        background: #ffebee;
        color: #c62828;
        padding: 12px 16px;
        border-radius: 6px;
        margin-bottom: 20px;
        border: 1px solid #ffcdd2;
    }

    .success-banner {
        background: #e8f5e9;
        color: #2e7d32;
        padding: 12px 16px;
        border-radius: 6px;
        margin-bottom: 20px;
        border: 1px solid #c8e6c9;
    }

    .current-version,
    .update-form,
    .versions-list {
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 8px;
        padding: 20px;
        margin-bottom: 24px;
    }

    h3 {
        font-size: 1.1rem;
        color: var(--text-color);
        margin-bottom: 16px;
    }

    .version-info {
        display: flex;
        align-items: center;
        justify-content: space-between;
    }

    .version {
        font-size: 1.5rem;
        font-weight: 600;
        font-family: monospace;
        color: var(--text-color);
    }

    .rollback-btn {
        padding: 8px 16px;
        background: var(--warning);
        border: none;
        color: white;
        border-radius: 4px;
        cursor: pointer;
    }

    .rollback-btn:hover:not(:disabled) {
        opacity: 0.9;
    }

    .rollback-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .form-group {
        margin-bottom: 20px;
    }

    .form-group > label {
        display: block;
        font-size: 0.9rem;
        color: var(--text-color);
        margin-bottom: 8px;
        font-weight: 500;
    }

    .form-group input[type="text"] {
        width: 100%;
        padding: 10px 12px;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        font-size: 0.95rem;
        background: var(--bg-color);
        color: var(--text-color);
    }

    .form-group input:focus {
        outline: none;
        border-color: var(--primary);
    }

    .url-header {
        display: flex;
        justify-content: space-between;
        align-items: center;
        margin-bottom: 12px;
    }

    .url-header label {
        font-size: 0.9rem;
        color: var(--text-color);
        font-weight: 500;
    }

    .helper-btn {
        padding: 4px 12px;
        font-size: 0.8rem;
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 4px;
        color: var(--text-color);
        cursor: pointer;
    }

    .helper-btn:hover:not(:disabled) {
        background: var(--border-color);
    }

    .url-input {
        display: flex;
        gap: 12px;
        align-items: center;
        margin-bottom: 8px;
    }

    .binary-name {
        width: 140px;
        font-family: monospace;
        font-size: 0.85rem;
        color: var(--muted-color);
    }

    .url-input input {
        flex: 1;
        padding: 8px 12px;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        font-size: 0.85rem;
        background: var(--bg-color);
        color: var(--text-color);
    }

    .update-btn {
        width: 100%;
        padding: 12px;
        background: var(--primary);
        border: none;
        color: white;
        border-radius: 6px;
        font-size: 1rem;
        cursor: pointer;
    }

    .update-btn:hover:not(:disabled) {
        background: var(--primary-hover);
    }

    .update-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    table {
        width: 100%;
        border-collapse: collapse;
    }

    th, td {
        padding: 10px 12px;
        text-align: left;
        border-bottom: 1px solid var(--border-color);
    }

    th {
        font-size: 0.85rem;
        color: var(--muted-color);
        font-weight: 500;
    }

    td {
        font-size: 0.9rem;
        color: var(--text-color);
    }

    .version-cell {
        font-family: monospace;
    }

    tr.current {
        background: rgba(0, 188, 212, 0.1);
    }

    .current-badge {
        background: var(--primary);
        color: white;
        padding: 2px 8px;
        border-radius: 4px;
        font-size: 0.75rem;
    }
</style>
