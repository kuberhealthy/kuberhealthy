import { writable } from 'svelte/store';

// checks holds all check status details keyed by name.
export const checks = writable({});

// currentCheck holds the name of the check currently being viewed.
export const currentCheck = writable('');

// refreshStatus exposes the status refresh function to components that need to trigger it.
export const refreshStatus = writable(async () => {});
