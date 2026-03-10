<script>
    import { activeLibraryView } from './libraryStores.js';
    import MyLibraryView from './MyLibraryView.svelte';
    import PublicationReader from './PublicationReader.svelte';
    import BookmarksView from './BookmarksView.svelte';

    export let isLoggedIn = false;
    export let userPubkey = "";
    export let userSigner = null;
    export let subView = "my-library"; // my-library, bookmarks, new

    // Sync from sidebar navigation
    $: {
        if (subView === "my-library") activeLibraryView.set("my-library");
        else if (subView === "bookmarks") activeLibraryView.set("bookmarks");
        else if (subView === "new") activeLibraryView.set("editor");
    }
</script>

<div class="library-view">
    {#if $activeLibraryView === "reader"}
        <PublicationReader {isLoggedIn} {userPubkey} />
    {:else if $activeLibraryView === "bookmarks"}
        <BookmarksView {isLoggedIn} {userPubkey} />
    {:else}
        <MyLibraryView {isLoggedIn} {userPubkey} {userSigner} />
    {/if}
</div>

<style>
    .library-view {
        width: 100%;
        height: 100%;
        overflow: hidden;
        display: flex;
    }
</style>
