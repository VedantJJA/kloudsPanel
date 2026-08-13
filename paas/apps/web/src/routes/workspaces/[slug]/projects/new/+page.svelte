<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { ArrowLeft, Save, Loader2, FolderOpen } from 'lucide-svelte';

  const { slug } = $derived($page.params);
  let name = $state('');
  let projSlug = $state('');
  let loading = $state(false);
  let error = $state<string | null>(null);
  let workspaceId = $state('');

  // Fetch workspace to get its ID
  import { onMount } from 'svelte';
  onMount(async () => {
    const res = await fetch(`/api/v1/workspaces/${slug}`, { credentials: 'include' });
    if (res.ok) {
      const ws = await res.json();
      workspaceId = ws.id;
    } else {
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
        body: JSON.stringify({ workspaceId, name, slug: projSlug })
      });
      if (!res.ok) {
        const d = await res.json();
        throw new Error(d.detail || d.message || 'Failed to create project');
      }
      const proj = await res.json();
      goto(`/projects/${proj.id}`);
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
  <button class="btn btn-secondary" onclick={() => goto(`/workspaces/${slug}`)} style="margin-right: 1rem; padding: 0.5rem;">
    <ArrowLeft size={16} />
  </button>
  <div>
    <h1 class="page-title">New Project</h1>
    <p class="page-subtitle">Create a project to organize and deploy your services.</p>
  </div>
</div>

<div class="card" style="max-width: 600px;">
  <form onsubmit={createProject}>
    <div style="margin-bottom: 1.5rem;">
      <label for="proj-name" style="display: block; margin-bottom: 0.5rem; font-weight: 500;">Project Name</label>
      <input
        id="proj-name"
        type="text"
        class="input"
        placeholder="e.g. My Web App, Backend API"
        bind:value={name}
        required
        style="width: 100%; box-sizing: border-box;"
      />
    </div>

    <div style="margin-bottom: 2rem;">
      <label for="proj-slug" style="display: block; margin-bottom: 0.5rem; font-weight: 500;">URL Slug</label>
      <div style="display: flex; align-items: center;">
        <span style="padding: 0.5rem 0.75rem; background: var(--bg-secondary); border: 1px solid var(--border-light); border-right: none; border-radius: 6px 0 0 6px; color: var(--text-muted); font-family: monospace;">/projects/</span>
        <input
          id="proj-slug"
          type="text"
          class="input"
          placeholder="my-web-app"
          bind:value={projSlug}
          oninput={() => projSlugEdited = true}
          required
          pattern="[a-z0-9-]+"
          style="width: 100%; border-radius: 0 6px 6px 0; box-sizing: border-box; font-family: monospace;"
        />
      </div>
      <p class="text-xs text-muted" style="margin-top: 0.5rem;">Only lowercase letters, numbers, and hyphens.</p>
    </div>

    {#if error}
      <div class="alert alert-error" style="margin-bottom: 1.5rem;">{error}</div>
    {/if}

    <div style="display: flex; justify-content: flex-end; gap: 1rem; padding-top: 1rem; border-top: 1px solid var(--border-light);">
      <button type="button" class="btn btn-secondary" onclick={() => goto(`/workspaces/${slug}`)} disabled={loading}>
        Cancel
      </button>
      <button type="submit" class="btn btn-primary" disabled={loading || !name || !projSlug || !workspaceId}>
        {#if loading}
          <Loader2 size={16} style="margin-right: 0.5rem;" />
          Creating...
        {:else}
          <Save size={16} style="margin-right: 0.5rem;" />
          Create Project
        {/if}
      </button>
    </div>
  </form>
</div>
