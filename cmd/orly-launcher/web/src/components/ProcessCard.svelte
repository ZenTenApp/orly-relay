<script>
    import { createEventDispatcher } from 'svelte';

    export let process;
    export let isLoading = false;

    const dispatch = createEventDispatcher();

    // Categories that are mutually exclusive (only one can be enabled)
    const exclusiveCategories = ['database', 'acl'];

    // Check if this process can be toggled (relay is always on)
    $: canToggle = process.category !== 'relay';

    // Check if this is in an exclusive category
    $: isExclusive = exclusiveCategories.includes(process.category);

    function getStatusColor(status) {
        switch (status) {
            case 'running': return 'var(--success)';
            case 'stopped': return 'var(--muted-color)';
            case 'disabled': return 'var(--muted-color)';
            case 'crashed': return 'var(--error)';
            default: return 'var(--muted-color)';
        }
    }

    function getStatusIcon(status) {
        switch (status) {
            case 'running': return '●';
            case 'stopped': return '○';
            case 'disabled': return '◌';
            case 'crashed': return '✗';
            default: return '?';
        }
    }

    function getCategoryLabel(category) {
        switch (category) {
            case 'database': return 'Database';
            case 'acl': return 'Access Control';
            case 'sync': return 'Sync Service';
            case 'certs': return 'Certificates';
            case 'relay': return 'Relay';
            default: return category;
        }
    }

    function handleStart() {
        dispatch('start', { service: process.name });
    }

    function handleStop() {
        dispatch('stop', { service: process.name });
    }

    function handleRestart() {
        dispatch('restart', { service: process.name });
    }

    function handleToggleEnabled(event) {
        const newEnabled = event.target.checked;
        dispatch('toggle-enabled', {
            service: process.name,
            enabled: newEnabled,
            category: process.category,
            isExclusive: isExclusive
        });
    }

    $: canStart = process.enabled && process.status !== 'running';
    $: canStop = process.status === 'running';
    $: canRestart = process.status === 'running';
</script>

