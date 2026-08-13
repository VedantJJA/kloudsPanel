<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Loader2, Box, Trash2 } from 'lucide-svelte';

  let workspaces = $state<Array<{id: string, name: string, slug: string}>>([]);
  let loading = $state(true);

  async function loadWorkspaces() {
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
  }

  onMount(() => {
    loadWorkspaces();
  });

  async function deleteWorkspace(e: Event, ws: any) {
    e.preventDefault();
    const id = ws.id || ws.ID || ws.slug || ws.Slug;
    if (!id || id === 'undefined') {
      alert('Error: Workspace has no valid ID or slug to delete.');
      return;
    }
    if (!confirm(`Are you sure you want to delete workspace "${ws.name || ws.Name || id}"?`)) return;
    try {
      const res = await fetch(`/api/v1/workspaces/${encodeURIComponent(id)}`, { method: 'DELETE', credentials: 'include' });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        alert('Failed to delete workspace: ' + (d.detail || d.message || res.statusText));
      }
      await loadWorkspaces();
    } catch (e: any) {
      console.error(e);
      alert('Failed to delete workspace: ' + e.message);
    }
  }
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
    <div class="animate-spin text-muted" style="margin-bottom:1rem">
      <Loader2 size={48} />
    </div>
    <p>Loading workspaces…</p>
  </div>
{:else if workspaces.length === 0}
  <div class="empty-state">
    <div class="empty-state-icon" style="color:var(--text-muted); margin-bottom: 1rem;"><Box size={48} /></div>
    <h3>No workspaces yet</h3>
    <p>Create your first workspace to start deploying applications.</p>
    <button class="btn btn-primary mt-4" onclick={() => goto('/workspaces/new')}>
      Create Workspace
    </button>
  </div>
{:else}
  <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(300px,1fr));gap:1rem">
    {#each workspaces as ws}
      {@const targetSlug = ws.slug || (ws as any).Slug || ws.id || (ws as any).ID}
      {@const displayName = ws.name || (ws as any).Name || 'Unnamed Workspace'}
      <div 
        class="card" 
        style="cursor:pointer" 
        onclick={() => targetSlug && goto(`/workspaces/${targetSlug}`)}
        onkeydown={(e) => e.key === 'Enter' && targetSlug && goto(`/workspaces/${targetSlug}`)}
        role="button"
        tabindex="0"
      >
        <div class="card-header" style="display:flex; justify-content:space-between; align-items:flex-start">
          <div>
            <h3 style="margin:0">{displayName}</h3>
            <span class="text-xs text-muted font-mono">/{targetSlug || 'no-slug'}</span>
          </div>
          <div style="display:flex; gap:0.5rem; align-items:center;">
            <span class="badge badge-running">active</span>
            <button 
              class="btn btn-secondary" 
              style="padding:4px; color:var(--color-error); border:none; background:transparent" 
              aria-label="Delete Workspace" 
              onclick={(e) => { e.stopPropagation(); deleteWorkspace(e, ws); }}
            >
              <Trash2 size={16} />
            </button>
          </div>
        </div>
        <div class="text-sm text-muted">Click to open workspace →</div>
      </div>
    {/each}
  </div>
{/if}
