<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Loader2, Box, Trash2, Plus, ArrowRight, FolderKanban } from 'lucide-svelte';

  let workspaces = $state<Array<{ id: string; name: string; slug: string }>>([]);
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
    e.stopPropagation();
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
  <title>Workspaces - kloudsPanel</title>
</svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Workspaces</h1>
    <p class="page-subtitle">Manage deployment environments, projects, and permissions</p>
  </div>
  <button class="btn btn-primary" onclick={() => goto('/workspaces/new')}>
    <Plus size={16} />
    New Workspace
  </button>
</div>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem">
      <Loader2 size={36} />
    </div>
    <p>Loading workspaces...</p>
  </div>
{:else if workspaces.length === 0}
  <div class="empty-state">
    <div class="empty-state-icon"><Box size={40} /></div>
    <h3>No workspaces yet</h3>
    <p>Create your first workspace to start deploying applications.</p>
    <button class="btn btn-primary" style="margin-top:1rem" onclick={() => goto('/workspaces/new')}>
      <Plus size={16} /> Create Workspace
    </button>
  </div>
{:else}
  <div class="table-wrapper">
    <table>
      <thead>
        <tr>
          <th>Workspace Name</th>
          <th>Slug</th>
          <th>Status</th>
          <th style="text-align:right;">Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each workspaces as ws}
          {@const targetSlug = ws.slug || (ws as any).Slug || ws.id || (ws as any).ID}
          {@const displayName = ws.name || (ws as any).Name || 'Unnamed Workspace'}
          <tr 
            style="cursor:pointer" 
            onclick={() => targetSlug && goto(`/workspaces/${targetSlug}`)}
          >
            <td>
              <div style="display:flex; align-items:center; gap:10px;">
                <div style="width:32px; height:32px; border-radius:var(--radius-sm); background:var(--color-surface-subtle); border:1px solid var(--color-border); display:flex; align-items:center; justify-content:center; color:var(--color-ink);">
                  <FolderKanban size={16} />
                </div>
                <div>
                  <a href={`/workspaces/${targetSlug}`} style="font-weight:600; color:var(--color-ink); font-size:0.875rem;">
                    {displayName}
                  </a>
                </div>
              </div>
            </td>
            <td>
              <span class="font-mono text-xs text-muted">/{targetSlug || 'no-slug'}</span>
            </td>
            <td>
              <span class="badge badge-running">active</span>
            </td>
            <td style="text-align:right;" onclick={(e) => e.stopPropagation()}>
              <div style="display:inline-flex; align-items:center; gap:6px;">
                <button 
                  class="btn btn-secondary" 
                  style="padding:4px 10px; min-height:30px; font-size:0.75rem;" 
                  onclick={() => targetSlug && goto(`/workspaces/${targetSlug}`)}
                >
                  Open <ArrowRight size={13} />
                </button>
                <button 
                  class="btn btn-secondary" 
                  style="padding:4px 8px; min-height:30px; color:var(--color-danger); border-color:transparent;" 
                  aria-label="Delete Workspace" 
                  onclick={(e) => deleteWorkspace(e, ws)}
                >
                  <Trash2 size={14} />
                </button>
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}
