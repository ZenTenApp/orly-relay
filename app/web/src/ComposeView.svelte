<script>
    export let composeEventJson = "";

    import { createEventDispatcher } from "svelte";
    const dispatch = createEventDispatcher();

    function reformatJson() {
        dispatch("reformatJson");
    }

    function signEvent() {
        dispatch("signEvent");
    }

    function publishEvent() {
        dispatch("publishEvent");
    }
</script>

<div class="compose-view">
    <div class="compose-header">
        <button class="compose-btn reformat-btn" on:click={reformatJson}
            >Reformat</button
        >
        <button class="compose-btn sign-btn" on:click={signEvent}>Sign</button>
        <button class="compose-btn publish-btn" on:click={publishEvent}
            >Publish</button
        >
    </div>
    <div class="compose-editor">
        <textarea
            bind:value={composeEventJson}
            class="compose-textarea"
            placeholder="Enter your Nostr event JSON here..."
            spellcheck="false"
        ></textarea>
    </div>
</div>

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

    .publish-btn {
        background: var(--success);
        color: var(--text-color);
    }

    .publish-btn:hover {
        background: var(--success);
        filter: brightness(0.9);
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
            left: 160px;
        }
    }
</style>
