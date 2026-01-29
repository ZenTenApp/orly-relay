<script>
    export let composeEventJson = "";
    export let userPubkey = "";
    export let userRole = "";
    export let policyEnabled = false;
    export let publishError = "";
    export let localOnly = true;

    import { createEventDispatcher } from "svelte";
    import EventTemplateSelector from "./EventTemplateSelector.svelte";

    const dispatch = createEventDispatcher();

    let isTemplateSelectorOpen = false;

    function reformatJson() {
        dispatch("reformatJson");
    }

    function signEvent() {
        dispatch("signEvent");
    }

    function publishEvent() {
        dispatch("publishEvent");
    }

    function openTemplateSelector() {
        isTemplateSelectorOpen = true;
    }

    function handleTemplateSelect(event) {
        const { kind, template } = event.detail;
        composeEventJson = JSON.stringify(template, null, 2);
        dispatch("templateSelected", { kind, template });
    }

    function handleTemplateSelectorClose() {
        isTemplateSelectorOpen = false;
    }

    function clearError() {
        publishError = "";
        dispatch("clearError");
    }
</script>

<div class="compose-view">
    <div class="compose-header">
        <button class="compose-btn template-btn" on:click={openTemplateSelector}
            >Generate Template</button
        >
        <button class="compose-btn reformat-btn" on:click={reformatJson}
            >Reformat</button
        >
        <button class="compose-btn sign-btn" on:click={signEvent}>Sign</button>
        <label class="local-only-label">
            <input type="checkbox" bind:checked={localOnly} />
            This relay only
        </label>
        <button class="compose-btn publish-btn" on:click={publishEvent}
            >Publish</button
        >
    </div>

    {#if publishError}
        <div class="error-banner">
            <div class="error-content">
                <span class="error-icon">&#9888;</span>
                <span class="error-message">{publishError}</span>
            </div>
            <button class="error-dismiss" on:click={clearError}>&times;</button>
        </div>
    {/if}

    <div class="compose-editor">
        <textarea
            bind:value={composeEventJson}
            class="compose-textarea"
            placeholder="Enter your Nostr event JSON here, or click 'Generate Template' to start with a template..."
            spellcheck="false"
        ></textarea>
    </div>
</div>

<EventTemplateSelector
    bind:isOpen={isTemplateSelectorOpen}
    {userPubkey}
    on:select={handleTemplateSelect}
    on:close={handleTemplateSelectorClose}
/>

<style>
    .compose-view {
        position: fixed;
        top: 3em;
        left: 200px;
        right: 0;
        bottom: 0;
        display: flex;
        flex-direction: column;
        background: transparent;
    }

    .compose-header {
        display: flex;
        gap: 0.5em;
        padding: 0.5em;
        background: transparent;
    }

    .compose-btn {
        padding: 0.5em 1em;
        border: 1px solid var(--border-color);
        border-radius: 0.25rem;
        background: var(--button-bg);
        color: var(--button-text);
        cursor: pointer;
        font-size: 0.9rem;
        transition: background-color 0.2s;
    }

    .compose-btn:hover {
        background: var(--button-hover-bg);
    }

    .template-btn {
        background: var(--primary);
        color: var(--text-color);
    }

    .template-btn:hover {
        background: var(--primary);
        filter: brightness(0.9);
    }

    .reformat-btn {
        background: var(--info);
        color: var(--text-color);
    }

    .reformat-btn:hover {
        background: var(--info);
        filter: brightness(0.9);
    }

    .sign-btn {
        background: var(--warning);
        color: var(--text-color);
    }

    .sign-btn:hover {
        background: var(--warning);
        filter: brightness(0.9);
    }

    .local-only-label {
        display: flex;
        align-items: center;
        gap: 0.35em;
        font-size: 0.85rem;
        color: var(--text-color);
        cursor: pointer;
        user-select: none;
        margin-left: auto;
        padding: 0 0.5em;
        white-space: nowrap;
    }

    .local-only-label input[type="checkbox"] {
        cursor: pointer;
        accent-color: var(--accent-color);
    }

    .publish-btn {
        background: var(--success);
        color: var(--text-color);
    }

    .publish-btn:hover {
        background: var(--success);
        filter: brightness(0.9);
    }

    .error-banner {
        display: flex;
        align-items: center;
        justify-content: space-between;
        padding: 0.75em 1em;
        margin: 0 0.5em;
        background: #f8d7da;
        border: 1px solid #f5c6cb;
        border-radius: 0.25rem;
        color: #721c24;
    }

    :global(.dark-theme) .error-banner {
        background: #4a1c24;
        border-color: #6a2c34;
        color: #f8d7da;
    }

    .error-content {
        display: flex;
        align-items: center;
        gap: 0.5em;
    }

    .error-icon {
        font-size: 1.2em;
    }

    .error-message {
        font-size: 0.9rem;
        line-height: 1.4;
    }

    .error-dismiss {
        background: none;
        border: none;
        font-size: 1.25rem;
        cursor: pointer;
        color: inherit;
        padding: 0 0.25em;
        opacity: 0.7;
    }

    .error-dismiss:hover {
        opacity: 1;
    }

    .compose-editor {
        flex: 1;
        display: flex;
        flex-direction: column;
        padding: 0;
    }

    .compose-textarea {
        flex: 1;
        width: 100%;
        padding: 1em;
        border-radius: 0.5em;
        background: var(--input-bg);
        color: var(--input-text-color);
        font-family: monospace;
        font-size: 0.9em;
        line-height: 1.4;
        resize: vertical;
        outline: none;
    }

    .compose-textarea:focus {
        border-color: var(--accent-color);
        box-shadow: 0 0 0 2px rgba(0, 123, 255, 0.25);
    }

    @media (max-width: 1280px) {
        .compose-view {
            left: 60px;
        }
    }

    @media (max-width: 640px) {
        .compose-view {
            left: 0;
        }

        .compose-header {
            flex-wrap: wrap;
        }

        .compose-btn {
            flex: 1;
            min-width: calc(50% - 0.5em);
        }
    }
</style>
