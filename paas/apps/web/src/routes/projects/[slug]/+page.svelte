<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Loader2, Rocket, Wrench, Database, X, Save, Trash2, Plus, Server, Globe, ExternalLink, ArrowRight } from 'lucide-svelte';
  import FrameworkIcon from '$lib/components/icons/FrameworkIcon.svelte';

  const slug = $derived($page.params.slug);
  let project = $state<any>(null);
  let services = $state<any[]>([]);
  let databases = $state<any[]>([]);
  let loading = $state(true);

  function getPreset(svc: any): string {
    try {
      if (svc.resource_json || svc.ResourceJSON) {
        const r = JSON.parse(svc.resource_json || svc.ResourceJSON);
        if (r.presetId) return r.presetId;
      }
    } catch {}
    return svc.kind || svc.Kind || 'node';
  }

  async function loadProjectData() {
    const targetSlug = $page.params.slug || slug || '';
    if (!targetSlug) {
      loading = false;
      return;
    }
    try {
      const [projRes, svcRes, dbRes] = await Promise.all([
        fetch(`/api/v1/projects/${encodeURIComponent(targetSlug)}`, { credentials: 'include' }),
        fetch(`/api/v1/services?projectId=${encodeURIComponent(targetSlug)}`, { credentials: 'include' }),
        fetch(`/api/v1/databases?projectId=${encodeURIComponent(targetSlug)}`, { credentials: 'include' }),
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
  <title>{project?.name || project?.Name || 'Project'} - kloudsPanel</title>
</svelte:head>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={36} /></div>
    <p>Loading project...</p>
  </div>
{:else}
  <div class="page-header">
    <div>
      <div class="page-breadcrumbs">
        <a href="/workspaces">Workspaces</a>
        <span>/</span>
        {#if project?.workspace_slug || project?.WorkspaceSlug || project?.workspace_id || project?.WorkspaceID}
          <a href="/workspaces/{project.workspace_slug || project.WorkspaceSlug || project.workspace_id || project.WorkspaceID}">
            {project.workspace_name || project.WorkspaceName || 'Workspace'}
          </a>
          <span>/</span>
        {/if}
        <span>{project?.name || project?.Name || slug}</span>
      </div>
      <h1 class="page-title">{project?.name || project?.Name || slug}</h1>
      <p class="page-subtitle">{project?.description || 'Project environments and deployed services'}</p>
    </div>
    <div style="display:flex; gap:0.6rem; align-items:center;">
      <button class="btn btn-secondary" style="color:var(--color-danger); border-color:var(--color-border);" onclick={deleteProject}>
        <Trash2 size={14} /> Delete Project
      </button>
      <button class="btn btn-primary" onclick={() => goto(`/projects/${slug}/services/new`)}>
        <Rocket size={15} /> Deploy Service
      </button>
    </div>
  </div>

  <!-- Services Section -->
  <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:0.75rem;">
    <h2 style="font-size:1rem; font-weight:600; color:var(--color-ink); margin:0;">Services ({services.length})</h2>
    {#if services.length > 0}
      <button class="btn btn-secondary" style="padding:4px 10px; font-size:0.75rem; min-height:30px;" onclick={() => goto(`/projects/${slug}/services/new`)}>
        <Plus size={13} /> New Service
      </button>
    {/if}
  </div>

  {#if services.length === 0}
    <div class="empty-state" style="margin-bottom:2rem;">
      <div class="empty-state-icon"><Wrench size={36} /></div>
      <h3>No services deployed yet</h3>
      <p>Deploy Node.js, Python, Go, Java, Rust, PHP, Static sites, Workers, or custom Docker images.</p>
      <button class="btn btn-primary" onclick={() => goto(`/projects/${slug}/services/new`)} style="margin-top:0.75rem">
        <Rocket size={14} /> Deploy First Service
      </button>
    </div>
  {:else}
    <div class="table-wrapper" style="margin-bottom:2rem;">
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
            <tr style="cursor:pointer;" onclick={() => goto(`/services/${svcId}/overview`)}>
              <td>
                <div style="display:flex; align-items:center; gap:8px;">
                  <FrameworkIcon name={getPreset(svc)} size={18} />
                  <a href="/services/{svcId}/overview" style="font-weight:600; color:var(--color-ink); font-size:0.875rem;">
                    {svc.name || svc.Name}
                  </a>
                </div>
              </td>
              <td><span class="badge" style="background:var(--color-surface-subtle); color:var(--color-ink-secondary); text-transform:capitalize;">{svc.kind || svc.Kind || 'web'}</span></td>
              <td>
                <span class={statusClass(svc.runtime_status || svc.RuntimeStatus)}>
                  {#if (svc.runtime_status || svc.RuntimeStatus) === 'deploying'}
                    <Loader2 size={11} class="animate-spin" style="margin-right:2px;" />
                  {/if}
                  {svc.runtime_status || svc.RuntimeStatus || 'draft'}
                </span>
              </td>
              <td>
                {#if svc.endpoint_url || svc.domain || svcSlug}
                  <a 
                    href={svc.endpoint_url || (svc.domain ? `https://${svc.domain}` : `https://${svcSlug}.${typeof window !== 'undefined' ? window.location.hostname : 'example.com'}`)} 
                    target="_blank" 
                    rel="noreferrer"
                    onclick={(e) => e.stopPropagation()}
                    style="display:inline-flex; align-items:center; gap:4px; font-size:0.75rem; color:var(--color-ink); font-weight:500;"
                  >
                    <Globe size={13} /> {svc.domain || (typeof window !== 'undefined' ? `${svcSlug}.${window.location.hostname}` : `${svcSlug}.example.com`)} <ExternalLink size={11} />
                  </a>
                {:else}
                  <span class="text-muted text-xs">-</span>
                {/if}
              </td>
              <td><span class="font-mono text-xs text-muted">:{svc.internal_port || svc.InternalPort || 80}</span></td>
              <td style="text-align:right;" onclick={(e) => e.stopPropagation()}>
                <div style="display:inline-flex; align-items:center; gap:6px;">
                  <a href="/services/{svcId}/overview" class="btn btn-secondary" style="padding:3px 10px; min-height:28px; font-size:0.75rem;">
                    Manage <ArrowRight size={12} />
                  </a>
                  <button 
                    class="btn btn-secondary" 
                    style="padding:3px 6px; min-height:28px; color:var(--color-danger); border-color:transparent;" 
                    aria-label="Delete Service"
                    onclick={(e) => deleteService(e, svc)}
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

  <!-- Databases Section -->
  <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:0.75rem;">
    <h2 style="font-size:1rem; font-weight:600; color:var(--color-ink); margin:0;">Databases ({databases.length})</h2>
    <a href="/databases/new" class="btn btn-secondary" style="padding:4px 10px; font-size:0.75rem; min-height:30px;">
      <Plus size={13} /> New Database
    </a>
  </div>

  {#if databases.length === 0}
    <div class="empty-state">
      <div class="empty-state-icon"><Database size={36} /></div>
      <h3>No databases attached</h3>
      <p>Provision managed PostgreSQL, MySQL, Redis, MongoDB, or ClickHouse instances.</p>
      <a href="/databases/new" class="btn btn-secondary" style="margin-top:0.75rem; font-size:0.75rem;">
        <Database size={14} /> Provision Database
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
            <th style="text-align:right;">Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each databases as db}
            {@const dbId = db.id || db.ID}
            <tr style="cursor:pointer;" onclick={() => goto(`/databases/${dbId}/overview`)}>
              <td>
                <div style="display:flex; align-items:center; gap:8px;">
                  <FrameworkIcon name={db.engine || db.Engine} size={18} />
                  <a href="/databases/{dbId}/overview" style="font-weight:600; color:var(--color-ink); font-size:0.875rem;">
                    {db.name || db.Name}
                  </a>
                </div>
              </td>
              <td><span class="badge" style="background:rgba(56,189,248,0.12); color:#38bdf8; text-transform:uppercase;">{db.engine || db.Engine}</span></td>
              <td><span class="font-mono text-xs text-muted">{db.internal_hostname || db.InternalHostname || '-'}</span></td>
              <td><span class="font-mono text-xs text-muted">:{db.internal_port || db.InternalPort || '-'}</span></td>
              <td><span class={statusClass(db.runtime_status || db.RuntimeStatus)}>{db.runtime_status || db.RuntimeStatus || 'provisioning'}</span></td>
              <td style="text-align:right;" onclick={(e) => e.stopPropagation()}>
                <div style="display:inline-flex; align-items:center; gap:6px;">
                  <a href="/databases/{dbId}/overview" class="btn btn-secondary" style="padding:3px 10px; min-height:28px; font-size:0.75rem;">
                    Manage <ArrowRight size={12} />
                  </a>
                  <button 
                    class="btn btn-secondary" 
                    style="padding:3px 6px; min-height:28px; color:var(--color-danger); border-color:transparent;" 
                    aria-label="Delete Database"
                    onclick={(e) => deleteDatabase(e, db)}
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
