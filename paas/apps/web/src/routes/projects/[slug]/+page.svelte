<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Loader2, Rocket, Wrench, Database, X, Save, Trash2, Plus, Server, Globe, ExternalLink } from 'lucide-svelte';

  const { slug } = $derived($page.params);
  let project = $state<any>(null);
  let services = $state<any[]>([]);
  let databases = $state<any[]>([]);
  let loading = $state(true);

  async function loadProjectData() {
    try {
      const [projRes, svcRes, dbRes] = await Promise.all([
        fetch(`/api/v1/projects/${slug}`, { credentials: 'include' }),
        fetch(`/api/v1/services?projectId=${slug}`, { credentials: 'include' }),
        fetch(`/api/v1/databases?projectId=${slug}`, { credentials: 'include' }),
      ]);
      if (projRes.ok) {
        project = await projRes.json();
      }
      if (svcRes.ok) {
        services = (await svcRes.json()).services ?? [];
      }
      if (dbRes.ok) {
        databases = (await dbRes.json()).databases ?? [];
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  let pollInterval: any = null;

  onMount(() => {
    loadProjectData();
    pollInterval = setInterval(loadProjectData, 4000);
    return () => {
      if (pollInterval) clearInterval(pollInterval);
    };
  });

  async function deleteService(e: Event, svc: any) {
    e.preventDefault();
    e.stopPropagation();
    const id = svc.id || svc.ID;
    if (!confirm(`Are you sure you want to delete service "${svc.name || svc.Name || id}"? All associated deployments will be deleted.`)) return;
    try {
      const res = await fetch(`/api/v1/services/${encodeURIComponent(id)}`, { method: 'DELETE', credentials: 'include' });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        alert('Failed to delete service: ' + (d.detail || d.message || res.statusText));
      }
      await loadProjectData();
    } catch (e: any) {
      alert('Failed to delete service: ' + e.message);
    }
  }

  async function deleteDatabase(e: Event, db: any) {
    e.preventDefault();
    e.stopPropagation();
    const id = db.id || db.ID;
    if (!confirm(`Are you sure you want to delete database "${db.name || db.Name || id}"?`)) return;
    try {
      const res = await fetch(`/api/v1/databases/${encodeURIComponent(id)}`, { method: 'DELETE', credentials: 'include' });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        alert('Failed to delete database: ' + (d.detail || d.message || res.statusText));
      }
      await loadProjectData();
    } catch (e: any) {
      alert('Failed to delete database: ' + e.message);
    }
  }

  async function deleteProject() {
    const id = project?.id || project?.ID || slug;
    if (!confirm(`Are you sure you want to delete project "${project?.name || project?.Name || id}"? All services and databases in this project will be permanently deleted.`)) return;
    try {
      const res = await fetch(`/api/v1/projects/${encodeURIComponent(id)}`, { method: 'DELETE', credentials: 'include' });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        alert('Failed to delete project: ' + (d.detail || d.message || res.statusText));
        return;
      }
      const wsId = project?.workspace_id || project?.WorkspaceID;
      if (wsId) {
        goto(`/workspaces/${wsId}`);
      } else {
        goto('/workspaces');
      }
    } catch (e: any) {
      alert('Failed to delete project: ' + e.message);
    }
  }

  const statusClass = (s: string) => `badge badge-${s || 'draft'}`;
</script>

<svelte:head>
  <title>{project?.name || project?.Name || 'Project'} - kloudsPanel</title>
