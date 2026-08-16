<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Loader2, Box, FolderKanban, Trash2, Plus, ArrowRight } from 'lucide-svelte';

  const slug = $derived($page.params.slug);
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
  <title>{workspace?.name || workspace?.Name || slug} - kloudsPanel</title>
</svelte:head>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={36} /></div>
    <p>Loading projects...</p>
  </div>
{:else}
  <div class="page-header">
    <div>
      <div class="page-breadcrumbs">
        <a href="/workspaces">Workspaces</a>
        <span>/</span>
        <span>{workspace?.name || workspace?.Name || slug}</span>
      </div>
      <h1 class="page-title">{workspace?.name || workspace?.Name || slug}</h1>
      <p class="page-subtitle">Manage projects, applications, and microservices</p>
    </div>
    <button class="btn btn-primary" onclick={() => goto(`/workspaces/${slug}/projects/new`)}>
      <Plus size={16} />
      New Project
    </button>
  </div>

  {#if projects.length === 0}
    <div class="empty-state">
      <div class="empty-state-icon"><Box size={40} /></div>
      <h3>No projects yet</h3>
      <p>Create your first project to start deploying services.</p>
      <button class="btn btn-primary" style="margin-top:1rem" onclick={() => goto(`/workspaces/${slug}/projects/new`)}>
        <Plus size={16} /> Create Project
      </button>
    </div>
  {:else}
    <div class="table-wrapper">
      <table>
        <thead>
          <tr>
            <th>Project</th>
            <th>Slug</th>
            <th>Description</th>
            <th>Status</th>
            <th style="text-align:right;">Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each projects as proj}
            {@const projId = proj.id || proj.ID}
            <tr 
              style="cursor:pointer"
              onclick={() => goto(`/projects/${projId}`)}
            >
              <td>
                <div style="display:flex; align-items:center; gap:10px;">
                  <div style="width:32px; height:32px; border-radius:var(--radius-sm); background:var(--color-surface-subtle); border:1px solid var(--color-border); display:flex; align-items:center; justify-content:center; color:var(--color-ink);">
                    <FolderKanban size={16} />
                  </div>
                  <div>
                    <a href={`/projects/${projId}`} style="font-weight:600; color:var(--color-ink); font-size:0.875rem;">
                      {proj.name || proj.Name}
                    </a>
                  </div>
                </div>
              </td>
              <td>
                <span class="font-mono text-xs text-muted">/{proj.slug || proj.Slug || ''}</span>
              </td>
              <td>
                <span class="text-xs text-muted truncate" style="max-width:240px; display:inline-block;">{proj.description || 'No description'}</span>
              </td>
              <td>
                <span class="badge badge-running">active</span>
              </td>
              <td style="text-align:right;" onclick={(e) => e.stopPropagation()}>
                <div style="display:inline-flex; align-items:center; gap:6px;">
                  <button 
                    class="btn btn-secondary" 
                    style="padding:4px 10px; min-height:30px; font-size:0.75rem;" 
                    onclick={() => goto(`/projects/${projId}`)}
                  >
                    Open <ArrowRight size={13} />
                  </button>
                  <button 
                    class="btn btn-secondary" 
                    style="padding:4px 8px; min-height:30px; color:var(--color-danger); border-color:transparent;" 
                    aria-label="Delete Project" 
                    onclick={(e) => deleteProject(e, proj)}
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
{/if}
