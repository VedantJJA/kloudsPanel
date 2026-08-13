<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Loader2, Rocket, Wrench, Database, X, Save, Trash2, Plus, Server, Globe } from 'lucide-svelte';

  const { slug } = $derived($page.params);
  let project = $state<any>(null);
  let services = $state<any[]>([]);
  let databases = $state<any[]>([]);
  let loading = $state(true);
  let showDeployModal = $state(false);

  // New service form state
  let svcName = $state('');
  let svcKind = $state('web');
  let svcImage = $state('nginx:alpine');
  let svcPort = $state(80);
  let svcAutoDeploy = $state(true);
  let svcEnvVars = $state<Array<{ key: string; value: string }>>([]);
  let svcSaving = $state(false);
  let svcError = $state<string | null>(null);

  const imagePresets = [
    { label: 'Nginx (Static/Reverse Proxy)', image: 'nginx:alpine', port: 80, kind: 'web' },
    { label: 'Node.js (Express/Nest/Next)', image: 'node:18-alpine', port: 3000, kind: 'web' },
    { label: 'Python (FastAPI/Flask/Django)', image: 'python:3.11-slim', port: 8000, kind: 'web' },
    { label: 'Go (HTTP Server)', image: 'golang:1.22-alpine', port: 8080, kind: 'web' },
    { label: 'Custom Docker Image', image: '', port: 80, kind: 'web' },
  ];

  function selectPreset(p: any) {
    if (p.image) {
      svcImage = p.image;
    }
    svcPort = p.port;
    svcKind = p.kind;
  }

  function addEnvVar() {
    svcEnvVars = [...svcEnvVars, { key: '', value: '' }];
  }

  function removeEnvVar(index: number) {
    svcEnvVars = svcEnvVars.filter((_, i) => i !== index);
  }

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

  onMount(() => {
    loadProjectData();
  });

  async function createService(e: Event) {
    e.preventDefault();
    svcSaving = true;
    svcError = null;
    try {
      const svcSlug = svcName.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)+/g, '');
      const envMap: Record<string, string> = {};
      for (const item of svcEnvVars) {
        if (item.key.trim()) {
          envMap[item.key.trim()] = item.value;
        }
      }
      const res = await fetch('/api/v1/services', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ 
          projectId: slug, 
          name: svcName, 
          slug: svcSlug, 
          kind: svcKind,
          internalPort: Number(svcPort) || 80,
          resourceJson: JSON.stringify({ image: svcImage, env: envMap, autoDeploy: svcAutoDeploy })
        })
      });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        throw new Error(d.detail || d.message || 'Failed to create service');
      }
      const svc = await res.json();
      const svcId = svc.id || svc.ID;
      
      // Trigger initial deployment automatically
      await fetch(`/api/v1/services/${svcId}/deploy`, { method: 'POST', credentials: 'include' });

      showDeployModal = false;
      goto(`/services/${svcId}/overview`);
    } catch (e: any) {
      svcError = e.message;
    } finally {
      svcSaving = false;
    }
  }

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
  <title>{project?.name || project?.Name || 'Project'} — kloudsPanel</title>
