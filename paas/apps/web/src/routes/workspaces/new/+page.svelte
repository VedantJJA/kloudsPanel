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
      
      goto('/workspaces');
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>New Workspace — kloudsPanel</title>
</svelte:head>

<div class="page-header" style="margin-bottom: 2rem;">
  <button class="btn btn-secondary" onclick={() => goto('/workspaces')} style="margin-right: 1rem; padding: 0.5rem;">
    <ArrowLeft size={16} />
  </button>
  <div>
    <h1 class="page-title">New Workspace</h1>
    <p class="page-subtitle">Create a new environment to organize your projects and databases.</p>
  </div>
</div>

<div class="card" style="max-width: 600px;">
  <form onsubmit={createWorkspace}>
    <div style="margin-bottom: 1.5rem;">
      <label for="name" style="display: block; margin-bottom: 0.5rem; font-weight: 500;">Workspace Name</label>
      <input 
        id="name" 
        type="text" 
        class="input" 
        placeholder="e.g. Production, Acme Corp" 
        bind:value={name} 
        required 
        style="width: 100%; box-sizing: border-box;"
      />
    </div>

    <div style="margin-bottom: 2rem;">
      <label for="slug" style="display: block; margin-bottom: 0.5rem; font-weight: 500;">URL Slug</label>
      <div style="display: flex; align-items: center;">
        <span style="padding: 0.5rem 0.75rem; background: var(--bg-secondary); border: 1px solid var(--border-light); border-right: none; border-radius: 6px 0 0 6px; color: var(--text-muted); font-family: monospace;">/workspaces/</span>
        <input 
          id="slug" 
          type="text" 
          class="input" 
          placeholder="acme-corp" 
          bind:value={slug} 
          oninput={() => slugEdited = true}
          required 
          pattern="[a-z0-9-]+"
          style="width: 100%; border-radius: 0 6px 6px 0; box-sizing: border-box; font-family: monospace;"
        />
      </div>
      <p class="text-xs text-muted" style="margin-top: 0.5rem;">Only lowercase letters, numbers, and hyphens.</p>
    </div>

    {#if error}
      <div class="alert alert-error" style="margin-bottom: 1.5rem;">
        {error}
      </div>
    {/if}

    <div style="display: flex; justify-content: flex-end; gap: 1rem; padding-top: 1rem; border-top: 1px solid var(--border-light);">
      <button type="button" class="btn btn-secondary" onclick={() => goto('/workspaces')} disabled={loading}>
        Cancel
      </button>
      <button type="submit" class="btn btn-primary" disabled={loading || !name || !slug}>
        {#if loading}
          <Loader2 size={16} class="animate-spin" style="margin-right: 0.5rem;" />
          Creating...
        {:else}
          <Save size={16} style="margin-right: 0.5rem;" />
          Create Workspace
        {/if}
      </button>
    </div>
  </form>
</div>
