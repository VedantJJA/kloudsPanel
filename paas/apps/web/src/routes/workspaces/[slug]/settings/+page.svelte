<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Settings, Save, Trash2, Loader2, AlertTriangle } from 'lucide-svelte';

  const { slug } = $derived($page.params);
  let workspace = $state<any>(null);
  let name = $state('');
  let loading = $state(true);
  let saving = $state(false);
  let saved = $state(false);
  let error = $state('');
  let deleting = $state(false);

  async function loadData() {
    try {
      const res = await fetch(`/api/v1/workspaces/${slug}`, { credentials: 'include' });
      if (res.ok) {
        workspace = await res.json();
        name = workspace.name || workspace.Name || '';
      } else {
        goto('/workspaces');
      }
    } catch {
      goto('/workspaces');
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadData();
  });

  async function handleUpdate(e: Event) {
    e.preventDefault();
    saving = true;
    error = '';
    saved = false;
    try {
      const res = await fetch(`/api/v1/workspaces/${slug}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ name })
      });
      if (res.ok) {
        saved = true;
        await loadData();
        setTimeout(() => saved = false, 3000);
      } else {
        error = 'Failed to update workspace';
      }
    } finally {
      saving = false;
    }
  }

  async function handleDelete() {
    const wsId = workspace?.id || workspace?.ID;
    if (!confirm(`Are you sure you want to permanently delete workspace "${workspace?.name || slug}"? All projects, services, and databases within this workspace will be deleted.`)) return;

    deleting = true;
    try {
      const res = await fetch(`/api/v1/workspaces/${encodeURIComponent(wsId)}`, {
        method: 'DELETE',
        credentials: 'include'
      });
      if (res.ok) {
        goto('/workspaces');
      } else {
        alert('Failed to delete workspace');
      }
    } finally {
      deleting = false;
    }
  }
</script>

<svelte:head>
  <title>Workspace Settings - {workspace?.name || slug} - kloudsPanel</title>
</svelte:head>

<div class="page-header" style="margin-bottom: 1.5rem;">
  <div>
    <div class="page-breadcrumbs">
      <a href="/workspaces">Workspaces</a> /
      <a href="/workspaces/{slug}">{workspace?.name || slug}</a> /
      <span>Settings</span>
    </div>
    <h1 class="page-title">Workspace Settings</h1>
    <p class="page-subtitle">Configure workspace details and manage resource lifecycles</p>
  </div>
</div>

{#if loading}
  <div style="text-align: center; padding: 2rem;">
    <Loader2 size={32} class="animate-spin text-muted" />
  </div>
{:else}
  <div style="display: flex; flex-direction: column; gap: 1.5rem; max-width: 720px;">
    <!-- General Settings Card -->
    <div class="card" style="padding: 1.5rem; background: var(--color-surface); border: 1px solid var(--color-border);">
      <div class="card-header" style="margin-bottom: 1.25rem;">
        <h3 style="margin: 0; font-size: 1.05rem;">General Information</h3>
      </div>

      {#if saved}
        <div style="background:#d1fae5;border:1px solid #6ee7b7;color:#065f46;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem">
          ✓ Workspace updated successfully.
        </div>
      {/if}

      <form onsubmit={handleUpdate}>
        <div class="form-group">
          <label class="form-label" for="ws-name">Workspace Name</label>
          <input id="ws-name" type="text" class="form-input" bind:value={name} required />
        </div>

        <div class="form-group">
          <label class="form-label" for="ws-slug">Workspace Slug (Immutable)</label>
          <input id="ws-slug" type="text" class="form-input font-mono" value={slug} disabled />
        </div>

        <button type="submit" class="btn btn-primary" style="display: inline-flex; align-items: center; gap: 6px;" disabled={saving}>
          {#if saving}<Loader2 size={14} class="animate-spin" /> Saving...{:else}<Save size={14} /> Save Changes{/if}
        </button>
      </form>
    </div>

    <!-- Danger Zone Card -->
    <div class="card" style="padding: 1.5rem; background: var(--color-surface); border: 1px solid #fca5a5;">
      <div class="card-header" style="margin-bottom: 1rem;">
        <h3 style="margin: 0; font-size: 1.05rem; color: var(--color-error); display: flex; align-items: center; gap: 6px;">
          <AlertTriangle size={18} /> Danger Zone
        </h3>
      </div>

      <p class="text-sm text-muted" style="margin-bottom: 1.25rem;">
        Permanently delete this workspace and all projects, services, deployments, and attached databases. This action cannot be undone.
      </p>

      <button
        type="button"
        class="btn btn-danger"
        style="background: var(--color-danger); border-color: var(--color-danger); color: #fff; display: inline-flex; align-items: center; gap: 6px;"
        onclick={handleDelete}
        disabled={deleting}
      >
        {#if deleting}<Loader2 size={14} class="animate-spin" /> Deleting...{:else}<Trash2 size={14} /> Delete Workspace{/if}
      </button>
    </div>
  </div>
{/if}
