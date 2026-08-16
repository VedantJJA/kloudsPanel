<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Settings, Save, Trash2, Loader2, AlertTriangle, Zap, ShieldAlert, Sliders } from 'lucide-svelte';

  const { slug } = $derived($page.params);
  let workspace = $state<any>(null);
  let name = $state('');
  let loading = $state(true);
  let saving = $state(false);
  let saved = $state(false);
  let error = $state('');
  let deleting = $state(false);

  type WorkspaceSettingsTab = 'general' | 'danger';
  let activeTab = $state<WorkspaceSettingsTab>('general');

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

  const tabs: Array<{ id: WorkspaceSettingsTab; label: string; icon: any }> = [
    { id: 'general', label: 'General Info', icon: Sliders },
    { id: 'danger', label: 'Danger Zone', icon: AlertTriangle }
  ];
</script>

<svelte:head>
  <title>Workspace Settings - {workspace?.name || slug} - kloudsPanel</title>
</svelte:head>

<div class="page-header">
  <div>
    <div class="page-breadcrumbs">
      <a href="/workspaces">Workspaces</a>
      <span>/</span>
      <a href="/workspaces/{slug}">{workspace?.name || slug}</a>
      <span>/</span>
      <span>Settings</span>
    </div>
    <h1 class="page-title">Workspace Settings</h1>
    <p class="page-subtitle">Configure workspace details and manage resource lifecycles</p>
  </div>
</div>

<!-- Tabs Bar -->
<div class="tabs-bar">
  {#each tabs as t}
    {@const Icon = t.icon}
    <button
      type="button"
      class="tab-btn"
      class:active={activeTab === t.id}
      onclick={() => { activeTab = t.id; error = ''; saved = false; }}
    >
      <Icon size={15} />
      <span>{t.label}</span>
    </button>
  {/each}
</div>

{#if loading}
  <div style="text-align: center; padding: 2rem;">
    <Loader2 size={28} class="animate-spin text-muted" />
  </div>
{:else}
  <div style="max-width: 680px;">
    {#if activeTab === 'general'}
      <!-- General Settings Card -->
      <div class="card">
        <div class="card-header">
          <h3 style="margin: 0; font-size: 0.9375rem;">General Information</h3>
        </div>

        {#if saved}
          <div style="background: var(--color-success-subtle); border: 1px solid rgba(52,211,153,0.3); color: var(--color-success); border-radius: var(--radius-md); padding: 0.75rem 1rem; font-size: 0.8125rem; margin-bottom: 1.25rem;">
            Workspace updated successfully.
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

          <button type="submit" class="btn btn-primary" style="margin-top: 0.5rem;" disabled={saving}>
            {#if saving}<Loader2 size={14} class="animate-spin" /> Saving...{:else}<Save size={14} /> Save Changes{/if}
          </button>
        </form>
      </div>

    {:else if activeTab === 'danger'}
      <!-- Danger Zone Card -->
      <div class="card" style="border-color: rgba(248, 113, 113, 0.35);">
        <div class="card-header" style="border-bottom-color: rgba(248, 113, 113, 0.2);">
          <h3 style="margin: 0; font-size: 0.9375rem; color: var(--color-danger); display: flex; align-items: center; gap: 6px;">
            <AlertTriangle size={16} /> Danger Zone
          </h3>
        </div>

        <p class="text-sm text-muted" style="margin-bottom: 1.25rem;">
          Permanently delete this workspace and all associated projects, services, deployments, and databases. This action cannot be undone.
        </p>

        <button
          type="button"
          class="btn btn-danger"
          onclick={handleDelete}
          disabled={deleting}
        >
          {#if deleting}<Loader2 size={14} class="animate-spin" /> Deleting...{:else}<Trash2 size={14} /> Delete Workspace{/if}
        </button>
      </div>
    {/if}
  </div>
{/if}
