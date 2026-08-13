<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Loader2, Box, FolderOpen, Trash2 } from 'lucide-svelte';

  const { slug } = $derived($page.params);
  let workspace = $state<any>(null);
  let projects = $state<any[]>([]);
  let loading = $state(true);

  async function loadProjects() {
    try {
      const wsRes = await fetch(`/api/v1/workspaces/${slug}`, { credentials: 'include' });
      if (!wsRes.ok) { 
        console.error('Failed to load workspace:', wsRes.status);
        goto('/workspaces'); 
        return; 
      }
      workspace = await wsRes.json();
      const wsId = workspace.id || workspace.ID;
      if (wsId) {
        const projRes = await fetch(`/api/v1/projects?workspaceId=${wsId}`, { credentials: 'include' });
        if (projRes.ok) {
          const projData = await projRes.json();
          projects = projData.projects ?? [];
        }
      }
    } catch (e) {
      console.error(e);
      goto('/workspaces');
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadProjects();
  });

  async function deleteProject(e: Event, proj: any) {
    e.preventDefault();
    e.stopPropagation();
    const id = proj.id || proj.ID;
    if (!confirm(`Are you sure you want to delete project "${proj.name || proj.Name || id}"? All services and deployments in this project will be deleted.`)) return;
    try {
      const res = await fetch(`/api/v1/projects/${encodeURIComponent(id)}`, { method: 'DELETE', credentials: 'include' });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        alert('Failed to delete project: ' + (d.detail || d.message || res.statusText));
      }
      await loadProjects();
    } catch (e: any) {
      console.error(e);
      alert('Failed to delete project: ' + e.message);
    }
  }
</script>

<svelte:head>
  <title>{workspace?.name || workspace?.Name || slug} — kloudsPanel</title>
</svelte:head>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={48} /></div>
    <p>Loading…</p>
  </div>
{:else}
  <div class="page-header">
    <div>
      <p class="text-xs text-muted" style="margin-bottom:0.25rem">
        <a href="/workspaces">Workspaces</a> /
      </p>
      <h1 class="page-title">{workspace?.name || workspace?.Name || slug}</h1>
      <p class="page-subtitle">Manage projects and services in this workspace</p>
    </div>
    <button class="btn btn-primary" onclick={() => goto(`/workspaces/${slug}/projects/new`)}>
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
        <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
      </svg>
      New Project
    </button>
  </div>

  {#if projects.length === 0}
    <div class="empty-state">
      <div class="empty-state-icon"><Box size={48} /></div>
      <h3>No projects yet</h3>
      <p>Create your first project to start deploying services.</p>
      <button class="btn btn-primary mt-4" onclick={() => goto(`/workspaces/${slug}/projects/new`)}>Create Project</button>
    </div>
  {:else}
    <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(320px,1fr));gap:1rem">
      {#each projects as proj}
        {@const projId = proj.id || proj.ID}
        <div 
          class="card" 
          style="cursor:pointer"
          onclick={() => goto(`/projects/${projId}`)}
          onkeydown={(e) => e.key === 'Enter' && goto(`/projects/${projId}`)}
          role="button"
          tabindex="0"
        >
          <div class="card-header" style="display:flex; justify-content:space-between; align-items:flex-start">
            <div>
              <h3 style="margin:0">{proj.name || proj.Name}</h3>
              <span class="text-xs text-muted font-mono">{proj.slug || proj.Slug || ''}</span>
            </div>
            <div style="display:flex; gap:0.5rem; align-items:center;">
              <span class="badge badge-running">active</span>
              <button 
                class="btn btn-secondary" 
                style="padding:4px; color:var(--color-error); border:none; background:transparent" 
                aria-label="Delete Project" 
                onclick={(e) => deleteProject(e, proj)}
              >
                <Trash2 size={16} />
              </button>
            </div>
          </div>
          <p class="text-sm text-muted" style="margin:0 0 0.5rem 0">{proj.description ?? 'No description'}</p>
          <div class="text-xs text-muted" style="color:var(--color-accent)">Open project →</div>
        </div>
      {/each}
    </div>
  {/if}
{/if}
