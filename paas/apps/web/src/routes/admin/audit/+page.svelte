<script lang="ts">
  import { onMount } from 'svelte';
  import { 
    ClipboardList, 
    Search, 
    RefreshCw, 
    Loader2, 
    Shield, 
    Rocket, 
    Database, 
    User, 
    Folder, 
    Key, 
    Eye, 
    X, 
    Clock, 
    CheckCircle2, 
    AlertTriangle,
    Layers
  } from 'lucide-svelte';

  let events = $state<any[]>([]);
  let loading = $state(true);
  let refreshing = $state(false);
  let searchQuery = $state('');
  let selectedCategory = $state<string>('all');
  let selectedEvent = $state<any>(null);

  async function loadAuditEvents() {
    try {
      const res = await fetch('/api/v1/admin/audit?limit=150', { credentials: 'include' });
      if (res.ok) {
        events = (await res.json()).events ?? [];
      }
    } finally {
      loading = false;
      refreshing = false;
    }
  }

  onMount(() => {
    loadAuditEvents();
  });

  async function refresh() {
    refreshing = true;
    await loadAuditEvents();
  }

  const categories = [
    { id: 'all', label: 'All Events' },
    { id: 'auth', label: 'Auth & Users' },
    { id: 'service', label: 'Services & Deploys' },
    { id: 'database', label: 'Databases' },
    { id: 'workspace', label: 'Workspaces' },
  ];

  let filteredEvents = $derived(
    events.filter(e => {
      // Category filter
      if (selectedCategory !== 'all') {
        const act = (e.action || '').toLowerCase();
        const tgt = (e.target_type || '').toLowerCase();
        if (selectedCategory === 'auth' && !act.includes('user') && !act.includes('auth') && !act.includes('login') && !act.includes('session') && !act.includes('approve') && !act.includes('reject')) return false;
        if (selectedCategory === 'service' && !act.includes('service') && !act.includes('deploy') && !tgt.includes('service') && !tgt.includes('deploy')) return false;
        if (selectedCategory === 'database' && !act.includes('db') && !act.includes('database') && !tgt.includes('database')) return false;
        if (selectedCategory === 'workspace' && !act.includes('workspace') && !tgt.includes('workspace')) return false;
      }

      // Search filter
      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase();
        const actionMatch = (e.action || '').toLowerCase().includes(q);
        const targetTypeMatch = (e.target_type || '').toLowerCase().includes(q);
        const targetIdMatch = (e.target_id || '').toLowerCase().includes(q);
        const actorMatch = (e.actor_kind || '').toLowerCase().includes(q) || (e.actor_user_id || '').toLowerCase().includes(q);
        const metaMatch = (e.metadata || '').toLowerCase().includes(q);
        return actionMatch || targetTypeMatch || targetIdMatch || actorMatch || metaMatch;
      }

      return true;
    })
  );

  function getActionBadge(action: string) {
    const act = (action || '').toLowerCase();
    if (act.includes('delete') || act.includes('destroy') || act.includes('reject') || act.includes('failed')) {
      return { bg: 'rgba(239, 68, 68, 0.1)', text: '#dc2626', border: 'rgba(239, 68, 68, 0.3)' };
    }
    if (act.includes('create') || act.includes('deploy') || act.includes('approve') || act.includes('login')) {
      return { bg: 'rgba(16, 185, 129, 0.1)', text: '#059669', border: 'rgba(16, 185, 129, 0.3)' };
    }
    if (act.includes('update') || act.includes('patch') || act.includes('set')) {
      return { bg: 'rgba(59, 130, 246, 0.1)', text: '#2563eb', border: 'rgba(59, 130, 246, 0.3)' };
    }
    return { bg: 'rgba(100, 116, 139, 0.1)', text: '#475569', border: 'rgba(100, 116, 139, 0.3)' };
  }

  function formatTime(iso: string) {
    if (!iso) return '-';
    const date = new Date(iso);
    return date.toLocaleString();
  }

  function formatRelative(iso: string) {
    if (!iso) return '';
    try {
      const now = new Date().getTime();
      const time = new Date(iso).getTime();
      const diffSec = Math.floor((now - time) / 1000);
      if (diffSec < 60) return `${diffSec}s ago`;
      const diffMin = Math.floor(diffSec / 60);
      if (diffMin < 60) return `${diffMin}m ago`;
      const diffHours = Math.floor(diffMin / 60);
      if (diffHours < 24) return `${diffHours}h ago`;
      const diffDays = Math.floor(diffHours / 24);
      return `${diffDays}d ago`;
    } catch {
      return '';
    }
  }

  function formatJson(str: string) {
    if (!str) return '{}';
    try {
      return JSON.stringify(JSON.parse(str), null, 2);
    } catch {
      return str;
    }
  }
</script>

