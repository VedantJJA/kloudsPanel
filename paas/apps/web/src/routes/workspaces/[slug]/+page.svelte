<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';

  const { slug } = $derived($page.params);
  let workspace = $state<any>(null);
  let projects = $state<any[]>([]);
  let loading = $state(true);

  onMount(async () => {
    try {
      const [wsRes, projRes] = await Promise.all([
        fetch(`/api/v1/workspaces/${slug}`, { credentials: 'include' }),
        fetch(`/api/v1/projects?workspace_slug=${slug}`, { credentials: 'include' }),
      ]);
      if (!wsRes.ok) { goto('/workspaces'); return; }
      workspace = await wsRes.json();
      const projData = await projRes.json();
      projects = projData.projects ?? [];
    } finally {
      loading = false;
    }
  });
</script>

<svelte:head>
  <title>{workspace?.name ?? slug} — kloudsPanel</title>
</svelte:head>

{#if loading}
  <div class="empty-state"><div style="font-size:2rem;opacity:0.4;margin-bottom:1rem">⏳</div><p>Loading…</p></div>
{:else}
  <div class="page-header">
    <div>
      <p class="text-xs text-muted" style="margin-bottom:0.25rem">
        <a href="/workspaces">Workspaces</a> /
      </p>
      <h1 class="page-title">{workspace?.name ?? slug}</h1>
      <p class="page-subtitle">Manage projects and services in this workspace</p>
    </div>
    <button class="btn btn-primary" onclick={() => goto(`/workspaces/${slug}/projects/new`)}>
      <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
        <line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>
      </svg>
      New Project
    </button>
  </div>

  {#if projects.length === 0}
    <div class="empty-state">
      <div class="empty-state-icon">📦</div>
      <h3>No projects yet</h3>
      <p>Create your first project to start deploying services.</p>
      <button class="btn btn-primary mt-4">Create Project</button>
    </div>
  {:else}
    <div style="display:grid;grid-template-columns:repeat(auto-fill,minmax(320px,1fr));gap:1rem">
      {#each projects as proj}
        <a href="/projects/{proj.id}" style="text-decoration:none">
          <div class="card" style="cursor:pointer">
            <div class="card-header">
              <div>
                <h3 style="margin:0">{proj.name}</h3>
                <span class="text-xs text-muted">{proj.source_kind}</span>
              </div>
              <span class="badge badge-running">active</span>
            </div>
            <p class="text-sm text-muted" style="margin:0">{proj.description ?? 'No description'}</p>
          </div>
        </a>
      {/each}
    </div>
  {/if}
{/if}
