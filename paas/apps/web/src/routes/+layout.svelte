<script lang="ts">
  import '../app.css';
  import Sidebar from '$lib/components/layout/Sidebar.svelte';
  import MobileHeader from '$lib/components/layout/MobileHeader.svelte';
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { theme } from '$lib/stores/theme';

  let { children } = $props();

  onMount(() => {
    theme.init();
  });

  // Determine if we should show the shell (not on login/signup/pending)
  const noShellRoutes = ['/login', '/signup', '/access/pending'];
</script>

<svelte:head>
  <title>kloudsPanel - Self-Hosted PaaS</title>
  <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no" />
  <meta name="description" content="Lightweight self-hosted platform-as-a-service. Deploy any stack with zero Docker knowledge." />
  {@html `<script>
    try {
      var t = localStorage.getItem('klouds_theme');
      var dark = t === 'dark' || ((!t || t === 'system') && window.matchMedia('(prefers-color-scheme: dark)').matches);
      if (dark) {
        document.documentElement.setAttribute('data-theme', 'dark');
        document.documentElement.classList.add('dark');
      } else {
        document.documentElement.setAttribute('data-theme', 'light');
        document.documentElement.classList.remove('dark');
      }
    } catch(e) {}
  </script>`}
</svelte:head>

{#if noShellRoutes.some(r => $page.url.pathname.startsWith(r))}
  {@render children()}
{:else}
  <div class="app-shell">
    <Sidebar />
    <div class="main-wrapper">
      <MobileHeader />
      <main class="main-content">
        {@render children()}
      </main>
    </div>
  </div>
{/if}
