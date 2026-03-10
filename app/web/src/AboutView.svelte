<script>
    import { relayInfo as relayInfoStore, relayUrl } from './stores.js';

    export let show = false;
    export let version = "";

    import { createEventDispatcher } from 'svelte';
    const dispatch = createEventDispatcher();

    function close() {
        dispatch('close');
    }

    function handleKeydown(e) {
        if (e.key === 'Escape') close();
    }
</script>

<svelte:window on:keydown={handleKeydown} />

{#if show}
    <!-- svelte-ignore a11y-click-events-have-key-events -->
    <!-- svelte-ignore a11y-no-static-element-interactions -->
    <div class="about-overlay" on:click={close}>
        <div class="about-modal" on:click|stopPropagation>
            <button class="close-btn" on:click={close}>x</button>
            <div class="about-content">
                <div class="about-logo">smesh</div>
                <div class="about-tagline">distributed operating system</div>

                {#if version}
                    <div class="about-version">v{version}</div>
                {/if}

                {#if $relayInfoStore}
                    <div class="about-relay">
                        <div class="relay-name">{$relayInfoStore.name || 'Relay'}</div>
                        {#if $relayInfoStore.description}
                            <div class="relay-desc">{$relayInfoStore.description}</div>
                        {/if}
                        <div class="relay-url">{$relayUrl}</div>
                    </div>
                {/if}

                <div class="about-footer">
                    Powered by ORLY
                </div>
            </div>
        </div>
    </div>
{/if}

<style>
    .about-overlay {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background: rgba(0, 0, 0, 0.6);
        z-index: 2000;
        display: flex;
        align-items: center;
        justify-content: center;
    }

    .about-modal {
        background: var(--card-bg);
        border: 1px solid var(--border-color);
        border-radius: 12px;
        padding: 2em;
        max-width: 360px;
        width: 90%;
        text-align: center;
        position: relative;
    }

    .close-btn {
        position: absolute;
        top: 0.5em;
        right: 0.75em;
        background: none;
        border: none;
        color: var(--text-muted);
        font-size: 1.2rem;
        cursor: pointer;
        padding: 0.25em;
    }

    .close-btn:hover {
        color: var(--text-color);
    }

    .about-logo {
        font-size: 2rem;
        font-weight: 800;
        color: var(--primary);
        letter-spacing: 0.1em;
        margin-bottom: 0.2em;
    }

    .about-tagline {
        font-size: 0.85rem;
        color: var(--text-muted);
        margin-bottom: 1em;
    }

    .about-version {
        font-size: 0.75rem;
        color: var(--text-muted);
        margin-bottom: 1.5em;
        font-family: monospace;
    }

    .about-relay {
        background: var(--bg-color);
        border: 1px solid var(--border-color);
        border-radius: 8px;
        padding: 0.75em;
        margin-bottom: 1.5em;
    }

    .relay-name {
        font-weight: 600;
        font-size: 0.85rem;
        color: var(--text-color);
        margin-bottom: 0.25em;
    }

    .relay-desc {
        font-size: 0.75rem;
        color: var(--text-muted);
        margin-bottom: 0.25em;
    }

    .relay-url {
        font-size: 0.7rem;
        color: var(--text-muted);
        font-family: monospace;
    }

    .about-footer {
        font-size: 0.7rem;
        color: var(--text-muted);
    }
</style>
