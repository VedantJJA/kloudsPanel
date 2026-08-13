<script lang="ts">
  import { onMount } from 'svelte';

  let events = $state<any[]>([]);
  let loading = $state(true);

  onMount(async () => {
    try {
      const res = await fetch('/api/v1/admin/audit?limit=100', { credentials: 'include' });
      events = (await res.json()).events ?? [];
    } finally {
      loading = false;
    }
  });

  const actionColor = (action: string) => {
    if (action.includes('delete') || action.includes('suspend')) return 'var(--color-danger)';
    if (action.includes('create') || action.includes('deploy') || action.includes('approve')) return 'var(--color-accent)';
    return 'var(--color-ink)';
  };
</script>

<svelte:head>
  <title>Audit Log — kloudsPanel</title>
</svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Audit Log</h1>
    <p class="page-subtitle">Append-only record of all platform actions</p>
  </div>
</div>

{#if loading}
  <div class="empty-state"><div style="opacity:0.4;font-size:2rem">⏳</div><p>Loading audit events…</p></div>
{:else}
  <div class="table-wrapper">
    <table>
      <thead>
        <tr>
          <th>Time</th>
          <th>Actor</th>
          <th>Action</th>
          <th>Target</th>
          <th>Workspace</th>
        </tr>
      </thead>
      <tbody>
        {#each events as e}
          <tr>
            <td class="font-mono text-xs">{e.occurred_at?.slice(0,19).replace('T',' ')}</td>
            <td class="text-sm">
              <span class="badge" style="background:#f1f5f9;color:#334155">{e.actor_kind}</span>
            </td>
            <td>
              <span style="font-size:0.875rem;font-weight:500;color:{actionColor(e.action)}">{e.action}</span>
            </td>
            <td class="text-sm">
              <span class="text-muted">{e.target_type}</span>
              <span class="font-mono text-xs" style="margin-left:0.25rem">{e.target_id?.slice(0,8)}…</span>
            </td>
            <td class="text-xs text-muted">{e.workspace_id?.slice(0,8) ?? '—'}</td>
          </tr>
        {/each}
        {#if events.length === 0}
          <tr>
            <td colspan="5" style="text-align:center;padding:2rem;color:var(--color-ink-secondary)">
              No audit events recorded yet
            </td>
          </tr>
        {/if}
      </tbody>
    </table>
  </div>
{/if}
