<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { ArrowLeft, Save, Loader2, Database, Check, ShieldCheck, KeyRound, Sparkles } from 'lucide-svelte';
  import FrameworkIcon from '$lib/components/icons/FrameworkIcon.svelte';

  let workspaces = $state<any[]>([]);
  let projects = $state<any[]>([]);
  
  let selectedWorkspace = $state('');
  let projectId = $state('');
  let name = $state('');
  let engine = $state('postgres');
  let selectedVersion = $state('16');
  let customPassword = $state('');
  let useAutoPassword = $state(true);
  
  let loading = $state(false);
  let error = $state<string | null>(null);

  interface EngineOption {
    id: string;
    name: string;
    desc: string;
    color: string;
    port: number;
    versions: Array<{ value: string; label: string; default?: boolean }>;
  }

  const engines: EngineOption[] = [
    {
      id: 'postgres',
      name: 'PostgreSQL',
      desc: 'Powerful object-relational SQL database with extensions support',
      color: '#336791',
      port: 5432,
      versions: [
        { value: '17', label: 'v17 (Latest)' },
        { value: '16', label: 'v16 (Recommended / Stable)', default: true },
        { value: '15', label: 'v15' },
        { value: '14', label: 'v14' },
        { value: '13', label: 'v13 (Legacy)' }
      ]
    },
    {
      id: 'mysql',
      name: 'MySQL',
      desc: 'Fast, reliable and ubiquitous relational database',
      color: '#00758F',
      port: 3306,
      versions: [
        { value: '9.0', label: 'v9.0 (Innovation)' },
        { value: '8.4', label: 'v8.4 (LTS)' },
        { value: '8.0', label: 'v8.0 (Recommended)', default: true },
        { value: '5.7', label: 'v5.7 (Legacy)' }
      ]
    },
    {
      id: 'redis',
      name: 'Redis',
      desc: 'Ultra-fast in-memory key-value store, cache and message broker',
      color: '#DC382D',
      port: 6379,
      versions: [
        { value: '7.4', label: 'v7.4 (Latest)' },
        { value: '7.2', label: 'v7.2 (Recommended / Stable)', default: true },
        { value: '7.0', label: 'v7.0' },
        { value: '6.2', label: 'v6.2 (Legacy)' }
      ]
    },
    {
      id: 'mongodb',
      name: 'MongoDB',
      desc: 'Flexible document-oriented JSON/BSON NoSQL database',
      color: '#47A248',
      port: 27017,
      versions: [
        { value: '8.0', label: 'v8.0 (Latest)' },
        { value: '7.0', label: 'v7.0 (Recommended)', default: true },
        { value: '6.0', label: 'v6.0' },
        { value: '5.0', label: 'v5.0' }
      ]
    },
    {
      id: 'clickhouse',
      name: 'ClickHouse',
      desc: 'Ultra-fast analytical columnar database for real-time big data',
      color: '#F3B400',
      port: 8123,
      versions: [
        { value: '24.8', label: 'v24.8 (Latest)' },
        { value: '24.3', label: 'v24.3 (LTS / Recommended)', default: true },
        { value: '23.8', label: 'v23.8 (LTS)' }
      ]
    }
  ];

  const currentEngineObj = $derived(engines.find(e => e.id === engine) || engines[0]);

  // When engine changes, switch to its default version
  $effect(() => {
    const defaultVer = currentEngineObj.versions.find(v => v.default)?.value || currentEngineObj.versions[0]?.value || '';
    selectedVersion = defaultVer;
  });

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
      const payload: any = {
        projectId,
        name,
        engine,
        version: selectedVersion
      };
      if (!useAutoPassword && customPassword.trim()) {
        payload.password = customPassword.trim();
      }

      const res = await fetch('/api/v1/databases', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(payload)
      });
      
      if (!res.ok) {
        const data = await res.json();
        throw new Error(data.detail || data.message || data.error || 'Failed to create database');
      }
      
      const db = await res.json();
      const dbId = db.id || db.ID;
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
      <h1 class="page-title">Provision Managed Database</h1>
      <p class="page-subtitle">Deploy a dedicated database instance with version selection and isolated private networking</p>
    </div>
  </div>
</div>

