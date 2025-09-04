import { writable } from 'svelte/store';

// checks holds all check status details keyed by name.
export const checks = writable({});

// currentCheck holds the name of the check currently being viewed.
export const currentCheck = writable('');
