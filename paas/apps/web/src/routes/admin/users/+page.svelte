<script lang="ts">
  import { onMount } from 'svelte';
  import { Loader2, Clock, Check, X, Shield, UserCheck, ShieldAlert, UserPlus, RefreshCw, ToggleLeft, ToggleRight } from 'lucide-svelte';

  let users = $state<any[]>([]);
  let pending = $state<any[]>([]);
  let loading = $state(true);
  let autoApprove = $state(true);
  let settingSaving = $state(false);

  async function load() {
    try {
      const [allRes, pendRes, setRes] = await Promise.all([
        fetch('/api/v1/admin/users', { credentials: 'include' }),
        fetch('/api/v1/admin/users?status=pending', { credentials: 'include' }),
        fetch('/api/v1/admin/settings', { credentials: 'include' })
      ]);
      if (allRes.ok) users = (await allRes.json()).users ?? [];
      if (pendRes.ok) pending = (await pendRes.json()).users ?? [];
      if (setRes.ok) {
        const s = await setRes.json();
        autoApprove = s.settings?.auto_approve_users ?? true;
      }
    } catch {}
    loading = false;
  }

  async function toggleAutoApprove() {
    settingSaving = true;
    const newVal = !autoApprove;
    try {
      const res = await fetch('/api/v1/admin/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ auto_approve_users: newVal })
      });
      if (res.ok) {
        autoApprove = newVal;
      }
    } catch {}
    settingSaving = false;
  }

  async function approve(id: string) {
    // Optimistic UI update
    pending = pending.filter(u => u.id !== id);
    users = users.map(u => u.id === id ? { ...u, status: 'active' } : u);

    try {
      await fetch(`/api/v1/admin/users/${id}/approve`, { method: 'POST', credentials: 'include' });
    } catch {}
    await load();
  }

  async function suspend(id: string) {
    // Optimistic UI update
    pending = pending.filter(u => u.id !== id);
    users = users.map(u => u.id === id ? { ...u, status: 'suspended' } : u);

    try {
      await fetch(`/api/v1/admin/users/${id}/suspend`, { method: 'POST', credentials: 'include' });
    } catch {}
    await load();
  }

  onMount(load);
</script>

<svelte:head>
  <title>User Management - kloudsPanel</title>
</svelte:head>

<div class="page-header" style="margin-bottom: 1.5rem;">
  <div>
    <h1 class="page-title">User Management</h1>
    <p class="page-subtitle">Review pending accounts, configure auto-approval policy, and manage user roles</p>
  </div>
  <button class="btn btn-secondary" onclick={load} style="font-size:0.8125rem; padding:6px 12px;">
    <RefreshCw size={14} /> Refresh
  </button>
</div>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={48} /></div>
    <p>Loading users...</p>
  </div>
{:else}
  <!-- Policy & Settings Card -->
  <div class="card" style="margin-bottom: 1.5rem; padding: 1.25rem; background: var(--color-surface); border: 1px solid var(--color-border);">
    <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:1rem;">
      <div style="display:flex; align-items:center; gap:0.75rem;">
        <div style="width:40px; height:40px; border-radius:var(--radius-md); background:rgba(0,166,166,0.1); color:var(--color-accent); display:flex; align-items:center; justify-content:center;">
          <UserPlus size={20} />
        </div>
        <div>
          <div style="font-weight:700; font-size:0.9375rem; color:var(--color-ink);">Registration Access Policy</div>
          <div class="text-xs text-muted" style="margin-top:2px;">
            {autoApprove ? 'Auto-Approve Enabled: New signups are activated immediately without manual approval.' : 'Manual Approval Required: An administrator must approve each new account.'}
          </div>
        </div>
      </div>

      <button 
        type="button" 
        class="btn {autoApprove ? 'btn-primary' : 'btn-secondary'}" 
        style="padding: 6px 14px; font-size: 0.8125rem; display:flex; align-items:center; gap:0.5rem;"
        onclick={toggleAutoApprove}
        disabled={settingSaving}
      >
        {#if settingSaving}
          <Loader2 size={14} class="animate-spin" /> Saving...
        {:else if autoApprove}
          <UserCheck size={16} /> Auto-Approve: ON
        {:else}
          <Shield size={16} /> Auto-Approve: OFF (Manual)
        {/if}
      </button>
    </div>
  </div>

  <!-- Pending approvals -->
  {#if pending.length > 0}
    <div class="alert alert-warning" style="margin-bottom:1.5rem;display:flex;align-items:center;gap:0.75rem;">
      <Clock size={20} />
      <div>
        <strong>Pending Approvals</strong>
        <p style="margin:0;font-size:0.875rem">{pending.length} account{pending.length > 1 ? 's' : ''} awaiting approval</p>
      </div>
    </div>

    <h2 style="font-size:1rem;font-weight:600;margin-bottom:0.75rem">Pending Approvals ({pending.length})</h2>
    <div class="table-wrapper" style="margin-bottom:2rem">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Email</th>
            <th>Requested</th>
            <th style="text-align:right;">Actions</th>
          </tr>
        </thead>
        <tbody>
          {#each pending as u}
            <tr>
              <td style="font-weight:600">{u.displayName || u.display_name || 'User'}</td>
              <td class="font-mono text-sm">{u.email}</td>
              <td class="text-xs text-muted">{u.createdAt?.slice(0,10) || u.created_at?.slice(0,10) || 'Recently'}</td>
              <td style="text-align:right">
                <button class="btn btn-primary" style="padding:0.25rem 0.6rem;font-size:0.75rem" onclick={() => approve(u.id)}>
                  <Check size={14} /> Approve
                </button>
                <button class="btn btn-secondary" style="padding:0.25rem 0.6rem;font-size:0.75rem;margin-left:0.5rem;color:var(--color-error)" onclick={() => suspend(u.id)}>
                  <X size={14} /> Reject
                </button>
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
      <thead>
        <tr>
          <th>Name</th>
          <th>Email</th>
          <th>Role</th>
          <th>Status</th>
          <th>Last Login</th>
        </tr>
      </thead>
      <tbody>
        {#each users as u}
          <tr>
            <td style="font-weight:600">{u.displayName || u.display_name || 'User'}</td>
            <td class="font-mono text-sm">{u.email}</td>
            <td>
              <span class="badge" style="background:#f1f5f9;color:#334155">{u.platformRole || u.platform_role || 'user'}</span>
            </td>
            <td>
              <span class="badge badge-{u.status === 'active' ? 'running' : u.status === 'pending' ? 'pending' : 'stopped'}">
                {u.status}
              </span>
            </td>
            <td class="text-xs text-muted">{u.lastLoginAt?.slice(0,10) || u.last_login_at?.slice(0,10) || 'Never'}</td>
          </tr>
        {/each}
        {#if users.length === 0}
          <tr><td colspan="5" style="text-align:center;padding:2rem;color:var(--color-ink-secondary)">No users found</td></tr>
        {/if}
      </tbody>
    </table>
  </div>
{/if}
