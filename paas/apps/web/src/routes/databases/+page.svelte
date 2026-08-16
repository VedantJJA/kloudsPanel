<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Database, Plus, Loader2, Trash2, ArrowRight } from 'lucide-svelte';
  import FrameworkIcon from '$lib/components/icons/FrameworkIcon.svelte';

  let databases = $state<any[]>([]);
  let loading = $state(true);

  async function loadDatabases() {
    try {
      const res = await fetch('/api/v1/databases', { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        databases = data.databases ?? [];
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadDatabases();
  });

  async function deleteDatabase(e: Event, db: any) {
    e.preventDefault();
    const id = db.id || db.ID;
    if (!confirm(`Are you sure you want to delete database "${db.name || db.Name || id}"? All data in this instance will be permanently deleted.`)) return;
    try {
      const res = await fetch(`/api/v1/databases/${encodeURIComponent(id)}`, { method: 'DELETE', credentials: 'include' });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        alert('Failed to delete database: ' + (d.detail || d.message || res.statusText));
      }
      await loadDatabases();
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
  <title>Databases - kloudsPanel</title>
</svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Databases</h1>
    <p class="page-subtitle">Managed PostgreSQL, MySQL, Redis, MongoDB, and ClickHouse instances</p>
  </div>
  <button class="btn btn-primary" onclick={() => goto('/databases/new')}>
    <Plus size={16} /> New Database
  </button>
</div>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={36} /></div>
    <p>Loading databases...</p>
  </div>
{:else if databases.length === 0}
  <div class="empty-state">
    <div class="empty-state-icon"><Database size={36} /></div>
    <h3>No databases provisioned yet</h3>
    <p>Create a high-performance managed database instance for your services.</p>
    <button class="btn btn-primary" style="margin-top:1rem;" onclick={() => goto('/databases/new')}>
      <Database size={15} /> Provision Database
    </button>
  </div>
{:else}
  <div class="table-wrapper">
    <table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Engine</th>
          <th>Internal Hostname</th>
          <th>Port</th>
          <th>Status</th>
          <th style="text-align:right;">Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each databases as db}
          {@const dbSlug = db.name ? db.name.toLowerCase().replace(/[^a-z0-9]+/g, '-') : (db.id || db.ID)}
          <tr style="cursor:pointer;" onclick={() => goto(`/databases/${dbSlug}/overview`)}>
            <td>
              <div style="display:flex; align-items:center; gap:8px;">
                <FrameworkIcon name={db.engine || db.Engine} size={18} />
                <a href="/databases/{dbSlug}/overview" style="color:var(--color-ink); text-decoration:none; font-weight:600; font-size:0.875rem;">
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
                <a href="/databases/{dbSlug}/overview" class="btn btn-secondary" style="padding:3px 10px; min-height:28px; font-size:0.75rem;">
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
