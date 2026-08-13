<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';

  let workspaces = $state<Array<{id: string, name: string, slug: string}>>([]);
  let loading = $state(true);

  onMount(async () => {
    try {
      const res = await fetch('/api/v1/workspaces', { credentials: 'include' });
      if (res.status === 401) {
        goto('/login');
        return;
      }
      const data = await res.json();
      workspaces = data.workspaces ?? [];
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  });
</script>

<svelte:head>
  <title>Workspaces — kloudsPanel</title>
</svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Workspaces</h1>
    <p class="page-subtitle">Manage your deployment environments and team access</p>
  </div>
  <button class="btn btn-primary" onclick={() => goto('/workspaces/new')}>
    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
      <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
    </svg>
    New Workspace
  </button>
</div>

{#if loading}
  <div class="empty-state">
    <div style="font-size:2rem;opacity:0.4;margin-bottom:1rem">⏳</div>
    <p>Loading workspaces…</p>
  </div>
{:else if workspaces.length === 0}
  <div class="empty-state">
    <div class="empty-state-icon">🏗️</div>
    <h3>No workspaces yet</h3>
    <p>Create your first workspace to start deploying applications.</p>
    <button class="btn btn-primary mt-4" onclick={() => goto('/workspaces/new')}>
      Create Workspace
    </button>
  </div>
{:else}
  <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(300px,1fr));gap:1rem">
    {#each workspaces as ws}
      <a href="/workspaces/{ws.slug}" style="text-decoration:none">
        <div class="card" style="cursor:pointer">
          <div class="card-header">
            <div>
              <h3 style="margin:0">{ws.name}</h3>
              <span class="text-xs text-muted font-mono">/{ws.slug}</span>
            </div>
            <span class="badge badge-running">active</span>
          </div>
          <div class="text-sm text-muted">Click to open workspace →</div>
        </div>
      </a>
    {/each}
  </div>
{/if}
