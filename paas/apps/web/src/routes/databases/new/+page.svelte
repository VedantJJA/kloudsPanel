<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { ArrowLeft, Save, Loader2, Database, Check } from 'lucide-svelte';

  let workspaces = $state<any[]>([]);
  let projects = $state<any[]>([]);
  
  let selectedWorkspace = $state('');
  let projectId = $state('');
  let name = $state('');
  let engine = $state('postgres');
  
  let loading = $state(false);
  let error = $state<string | null>(null);

  const engines = [
    { id: 'postgres', name: 'PostgreSQL 16', desc: 'Powerful object-relational SQL database', color: '#336791', port: 5432 },
    { id: 'mysql', name: 'MySQL 8.0', desc: 'Fast and reliable relational database', color: '#00758F', port: 3306 },
    { id: 'redis', name: 'Redis 7.2', desc: 'In-memory key-value cache and message broker', color: '#D82C20', port: 6379 },
    { id: 'mongodb', name: 'MongoDB 7.0', desc: 'Flexible document NoSQL database', color: '#47A248', port: 27017 },
    { id: 'clickhouse', name: 'ClickHouse 24.3', desc: 'Ultra-fast analytical columnar database', color: '#F3B400', port: 8123 },
  ];

  onMount(async () => {
    try {
      const res = await fetch('/api/v1/workspaces', { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        workspaces = data.workspaces ?? [];
        if (workspaces.length === 1) {
          selectedWorkspace = workspaces[0].id || workspaces[0].ID;
        }
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
        if (projects.length === 1) projectId = projects[0].id || projects[0].ID;
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
      
      const db = await res.json();
      const dbId = db.id || db.ID;
      // Navigate straight to the new database management dashboard
      goto(`/databases/${dbId}/overview`);
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }
</script>

<svelte:head>
  <title>New Database - kloudsPanel</title>
</svelte:head>

<div class="page-header" style="margin-bottom: 2rem;">
  <div style="display: flex; align-items: center; gap: 1rem;">
    <button 
      class="btn btn-secondary" 
      onclick={() => history.back()} 
      style="padding: 0; width: 40px; height: 40px; min-height: 40px; border-radius: var(--radius-md); display: flex; align-items: center; justify-content: center; flex-shrink: 0;"
      aria-label="Back"
    >
      <ArrowLeft size={18} />
    </button>
    <div>
      <h1 class="page-title">Provision Managed Database</h1>
      <p class="page-subtitle">Deploy a dedicated database instance with high availability and internal networking.</p>
    </div>
  </div>
</div>

<div class="card" style="max-width: 680px; margin-bottom: 3rem;">
  <form onsubmit={createDatabase}>
    
    <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; margin-bottom: 1.25rem;">
      <div class="form-group">
        <label for="workspace-select" class="form-label">Workspace</label>
        <select id="workspace-select" class="form-input" bind:value={selectedWorkspace} required>
          <option value="" disabled>Select Workspace...</option>
          {#each workspaces as ws}
            <option value={ws.id || ws.ID}>{ws.name || ws.Name}</option>
          {/each}
        </select>
      </div>

      <div class="form-group">
        <label for="project-select" class="form-label">Project</label>
        <select id="project-select" class="form-input" bind:value={projectId} required disabled={!selectedWorkspace || projects.length === 0}>
          <option value="" disabled>Select Project...</option>
          {#each projects as p}
            <option value={p.id || p.ID}>{p.name || p.Name}</option>
          {/each}
        </select>
      </div>
    </div>

    <div class="form-group" style="margin-bottom: 1.5rem;">
      <label for="db-name-input" class="form-label">Database Name</label>
      <input 
        id="db-name-input" 
        type="text" 
        class="form-input" 
        placeholder="e.g. production-db, auth-cache" 
        bind:value={name} 
        required 
        pattern="[a-z0-9-]+"
      />
      <p class="text-xs text-muted" style="margin-top: 0.25rem;">Only lowercase letters, numbers, and hyphens.</p>
    </div>

    <!-- Engine Selection Cards -->
    <div style="margin-bottom: 1.75rem;">
      <label for="db-engine-cards-container" class="form-label" style="margin-bottom: 0.75rem;">Select Database Engine</label>
      <div id="db-engine-cards-container" style="display: grid; grid-template-columns: 1fr; gap: 0.75rem;">
        {#each engines as eng}
          {@const isSelected = engine === eng.id}
          <button 
            type="button" 
            class="card"
            style="
              cursor: pointer;
              text-align: left;
              padding: 0.85rem 1rem;
              border: 2px solid {isSelected ? 'var(--color-accent)' : 'var(--color-border)'};
              background: {isSelected ? 'rgba(0,166,166,0.04)' : 'var(--color-surface)'};
              display: flex;
              align-items: center;
              justify-content: space-between;
              transition: all var(--transition-fast);
            "
            onclick={() => engine = eng.id}
          >
            <div style="display: flex; align-items: center; gap: 0.85rem;">
              <div style="display: flex; align-items: center; justify-content: center; width: 36px; height: 36px; border-radius: var(--radius-md); background: {eng.color}15; color: {eng.color};">
                <Database size={20} />
              </div>
              <div>
                <div style="font-size: 0.9375rem; font-weight: 600; color: var(--color-ink);">{eng.name}</div>
                <div class="text-xs text-muted">{eng.desc}</div>
              </div>
            </div>
            <div style="display: flex; align-items: center; gap: 0.75rem;">
              <span class="font-mono text-xs text-muted">Port {eng.port}</span>
              {#if isSelected}
                <span style="display: inline-flex; align-items: center; justify-content: center; width: 20px; height: 20px; border-radius: 50%; background: var(--color-accent); color: #fff;">
                  <Check size={12} strokeWidth={3} />
                </span>
              {/if}
            </div>
          </button>
        {/each}
      </div>
    </div>

    {#if error}
      <div class="alert alert-error" style="margin-bottom: 1.5rem; background: #fee2e2; border: 1px solid #fca5a5; color: #991b1b; padding: 0.75rem 1rem; border-radius: var(--radius-md); font-size: 0.875rem;">
        {error}
      </div>
    {/if}

    <div style="display: flex; justify-content: flex-end; gap: 1rem; padding-top: 1rem; border-top: 1px solid var(--color-border);">
      <button type="button" class="btn btn-secondary" onclick={() => history.back()} disabled={loading}>
        Cancel
      </button>
      <button type="submit" class="btn btn-primary" disabled={loading || !projectId || !name || !engine}>
        {#if loading}
          <Loader2 size={16} class="animate-spin" style="margin-right: 0.5rem;" />
          Provisioning Database...
        {:else}
          <Save size={16} style="margin-right: 0.5rem;" />
          Create Database
        {/if}
      </button>
    </div>
  </form>
</div>
