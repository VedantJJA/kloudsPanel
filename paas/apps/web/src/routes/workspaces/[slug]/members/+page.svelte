<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { Users, UserPlus, Shield, Loader2, Mail, Check } from 'lucide-svelte';

  const { slug } = $derived($page.params);
  let workspace = $state<any>(null);
  let members = $state<any[]>([]);
  let inviteEmail = $state('');
  let inviteRole = $state('developer');
  let inviting = $state(false);
  let inviteSaved = $state(false);
  let inviteError = $state('');
  let loading = $state(true);

  async function loadData() {
    try {
      const [wsRes, memRes] = await Promise.all([
        fetch(`/api/v1/workspaces/${slug}`, { credentials: 'include' }),
        fetch(`/api/v1/workspaces/${slug}/members`, { credentials: 'include' })
      ]);
      if (wsRes.ok) workspace = await wsRes.json();
      if (memRes.ok) {
        const d = await memRes.json();
        members = d.members ?? [];
      }
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadData();
  });

  async function handleInvite(e: Event) {
    e.preventDefault();
    inviting = true;
    inviteError = '';
    inviteSaved = false;
    try {
      const res = await fetch(`/api/v1/workspaces/${slug}/members`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ email: inviteEmail, role: inviteRole })
      });
      if (res.ok) {
        inviteSaved = true;
        inviteEmail = '';
        await loadData();
        setTimeout(() => inviteSaved = false, 3000);
      } else {
        const d = await res.json().catch(() => ({}));
        inviteError = d.error || 'Failed to add member to workspace';
      }
    } catch (e: any) {
      inviteError = 'Network error: ' + e.message;
    } finally {
      inviting = false;
    }
  }
</script>

<svelte:head>
  <title>Members - {workspace?.name || slug} - kloudsPanel</title>
</svelte:head>

<div class="page-header" style="margin-bottom: 1.5rem;">
  <div>
    <div class="page-breadcrumbs">
      <a href="/workspaces">Workspaces</a> /
      <a href="/workspaces/{slug}">{workspace?.name || slug}</a> /
      <span>Members</span>
    </div>
    <h1 class="page-title">Workspace Members</h1>
    <p class="page-subtitle">Manage access control and collaborate on projects within this workspace</p>
  </div>
</div>

<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(360px, 1fr)); gap: 1.5rem; margin-bottom: 2rem;">
  <!-- Members List Card -->
  <div class="card" style="padding: 1.5rem; background: var(--color-surface); border: 1px solid var(--color-border);">
    <div class="card-header" style="margin-bottom: 1rem;">
      <h3 style="margin: 0; font-size: 1.05rem;">Active Members ({members.length})</h3>
    </div>

    {#if loading}
      <div style="text-align: center; padding: 2rem;">
        <Loader2 size={24} class="animate-spin text-muted" />
      </div>
    {:else}
      <div style="display: flex; flex-direction: column; gap: 0.75rem;">
        {#each members as m}
          <div style="display: flex; justify-content: space-between; align-items: center; padding: 0.75rem 1rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-surface);">
            <div style="display: flex; align-items: center; gap: 0.75rem;">
              <div style="width: 36px; height: 36px; border-radius: 50%; background: rgba(0,166,166,0.1); color: var(--color-accent); display: flex; align-items: center; justify-content: center; font-weight: 700;">
                {(m.user?.display_name || m.user_id || 'U').charAt(0).toUpperCase()}
              </div>
              <div>
                <div style="font-weight: 600; font-size: 0.875rem;">{m.user?.display_name || m.user?.email || m.user_id}</div>
                <div class="text-xs text-muted">{m.user?.email || 'User ID: ' + m.user_id}</div>
              </div>
            </div>
            <span class="badge" style="background: rgba(0,166,166,0.1); color: var(--color-accent); font-size: 0.75rem; text-transform: capitalize;">
              {m.role || 'Member'}
            </span>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- Add Member Card -->
  <div class="card" style="padding: 1.5rem; background: var(--color-surface); border: 1px solid var(--color-border);">
    <div class="card-header" style="margin-bottom: 1rem;">
      <h3 style="margin: 0; font-size: 1.05rem;">Add Member to Workspace</h3>
    </div>

    {#if inviteSaved}
      <div style="background:#d1fae5;border:1px solid #6ee7b7;color:#065f46;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem">
        ✓ Member added to workspace.
      </div>
    {/if}

    {#if inviteError}
      <div style="background:#fee2e2;border:1px solid #fca5a5;color:#991b1b;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem">
        {inviteError}
      </div>
    {/if}

    <form onsubmit={handleInvite}>
      <div class="form-group">
        <label class="form-label" for="member-email">User Email</label>
        <input id="member-email" type="email" class="form-input" placeholder="user@company.com" bind:value={inviteEmail} required />
      </div>

      <div class="form-group">
        <label class="form-label" for="member-role">Workspace Role</label>
        <select id="member-role" class="form-select" bind:value={inviteRole}>
          <option value="developer">Developer (Deploy & Manage Services)</option>
          <option value="admin">Admin (Manage Services & Members)</option>
          <option value="viewer">Viewer (Read-only)</option>
        </select>
      </div>

      <button type="submit" class="btn btn-primary" style="display: inline-flex; align-items: center; gap: 6px;" disabled={inviting}>
        {#if inviting}<Loader2 size={14} class="animate-spin" /> Adding...{:else}<UserPlus size={14} /> Add to Workspace{/if}
      </button>
    </form>
  </div>
</div>
