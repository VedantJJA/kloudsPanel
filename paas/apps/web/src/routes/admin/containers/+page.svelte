<script lang="ts">
  import { onMount } from 'svelte';
  import { 
    Box, 
    Server, 
    Database, 
    Trash2, 
    RefreshCw, 
    AlertTriangle, 
    Check, 
    ExternalLink, 
    Search, 
    Zap, 
    ShieldAlert, 
    Globe, 
    Layers, 
    Activity, 
    Loader2, 
    X,
    Filter,
    HardDrive,
    Terminal
  } from 'lucide-svelte';

  interface ContainerItem {
    id: string;
    names: string;
    image: string;
    status: string;
    state: string;
    created_at: string;
    ports: string;
    size: string;
    type: 'service' | 'database' | 'build' | 'system' | 'other';
    slug?: string;
    is_orphan: boolean;
    has_traefik_config: boolean;
    workspace_name?: string;
    project_name?: string;
    service_name?: string;
    service_id?: string;
    database_id?: string;
  }

  let containers = $state<ContainerItem[]>([]);
  let loading = $state(true);
  let searchQuery = $state('');
  let filterType = $state<'all' | 'orphan' | 'service' | 'database' | 'system'>('all');
  let refreshing = $state(false);

  // Deletion modal state
  let selectedContainer = $state<ContainerItem | null>(null);
  let showDeleteModal = $state(false);
  let deleting = $state(false);

  // Bulk prune state
  let pruningOrphans = $state(false);
  let pruneResult = $state<any>(null);

  async function loadContainers() {
    refreshing = true;
    try {
      const res = await fetch('/api/v1/admin/containers', { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        containers = data.containers ?? [];
      }
    } catch (e) {
      console.error('Failed to load containers:', e);
    } finally {
      loading = false;
      refreshing = false;
    }
  }

  onMount(() => {
    loadContainers();
  });

  const totalCount = $derived(containers.length);
  const runningCount = $derived(containers.filter(c => c.state === 'running').length);
  const orphanCount = $derived(containers.filter(c => c.is_orphan).length);
  const serviceCount = $derived(containers.filter(c => c.type === 'service').length);
  const databaseCount = $derived(containers.filter(c => c.type === 'database').length);
  const systemCount = $derived(containers.filter(c => c.type === 'system').length);
  const traefikCount = $derived(containers.filter(c => c.has_traefik_config).length);

  const filteredContainers = $derived.by(() => {
    return containers.filter(c => {
      // Type filter
      if (filterType === 'orphan' && !c.is_orphan) return false;
      if (filterType === 'service' && c.type !== 'service') return false;
      if (filterType === 'database' && c.type !== 'database') return false;
      if (filterType === 'system' && c.type !== 'system') return false;

      // Search query
      if (!searchQuery.trim()) return true;
      const q = searchQuery.toLowerCase().trim();
      return (
        c.names.toLowerCase().includes(q) ||
        c.id.toLowerCase().includes(q) ||
        c.image.toLowerCase().includes(q) ||
        (c.slug && c.slug.toLowerCase().includes(q)) ||
        (c.service_name && c.service_name.toLowerCase().includes(q)) ||
        (c.project_name && c.project_name.toLowerCase().includes(q)) ||
        (c.workspace_name && c.workspace_name.toLowerCase().includes(q)) ||
        c.ports.toLowerCase().includes(q)
      );
    });
  });

  function promptDelete(container: ContainerItem) {
    selectedContainer = container;
    showDeleteModal = true;
  }

  async function executeDelete() {
    if (!selectedContainer) return;
    deleting = true;
    try {
      const res = await fetch(`/api/v1/admin/containers/${encodeURIComponent(selectedContainer.names || selectedContainer.id)}`, {
        method: 'DELETE',
        credentials: 'include'
      });
      if (res.ok) {
        showDeleteModal = false;
        selectedContainer = null;
        await loadContainers();
      } else {
        const err = await res.json().catch(() => ({}));
        alert('Failed to delete container: ' + (err.error || err.message || res.statusText));
      }
    } catch (e: any) {
      alert('Network error: ' + e.message);
    } finally {
      deleting = false;
    }
  }

  async function handlePruneAllOrphans() {
    if (!confirm(`Purge all ${orphanCount} floating/orphan container(s)? This will terminate their containers, wipe their Traefik reverse proxy configs, and prune unindexed volumes immediately.`)) {
      return;
    }
    pruningOrphans = true;
    pruneResult = null;
    try {
      const res = await fetch('/api/v1/admin/containers/prune-orphans', {
        method: 'POST',
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        pruneResult = data;
        await loadContainers();
        setTimeout(() => { pruneResult = null; }, 8000);
      } else {
        const err = await res.json().catch(() => ({}));
        alert('Failed to prune orphan containers: ' + (err.error || 'Unknown error'));
      }
    } catch (e: any) {
      alert('Error pruning containers: ' + e.message);
    } finally {
      pruningOrphans = false;
    }
  }

  function getStatusClass(state: string) {
    switch (state?.toLowerCase()) {
      case 'running':
        return 'badge-running';
      case 'restarting':
        return 'badge-building';
      case 'exited':
      case 'dead':
        return 'badge-stopped';
      case 'created':
      case 'paused':
        return 'badge-pending';
      default:
        return 'badge-stopped';
    }
  }
</script>

<svelte:head>
  <title>Containers & Host - Administration - kloudsPanel</title>
</svelte:head>

<div class="page-header" style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 1rem;">
  <div>
    <div class="page-breadcrumbs">
      <a href="/admin/setup">Administration</a>
      <span>/</span>
      <span>Containers & Host</span>
    </div>
    <h1 class="page-title" style="margin: 0; font-size: 1.5rem; font-weight: 600;">Containers & Host Instances</h1>
    <p class="page-subtitle" style="margin-top: 4px;">Inspect running Docker instances, detect floating projects, and purge unmanaged containers with full networking cleanup.</p>
  </div>

  <div style="display: flex; gap: 0.6rem; align-items: center;">
    <button 
      class="btn btn-secondary" 
      onclick={loadContainers} 
      disabled={refreshing}
      title="Refresh container list"
    >
      <RefreshCw size={14} class={refreshing ? 'animate-spin' : ''} /> Refresh
    </button>
    {#if orphanCount > 0}
      <button 
        class="btn btn-danger" 
        onclick={handlePruneAllOrphans}
        disabled={pruningOrphans}
        style="display: flex; align-items: center; gap: 6px;"
      >
        {#if pruningOrphans}
          <Loader2 size={14} class="animate-spin" /> Purging Floating Containers...
        {:else}
          <Zap size={14} /> Purge {orphanCount} Floating Container{orphanCount === 1 ? '' : 's'}
        {/if}
      </button>
    {/if}
  </div>
</div>

<!-- Banner if floating containers exist -->
{#if pruneResult}
  <div style="background: rgba(34,197,94,0.1); border: 1px solid rgba(34,197,94,0.3); border-radius: var(--radius-md); padding: 0.875rem 1.25rem; margin-bottom: 1.25rem; display: flex; align-items: center; justify-content: space-between;">
    <div style="display: flex; align-items: center; gap: 8px; color: #16a34a; font-weight: 600; font-size: 0.875rem;">
      <Check size={18} />
      {pruneResult.message}
    </div>
    <button class="btn btn-secondary" style="padding: 2px 8px; font-size: 0.75rem; min-height: 24px;" onclick={() => pruneResult = null}>Dismiss</button>
  </div>
{:else if orphanCount > 0}
  <div style="background: rgba(245, 158, 11, 0.08); border: 1px solid rgba(245, 158, 11, 0.25); border-radius: var(--radius-md); padding: 0.875rem 1.25rem; margin-bottom: 1.25rem; display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 0.75rem;">
    <div style="display: flex; align-items: center; gap: 10px;">
      <div style="width: 32px; height: 32px; border-radius: var(--radius-sm); background: rgba(245, 158, 11, 0.15); color: #f59e0b; display: flex; align-items: center; justify-content: center; flex-shrink: 0;">
        <AlertTriangle size={17} />
      </div>
      <div>
        <div style="font-weight: 600; color: var(--color-ink); font-size: 0.875rem;">
          {orphanCount} Floating / Unindexed Container{orphanCount === 1 ? '' : 's'} Detected
        </div>
        <p class="text-xs text-muted" style="margin: 2px 0 0 0;">
          These containers are running on Docker but belong to deleted workspaces or previous manual setups. Their Traefik reverse proxy routes and storage can be cleared.
        </p>
      </div>
    </div>
    <button 
      class="btn btn-secondary" 
      style="border-color: rgba(245,158,11,0.4); color: #f59e0b; font-size: 0.75rem; padding: 4px 12px; min-height: 30px;"
      onclick={() => filterType = 'orphan'}
    >
      Filter Floating Only
    </button>
  </div>
{/if}

<!-- Stat Overview Cards -->
<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; margin-bottom: 1.5rem;">
  <div class="card" style="padding: 1rem 1.25rem;">
    <div class="text-xs text-muted" style="margin-bottom: 4px;">Total Host Containers</div>
    <div style="font-size: 1.5rem; font-weight: 600; color: var(--color-ink);">{totalCount}</div>
    <div class="text-xs text-muted" style="margin-top: 2px;">{runningCount} actively running</div>
  </div>

  <div class="card" style="padding: 1rem 1.25rem; border-color: {orphanCount > 0 ? 'rgba(245, 158, 11, 0.35)' : 'var(--color-border)'};">
    <div class="text-xs text-muted" style="margin-bottom: 4px;">Floating / Orphaned</div>
    <div style="font-size: 1.5rem; font-weight: 600; color: {orphanCount > 0 ? '#f59e0b' : 'var(--color-ink)'};">{orphanCount}</div>
    <div class="text-xs" style="margin-top: 2px; color: {orphanCount > 0 ? '#f59e0b' : 'var(--color-ink-muted)'};">
      {orphanCount > 0 ? 'Unlinked from dashboard' : 'All containers indexed'}
    </div>
  </div>

  <div class="card" style="padding: 1rem 1.25rem;">
    <div class="text-xs text-muted" style="margin-bottom: 4px;">Web & App Services</div>
    <div style="font-size: 1.5rem; font-weight: 600; color: var(--color-ink);">{serviceCount}</div>
    <div class="text-xs text-muted" style="margin-top: 2px;">Deployed web containers</div>
  </div>

  <div class="card" style="padding: 1rem 1.25rem;">
    <div class="text-xs text-muted" style="margin-bottom: 4px;">Traefik Routers Configured</div>
    <div style="font-size: 1.5rem; font-weight: 600; color: #38bdf8;">{traefikCount}</div>
    <div class="text-xs text-muted" style="margin-top: 2px;">Reverse proxy dynamic files</div>
  </div>
</div>

<!-- Toolbar: Filter Tabs & Search -->
<div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 1rem; margin-bottom: 1rem;">
  <div style="display: inline-flex; background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: 3px; gap: 2px;">
    <button 
      class="btn" 
      style="padding: 4px 12px; min-height: 28px; font-size: 0.75rem; border: none; background: {filterType === 'all' ? 'var(--color-surface-subtle)' : 'transparent'}; color: {filterType === 'all' ? 'var(--color-ink)' : 'var(--color-ink-muted)'}; font-weight: {filterType === 'all' ? '600' : '400'};"
      onclick={() => filterType = 'all'}
    >
      All ({totalCount})
    </button>
    <button 
      class="btn" 
      style="padding: 4px 12px; min-height: 28px; font-size: 0.75rem; border: none; background: {filterType === 'orphan' ? 'rgba(245,158,11,0.15)' : 'transparent'}; color: {filterType === 'orphan' ? '#f59e0b' : 'var(--color-ink-muted)'}; font-weight: {filterType === 'orphan' ? '600' : '400'};"
      onclick={() => filterType = 'orphan'}
    >
      Floating / Orphans ({orphanCount})
    </button>
    <button 
      class="btn" 
      style="padding: 4px 12px; min-height: 28px; font-size: 0.75rem; border: none; background: {filterType === 'service' ? 'var(--color-surface-subtle)' : 'transparent'}; color: {filterType === 'service' ? 'var(--color-ink)' : 'var(--color-ink-muted)'}; font-weight: {filterType === 'service' ? '600' : '400'};"
      onclick={() => filterType = 'service'}
    >
      Services ({serviceCount})
    </button>
    <button 
      class="btn" 
      style="padding: 4px 12px; min-height: 28px; font-size: 0.75rem; border: none; background: {filterType === 'database' ? 'var(--color-surface-subtle)' : 'transparent'}; color: {filterType === 'database' ? 'var(--color-ink)' : 'var(--color-ink-muted)'}; font-weight: {filterType === 'database' ? '600' : '400'};"
      onclick={() => filterType = 'database'}
    >
      Databases ({databaseCount})
    </button>
    <button 
      class="btn" 
      style="padding: 4px 12px; min-height: 28px; font-size: 0.75rem; border: none; background: {filterType === 'system' ? 'var(--color-surface-subtle)' : 'transparent'}; color: {filterType === 'system' ? 'var(--color-ink)' : 'var(--color-ink-muted)'}; font-weight: {filterType === 'system' ? '600' : '400'};"
      onclick={() => filterType = 'system'}
    >
      System ({systemCount})
    </button>
  </div>

  <div style="position: relative; min-width: 260px; max-width: 380px; flex: 1;">
    <Search size={14} style="position: absolute; left: 10px; top: 50%; transform: translateY(-50%); color: var(--color-ink-muted);" />
    <input 
      type="text" 
      class="form-input" 
      placeholder="Search containers, images, ports, or IDs..." 
      bind:value={searchQuery}
      style="padding-left: 32px; min-height: 34px; font-size: 0.8125rem;"
    />
  </div>
</div>

<!-- Container Table -->
{#if loading}
  <div class="empty-state" style="padding: 3rem 1rem;">
    <Loader2 size={32} class="animate-spin text-muted" style="margin-bottom: 0.75rem;" />
    <p class="text-xs text-muted">Scanning Docker daemon and indexing containers...</p>
  </div>
{:else if filteredContainers.length === 0}
  <div class="empty-state" style="padding: 3rem 1rem;">
    <div class="empty-state-icon"><Box size={36} /></div>
    <h3>No matching containers found</h3>
    <p class="text-xs text-muted">{searchQuery ? 'Try adjusting your search criteria.' : 'No Docker containers match the selected filter category.'}</p>
  </div>
{:else}
  <div class="table-wrapper" style="margin-bottom: 2rem;">
    <table>
      <thead>
        <tr>
          <th>Container Name & ID</th>
          <th>Status</th>
          <th>Type</th>
          <th>Dashboard Status / Scope</th>
          <th>Networking / Traefik</th>
          <th>Size / Image</th>
          <th style="text-align: right;">Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each filteredContainers as c (c.id)}
          <tr>
            <!-- Container Name & ID -->
            <td>
              <div style="display: flex; align-items: center; gap: 8px;">
                <div style="width: 26px; height: 26px; border-radius: var(--radius-xs); background: {c.type === 'service' ? 'rgba(56,189,248,0.12)' : c.type === 'database' ? 'rgba(16,185,129,0.12)' : 'rgba(255,255,255,0.06)'}; color: {c.type === 'service' ? '#38bdf8' : c.type === 'database' ? '#10b981' : 'var(--color-ink)'}; display: flex; align-items: center; justify-content: center; flex-shrink: 0;">
                  {#if c.type === 'database'}
                    <Database size={13} />
                  {:else if c.type === 'service'}
                    <Server size={13} />
                  {:else}
                    <Box size={13} />
                  {/if}
                </div>
                <div>
                  <div style="font-weight: 600; color: var(--color-ink); font-size: 0.8125rem;">
                    {c.names}
                  </div>
                  <div class="font-mono text-xs text-muted" style="font-size: 0.7rem;">
                    ID: {c.id.substring(0, 12)}
                  </div>
                </div>
              </div>
            </td>

            <!-- Runtime Status -->
            <td>
              <span class="badge {getStatusClass(c.state)}" style="text-transform: capitalize;">
                {c.state || c.status}
              </span>
            </td>

            <!-- Container Type -->
            <td>
              {#if c.type === 'service'}
                <span class="badge" style="background: rgba(56,189,248,0.12); color: #38bdf8; font-size: 0.72rem;">Web Service</span>
              {:else if c.type === 'database'}
                <span class="badge" style="background: rgba(16,185,129,0.12); color: #10b981; font-size: 0.72rem;">Database</span>
              {:else if c.type === 'build'}
                <span class="badge" style="background: rgba(245,158,11,0.12); color: #f59e0b; font-size: 0.72rem;">Build Task</span>
              {:else if c.type === 'system'}
                <span class="badge" style="background: rgba(168,85,247,0.12); color: #c084fc; font-size: 0.72rem;">System Core</span>
              {:else}
                <span class="badge" style="background: rgba(255,255,255,0.06); color: var(--color-ink-muted); font-size: 0.72rem;">Other</span>
              {/if}
            </td>

            <!-- Dashboard Indexing Scope -->
            <td>
              {#if c.is_orphan}
                <span class="badge" style="background: rgba(239, 68, 68, 0.12); color: var(--color-danger); border: 1px solid rgba(239, 68, 68, 0.3); font-size: 0.72rem; display: inline-flex; align-items: center; gap: 4px;">
                  <AlertTriangle size={11} /> Floating / Orphan
                </span>
              {:else}
                <div style="font-size: 0.75rem; color: var(--color-ink);">
                  {#if c.workspace_name}
                    <span style="color: var(--color-ink-muted);">{c.workspace_name}</span> / 
                  {/if}
                  {#if c.project_name}
                    <span style="font-weight: 500;">{c.project_name}</span> / 
                  {/if}
                  <span style="color: var(--color-accent); font-weight: 500;">{c.service_name || c.slug || 'Active'}</span>
                </div>
              {/if}
            </td>

            <!-- Networking & Traefik -->
            <td>
              <div style="display: flex; flex-direction: column; gap: 2px;">
                {#if c.ports}
                  <span class="font-mono text-xs" style="color: var(--color-ink); font-size: 0.72rem;">{c.ports}</span>
                {:else}
                  <span class="text-xs text-muted">-</span>
                {/if}
                {#if c.has_traefik_config}
                  <span class="badge" style="background: rgba(56,189,248,0.1); color: #38bdf8; font-size: 0.68rem; align-self: flex-start; padding: 1px 6px;">
                    <Globe size={10} style="display: inline; margin-right: 3px;" /> Traefik Routed
                  </span>
                {/if}
              </div>
            </td>

            <!-- Size / Image -->
            <td>
              <div style="display: flex; flex-direction: column; gap: 1px;">
                <span class="font-mono text-xs text-muted" style="font-size: 0.7rem; max-width: 180px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;" title={c.image}>
                  {c.image}
                </span>
                {#if c.size}
                  <span class="text-xs text-muted" style="font-size: 0.68rem;">{c.size}</span>
                {/if}
              </div>
            </td>

            <!-- Actions -->
            <td style="text-align: right;">
              <button 
                class="btn btn-secondary" 
                style="padding: 4px 10px; min-height: 28px; font-size: 0.75rem; color: var(--color-danger); border-color: rgba(239,68,68,0.25); display: inline-flex; align-items: center; gap: 5px;"
                onclick={() => promptDelete(c)}
                title="Force delete container and wipe networking"
              >
                <Trash2 size={13} /> Delete & Clear
              </button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<!-- Deletion Confirmation Modal -->
{#if showDeleteModal && selectedContainer}
  <div 
    class="modal-backdrop"
    style="position: fixed; inset: 0; background: rgba(0,0,0,0.7); backdrop-filter: blur(4px); z-index: 1100; display: flex; align-items: center; justify-content: center; padding: 1rem;"
    onclick={(e) => { if (e.target === e.currentTarget && !deleting) showDeleteModal = false; }}
    role="presentation"
  >
    <div 
      class="modal-card"
      style="width: 100%; max-width: 520px; background: var(--color-surface); border: 1px solid rgba(239, 68, 68, 0.4); border-radius: var(--radius-lg); padding: 1.5rem; box-shadow: 0 25px 35px -5px rgba(0,0,0,0.4);"
    >
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; border-bottom: 1px solid var(--color-border); padding-bottom: 0.75rem;">
        <div style="display: flex; align-items: center; gap: 8px;">
          <div style="width: 32px; height: 32px; border-radius: var(--radius-sm); background: rgba(239,68,68,0.12); color: var(--color-danger); display: flex; align-items: center; justify-content: center; flex-shrink: 0;">
            <Trash2 size={18} />
          </div>
          <h3 style="margin: 0; font-size: 1.1rem; color: var(--color-ink); font-weight: 600;">Delete Container Instance</h3>
        </div>
        <button 
          type="button" 
          class="btn btn-secondary" 
          style="padding: 4px; min-height: 28px; width: 28px; height: 28px; border: none;"
          onclick={() => showDeleteModal = false}
          disabled={deleting}
        >
          <X size={16} />
        </button>
      </div>

      <div style="margin-bottom: 1.25rem; font-size: 0.8125rem;">
        <p style="margin: 0 0 0.75rem 0; color: var(--color-ink); line-height: 1.45;">
          You are about to permanently terminate and remove container <span class="font-mono" style="color: var(--color-danger); font-weight: 600;">{selectedContainer.names}</span>.
        </p>

        <div style="background: var(--color-surface-subtle); border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: 0.875rem; margin-bottom: 1rem;">
          <div style="font-weight: 600; color: var(--color-ink); margin-bottom: 6px; font-size: 0.78125rem;">
            Automatic Cleanup Actions:
          </div>
          <ul style="margin: 0 0 0 1.2rem; padding: 0; line-height: 1.5; color: var(--color-ink-secondary); font-size: 0.75rem;">
            <li>Forcefully stop & remove container (<code class="font-mono">docker rm -f</code>)</li>
            {#if selectedContainer.has_traefik_config || selectedContainer.type === 'service'}
              <li>Wipe Traefik dynamic reverse proxy configuration (<code class="font-mono">svc-{selectedContainer.slug || '*'}.yaml</code>)</li>
            {/if}
            <li>Purge associated persistent volumes and temporary build artifacts</li>
            {#if !selectedContainer.is_orphan}
              <li>Clean associated database entity records to prevent ghost references</li>
            {/if}
          </ul>
        </div>

        {#if selectedContainer.is_orphan}
          <div style="background: rgba(245,158,11,0.08); border: 1px solid rgba(245,158,11,0.25); border-radius: var(--radius-md); padding: 0.75rem; color: #f59e0b; font-size: 0.75rem;">
            <strong>Floating Container:</strong> This instance is not linked to any active project or workspace and is safe to purge.
          </div>
        {/if}
      </div>

      <div style="display: flex; justify-content: flex-end; gap: 0.5rem; border-top: 1px solid var(--color-border); padding-top: 1rem;">
        <button 
          type="button" 
          class="btn btn-secondary" 
          onclick={() => showDeleteModal = false}
          disabled={deleting}
        >
          Cancel
        </button>
        <button 
          type="button" 
          class="btn btn-danger"
          onclick={executeDelete}
          disabled={deleting}
          style="display: flex; align-items: center; gap: 6px;"
        >
          {#if deleting}
            <Loader2 size={14} class="animate-spin" /> Purging Container & Networking...
          {:else}
            <Trash2 size={14} /> Force Delete & Clear Networking
          {/if}
        </button>
      </div>
    </div>
  </div>
{/if}
