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

<div class="page-header">
  <div>
    <div class="page-breadcrumbs">
      <a href="/workspaces/{slug}">{workspace?.name || slug}</a>
      <span>/</span>
      <span>Members</span>
    </div>
    <h1 class="page-title">Workspace Members</h1>
    <p class="page-subtitle">Manage access control and collaborate on projects within this workspace</p>
  </div>
</div>

<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(340px, 1fr)); gap: 1.5rem; margin-bottom: 2rem;">
  <!-- Members List Card -->
  <div class="card">
    <div class="card-header">
      <h3 style="margin: 0; font-size: 0.9375rem;">Active Members ({members.length})</h3>
    </div>

    {#if loading}
      <div style="text-align: center; padding: 2rem;">
        <Loader2 size={24} class="animate-spin text-muted" />
      </div>
    {:else}
      <div style="display: flex; flex-direction: column; gap: 0.5rem;">
        {#each members as m}
          <div style="display: flex; justify-content: space-between; align-items: center; padding: 0.75rem 0.85rem; border: 1px solid var(--color-border); border-radius: var(--radius-md); background: var(--color-surface-subtle);">
            <div style="display: flex; align-items: center; gap: 0.75rem;">
              <div style="width: 32px; height: 32px; border-radius: var(--radius-sm); background: var(--color-surface); border: 1px solid var(--color-border); color: #ffffff; display: flex; align-items: center; justify-content: center; font-weight: 700; font-size: 0.8125rem;">
                {(m.user?.display_name || m.user_id || 'U').charAt(0).toUpperCase()}
              </div>
              <div>
                <div style="font-weight: 600; font-size: 0.8125rem; color: var(--color-ink);">{m.user?.display_name || m.user?.email || m.user_id}</div>
                <div class="text-xs text-muted font-mono">{m.user?.email || 'ID: ' + m.user_id}</div>
              </div>
            </div>
            <span class="badge" style="background: rgba(255,255,255,0.08); color: #ffffff; font-size: 0.7rem; text-transform: capitalize;">
              {m.role || 'Member'}
            </span>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- Add Member Card -->
  <div class="card">
    <div class="card-header">
      <h3 style="margin: 0; font-size: 0.9375rem;">Add Member</h3>
    </div>

    {#if inviteSaved}
      <div style="background: var(--color-success-subtle); border: 1px solid rgba(52,211,153,0.3); color: var(--color-success); border-radius: var(--radius-md); padding: 0.75rem 1rem; font-size: 0.8125rem; margin-bottom: 1.25rem;">
        Member added to workspace.
      </div>
    {/if}

    {#if inviteError}
      <div style="background: var(--color-danger-subtle); border: 1px solid rgba(248,113,113,0.3); color: var(--color-danger); border-radius: var(--radius-md); padding: 0.75rem 1rem; font-size: 0.8125rem; margin-bottom: 1.25rem;">
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

      <button type="submit" class="btn btn-primary" style="margin-top: 0.5rem;" disabled={inviting}>
        {#if inviting}<Loader2 size={14} class="animate-spin" /> Adding...{:else}<UserPlus size={14} /> Add to Workspace{/if}
      </button>
    </form>
  </div>
</div>
