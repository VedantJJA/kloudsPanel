import { writable } from 'svelte/store';

export type ThemeMode = 'light' | 'dark' | 'system';

function createThemeStore() {
  const { subscribe, set, update } = writable<ThemeMode>('dark');

  let mediaQuery: MediaQueryList | null = null;

  function getSystemTheme(): 'light' | 'dark' {
    if (typeof window === 'undefined') return 'dark';
    return window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
  }

  function applyTheme(mode: ThemeMode) {
    if (typeof document === 'undefined') return;
    const resolved = mode === 'system' ? getSystemTheme() : mode;
    document.documentElement.setAttribute('data-theme', resolved);
    if (resolved === 'dark') {
      document.documentElement.classList.add('dark');
    } else {
      document.documentElement.classList.remove('dark');
    }
  }

  function handleMediaChange() {
    update(current => {
      if (current === 'system') {
        applyTheme('system');
      }
      return current;
    });
  }

  return {
    subscribe,
    init: () => {
      if (typeof window === 'undefined') return;
      
      let saved = localStorage.getItem('klouds_theme') as ThemeMode | null;
      if (!saved || !['light', 'dark', 'system'].includes(saved)) {
        saved = 'dark';
      }

      set(saved);
      applyTheme(saved);

      if (typeof window.matchMedia === 'function') {
        mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
        mediaQuery.addEventListener('change', handleMediaChange);
      }
    },
    setTheme: (mode: ThemeMode) => {
      if (typeof window === 'undefined') return;
      localStorage.setItem('klouds_theme', mode);
      set(mode);
      applyTheme(mode);
    },
    toggle: () => {
      update(current => {
        const next: ThemeMode = current === 'dark' ? 'light' : 'dark';
        if (typeof window !== 'undefined') {
          localStorage.setItem('klouds_theme', next);
          applyTheme(next);
        }
        return next;
      });
    }
  };
}

export const theme = createThemeStore();
