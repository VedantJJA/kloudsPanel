<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { ArrowLeft, Save, Loader2 } from 'lucide-svelte';
  import { onMount } from 'svelte';

  const { slug } = $derived($page.params);
  let name = $state('');
  let projSlug = $state('');
  let loading = $state(false);
  let error = $state<string | null>(null);
  let workspaceId = $state('');

  // Fetch workspace to get its ID
  onMount(async () => {
    try {
      const res = await fetch(`/api/v1/workspaces/${slug}`, { credentials: 'include' });
      if (res.ok) {
        const ws = await res.json();
        workspaceId = ws.id || ws.ID;
      } else {
        goto('/workspaces');
      }
    } catch (e) {
      goto('/workspaces');
    }
  });

  let projSlugEdited = false;
  $effect(() => {
    if (!projSlugEdited && name) {
      projSlug = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)+/g, '');
    }
  });

  async function createProject(e: Event) {
    e.preventDefault();
    if (!workspaceId || !name || !projSlug) return;
    loading = true;
    error = null;
    try {
      const res = await fetch('/api/v1/projects', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ 
          workspaceId, 
          name, 
          slug: projSlug,
          sourceKind: 'empty'
        })
      });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        throw new Error(d.detail || d.message || 'Failed to create project');
      }
      const proj = await res.json();
      const projId = proj.id || proj.ID;
      goto(`/projects/${projId}`);
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>New Project — kloudsPanel</title>
</svelte:head>

<div class="page-header" style="margin-bottom: 2rem;">
  <div style="display: flex; align-items: center; gap: 1rem;">
    <button 
      class="btn btn-secondary" 
      onclick={() => goto(`/workspaces/${slug}`)} 
      style="padding: 0; width: 40px; height: 40px; min-height: 40px; border-radius: var(--radius-md); display: flex; align-items: center; justify-content: center; flex-shrink: 0;"
      aria-label="Back to Workspace"
    >
      <ArrowLeft size={18} />
    </button>
    <div>
      <h1 class="page-title">New Project</h1>
      <p class="page-subtitle">Create a project to organize and deploy your services.</p>
    </div>
  </div>
</div>

<div class="card" style="max-width: 600px;">
  <form onsubmit={createProject}>
    <div class="form-group" style="margin-bottom: 1.5rem;">
      <label for="proj-name" class="form-label">Project Name</label>
      <input
        id="proj-name"
        type="text"
        class="form-input"
        placeholder="e.g. My Web App, Backend API"
        bind:value={name}
        required
      />
    </div>

    <div class="form-group" style="margin-bottom: 2rem;">
      <label for="proj-slug" class="form-label">URL Slug</label>
      <div style="display: flex; align-items: stretch; width: 100%;">
        <span style="display: inline-flex; align-items: center; padding: 0 1rem; background: var(--color-canvas); border: 1px solid var(--color-border); border-right: none; border-radius: var(--radius-md) 0 0 var(--radius-md); color: var(--color-ink-secondary); font-family: var(--font-mono); font-size: 0.875rem; white-space: nowrap; flex-shrink: 0; user-select: none;">
          /projects/
        </span>
        <input
          id="proj-slug"
          type="text"
          class="form-input"
          placeholder="my-web-app"
          bind:value={projSlug}
          oninput={() => projSlugEdited = true}
          required
          pattern="[a-z0-9-]+"
          style="border-radius: 0 var(--radius-md) var(--radius-md) 0; font-family: var(--font-mono); flex: 1; min-width: 0;"
        />
      </div>
      <p class="text-xs text-muted" style="margin-top: 0.5rem;">Only lowercase letters, numbers, and hyphens.</p>
    </div>

    {#if error}
      <div class="alert alert-error" style="margin-bottom: 1.5rem; background: #fee2e2; border: 1px solid #fca5a5; color: #991b1b; padding: 0.75rem 1rem; border-radius: var(--radius-md); font-size: 0.875rem;">
        {error}
      </div>
    {/if}

    <div style="display: flex; justify-content: flex-end; gap: 1rem; padding-top: 1rem; border-top: 1px solid var(--color-border);">
      <button type="button" class="btn btn-secondary" onclick={() => goto(`/workspaces/${slug}`)} disabled={loading}>
        Cancel
      </button>
      <button type="submit" class="btn btn-primary" disabled={loading || !name || !projSlug || !workspaceId}>
        {#if loading}
          <Loader2 size={16} class="animate-spin" style="margin-right: 0.5rem;" />
          Creating...
        {:else}
          <Save size={16} style="margin-right: 0.5rem;" />
          Create Project
        {/if}
      </button>
    </div>
  </form>
</div>
