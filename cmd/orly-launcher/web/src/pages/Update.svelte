<script>
    import { onMount } from 'svelte';
    import { userSigner, userPubkey, binariesData, isLoading, error } from '../stores.js';
    import { fetchBinaries, updateBinaries, rollbackVersion, restartServices, restartService, fetchReleases } from '../api.js';

    let version = '';
    let releaseBaseUrl = '';
    let architecture = 'amd64';
    let updateResult = null;
    let isUpdating = false;
    let launcherUpdated = false;

    // Official releases - fetched via backend proxy to avoid CORS
    const RELEASES_BASE = 'https://git.nostrdev.com/mleku/next.orly.dev/releases/download';
    let availableReleases = [];
    let selectedRelease = '';
    let loadingReleases = false;

    // Category definitions with available options
    const categoryDefs = {
        launcher: {
            label: 'Launcher',
            options: [
                { value: 'orly-launcher', label: 'orly-launcher' },
                { value: 'custom', label: 'Custom' }
            ],
            required: true
        },
        relay: {
            label: 'Relay',
            options: [
                { value: 'orly', label: 'orly' },
                { value: 'custom', label: 'Custom' }
            ],
            required: true
        },
        database: {
            label: 'Database',
            options: [
                { value: 'orly-db-badger', label: 'Badger' },
                { value: 'orly-db-neo4j', label: 'Neo4j' },
                { value: 'custom', label: 'Custom' }
            ],
            required: true
        },
        acl: {
            label: 'ACL',
            options: [
                { value: 'none', label: 'None (disabled)' },
                { value: 'orly-acl-follows', label: 'Follows' },
                { value: 'orly-acl-managed', label: 'Managed' },
                { value: 'orly-acl-curation', label: 'Curation' },
                { value: 'custom', label: 'Custom' }
            ],
            required: false
        },
        sync: {
            label: 'Sync',
            options: [
                { value: 'none', label: 'None (disabled)' },
                { value: 'orly-sync-negentropy', label: 'Negentropy' },
                { value: 'custom', label: 'Custom' }
            ],
            required: false
        }
    };

    // Current selections for each category
    let categories = {
        launcher: { selected: 'orly-launcher', customUrl: '', url: '', installing: false, installed: false },
        relay: { selected: 'orly', customUrl: '', url: '', installing: false, installed: false },
        database: { selected: 'orly-db-badger', customUrl: '', url: '', installing: false, installed: false },
        acl: { selected: 'none', customUrl: '', url: '', installing: false, installed: false },
        sync: { selected: 'none', customUrl: '', url: '', installing: false, installed: false }
    };

    onMount(async () => {
        await loadBinaries();
        await loadAvailableReleases();
    });

    async function loadAvailableReleases() {
        loadingReleases = true;
        try {
            const result = await fetchReleases($userSigner, $userPubkey);
            if (result.releases) {
                availableReleases = result.releases.map(r => ({
                    tag: r.tag,
                    message: r.message || ''
                }));
            }
        } catch (e) {
            console.error('Failed to fetch releases:', e);
        } finally {
            loadingReleases = false;
        }
    }

    function handleReleaseSelect() {
        if (!selectedRelease) return;
        version = selectedRelease;
        releaseBaseUrl = `${RELEASES_BASE}/${selectedRelease}`;
        updateUrls();
    }

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

    function generateUrl(binaryName) {
        if (!releaseBaseUrl || !version) return '';
        const verNum = version.replace(/^v/, '');
        return `${releaseBaseUrl}/${binaryName}-${verNum}-linux-${architecture}`;
    }

    function updateUrls() {
        for (const key of Object.keys(categories)) {
            const cat = categories[key];
            if (cat.selected !== 'none' && cat.selected !== 'custom') {
                cat.url = generateUrl(cat.selected);
            } else if (cat.selected === 'custom') {
                cat.url = cat.customUrl;
            } else {
                cat.url = '';
            }
        }
        categories = categories;
    }

    function handleSelectionChange(categoryKey) {
        updateUrls();
    }

    function setReleaseUrl() {
        let inputUrl = prompt('Enter release URL (e.g., https://git.mleku.dev/mleku/next.orly.dev/releases/tag/v0.56.1):');
        if (!inputUrl) return;

        // Normalize the URL
        let cleanBase = inputUrl.replace(/\/$/, '');
        if (cleanBase.includes('/releases/tag/')) {
            cleanBase = cleanBase.replace('/releases/tag/', '/releases/download/');
        } else if (!cleanBase.includes('/releases/download/')) {
            const ver = version.trim() || 'v0.56.1';
            cleanBase = cleanBase + '/releases/download/' + ver;
        }

        // Extract version from URL
        const urlParts = cleanBase.split('/');
        const ver = urlParts[urlParts.length - 1];

        releaseBaseUrl = cleanBase;
        if (!version) {
            version = ver;
        }

        updateUrls();
    }

    function getBinaryName(categoryKey) {
        const cat = categories[categoryKey];
        if (cat.selected === 'custom') {
            // Try to extract binary name from URL
            const urlParts = cat.customUrl.split('/');
            const filename = urlParts[urlParts.length - 1];
            // Remove version suffix like -0.56.1-linux-amd64
            return filename.replace(/-[\d.]+-linux-(amd64|arm64)$/, '') || categoryKey;
        }
        return cat.selected;
    }

    function getEffectiveUrl(categoryKey) {
        const cat = categories[categoryKey];
        if (cat.selected === 'custom') {
            return cat.customUrl;
        }
        return cat.url;
    }

    async function installCategory(categoryKey) {
        const cat = categories[categoryKey];
        const url = getEffectiveUrl(categoryKey);

        if (!url.trim()) {
            $error = `URL is required for ${categoryDefs[categoryKey].label}`;
            return;
        }

        if (!version.trim()) {
            $error = 'Version is required';
            return;
        }

        cat.installing = true;
        categories = categories;
        $error = '';

        try {
            const binaryName = getBinaryName(categoryKey);
            const urls = { [binaryName]: url.trim() };
            const result = await updateBinaries($userSigner, $userPubkey, version.trim(), urls);

            if (result.success) {
                cat.installed = true;

                if (categoryKey === 'launcher') {
                    launcherUpdated = true;
                    updateResult = {
                        success: true,
                        message: `Downloaded ${binaryName}. Click 'Restart Launcher' to apply.`,
                        downloaded_files: result.downloaded_files
                    };
                } else {
                    updateResult = {
                        success: true,
                        message: `Downloaded ${binaryName}, restarting service...`,
                        downloaded_files: result.downloaded_files
                    };

                    try {
                        await restartService($userSigner, $userPubkey, binaryName);
                        updateResult = {
                            success: true,
                            message: `${binaryName} installed and restart initiated`,
                            downloaded_files: result.downloaded_files
                        };
                    } catch (restartErr) {
                        updateResult = {
                            success: true,
                            message: `Downloaded ${binaryName}, but restart failed: ${restartErr.message}`,
                            downloaded_files: result.downloaded_files
                        };
                    }
                }

                await loadBinaries();
            }
        } catch (e) {
            $error = `Failed to install ${categoryDefs[categoryKey].label}: ${e.message}`;
        } finally {
            cat.installing = false;
            categories = categories;
        }
    }

    async function handleInstallAll() {
        const urls = {};
        let hasLauncher = false;

        for (const key of Object.keys(categories)) {
            const cat = categories[key];
            if (cat.selected !== 'none') {
                const url = getEffectiveUrl(key);
                if (url.trim()) {
                    const binaryName = getBinaryName(key);
                    urls[binaryName] = url.trim();
                    if (key === 'launcher') hasLauncher = true;
                }
            }
        }

        if (!version.trim()) {
            $error = 'Version is required';
            return;
        }

        if (Object.keys(urls).length === 0) {
            $error = 'No binaries selected for installation';
            return;
        }

        isUpdating = true;
        updateResult = null;
        launcherUpdated = false;
        $error = '';

        try {
            updateResult = await updateBinaries($userSigner, $userPubkey, version.trim(), urls);
            await loadBinaries();

            if (hasLauncher && updateResult.success) {
                launcherUpdated = true;
            }
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

    async function handleRestartLauncher() {
        if (!confirm('Restart the launcher? This will briefly disconnect you.')) {
            return;
        }
        try {
            await restartServices($userSigner, $userPubkey);
            updateResult = {
                success: true,
                message: 'Launcher restart initiated. The page will reconnect automatically...'
            };
            setTimeout(() => {
                window.location.reload();
            }, 5000);
        } catch (e) {
            $error = e.message;
        }
    }

    // React to architecture or version changes
    $: if (architecture || version) {
        updateUrls();
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
            {#if launcherUpdated}
                <div class="launcher-restart">
                    <strong>Launcher was updated!</strong>
                    <button class="restart-launcher-btn" on:click={handleRestartLauncher}>
                        Restart Launcher Now
                    </button>
                </div>
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

        <div class="release-settings">
            <div class="form-row">
                <div class="form-group">
                    <label for="release-select">Official Release</label>
                    <select
                        id="release-select"
                        bind:value={selectedRelease}
                        on:change={handleReleaseSelect}
                        disabled={isUpdating || loadingReleases}
                    >
                        <option value="">
                            {loadingReleases ? 'Loading...' : '-- Select release --'}
                        </option>
                        {#each availableReleases as release}
                            <option value={release.tag}>
                                {release.tag}{release.message ? ` - ${release.message.slice(0, 40)}` : ''}
                            </option>
                        {/each}
                    </select>
                </div>
                <div class="form-group">
                    <label for="arch">Architecture</label>
                    <select id="arch" bind:value={architecture} disabled={isUpdating}>
                        <option value="amd64">AMD64 (x86_64)</option>
                        <option value="arm64">ARM64 (aarch64)</option>
                    </select>
                </div>
            </div>

            <div class="form-row custom-release-row">
                <div class="form-group">
                    <label for="version">Or Custom Version</label>
                    <input
                        type="text"
                        id="version"
                        bind:value={version}
                        placeholder="v0.56.1"
                        disabled={isUpdating}
                    />
                </div>
                <div class="form-group">
                    <label>&nbsp;</label>
                    <button class="helper-btn fill-btn" on:click={setReleaseUrl} disabled={isUpdating}>
                        Set Custom URL
                    </button>
                </div>
            </div>

            {#if releaseBaseUrl}
                <div class="release-url-display">
                    <span class="release-label">Release:</span>
                    <code>{releaseBaseUrl}</code>
                </div>
            {/if}
        </div>

        <div class="categories">
            {#each Object.entries(categoryDefs) as [key, def]}
                <div class="category-row">
                    <div class="category-header">
                        <span class="category-label">{def.label}</span>
                        {#if !def.required}
                            <span class="optional-badge">optional</span>
                        {/if}
                    </div>
                    <div class="category-controls">
                        <select
                            bind:value={categories[key].selected}
                            on:change={() => handleSelectionChange(key)}
                            disabled={isUpdating || categories[key].installing}
                        >
                            {#each def.options as opt}
                                <option value={opt.value}>{opt.label}</option>
                            {/each}
                        </select>

                        {#if categories[key].selected === 'custom'}
                            <input
                                type="text"
                                class="custom-url"
                                bind:value={categories[key].customUrl}
                                on:input={() => { categories[key].url = categories[key].customUrl; }}
                                placeholder="https://... (custom binary URL)"
                                disabled={isUpdating || categories[key].installing}
                            />
                        {:else if categories[key].selected !== 'none'}
                            <input
                                type="text"
                                class="url-display"
                                value={categories[key].url}
                                readonly
                                placeholder="Set release URL above"
                            />
                        {/if}

                        {#if categories[key].selected !== 'none'}
                            <button
                                class="install-btn"
                                on:click={() => installCategory(key)}
                                disabled={isUpdating || categories[key].installing || !getEffectiveUrl(key)}
                                title="Download and install this component"
                            >
                                {#if categories[key].installing}
                                    ...
                                {:else if categories[key].installed}
                                    Done
                                {:else}
                                    Install
                                {/if}
                            </button>
                        {/if}
                    </div>
                </div>
            {/each}
        </div>

        <button
            class="update-btn"
            on:click={handleInstallAll}
            disabled={isUpdating}
        >
            {isUpdating ? 'Installing...' : 'Install All Selected'}
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

    .launcher-restart {
        margin-top: 12px;
        padding-top: 12px;
        border-top: 1px solid #c8e6c9;
        display: flex;
        align-items: center;
        gap: 12px;
    }

    .restart-launcher-btn {
        padding: 8px 16px;
        background: #1976d2;
        border: none;
        color: white;
        border-radius: 4px;
        cursor: pointer;
        font-weight: 500;
    }

    .restart-launcher-btn:hover {
        background: #1565c0;
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

    .release-settings {
        margin-bottom: 24px;
        padding-bottom: 20px;
        border-bottom: 1px solid var(--border-color);
    }

    .form-row {
        display: flex;
        gap: 16px;
        align-items: flex-end;
    }

    .form-group {
        flex: 1;
    }

    .form-group label {
        display: block;
        font-size: 0.85rem;
        color: var(--text-color);
        margin-bottom: 6px;
        font-weight: 500;
    }

    .form-group input[type="text"],
    .form-group select {
        width: 100%;
        padding: 8px 12px;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        font-size: 0.9rem;
        background: var(--bg-color);
        color: var(--text-color);
    }

    .form-group input:focus,
    .form-group select:focus {
        outline: none;
        border-color: var(--primary);
    }

    .helper-btn {
        padding: 8px 16px;
        font-size: 0.85rem;
        background: var(--primary);
        border: none;
        border-radius: 4px;
        color: white;
        cursor: pointer;
        white-space: nowrap;
    }

    .helper-btn:hover:not(:disabled) {
        opacity: 0.9;
    }

    .helper-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .fill-btn {
        width: 100%;
    }

    .custom-release-row {
        margin-top: 12px;
        padding-top: 12px;
        border-top: 1px dashed var(--border-color);
    }

    .release-url-display {
        margin-top: 12px;
        padding: 8px 12px;
        background: var(--bg-color);
        border-radius: 4px;
        font-size: 0.8rem;
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .release-label {
        color: var(--muted-color);
    }

    .release-url-display code {
        color: var(--text-color);
        word-break: break-all;
    }

    .categories {
        display: flex;
        flex-direction: column;
        gap: 12px;
        margin-bottom: 20px;
    }

    .category-row {
        padding: 12px;
        background: var(--bg-color);
        border-radius: 6px;
        border: 1px solid var(--border-color);
    }

    .category-header {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 8px;
    }

    .category-label {
        font-weight: 600;
        color: var(--text-color);
        font-size: 0.95rem;
    }

    .optional-badge {
        font-size: 0.7rem;
        color: var(--muted-color);
        background: var(--border-color);
        padding: 2px 6px;
        border-radius: 3px;
    }

    .category-controls {
        display: flex;
        gap: 8px;
        align-items: center;
    }

    .category-controls select {
        min-width: 140px;
        padding: 6px 10px;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        font-size: 0.85rem;
        background: var(--card-bg);
        color: var(--text-color);
    }

    .category-controls .custom-url,
    .category-controls .url-display {
        flex: 1;
        padding: 6px 10px;
        border: 1px solid var(--border-color);
        border-radius: 4px;
        font-size: 0.8rem;
        font-family: monospace;
        background: var(--card-bg);
        color: var(--text-color);
    }

    .category-controls .url-display {
        background: var(--bg-color);
        color: var(--muted-color);
    }

    .install-btn {
        padding: 6px 14px;
        background: var(--primary);
        border: none;
        color: white;
        border-radius: 4px;
        cursor: pointer;
        font-size: 0.8rem;
        min-width: 70px;
    }

    .install-btn:hover:not(:disabled) {
        opacity: 0.9;
    }

    .install-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
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

    @media (max-width: 768px) {
        .form-row {
            flex-direction: column;
            gap: 12px;
        }

        .category-controls {
            flex-wrap: wrap;
        }

        .category-controls select {
            min-width: 100%;
        }

        .category-controls .custom-url,
        .category-controls .url-display {
            min-width: 100%;
        }
    }
</style>
