<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { ArrowLeft, Save, Loader2, Database } from 'lucide-svelte';

  let workspaces = $state<any[]>([]);
  let projects = $state<any[]>([]);
  
  let selectedWorkspace = $state('');
  let projectId = $state('');
  let name = $state('');
  let engine = $state('postgres');
  
  let loading = $state(false);
  let error = $state<string | null>(null);

  onMount(async () => {
    try {
      const res = await fetch('/api/v1/workspaces', { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        workspaces = data.workspaces ?? [];
      }
    } catch (e) {
      console.error(e);
    }
  });

  $effect(() => {
    if (selectedWorkspace) {
      fetchProjects(selectedWorkspace);
    } else {
      projects = [];
      projectId = '';
    }
  });

  async function fetchProjects(wsId: string) {
    try {
      const res = await fetch(`/api/v1/projects?workspaceId=${wsId}`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        projects = data.projects ?? [];
        if (projects.length === 1) projectId = projects[0].id;
      }
    } catch (e) {
      console.error(e);
    }
  }

  async function createDatabase(e: Event) {
    e.preventDefault();
    if (!projectId || !name || !engine) return;
    
    loading = true;
    error = null;
    
    try {
      const res = await fetch('/api/v1/databases', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ projectId, name, engine })
      });
      
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.detail || data.message || 'Failed to create database');
      }
      
      // Navigate to the project page after creation
      goto(`/projects/${projectId}`);
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>New Database — kloudsPanel</title>
</svelte:head>

<div class="page-header" style="margin-bottom: 2rem;">
  <button class="btn btn-secondary" onclick={() => history.back()} style="margin-right: 1rem; padding: 0.5rem;">
    <ArrowLeft size={16} />
  </button>
  <div>
    <h1 class="page-title">New Database</h1>
    <p class="page-subtitle">Provision a new managed database.</p>
  </div>
</div>

<div class="card" style="max-width: 600px;">
  <form onsubmit={createDatabase}>
    
    <div style="margin-bottom: 1.5rem;">
      <label for="workspace" style="display: block; margin-bottom: 0.5rem; font-weight: 500;">Workspace</label>
      <select id="workspace" class="input" bind:value={selectedWorkspace} required style="width: 100%; box-sizing: border-box;">
        <option value="" disabled>Select Workspace...</option>
        {#each workspaces as ws}
          <option value={ws.id}>{ws.name}</option>
        {/each}
      </select>
    </div>

    <div style="margin-bottom: 1.5rem;">
      <label for="project" style="display: block; margin-bottom: 0.5rem; font-weight: 500;">Project</label>
      <select id="project" class="input" bind:value={projectId} required disabled={!selectedWorkspace || projects.length === 0} style="width: 100%; box-sizing: border-box;">
        <option value="" disabled>Select Project...</option>
        {#each projects as p}
          <option value={p.id}>{p.name}</option>
        {/each}
      </select>
      {#if selectedWorkspace && projects.length === 0}
        <p class="text-xs text-muted" style="margin-top: 0.5rem;">No projects found in this workspace. Create one first.</p>
      {/if}
    </div>

    <div style="margin-bottom: 1.5rem;">
      <label for="name" style="display: block; margin-bottom: 0.5rem; font-weight: 500;">Database Name</label>
      <input 
        id="name" 
        type="text" 
        class="input" 
        placeholder="e.g. main-db, cache" 
        bind:value={name} 
        required 
        pattern="[a-z0-9-]+"
        style="width: 100%; box-sizing: border-box;"
      />
      <p class="text-xs text-muted" style="margin-top: 0.5rem;">Only lowercase letters, numbers, and hyphens.</p>
    </div>

    <div style="margin-bottom: 2rem;">
      <label style="display: block; margin-bottom: 0.5rem; font-weight: 500;">Engine</label>
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem;">
        <label class="card" style="cursor: pointer; border: 1px solid {engine === 'postgres' ? 'var(--color-primary)' : 'var(--border-light)'};">
          <input type="radio" bind:group={engine} value="postgres" style="display: none;" />
          <div style="display: flex; align-items: center; gap: 0.5rem; font-weight: 500;">
            <Database size={20} style="color: #336791;" /> PostgreSQL
          </div>
        </label>
        <label class="card" style="cursor: pointer; border: 1px solid {engine === 'mysql' ? 'var(--color-primary)' : 'var(--border-light)'};">
          <input type="radio" bind:group={engine} value="mysql" style="display: none;" />
          <div style="display: flex; align-items: center; gap: 0.5rem; font-weight: 500;">
            <Database size={20} style="color: #00758F;" /> MySQL
          </div>
        </label>
        <label class="card" style="cursor: pointer; border: 1px solid {engine === 'redis' ? 'var(--color-primary)' : 'var(--border-light)'};">
          <input type="radio" bind:group={engine} value="redis" style="display: none;" />
          <div style="display: flex; align-items: center; gap: 0.5rem; font-weight: 500;">
            <Database size={20} style="color: #D82C20;" /> Redis
          </div>
        </label>
        <label class="card" style="cursor: pointer; border: 1px solid {engine === 'mongodb' ? 'var(--color-primary)' : 'var(--border-light)'};">
          <input type="radio" bind:group={engine} value="mongodb" style="display: none;" />
          <div style="display: flex; align-items: center; gap: 0.5rem; font-weight: 500;">
            <Database size={20} style="color: #47A248;" /> MongoDB
          </div>
        </label>
      </div>
    </div>

    {#if error}
      <div class="alert alert-error" style="margin-bottom: 1.5rem;">
        {error}
      </div>
    {/if}

    <div style="display: flex; justify-content: flex-end; gap: 1rem; padding-top: 1rem; border-top: 1px solid var(--border-light);">
      <button type="button" class="btn btn-secondary" onclick={() => history.back()} disabled={loading}>
        Cancel
      </button>
      <button type="submit" class="btn btn-primary" disabled={loading || !projectId || !name || !engine}>
        {#if loading}
          <Loader2 size={16} class="animate-spin" style="margin-right: 0.5rem;" />
          Provisioning...
        {:else}
          <Save size={16} style="margin-right: 0.5rem;" />
          Create Database
        {/if}
      </button>
    </div>
  </form>
</div>
