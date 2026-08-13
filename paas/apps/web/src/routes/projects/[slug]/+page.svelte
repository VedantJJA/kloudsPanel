<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Loader2, Rocket, Wrench, Database, X, Save } from 'lucide-svelte';

  const { slug } = $derived($page.params);
  let project = $state<any>(null);
  let services = $state<any[]>([]);
  let databases = $state<any[]>([]);
  let loading = $state(true);
  let showDeployModal = $state(false);

  // New service form state
  let svcName = $state('');
  let svcKind = $state('web');
  let svcSaving = $state(false);
  let svcError = $state<string | null>(null);

  onMount(async () => {
    try {
      const [projRes, svcRes, dbRes] = await Promise.all([
        fetch(`/api/v1/projects/${slug}`, { credentials: 'include' }),
        fetch(`/api/v1/services?projectId=${slug}`, { credentials: 'include' }),
        fetch(`/api/v1/databases?projectId=${slug}`, { credentials: 'include' }),
      ]);
      project = await projRes.json();
      services = (await svcRes.json()).services ?? [];
      databases = (await dbRes.json()).databases ?? [];
    } finally {
      loading = false;
    }
  });

  async function createService(e: Event) {
    e.preventDefault();
    svcSaving = true;
    svcError = null;
    try {
      const svcSlug = svcName.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)+/g, '');
      const res = await fetch('/api/v1/services', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ projectId: slug, name: svcName, slug: svcSlug, kind: svcKind })
      });
      if (!res.ok) {
        const d = await res.json();
        throw new Error(d.detail || d.message || 'Failed to create service');
      }
      const svc = await res.json();
      showDeployModal = false;
      goto(`/services/${svc.id}/overview`);
    } catch (e: any) {
      svcError = e.message;
    } finally {
      svcSaving = false;
    }
  }

  const statusClass = (s: string) => `badge badge-${s}`;
</script>

<svelte:head>
  <title>{project?.name ?? 'Project'} — kloudsPanel</title>
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
        <a href="/workspaces/{project?.workspace_id}">Workspace</a> /
      </p>
      <h1 class="page-title">{project?.name ?? slug}</h1>
      <p class="page-subtitle">{project?.description ?? ''}</p>
    </div>
    <button class="btn btn-primary" onclick={() => showDeployModal = true}>
      <Rocket size={16} /> Deploy Service
    </button>
  </div>

  <!-- Services -->
  <h2 style="font-size:1rem;font-weight:600;margin-bottom:1rem;color:var(--color-ink)">Services</h2>
  {#if services.length === 0}
    <div class="empty-state" style="padding:2rem">
      <div class="empty-state-icon"><Wrench size={48} /></div>
      <h3>No services yet</h3>
      <p>Deploy your first service using the wizard.</p>
      <button class="btn btn-primary" onclick={() => showDeployModal = true} style="margin-top:1rem">Deploy Service</button>
    </div>
  {:else}
    <div class="table-wrapper" style="margin-bottom:2rem">
      <table>
        <thead>
          <tr>
            <th>Name</th><th>Kind</th><th>Status</th><th>Domain</th><th></th>
          </tr>
        </thead>
        <tbody>
          {#each services as svc}
            <tr>
              <td><a href="/services/{svc.id}/overview" style="font-weight:500">{svc.name}</a></td>
              <td><span class="badge" style="background:#f1f5f9;color:#334155">{svc.kind}</span></td>
              <td><span class={statusClass(svc.runtime_status)}>{svc.runtime_status}</span></td>
              <td><span class="font-mono text-xs">{svc.domain ?? '—'}</span></td>
              <td style="text-align:right">
                <a href="/services/{svc.id}/overview" class="btn btn-secondary" style="padding:4px 12px;min-height:32px;font-size:0.8125rem">
                  Open →
                </a>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  <!-- Databases -->
  <h2 style="font-size:1rem;font-weight:600;margin-bottom:1rem;color:var(--color-ink)">Databases</h2>
  {#if databases.length === 0}
    <div class="empty-state" style="padding:2rem">
      <div class="empty-state-icon"><Database size={48} /></div>
      <h3>No databases yet</h3>
      <p>Add a PostgreSQL, MySQL, Redis, MongoDB, or ClickHouse database.</p>
      <a href="/databases/new" class="btn btn-secondary" style="margin-top:1rem">Add Database</a>
    </div>
  {:else}
    <div class="table-wrapper">
      <table>
        <thead><tr><th>Name</th><th>Engine</th><th>Status</th></tr></thead>
        <tbody>
          {#each databases as db}
            <tr>
              <td>{db.name}</td>
              <td>{db.engine} {db.engine_version}</td>
              <td><span class={statusClass(db.runtime_status)}>{db.runtime_status}</span></td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
{/if}

<!-- Deploy Service Modal -->
{#if showDeployModal}
  <div style="position:fixed;inset:0;background:rgba(0,0,0,0.6);z-index:100;display:flex;align-items:center;justify-content:center;" onclick={() => showDeployModal = false}>
    <div class="card" style="width:min(480px,90vw);max-height:80vh;overflow-y:auto;" onclick={(e) => e.stopPropagation()}>
      <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:1.5rem">
        <h2 style="margin:0;font-size:1.25rem">Deploy New Service</h2>
        <button class="btn btn-secondary" style="padding:0.25rem" onclick={() => showDeployModal = false}><X size={18} /></button>
      </div>
      <form onsubmit={createService}>
        <div style="margin-bottom:1.25rem">
          <label style="display:block;margin-bottom:0.5rem;font-weight:500">Service Name</label>
          <input type="text" class="input" placeholder="e.g. web-server, api" bind:value={svcName} required style="width:100%;box-sizing:border-box" />
        </div>
        <div style="margin-bottom:1.5rem">
          <label style="display:block;margin-bottom:0.5rem;font-weight:500">Type</label>
          <select class="input" bind:value={svcKind} style="width:100%;box-sizing:border-box">
            <option value="web">Web Service</option>
            <option value="worker">Background Worker</option>
            <option value="cron">Cron Job</option>
            <option value="static">Static Site</option>
          </select>
        </div>
        {#if svcError}
          <div class="alert alert-error" style="margin-bottom:1rem">{svcError}</div>
        {/if}
        <div style="display:flex;justify-content:flex-end;gap:0.75rem;padding-top:1rem;border-top:1px solid var(--border-light)">
          <button type="button" class="btn btn-secondary" onclick={() => showDeployModal = false} disabled={svcSaving}>Cancel</button>
          <button type="submit" class="btn btn-primary" disabled={svcSaving || !svcName}>
            {#if svcSaving}<Loader2 size={16} style="margin-right:0.5rem" />Creating...{:else}<Rocket size={16} style="margin-right:0.5rem" />Deploy{/if}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}