</svelte:head>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={48} /></div>
    <p>Loading project…</p>
  </div>
{:else}
  <div class="page-header">
    <div>
      <p class="text-xs text-muted" style="margin-bottom:0.25rem">
        <a href="/workspaces">Workspaces</a> /
        {#if project?.workspace_id || project?.WorkspaceID}
          <a href="/workspaces/{project.workspace_id || project.WorkspaceID}">Workspace</a> /
        {/if}
      </p>
      <h1 class="page-title">{project?.name || project?.Name || slug}</h1>
      <p class="page-subtitle">{project?.description ?? 'Project environments & deployments'}</p>
    </div>
    <div style="display:flex; gap:0.75rem; align-items:center;">
      <button class="btn btn-secondary" style="color:var(--color-error); border-color:var(--color-border);" onclick={deleteProject}>
        <Trash2 size={16} /> Delete Project
      </button>
      <button class="btn btn-primary" onclick={() => showDeployModal = true}>
        <Rocket size={16} /> Deploy Service
      </button>
    </div>
  </div>

  <!-- Services -->
  <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:1rem;">
    <h2 style="font-size:1.125rem; font-weight:600; color:var(--color-ink); margin:0;">Services ({services.length})</h2>
    {#if services.length > 0}
      <button class="btn btn-primary" style="padding:0.35rem 0.85rem; font-size:0.8125rem;" onclick={() => showDeployModal = true}>
        <Plus size={14} /> New Service
      </button>
    {/if}
  </div>

  {#if services.length === 0}
    <div class="empty-state" style="padding:2.5rem; background:var(--color-surface); border:1px solid var(--color-border); border-radius:var(--radius-lg); margin-bottom:2rem;">
      <div class="empty-state-icon"><Wrench size={48} /></div>
      <h3>No services deployed yet</h3>
      <p>Deploy your web applications, APIs, workers, or static sites.</p>
      <button class="btn btn-primary" onclick={() => showDeployModal = true} style="margin-top:1rem">
        <Rocket size={16} /> Deploy First Service
      </button>
    </div>
  {:else}
    <div class="table-wrapper" style="margin-bottom:2.5rem;">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Kind</th>
            <th>Status</th>
            <th>Port</th>
            <th>Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each services as svc}
            {@const svcId = svc.id || svc.ID}
            <tr>
              <td>
                <a href="/services/{svcId}/overview" style="font-weight:600; color:var(--color-ink);">
                  {svc.name || svc.Name}
                </a>
              </td>
              <td><span class="badge" style="background:#f1f5f9; color:#334155;">{svc.kind || svc.Kind || 'web'}</span></td>
              <td><span class={statusClass(svc.runtime_status || svc.RuntimeStatus)}>{svc.runtime_status || svc.RuntimeStatus || 'draft'}</span></td>
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
            <tr>
              <td style="font-weight:600;">{db.name || db.Name}</td>
              <td><span class="badge" style="background:#e0f2fe; color:#0369a1;">{db.engine || db.Engine}</span></td>
              <td><span class="font-mono text-xs">{db.internal_hostname || db.InternalHostname || '—'}</span></td>
              <td><span class="font-mono text-xs">:{db.internal_port || db.InternalPort || '—'}</span></td>
              <td><span class={statusClass(db.runtime_status || db.RuntimeStatus)}>{db.runtime_status || db.RuntimeStatus || 'ready'}</span></td>
              <td style="text-align:right;">
                <button 
                  class="btn btn-secondary" 
                  style="padding:4px 8px; min-height:32px; color:var(--color-error); border-color:transparent;" 
                  aria-label="Delete Database"
                  onclick={(e) => deleteDatabase(e, db)}
                >
                  <Trash2 size={16} />
                </button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
{/if}

<!-- Deploy Service Modal -->
{#if showDeployModal}
  <div 
    style="position:fixed; inset:0; background:rgba(11,31,58,0.5); z-index:100; display:flex; align-items:center; justify-content:center; padding:1rem;"
    onclick={() => showDeployModal = false}
    onkeydown={(e) => e.key === 'Escape' && (showDeployModal = false)}
    role="button"
    tabindex="0"
  >
    <div 
      class="card" 
      style="width:min(580px, 95vw); max-height:85vh; overflow-y:auto; padding:1.75rem;" 
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      tabindex="-1"
    >
      <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:1.5rem; border-bottom:1px solid var(--color-border); padding-bottom:1rem;">
        <div>
          <h2 style="margin:0; font-size:1.25rem;">Deploy New Service</h2>
          <p class="text-xs text-muted" style="margin-top:0.25rem;">Configure container image, ports, and environment variables.</p>
        </div>
        <button class="btn btn-secondary" style="padding:0.4rem; min-height:auto; border-radius:var(--radius-sm);" onclick={() => showDeployModal = false}>
          <X size={18} />
        </button>
      </div>

      <form onsubmit={createService}>
        <div class="form-group" style="margin-bottom:1.25rem;">
          <label for="svc-name-input" class="form-label">Service Name</label>
          <input 
            id="svc-name-input" 
            type="text" 
            class="form-input" 
            placeholder="e.g. web-app, api-server" 
            bind:value={svcName} 
            required 
          />
        </div>

        <div class="form-group" style="margin-bottom:1.25rem;">
          <label for="svc-preset-select" class="form-label">Runtime / Image Preset</label>
          <div style="display:grid; grid-template-columns:1fr; gap:0.5rem;">
            {#each imagePresets as p}
              <button 
                type="button" 
                class="card" 
                style="padding:0.6rem 0.85rem; text-align:left; cursor:pointer; border:1px solid {svcImage === p.image ? 'var(--color-accent)' : 'var(--color-border)'}; background:{svcImage === p.image ? 'rgba(0,166,166,0.05)' : 'var(--color-surface)'}; display:flex; align-items:center; justify-content:space-between;"
                onclick={() => selectPreset(p)}
              >
                <div>
                  <div style="font-size:0.875rem; font-weight:600;">{p.label}</div>
                  {#if p.image}
                    <span class="font-mono text-xs text-muted">{p.image}</span>
                  {/if}
                </div>
                <span class="badge" style="background:#f1f5f9; color:#475569;">Port {p.port}</span>
              </button>
            {/each}
          </div>
        </div>

        <div style="display:grid; grid-template-columns:1fr 120px; gap:1rem; margin-bottom:1.25rem;">
          <div class="form-group">
            <label for="svc-image-input" class="form-label">Docker Image Reference</label>
            <input 
              id="svc-image-input" 
              type="text" 
              class="form-input font-mono" 
              placeholder="e.g. nginx:alpine, redis:7" 
              bind:value={svcImage} 
              required 
            />
          </div>

          <div class="form-group">
            <label for="svc-port-input" class="form-label">Internal Port</label>
            <input 
              id="svc-port-input" 
              type="number" 
              class="form-input font-mono" 
              placeholder="80" 
              bind:value={svcPort} 
              required 
              min="1" 
              max="65535" 
            />
          </div>
        </div>

        <!-- Environment Variables -->
        <div style="margin-bottom:1.5rem;">
          <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:0.5rem;">
            <label class="form-label" style="margin:0;">Environment Variables</label>
            <button type="button" class="btn btn-secondary" style="padding:2px 8px; min-height:28px; font-size:0.75rem;" onclick={addEnvVar}>
              <Plus size={12} /> Add Variable
            </button>
          </div>
          {#if svcEnvVars.length === 0}
            <p class="text-xs text-muted" style="margin:0;">No environment variables added.</p>
          {:else}
            <div style="display:flex; flex-direction:column; gap:0.5rem;">
              {#each svcEnvVars as env, i}
                <div style="display:flex; gap:0.5rem; align-items:center;">
                  <input type="text" class="form-input font-mono text-xs" placeholder="KEY" bind:value={env.key} style="flex:1;" />
                  <input type="text" class="form-input font-mono text-xs" placeholder="VALUE" bind:value={env.value} style="flex:1;" />
                  <button type="button" class="btn btn-secondary" style="padding:4px; color:var(--color-error);" onclick={() => removeEnvVar(i)}>
                    <X size={14} />
                  </button>
                </div>
              {/each}
            </div>
          {/if}
        </div>

        {#if svcError}
          <div class="alert alert-error" style="margin-bottom:1rem; background:#fee2e2; border:1px solid #fca5a5; color:#991b1b; padding:0.75rem 1rem; border-radius:var(--radius-md); font-size:0.875rem;">
            {svcError}
          </div>
        {/if}

        <div style="display:flex; justify-content:flex-end; gap:0.75rem; padding-top:1rem; border-top:1px solid var(--color-border);">
          <button type="button" class="btn btn-secondary" onclick={() => showDeployModal = false} disabled={svcSaving}>Cancel</button>
          <button type="submit" class="btn btn-primary" disabled={svcSaving || !svcName || !svcImage}>
            {#if svcSaving}
              <Loader2 size={16} class="animate-spin" style="margin-right:0.5rem;" />
              Deploying...
            {:else}
              <Rocket size={16} style="margin-right:0.5rem;" />
              Deploy Service
            {/if}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}
