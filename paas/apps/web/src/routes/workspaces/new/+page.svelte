<script lang="ts">
  import { goto } from '$app/navigation';
  import { Box, ArrowLeft, Save, Loader2 } from 'lucide-svelte';

  let name = $state('');
  let slug = $state('');
  let loading = $state(false);
  let error = $state<string | null>(null);

  // Auto-generate slug from name if user hasn't edited it manually
  let slugEdited = false;
  $effect(() => {
    if (!slugEdited && name) {
      slug = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)+/g, '');
    }
  });

  async function createWorkspace(e: Event) {
    e.preventDefault();
    if (!name || !slug) return;
    
    loading = true;
    error = null;
    
    try {
      const res = await fetch('/api/v1/workspaces', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ name, slug })
      });
      
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.detail || data.message || 'Failed to create workspace');
      }
      
      const data = await res.json();
      const targetSlug = data.slug || data.Slug || slug;
      goto(`/workspaces/${targetSlug}`);
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>New Workspace - kloudsPanel</title>
</svelte:head>

<div class="page-header">
  <div style="display: flex; align-items: center; gap: 0.75rem;">
    <button 
      class="btn btn-secondary" 
      onclick={() => history.back()} 
      style="padding: 0; width: 34px; height: 34px; min-height: 34px; border-radius: var(--radius-md); display: flex; align-items: center; justify-content: center; flex-shrink: 0;"
      aria-label="Back"
    >
      <ArrowLeft size={16} />
    </button>
    <div>
      <h1 class="page-title">New Workspace</h1>
      <p class="page-subtitle">Create a new environment to organize your projects and databases</p>
    </div>
  </div>
</div>

<div class="card" style="max-width: 580px;">
  <form onsubmit={createWorkspace}>
    <div class="form-group" style="margin-bottom: 1.25rem;">
      <label for="name" class="form-label">Workspace Name</label>
      <input 
        id="name" 
        type="text" 
        class="form-input" 
        placeholder="e.g. Production, Team Workspace" 
        bind:value={name} 
        required 
      />
    </div>

    <div class="form-group" style="margin-bottom: 1.5rem;">
      <label for="slug" class="form-label">URL Slug</label>
      <div style="display: flex; align-items: stretch; width: 100%;">
        <span style="display: inline-flex; align-items: center; padding: 0 0.85rem; background: var(--color-surface-subtle); border: 1px solid var(--color-border); border-right: none; border-radius: var(--radius-md) 0 0 var(--radius-md); color: var(--color-ink-secondary); font-family: var(--font-mono); font-size: 0.8125rem; white-space: nowrap; flex-shrink: 0; user-select: none;">
          /workspaces/
        </span>
        <input 
          id="slug" 
          type="text" 
          class="form-input" 
          placeholder="production-team" 
          bind:value={slug} 
          oninput={() => slugEdited = true}
          required 
          pattern="[a-z0-9-]+"
          style="border-radius: 0 var(--radius-md) var(--radius-md) 0; font-family: var(--font-mono); flex: 1; min-width: 0;"
        />
      </div>
      <p class="text-xs text-muted" style="margin-top: 0.35rem;">Only lowercase letters, numbers, and hyphens.</p>
    </div>

    {#if error}
      <div style="margin-bottom: 1.25rem; background: var(--color-danger-subtle); border: 1px solid rgba(248,113,113,0.3); color: var(--color-danger); padding: 0.75rem 1rem; border-radius: var(--radius-md); font-size: 0.8125rem;">
        {error}
      </div>
    {/if}

    <div style="display: flex; justify-content: flex-end; gap: 0.75rem; padding-top: 1rem; border-top: 1px solid var(--color-border-subtle);">
      <button type="button" class="btn btn-secondary" onclick={() => goto('/workspaces')} disabled={loading}>
        Cancel
      </button>
      <button type="submit" class="btn btn-primary" disabled={loading || !name || !slug}>
        {#if loading}
          <Loader2 size={15} class="animate-spin" style="margin-right: 0.4rem;" />
          Creating...
        {:else}
          <Save size={15} style="margin-right: 0.4rem;" />
          Create Workspace
        {/if}
      </button>
    </div>
  </form>
</div>
