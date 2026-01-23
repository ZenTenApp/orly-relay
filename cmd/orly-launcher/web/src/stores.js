import { writable } from 'svelte/store';

// Authentication state
export const isLoggedIn = writable(false);
export const userPubkey = writable('');
export const userSigner = writable(null);
export const authMethod = writable(''); // 'extension' or 'nsec'

// Status data
export const statusData = writable(null);
export const configData = writable(null);
export const binariesData = writable(null);

// Loading states
export const isLoading = writable(false);
export const error = writable('');