<svelte:head>
  <title>Audit Log - kloudsPanel</title>
</svelte:head>

<div class="page-header" style="margin-bottom: 1.5rem;">
  <div>
    <h1 class="page-title">Platform Audit Log</h1>
    <p class="page-subtitle">Immutable append-only trail of all administrative and lifecycle security events</p>
  </div>
  <button 
    type="button" 
    class="btn btn-secondary" 
    style="display: flex; align-items: center; gap: 6px; font-size: 0.8125rem;"
    onclick={refresh}
    disabled={refreshing}
  >
    <RefreshCw size={14} class={refreshing ? 'animate-spin' : ''} />
    Refresh Trail
  </button>
</div>

<!-- Category Filters & Search -->
<div class="card" style="padding: 1rem 1.25rem; margin-bottom: 1.5rem; background: var(--color-surface); border: 1px solid var(--color-border);">
  <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 1rem;">
    <!-- Filter Pills -->
    <div style="display: flex; gap: 0.5rem; flex-wrap: wrap;">
      {#each categories as cat}
        <button
          type="button"
          class="btn"
          style="padding: 4px 12px; font-size: 0.8125rem; border-radius: 9999px; min-height: 32px; background: {selectedCategory === cat.id ? 'var(--color-accent)' : 'transparent'}; color: {selectedCategory === cat.id ? '#fff' : 'var(--color-ink)'}; border: 1px solid {selectedCategory === cat.id ? 'var(--color-accent)' : 'var(--color-border)'};"
          onclick={() => selectedCategory = cat.id}
        >
          {cat.label}
        </button>
      {/each}
    </div>

    <!-- Search Input -->
    <div style="position: relative; min-width: 260px;">
      <Search size={15} style="position: absolute; left: 10px; top: 10px; color: var(--color-ink-muted);" />
      <input
        type="text"
        class="form-input"
        style="padding-left: 2.25rem; font-size: 0.8125rem; height: 36px;"
        placeholder="Filter by action, target, actor..."
        bind:value={searchQuery}
      />
      {#if searchQuery}
        <button
          type="button"
          style="position: absolute; right: 8px; top: 8px; background: none; border: none; cursor: pointer; color: var(--color-ink-muted);"
          onclick={() => searchQuery = ''}
        >
          <X size={14} />
        </button>
      {/if}
    </div>
  </div>
</div>

<!-- Events Table -->
{#if loading}
  <div class="card" style="text-align: center; padding: 3rem;">
    <Loader2 size={36} class="animate-spin text-muted" style="margin-bottom: 0.75rem;" />
    <p class="text-sm text-muted">Loading audit trail...</p>
  </div>
{:else if filteredEvents.length === 0}
  <div class="card" style="text-align: center; padding: 3rem; border: 1px dashed var(--color-border);">
    <ClipboardList size={40} style="color: var(--color-ink-muted); margin-bottom: 0.75rem;" />
    <h3 style="margin: 0 0 0.5rem 0; font-size: 1.1rem;">No matching audit events</h3>
    <p class="text-sm text-muted" style="margin: 0;">Try adjusting your search filters or check back after administrative actions.</p>
  </div>
{:else}
  <div class="table-wrapper card" style="padding: 0; overflow: hidden; border: 1px solid var(--color-border);">
    <table>
      <thead>
        <tr>
          <th style="width: 170px;">Timestamp</th>
          <th style="width: 130px;">Actor</th>
          <th>Action</th>
          <th>Target</th>
          <th>Workspace</th>
          <th style="text-align: right; width: 90px;">Details</th>
        </tr>
      </thead>
      <tbody>
        {#each filteredEvents as e}
          {@const badge = getActionBadge(e.action)}
          <tr style="cursor: pointer;" onclick={() => selectedEvent = e}>
            <td>
              <div style="font-size: 0.8125rem; font-weight: 500; color: var(--color-ink);">
                {formatRelative(e.occurred_at || e.OccurredAt)}
              </div>
              <div class="font-mono text-xs text-muted">
                {formatTime(e.occurred_at || e.OccurredAt)}
              </div>
            </td>
            <td>
              <span class="badge" style="background: rgba(0,0,0,0.05); color: var(--color-ink); font-size: 0.75rem; text-transform: capitalize;">
                {e.actor_kind || e.ActorKind || 'User'}
              </span>
            </td>
            <td>
              <span 
                class="badge" 
                style="background: {badge.bg}; color: {badge.text}; border: 1px solid {badge.border}; font-size: 0.75rem; font-family: var(--font-mono); font-weight: 600;"
              >
                {e.action || e.Action}
              </span>
            </td>
            <td>
              <div style="display: flex; align-items: center; gap: 0.4rem; font-size: 0.8125rem;">
                <span style="font-weight: 600; text-transform: capitalize; color: var(--color-ink);">
                  {e.target_type || e.TargetType || 'resource'}
                </span>
                <span class="font-mono text-xs text-muted">
                  {(e.target_id || e.TargetID || '').slice(0, 10)}...
                </span>
              </div>
            </td>
            <td>
              <span class="font-mono text-xs text-muted">
                {e.workspace_id || e.WorkspaceID ? (e.workspace_id || e.WorkspaceID).slice(0, 10) + '...' : 'Global'}
              </span>
            </td>
            <td style="text-align: right;">
              <button 
                type="button" 
                class="btn btn-secondary" 
                style="padding: 3px 8px; font-size: 0.75rem;"
                onclick={(evt) => { evt.stopPropagation(); selectedEvent = e; }}
              >
                <Eye size={13} /> View
              </button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<!-- Event Inspector Modal -->
{#if selectedEvent}
  <div 
    role="presentation"
    style="position: fixed; inset: 0; background: rgba(11,31,58,0.65); z-index: 1000; display: flex; align-items: center; justify-content: center; padding: 1rem;"
    onclick={() => selectedEvent = null}
    onkeydown={(e) => e.key === 'Escape' && (selectedEvent = null)}
  >
    <div 
      role="dialog"
      aria-label="Audit Event Details"
      tabindex="-1"
      style="background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius-lg); width: 100%; max-width: 680px; max-height: 90vh; display: flex; flex-direction: column; overflow: hidden; box-shadow: 0 20px 25px -5px rgba(0,0,0,0.3);"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
    >
      <!-- Modal Header -->
      <div style="display: flex; justify-content: space-between; align-items: center; padding: 1.25rem 1.5rem; border-bottom: 1px solid var(--color-border);">
        <div style="display: flex; align-items: center; gap: 0.6rem;">
          <Shield size={20} style="color: var(--color-accent);" />
          <h3 style="margin: 0; font-size: 1.1rem;">Audit Event Details</h3>
        </div>
        <button 
          type="button" 
          style="background: none; border: none; cursor: pointer; color: var(--color-ink-muted);"
          onclick={() => selectedEvent = null}
        >
          <X size={18} />
        </button>
      </div>

      <!-- Modal Body -->
      <div style="padding: 1.25rem 1.5rem; overflow-y: auto; display: flex; flex-direction: column; gap: 1rem;">
        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(140px, 1fr)); gap: 1rem;">
          <div>
            <div class="text-xs text-muted" style="margin-bottom: 2px;">Event Action</div>
            <span class="badge" style="font-family: var(--font-mono); font-size: 0.8125rem;">
              {selectedEvent.action || selectedEvent.Action}
            </span>
          </div>

          <div>
            <div class="text-xs text-muted" style="margin-bottom: 2px;">Timestamp</div>
            <div class="font-mono text-xs" style="font-weight: 600;">
              {formatTime(selectedEvent.occurred_at || selectedEvent.OccurredAt)}
            </div>
          </div>

          <div>
            <div class="text-xs text-muted" style="margin-bottom: 2px;">Actor ID / Kind</div>
            <div class="font-mono text-xs">
              {selectedEvent.actor_user_id || selectedEvent.ActorUserID || 'System'} ({selectedEvent.actor_kind || selectedEvent.ActorKind})
            </div>
          </div>

          <div>
            <div class="text-xs text-muted" style="margin-bottom: 2px;">Target Entity</div>
            <div class="font-mono text-xs">
              {selectedEvent.target_type || selectedEvent.TargetType}: {selectedEvent.target_id || selectedEvent.TargetID}
            </div>
          </div>
        </div>

        {#if selectedEvent.request_id || selectedEvent.RequestID}
          <div>
            <div class="text-xs text-muted" style="margin-bottom: 2px;">Request Trace ID</div>
            <div class="font-mono text-xs" style="background: rgba(0,0,0,0.03); padding: 4px 8px; border-radius: 4px;">
              {selectedEvent.request_id || selectedEvent.RequestID}
            </div>
          </div>
        {/if}

        <!-- JSON Payload Metadata -->
        <div>
          <div class="text-xs text-muted" style="margin-bottom: 6px;">Event Metadata & Payload</div>
          <pre style="background: #0f172a; color: #f8fafc; padding: 1rem; border-radius: var(--radius-md); font-size: 0.75rem; overflow-x: auto; max-height: 240px; margin: 0;">{formatJson(selectedEvent.metadata || selectedEvent.Metadata)}</pre>
        </div>
      </div>

      <!-- Modal Footer -->
      <div style="padding: 1rem 1.5rem; border-top: 1px solid var(--color-border); display: flex; justify-content: flex-end;">
        <button type="button" class="btn btn-secondary" onclick={() => selectedEvent = null}>
          Close
        </button>
      </div>
    </div>
  </div>
{/if}
