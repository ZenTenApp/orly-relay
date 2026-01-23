<script>
    export let process;

    function getStatusColor(status) {
        switch (status) {
            case 'running': return 'var(--success)';
            case 'stopped': return 'var(--muted-color)';
            case 'crashed': return 'var(--error)';
            default: return 'var(--muted-color)';
        }
    }

    function getStatusIcon(status) {
        switch (status) {
            case 'running': return '●';
            case 'stopped': return '○';
            case 'crashed': return '✗';
            default: return '?';
        }
    }
</script>

<div class="process-card">
    <div class="process-header">
        <span class="status-indicator" style="color: {getStatusColor(process.status)}">
            {getStatusIcon(process.status)}
        </span>
        <span class="process-name">{process.name}</span>
    </div>

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

        <div class="detail-row">
            <span class="label">Binary:</span>
            <span class="value binary">{process.binary}</span>
        </div>

        {#if process.restarts > 0}
            <div class="detail-row">
                <span class="label">Restarts:</span>
                <span class="value warning">{process.restarts}</span>
            </div>
        {/if}
    </div>
</div>

<style>
    .process-card {
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 8px;
        padding: 16px;
    }

    .process-header {
        display: flex;
        align-items: center;
        gap: 8px;
        margin-bottom: 12px;
    }

    .status-indicator {
        font-size: 1.2rem;
    }

    .process-name {
        font-weight: 600;
        font-size: 1rem;
        color: var(--text-color);
    }

    .process-details {
        display: flex;
        flex-direction: column;
        gap: 6px;
    }

    .detail-row {
        display: flex;
        justify-content: space-between;
        font-size: 0.85rem;
    }

    .label {
        color: var(--muted-color);
    }

    .value {
        color: var(--text-color);
        font-family: monospace;
    }

    .value.binary {
        font-size: 0.75rem;
        max-width: 150px;
        overflow: hidden;
        text-overflow: ellipsis;
        white-space: nowrap;
    }

    .value.warning {
        color: var(--warning);
    }
</style>