<div class="card" style="max-width: 680px; margin-bottom: 2rem;">
  <form onsubmit={createDatabase}>
    
    <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; margin-bottom: 1.25rem;">
      <div class="form-group">
        <label for="workspace-select" class="form-label">Workspace</label>
        <select id="workspace-select" class="form-select" bind:value={selectedWorkspace} required>
          <option value="" disabled>Select Workspace...</option>
          {#each workspaces as ws}
            <option value={ws.id || ws.ID}>{ws.name || ws.Name}</option>
          {/each}
        </select>
      </div>

      <div class="form-group">
        <label for="project-select" class="form-label">Project</label>
        <select id="project-select" class="form-select" bind:value={projectId} required disabled={!selectedWorkspace || projects.length === 0}>
          <option value="" disabled>Select Project...</option>
          {#each projects as p}
            <option value={p.id || p.ID}>{p.name || p.Name}</option>
          {/each}
        </select>
      </div>
    </div>

    <div class="form-group" style="margin-bottom: 1.5rem;">
      <label for="db-name-input" class="form-label">Database Name / Identifier</label>
      <input 
        id="db-name-input" 
        type="text" 
        class="form-input" 
        placeholder="e.g. production-db, auth-cache, user-store" 
        bind:value={name} 
        required 
        pattern="[a-z0-9-]+"
      />
      <p class="text-xs text-muted" style="margin-top: 0.25rem;">Only lowercase letters, numbers, and hyphens.</p>
    </div>

    <!-- Engine Selection Cards -->
    <div style="margin-bottom: 1.5rem;">
      <label for="db-engine-cards-container" class="form-label" style="margin-bottom: 0.5rem; display:block;">Select Database Engine</label>
      <div id="db-engine-cards-container" style="display: grid; grid-template-columns: 1fr; gap: 0.5rem;">
        {#each engines as eng}
          {@const isSelected = engine === eng.id}
          <button 
            type="button" 
            class="card"
            style="
              cursor: pointer;
              text-align: left;
              padding: 0.75rem 1rem;
              border: 1px solid {isSelected ? '#ffffff' : 'var(--color-border)'};
              background: {isSelected ? 'var(--color-surface-subtle)' : 'var(--color-surface)'};
              display: flex;
              align-items: center;
              justify-content: space-between;
              transition: all var(--transition-fast);
            "
            onclick={() => engine = eng.id}
          >
            <div style="display: flex; align-items: center; gap: 0.75rem;">
              <div style="display: flex; align-items: center; justify-content: center; width: 34px; height: 34px; border-radius: var(--radius-sm); background: var(--color-surface); border: 1px solid var(--color-border);">
                <FrameworkIcon name={eng.id} size={20} />
              </div>
              <div>
                <div style="font-size: 0.875rem; font-weight: 600; color: var(--color-ink);">{eng.name}</div>
                <div class="text-xs text-muted">{eng.desc}</div>
              </div>
            </div>
            <div style="display: flex; align-items: center; gap: 0.75rem;">
              <span class="font-mono text-xs text-muted">Port {eng.port}</span>
              {#if isSelected}
                <span style="display: inline-flex; align-items: center; justify-content: center; width: 18px; height: 18px; border-radius: var(--radius-sm); background: #ffffff; color: #090a0f;">
                  <Check size={12} strokeWidth={3} />
                </span>
              {/if}
            </div>
          </button>
        {/each}
      </div>
    </div>

    <!-- Dynamic Version Selection Box -->
    <div style="padding: 1.25rem; background: var(--color-surface-subtle); border: 1px solid var(--color-border); border-radius: var(--radius-md); margin-bottom: 1.5rem;">
      <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.5rem;">
        <div>
          <label for="engine-version-select" class="form-label" style="margin: 0;">
            {currentEngineObj.name} Engine Version
          </label>
        </div>
      </div>

      <select id="engine-version-select" class="form-select font-mono text-xs" bind:value={selectedVersion}>
        {#each currentEngineObj.versions as v}
          <option value={v.value}>{v.label}</option>
        {/each}
      </select>
    </div>

    <!-- Security & Password Credentials Box -->
    <div style="padding: 1.25rem; background: var(--color-surface-subtle); border: 1px solid var(--color-border); border-radius: var(--radius-md); margin-bottom: 1.5rem;">
      <div style="display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.75rem;">
        <ShieldCheck size={16} style="color: #ffffff;" />
        <span style="font-size: 0.8125rem; font-weight: 600; color: var(--color-ink);">Security and Master Credentials</span>
      </div>

      <div style="display: flex; flex-direction: column; gap: 0.6rem;">
        <label style="display: flex; align-items: center; gap: 0.5rem; font-size: 0.8125rem; cursor: pointer; color: var(--color-ink);">
          <input type="radio" name="pwd_mode" checked={useAutoPassword} onchange={() => useAutoPassword = true} />
          <span><strong>Auto-generate strong password</strong> (128-bit entropy, Recommended)</span>
        </label>
        <label style="display: flex; align-items: center; gap: 0.5rem; font-size: 0.8125rem; cursor: pointer; color: var(--color-ink);">
          <input type="radio" name="pwd_mode" checked={!useAutoPassword} onchange={() => useAutoPassword = false} />
          <span>Set custom master password</span>
        </label>

        {#if !useAutoPassword}
          <div style="margin-top: 0.35rem;">
            <input 
              type="password" 
              class="form-input font-mono text-xs" 
              placeholder="Enter secure master password..." 
              bind:value={customPassword}
              required={!useAutoPassword}
            />
          </div>
        {/if}
      </div>
    </div>

    {#if error}
      <div style="margin-bottom: 1.25rem; background: var(--color-danger-subtle); border: 1px solid rgba(248,113,113,0.3); color: var(--color-danger); padding: 0.75rem 1rem; border-radius: var(--radius-md); font-size: 0.8125rem;">
        {error}
      </div>
    {/if}

    <div style="display: flex; justify-content: flex-end; gap: 0.75rem; padding-top: 1rem; border-top: 1px solid var(--color-border-subtle);">
      <button type="button" class="btn btn-secondary" onclick={() => history.back()} disabled={loading}>
        Cancel
      </button>
      <button type="submit" class="btn btn-primary" disabled={loading || !projectId || !name || !engine}>
        {#if loading}
          <Loader2 size={14} class="animate-spin" style="margin-right: 0.4rem;" />
          Provisioning Database...
        {:else}
          <Save size={14} style="margin-right: 0.4rem;" />
          Create {currentEngineObj.name} Database
        {/if}
      </button>
    </div>
  </form>
</div>
