<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Database, Plus, Loader2, Trash2 } from 'lucide-svelte';

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

  const statusClass = (s: string) => `badge badge-${s || 'ready'}`;
</script>

<svelte:head>
  <title>Databases - kloudsPanel</title>
</svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Databases</h1>
    <p class="page-subtitle">Managed PostgreSQL, MySQL, Redis, MongoDB, and ClickHouse databases</p>
  </div>
  <button class="btn btn-primary" onclick={() => goto('/databases/new')}>
    <Plus size={16} /> New Database
  </button>
</div>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={48} /></div>
    <p>Loading databases…</p>
  </div>
{:else if databases.length === 0}
  <div class="empty-state" style="background:var(--color-surface); border:1px solid var(--color-border); border-radius:var(--radius-lg); padding:3rem;">
    <div class="empty-state-icon"><Database size={48} /></div>
    <h3>No databases provisioned yet</h3>
    <p>Create a high-performance managed database instance for your services.</p>
    <button class="btn btn-primary mt-4" onclick={() => goto('/databases/new')}>
      <Database size={16} /> Provision Database
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
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each databases as db}
          <tr style="cursor:pointer;" onclick={() => goto(`/databases/${db.id || db.ID}/overview`)}>
            <td style="font-weight:600;">
              <a href="/databases/{db.id || db.ID}/overview" style="color:var(--color-ink); text-decoration:none; font-weight:700;">
                {db.name || db.Name}
              </a>
            </td>
            <td><span class="badge" style="background:#e0f2fe; color:#0369a1; text-transform:uppercase; font-weight:700;">{db.engine || db.Engine}</span></td>
            <td><span class="font-mono text-xs">{db.internal_hostname || db.InternalHostname || '-'}</span></td>
            <td><span class="font-mono text-xs">:{db.internal_port || db.InternalPort || '-'}</span></td>
            <td><span class={statusClass(db.runtime_status || db.RuntimeStatus)}>{db.runtime_status || db.RuntimeStatus || 'ready'}</span></td>
            <td style="text-align:right;" onclick={(e) => e.stopPropagation()}>
              <button 
                class="btn btn-secondary" 
                style="padding:4px 8px; min-height:32px; color:var(--color-danger); border-color:transparent;" 
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
