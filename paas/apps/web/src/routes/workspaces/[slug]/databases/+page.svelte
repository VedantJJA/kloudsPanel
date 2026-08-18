<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Database, Plus, Loader2, Trash2, ArrowRight, Server, ShieldCheck } from 'lucide-svelte';
  import FrameworkIcon from '$lib/components/icons/FrameworkIcon.svelte';

  const slug = $derived($page.params.slug || '');
  let workspace = $state<any>(null);
  let databases = $state<any[]>([]);
  let loading = $state(true);

  async function loadData() {
    if (!slug) return;
    try {
      const [wsRes, dbRes] = await Promise.all([
        fetch(`/api/v1/workspaces/${encodeURIComponent(slug)}`, { credentials: 'include' }),
        fetch(`/api/v1/databases?workspaceSlug=${encodeURIComponent(slug)}`, { credentials: 'include' })
      ]);

      if (wsRes.ok) {
        workspace = await wsRes.json();
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
  <title>Databases - {workspace?.name || slug} - kloudsPanel</title>
</svelte:head>

<div class="page-header">
  <div>
    <div class="page-breadcrumbs">
      <a href="/workspaces/{slug}">{workspace?.name || slug}</a>
      <span>/</span>
      <span>Databases</span>
    </div>
    <h1 class="page-title">Workspace Databases</h1>
    <p class="page-subtitle">Managed PostgreSQL, MySQL, Redis, MongoDB, and ClickHouse instances for this workspace</p>
  </div>
  {#if databases.length > 0}
    <button class="btn btn-primary" onclick={() => goto(`/databases/new?workspaceSlug=${slug}`)}>
      <Plus size={16} /> New Database
    </button>
  {/if}
</div>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={36} /></div>
    <p>Loading workspace databases...</p>
  </div>
{:else if databases.length === 0}
  <div class="empty-state">
    <div class="empty-state-icon"><Database size={40} /></div>
    <h3>No databases provisioned in this workspace</h3>
    <p>Create a dedicated PostgreSQL, MySQL, Redis, MongoDB, or ClickHouse database for your services.</p>
    <button class="btn btn-primary" style="margin-top:1rem;" onclick={() => goto(`/databases/new?workspaceSlug=${slug}`)}>
      <Database size={15} /> Provision Database
    </button>
  </div>
{:else}
  <div class="table-wrapper">
    <table>
      <thead>
        <tr>
          <th>Database</th>
          <th>Engine</th>
          <th>Status</th>
          <th>Internal Hostname</th>
          <th>Internal Port</th>
          <th>Created</th>
          <th style="text-align:right">Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each databases as db}
          {@const id = db.id || db.ID}
          {@const name = db.name || db.Name}
          {@const engine = db.engine || db.Engine || 'postgres'}
          {@const status = db.runtime_status || db.runtimeStatus || db.RuntimeStatus || 'provisioning'}
          {@const host = db.internal_hostname || db.internalHostname || `${db.name}.paas`}
          {@const port = db.internal_port || db.internalPort || 5432}
          {@const created = db.created_at || db.createdAt}
          <tr style="cursor:pointer" onclick={() => goto(`/databases/${id}/overview`)}>
            <td>
              <div style="display:flex; align-items:center; gap:0.6rem;">
                <div style="width:32px; height:32px; border-radius:var(--radius-sm); background:rgba(0,166,166,0.1); display:flex; align-items:center; justify-content:center; color:var(--color-accent); flex-shrink:0;">
                  <Database size={16} />
                </div>
                <div>
                  <div style="font-weight:600; color:var(--color-ink);">{name}</div>
                  <div class="text-xs text-muted font-mono">{id.substring(0, 8)}</div>
                </div>
              </div>
            </td>
            <td>
              <span class="badge" style="background:rgba(255,255,255,0.06); text-transform:capitalize; font-weight:600;">
                {engine} {db.engine_version || db.engineVersion || ''}
              </span>
            </td>
            <td>
              <span class={statusClass(status)} style="text-transform:capitalize;">
                {status}
              </span>
            </td>
            <td class="font-mono text-xs" style="color:var(--color-ink-muted);">
              {host}
            </td>
            <td class="font-mono text-xs">
              :{port}
            </td>
            <td class="text-xs text-muted">
              {created ? new Date(created).toLocaleDateString() : 'Just now'}
            </td>
            <td style="text-align:right" onclick={(e) => e.stopPropagation()}>
              <div style="display:flex; justify-content:flex-end; gap:0.4rem;">
                <button
                  class="btn btn-secondary"
                  style="padding:4px 8px; font-size:0.75rem;"
                  onclick={() => goto(`/databases/${id}/overview`)}
                  title="View Connection & Credentials"
                >
                  Details <ArrowRight size={12} />
                </button>
                <button
                  class="btn btn-secondary"
                  style="padding:4px 6px; color:#ef4444;"
                  onclick={(e) => deleteDatabase(e, db)}
                  title="Delete database"
                >
                  <Trash2 size={13} />
                </button>
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}
