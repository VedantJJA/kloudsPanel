<script lang="ts">
  import { onMount } from 'svelte';

  let users = $state<any[]>([]);
  let pending = $state<any[]>([]);
  let loading = $state(true);

  async function load() {
    const [allRes, pendRes] = await Promise.all([
      fetch('/api/v1/admin/users', { credentials: 'include' }),
      fetch('/api/v1/admin/users?status=pending', { credentials: 'include' }),
    ]);
    users = (await allRes.json()).users ?? [];
    pending = (await pendRes.json()).users ?? [];
    loading = false;
  }

  async function approve(id: string) {
    await fetch(`/api/v1/admin/users/${id}/approve`, { method: 'POST', credentials: 'include' });
    await load();
  }

  async function suspend(id: string) {
    await fetch(`/api/v1/admin/users/${id}/suspend`, { method: 'POST', credentials: 'include' });
    await load();
  }

  onMount(load);
</script>

<svelte:head>
  <title>User Management — kloudsPanel</title>
</svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">User Management</h1>
    <p class="page-subtitle">Review pending accounts and manage user roles</p>
  </div>
</div>

{#if loading}
  <div class="empty-state"><div style="opacity:0.4;font-size:2rem">⏳</div><p>Loading…</p></div>
{:else}
  <!-- Pending approvals -->
  {#if pending.length > 0}
    <div style="
      background:#fef3c7;border:1px solid #fbbf24;border-radius:var(--radius-md);
      padding:1rem 1.25rem;margin-bottom:1.5rem;display:flex;align-items:center;justify-content:space-between;
    ">
      <div style="font-size:0.9375rem;font-weight:500">
        ⏳ {pending.length} account{pending.length > 1 ? 's' : ''} awaiting approval
      </div>
    </div>

    <h2 style="font-size:1rem;font-weight:600;margin-bottom:0.75rem">Pending Approvals</h2>
    <div class="table-wrapper" style="margin-bottom:2rem">
      <table>
        <thead><tr><th>Name</th><th>Email</th><th>Requested</th><th>Actions</th></tr></thead>
        <tbody>
          {#each pending as u}
            <tr>
              <td style="font-weight:500">{u.display_name}</td>
              <td class="font-mono text-sm">{u.email}</td>
              <td class="text-xs text-muted">{u.created_at?.slice(0,10) ?? '—'}</td>
              <td>
                <div style="display:flex;gap:0.5rem">
                  <button class="btn btn-primary" style="padding:4px 12px;min-height:32px;font-size:0.8125rem" onclick={() => approve(u.id)}>
                    ✓ Approve
                  </button>
                  <button class="btn btn-danger" style="padding:4px 12px;min-height:32px;font-size:0.8125rem" onclick={() => suspend(u.id)}>
                    ✗ Reject
                  </button>
                </div>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  <!-- All users -->
  <h2 style="font-size:1rem;font-weight:600;margin-bottom:0.75rem">All Users ({users.length})</h2>
  <div class="table-wrapper">
    <table>
      <thead><tr><th>Name</th><th>Email</th><th>Role</th><th>Status</th><th>Last Login</th></tr></thead>
      <tbody>
        {#each users as u}
          <tr>
            <td style="font-weight:500">{u.display_name}</td>
            <td class="font-mono text-sm">{u.email}</td>
            <td>
              <span class="badge" style="background:#f1f5f9;color:#334155">{u.platform_role}</span>
            </td>
            <td>
              <span class="badge badge-{u.status === 'active' ? 'running' : u.status === 'pending' ? 'pending' : 'stopped'}">
                {u.status}
              </span>
            </td>
            <td class="text-xs text-muted">{u.last_login_at?.slice(0,10) ?? 'Never'}</td>
          </tr>
        {/each}
        {#if users.length === 0}
          <tr><td colspan="5" style="text-align:center;padding:2rem;color:var(--color-ink-secondary)">No users found</td></tr>
        {/if}
      </tbody>
    </table>
  </div>
{/if}