<div class="process-card" class:disabled={!process.enabled}>
    <div class="process-header">
        <span class="status-indicator" style="color: {getStatusColor(process.status)}">
            {getStatusIcon(process.status)}
        </span>
        <div class="name-section">
            <span class="process-name">{process.name}</span>
            <span class="category-badge" class:exclusive={isExclusive}>{getCategoryLabel(process.category)}</span>
        </div>
        {#if canToggle}
            <label class="enable-toggle" title={process.enabled ? 'Disable' : 'Enable'}>
                <input
                    type="checkbox"
                    checked={process.enabled}
                    on:change={handleToggleEnabled}
                    disabled={isLoading || process.status === 'running'}
                />
                <span class="toggle-slider"></span>
            </label>
        {:else}
            <span class="badge required-badge">always on</span>
        {/if}
    </div>

    <p class="description">{process.description}</p>

    <div class="process-details">
        <div class="detail-row">
            <span class="label">Status:</span>
            <span class="value" style="color: {getStatusColor(process.status)}">
                {process.status}
            </span>
        </div>

        {#if process.pid > 0}
            <div class="detail-row">
                <span class="label">PID:</span>
                <span class="value">{process.pid}</span>
            </div>
        {/if}

        {#if process.restarts > 0}
            <div class="detail-row">
                <span class="label">Restarts:</span>
                <span class="value warning">{process.restarts}</span>
            </div>
        {/if}
    </div>

    <div class="process-actions">
        {#if canStart}
            <button class="action-btn start-btn" on:click={handleStart} disabled={isLoading} title="Start service">
                ▶
            </button>
        {/if}
        {#if canStop}
            <button class="action-btn stop-btn" on:click={handleStop} disabled={isLoading} title="Stop service">
                ■
            </button>
        {/if}
        {#if canRestart}
            <button class="action-btn restart-btn" on:click={handleRestart} disabled={isLoading} title="Restart service">
                ↻
            </button>
        {/if}
        {#if !process.enabled && !canStart && !canStop}
            <span class="hint">Enable to start</span>
        {/if}
    </div>
</div>

<style>
    .process-card {
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 8px;
        padding: 16px;
        display: flex;
        flex-direction: column;
        gap: 10px;
    }

    .process-card.disabled {
        opacity: 0.6;
    }

    .process-header {
        display: flex;
        align-items: center;
        gap: 8px;
    }

    .status-indicator {
        font-size: 1.2rem;
        flex-shrink: 0;
    }

    .name-section {
        flex: 1;
        display: flex;
        flex-direction: column;
        gap: 2px;
    }

    .process-name {
        font-weight: 600;
        font-size: 0.95rem;
        color: var(--text-color);
    }

    .category-badge {
        font-size: 0.65rem;
        padding: 1px 4px;
        border-radius: 3px;
        text-transform: uppercase;
        background: var(--border-color);
        color: var(--muted-color);
        width: fit-content;
    }

    .category-badge.exclusive {
        background: var(--warning, #ff9800);
        color: white;
        opacity: 0.8;
    }

    .description {
        font-size: 0.8rem;
        color: var(--muted-color);
        margin: 0;
        line-height: 1.3;
    }

    .badge {
        font-size: 0.65rem;
        padding: 2px 6px;
        border-radius: 4px;
        text-transform: uppercase;
        flex-shrink: 0;
    }

    .required-badge {
        background: var(--text-color);
        color: var(--card-bg);
        opacity: 0.4;
    }

    .enable-toggle {
        position: relative;
        display: inline-block;
        width: 36px;
        height: 20px;
        cursor: pointer;
        flex-shrink: 0;
    }

    .enable-toggle input {
        opacity: 0;
        width: 0;
        height: 0;
    }

    .toggle-slider {
        position: absolute;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background-color: var(--muted-color);
        border-radius: 20px;
        transition: 0.2s;
    }

    .toggle-slider:before {
        position: absolute;
        content: "";
        height: 14px;
        width: 14px;
        left: 3px;
        bottom: 3px;
        background-color: white;
        border-radius: 50%;
        transition: 0.2s;
    }

    .enable-toggle input:checked + .toggle-slider {
        background-color: var(--success, #4caf50);
    }

    .enable-toggle input:checked + .toggle-slider:before {
        transform: translateX(16px);
    }

    .enable-toggle input:disabled + .toggle-slider {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .process-details {
        display: flex;
        flex-direction: column;
        gap: 4px;
    }

    .detail-row {
        display: flex;
        justify-content: space-between;
        font-size: 0.8rem;
    }

    .label {
        color: var(--muted-color);
    }

    .value {
        color: var(--text-color);
        font-family: monospace;
    }

    .value.warning {
        color: var(--warning);
    }

    .process-actions {
        display: flex;
        gap: 8px;
        align-items: center;
        margin-top: 4px;
        padding-top: 10px;
        border-top: 1px solid var(--border-color);
    }

    .action-btn {
        width: 32px;
        height: 32px;
        border-radius: 4px;
        border: none;
        cursor: pointer;
        font-size: 1rem;
        display: flex;
        align-items: center;
        justify-content: center;
        transition: opacity 0.2s, transform 0.1s;
    }

    .action-btn:hover:not(:disabled) {
        transform: scale(1.05);
    }

    .action-btn:disabled {
        opacity: 0.5;
        cursor: not-allowed;
    }

    .start-btn {
        background: var(--success, #4caf50);
        color: white;
    }

    .stop-btn {
        background: var(--error, #f44336);
        color: white;
    }

    .restart-btn {
        background: var(--warning, #ff9800);
        color: white;
    }

    .hint {
        font-size: 0.75rem;
        color: var(--muted-color);
        font-style: italic;
    }
</style>
