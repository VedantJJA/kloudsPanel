<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';

  const { slug } = $derived($page.params);
  let project = $state<any>(null);
  let services = $state<any[]>([]);
  let databases = $state<any[]>([]);
  let loading = $state(true);
  let showDeployModal = $state(false);

  onMount(async () => {
    try {
      const [projRes, svcRes, dbRes] = await Promise.all([
        fetch(`/api/v1/projects/${slug}`, { credentials: 'include' }),
        fetch(`/api/v1/services?project_id=${slug}`, { credentials: 'include' }),
        fetch(`/api/v1/databases?project_id=${slug}`, { credentials: 'include' }),
      ]);
      project = await projRes.json();
      services = (await svcRes.json()).services ?? [];
      databases = (await dbRes.json()).databases ?? [];
    } finally {
      loading = false;
    }
  });

  const statusClass = (s: string) => `badge badge-${s}`;
</script>

<svelte:head>
  <title>{project?.name ?? 'Project'} — kloudsPanel</title>
</svelte:head>

{#if loading}
  <div class="empty-state"><div style="opacity:0.4;font-size:2rem">⏳</div><p>Loading…</p></div>
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
      🚀 Deploy Service
    </button>
  </div>

  <!-- Services -->
  <h2 style="font-size:1rem;font-weight:600;margin-bottom:1rem;color:var(--color-ink)">Services</h2>
  {#if services.length === 0}
    <div class="empty-state" style="padding:2rem">
      <div class="empty-state-icon">🔧</div>
      <h3>No services yet</h3>
      <p>Deploy your first service using the wizard.</p>
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
      <div class="empty-state-icon">🗄️</div>
      <h3>No databases yet</h3>
      <p>Add a PostgreSQL, MySQL, Redis, MongoDB, or ClickHouse database.</p>
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
