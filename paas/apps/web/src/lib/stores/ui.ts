import { writable } from 'svelte/store';

export const isMobileNavOpen = writable<boolean>(false);

export function toggleMobileNav() {
  isMobileNavOpen.update(v => !v);
}

export function closeMobileNav() {
  isMobileNavOpen.set(false);
}

export function openMobileNav() {
  isMobileNavOpen.set(true);
}