</svelte:head>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={48} /></div>
    <p>Loading project...</p>
  </div>
{:else}
  <div class="page-header">
    <div>
      <div class="page-breadcrumbs">
        <a href="/workspaces">Workspaces</a> /
        {#if project?.workspace_id || project?.WorkspaceID}
          <a href="/workspaces/{project.workspace_id || project.WorkspaceID}">Workspace</a> /
        {/if}
        <span>{project?.name || project?.Name || slug}</span>
      </div>
      <h1 class="page-title">{project?.name || project?.Name || slug}</h1>
      <p class="page-subtitle">{project?.description ?? 'Project environments & deployments'}</p>
    </div>
    <div style="display:flex; gap:0.75rem; align-items:center;">
      <button class="btn btn-secondary" style="color:var(--color-error); border-color:var(--color-border);" onclick={deleteProject}>
        <Trash2 size={16} /> Delete Project
      </button>
      <button class="btn btn-primary" onclick={() => goto(`/projects/${slug}/services/new`)}>
        <Rocket size={16} /> Deploy Service
      </button>
    </div>
  </div>

  <!-- Services -->
  <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:1rem;">
    <h2 style="font-size:1.125rem; font-weight:600; color:var(--color-ink); margin:0;">Services ({services.length})</h2>
    {#if services.length > 0}
      <button class="btn btn-primary" style="padding:0.35rem 0.85rem; font-size:0.8125rem;" onclick={() => goto(`/projects/${slug}/services/new`)}>
        <Plus size={14} /> New Service
      </button>
    {/if}
  </div>

  {#if services.length === 0}
    <div class="empty-state" style="padding:2.5rem; background:var(--color-surface); border:1px solid var(--color-border); border-radius:var(--radius-lg); margin-bottom:2.5rem;">
      <div class="empty-state-icon"><Wrench size={48} /></div>
      <h3>No services deployed yet</h3>
      <p>Deploy Node.js, Python, Go, Java, Rust, PHP, Static sites, Workers, or custom Docker images.</p>
      <button class="btn btn-primary" onclick={() => goto(`/projects/${slug}/services/new`)} style="margin-top:1rem">
        <Rocket size={16} /> Deploy First Service
      </button>
    </div>
  {:else}
    <div class="table-wrapper" style="margin-bottom:2.5rem;">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Status</th>
            <th>Live Endpoint</th>
            <th>Port</th>
            <th style="text-align:right;">Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each services as svc}
            {@const svcId = svc.id || svc.ID}
            {@const svcSlug = svc.slug || svc.Slug}
            <tr>
              <td>
                <a href="/services/{svcId}/overview" style="font-weight:600; color:var(--color-ink); font-size: 0.9375rem;">
                  {svc.name || svc.Name}
                </a>
              </td>
              <td><span class="badge" style="background:#f1f5f9; color:#334155; text-transform:capitalize;">{svc.kind || svc.Kind || 'web'}</span></td>
              <td>
                <span class={statusClass(svc.runtime_status || svc.RuntimeStatus)}>
                  {#if (svc.runtime_status || svc.RuntimeStatus) === 'deploying'}
                    <span class="animate-spin" style="display:inline-block; margin-right:4px;">⟳</span>
                  {/if}
                  {svc.runtime_status || svc.RuntimeStatus || 'draft'}
                </span>
              </td>
              <td>
                {#if svc.endpoint_url || svcSlug}
                  <a 
                    href={svc.endpoint_url || `https://${svcSlug}.klouds.online`} 
                    target="_blank" 
                    rel="noreferrer"
                    style="display:inline-flex; align-items:center; gap:4px; font-size:0.8125rem; color:var(--color-accent); font-weight:500;"
                  >
                    <Globe size={13} /> {svcSlug}.klouds.online <ExternalLink size={11} />
                  </a>
                {:else}
                  <span class="text-muted text-xs">-</span>
                {/if}
              </td>
              <td><span class="font-mono text-xs">:{svc.internal_port || svc.InternalPort || 80}</span></td>
              <td style="text-align:right;">
                <div style="display:inline-flex; align-items:center; gap:0.5rem;">
                  <a href="/services/{svcId}/overview" class="btn btn-secondary" style="padding:4px 12px; min-height:32px; font-size:0.8125rem;">
                    Manage →
                  </a>
                  <button 
                    class="btn btn-secondary" 
                    style="padding:4px 8px; min-height:32px; color:var(--color-error); border-color:transparent;" 
                    aria-label="Delete Service"
                    onclick={(e) => deleteService(e, svc)}
                  >
                    <Trash2 size={16} />
                  </button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  <!-- Databases -->
  <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:1rem;">
    <h2 style="font-size:1.125rem; font-weight:600; color:var(--color-ink); margin:0;">Databases ({databases.length})</h2>
    <a href="/databases/new" class="btn btn-secondary" style="padding:0.35rem 0.85rem; font-size:0.8125rem;">
      <Plus size={14} /> New Database
    </a>
  </div>

  {#if databases.length === 0}
    <div class="empty-state" style="padding:2.5rem; background:var(--color-surface); border:1px solid var(--color-border); border-radius:var(--radius-lg);">
      <div class="empty-state-icon"><Database size={48} /></div>
      <h3>No databases attached</h3>
      <p>Provision managed PostgreSQL, MySQL, Redis, MongoDB, or ClickHouse instances.</p>
      <a href="/databases/new" class="btn btn-secondary" style="margin-top:1rem">
        <Database size={16} /> Provision Database
      </a>
    </div>
  {:else}
    <div class="table-wrapper">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Engine</th>
            <th>Internal Host</th>
            <th>Port</th>
            <th>Status</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each databases as db}
            {@const dbId = db.id || db.ID}
            <tr>
              <td>
                <a href="/databases/{dbId}/overview" style="font-weight:600; color:var(--color-ink);">
                  {db.name || db.Name}
                </a>
              </td>
              <td><span class="badge" style="background:#e0f2fe; color:#0369a1; text-transform:uppercase;">{db.engine || db.Engine}</span></td>
              <td><span class="font-mono text-xs">{db.internal_hostname || db.InternalHostname || '-'}</span></td>
              <td><span class="font-mono text-xs">:{db.internal_port || db.InternalPort || '-'}</span></td>
              <td><span class={statusClass(db.runtime_status || db.RuntimeStatus)}>{db.runtime_status || db.RuntimeStatus || 'ready'}</span></td>
              <td style="text-align:right;">
                <div style="display:inline-flex; align-items:center; gap:0.5rem;">
                  <a href="/databases/{dbId}/overview" class="btn btn-secondary" style="padding:4px 12px; min-height:32px; font-size:0.8125rem;">
                    Manage →
                  </a>
                  <button 
                    class="btn btn-secondary" 
                    style="padding:4px 8px; min-height:32px; color:var(--color-error); border-color:transparent;" 
                    aria-label="Delete Database"
                    onclick={(e) => deleteDatabase(e, db)}
                  >
                    <Trash2 size={16} />
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
