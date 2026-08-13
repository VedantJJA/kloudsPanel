<script lang="ts">
  import '../app.css';
  import Sidebar from '$lib/components/layout/Sidebar.svelte';
  import { page } from '$app/stores';

  let { children } = $props();

  // Determine if we should show the shell (not on login/signup/pending)
  const noShellRoutes = ['/login', '/signup', '/access/pending'];
  $effect(() => {
    // Reactive to page changes
  });
</script>

<svelte:head>
  <title>kloudsPanel — Self-Hosted PaaS</title>
  <meta name="description" content="Lightweight self-hosted platform-as-a-service. Deploy any stack with zero Docker knowledge." />
</svelte:head>

{#if noShellRoutes.some(r => $page.url.pathname.startsWith(r))}
  {@render children()}
{:else}
  <div class="app-shell">
    <Sidebar />
    <main class="main-content">
      {@render children()}
    </main>
  </div>
{/if}
