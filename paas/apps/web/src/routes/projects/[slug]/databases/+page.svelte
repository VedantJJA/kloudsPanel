<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Database, Plus, Loader2, Trash2, ArrowRight, Server, ShieldCheck, Layers } from 'lucide-svelte';
  import FrameworkIcon from '$lib/components/icons/FrameworkIcon.svelte';

  const slug = $derived($page.params.slug || '');
  let project = $state<any>(null);
  let databases = $state<any[]>([]);
  let loading = $state(true);

  async function loadData() {
    if (!slug) return;
    try {
      const [projRes, dbRes] = await Promise.all([
        fetch(`/api/v1/projects/${encodeURIComponent(slug)}`, { credentials: 'include' }),
        fetch(`/api/v1/databases?projectId=${encodeURIComponent(slug)}`, { credentials: 'include' })
      ]);

      if (projRes.ok) {
        project = await projRes.json();
      }
      if (dbRes.ok) {
        const data = await dbRes.json();
        databases = data.databases ?? [];
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadData();
  });

  async function deleteDatabase(e: Event, db: any) {
    e.preventDefault();
    e.stopPropagation();
    const id = db.id || db.ID;
    if (!confirm(`Are you sure you want to delete database "${db.name || db.Name || id}"? All data in this instance will be permanently deleted.`)) return;
    try {
      const res = await fetch(`/api/v1/databases/${encodeURIComponent(id)}`, { method: 'DELETE', credentials: 'include' });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        alert('Failed to delete database: ' + (d.detail || d.message || res.statusText));
      }
      await loadData();
    } catch (e: any) {
      alert('Failed to delete database: ' + e.message);
    }
  }

  function statusClass(status: string) {
    switch (status?.toLowerCase()) {
      case 'ready':
      case 'running':
        return 'badge badge-running';
      case 'deploying':
      case 'building':
      case 'starting':
      case 'restarting':
      case 'provisioning':
        return 'badge badge-building';
      case 'failed':
      case 'error':
      case 'dead':
        return 'badge badge-failed';
      case 'stopped':
      case 'paused':
      case 'exited':
        return 'badge badge-stopped';
      default:
        return 'badge badge-pending';
    }
  }
</script>

<svelte:head>
  <title>Databases - {project?.name || slug} - kloudsPanel</title>
</svelte:head>

<div class="page-header" style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 1rem;">
  <div>
    <div class="page-breadcrumbs">
      {#if project?.workspace_slug || project?.WorkspaceSlug || project?.workspace_id || project?.WorkspaceID}
        <a href="/workspaces/{project.workspace_slug || project.WorkspaceSlug || project.workspace_id || project.WorkspaceID}">
          {project.workspace_name || project.WorkspaceName || 'Workspace'}
        </a>
        <span>/</span>
      {/if}
      <a href="/projects/{slug}">{project?.name || slug}</a>
      <span>/</span>
      <span>Databases</span>
    </div>
    <h1 class="page-title" style="margin: 0; font-size: 1.5rem; font-weight: 600;">Project Databases</h1>
    <p class="page-subtitle" style="margin-top: 4px;">Databases attached to {project?.name || slug} with private networking for deployed services.</p>
  </div>
  {#if databases.length > 0}
    <button 
      class="btn btn-primary" 
      onclick={() => goto(`/databases/new?projectId=${project?.id || project?.ID || slug}&workspaceSlug=${project?.workspace_slug || ''}`)}
    >
      <Plus size={16} /> New Database
    </button>
  {/if}
</div>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem">
      <Loader2 size={36} />
    </div>
    <p>Loading project databases...</p>
  </div>
{:else if databases.length === 0}
  <div class="empty-state" style="padding: 3rem 1rem;">
    <div class="empty-state-icon"><Database size={40} /></div>
    <h3>No databases linked to this project</h3>
    <p class="text-xs text-muted">Provision a dedicated PostgreSQL, MySQL, Redis, MongoDB, or ClickHouse instance for {project?.name || slug}.</p>
    <button 
      class="btn btn-primary" 
      style="margin-top:1rem" 
      onclick={() => goto(`/databases/new?projectId=${project?.id || project?.ID || slug}&workspaceSlug=${project?.workspace_slug || ''}`)}
    >
      <Plus size={16} /> Provision Database
    </button>
  </div>
{:else}
  <div class="table-wrapper">
    <table>
      <thead>
        <tr>
          <th>Database Name</th>
          <th>Engine</th>
          <th>Internal Hostname</th>
          <th>Port</th>
          <th>Status</th>
          <th style="text-align:right;">Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each databases as db}
          {@const dbId = db.id || db.ID}
          <tr 
            style="cursor:pointer" 
            onclick={() => goto(`/databases/${dbId}/overview`)}
          >
            <td>
              <div style="display:flex; align-items:center; gap:8px;">
                <FrameworkIcon name={db.engine || db.Engine || 'database'} size={20} />
                <div>
                  <div style="font-weight:600; color:var(--color-ink); font-size:0.875rem;">
                    {db.name || db.Name}
                  </div>
                  <div class="text-xs text-muted" style="font-size:0.7rem;">
                    {db.database_name || db.DatabaseName || 'defaultdb'}
                  </div>
                </div>
              </div>
            </td>
            <td>
              <span class="badge" style="background:var(--color-surface-subtle); color:var(--color-ink-secondary); text-transform:capitalize;">
                {db.engine || db.Engine} {db.engine_version || db.EngineVersion || ''}
              </span>
            </td>
            <td>
              <span class="font-mono text-xs text-muted">
                {db.internal_hostname || db.InternalHostname || '-'}
              </span>
            </td>
            <td>
              <span class="font-mono text-xs text-muted">
                :{db.internal_port || db.InternalPort || 5432}
              </span>
            </td>
            <td>
              <span class={statusClass(db.runtime_status || db.RuntimeStatus)}>
                {#if (db.runtime_status || db.RuntimeStatus) === 'provisioning' || (db.runtime_status || db.RuntimeStatus) === 'starting'}
                  <Loader2 size={11} class="animate-spin" style="margin-right:2px;" />
                {/if}
                {db.runtime_status || db.RuntimeStatus || 'ready'}
              </span>
            </td>
            <td style="text-align:right;" onclick={(e) => e.stopPropagation()}>
              <div style="display:inline-flex; align-items:center; gap:6px;">
                <a href="/databases/{dbId}/overview" class="btn btn-secondary" style="padding:3px 10px; min-height:28px; font-size:0.75rem;">
                  Manage <ArrowRight size={12} />
                </a>
                <button 
                  class="btn btn-secondary" 
                  style="padding:3px 6px; min-height:28px; color:var(--color-danger); border-color:transparent;" 
                  onclick={(e) => deleteDatabase(e, db)}
                  title="Delete database"
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
