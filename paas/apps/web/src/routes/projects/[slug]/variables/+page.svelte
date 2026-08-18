<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { 
    Key, 
    Link2, 
    Save, 
    Check, 
    Loader2, 
    AlertTriangle, 
    Sparkles, 
    Layers, 
    Lock, 
    Eye, 
    EyeOff,
    FolderKanban,
    Plus
  } from 'lucide-svelte';

  const slug = $derived($page.params.slug);
  let project = $state<any>(null);
  let workspace = $state<any>(null);
  let envGroups = $state<any[]>([]);
  let globalVariables = $state<{ key: string; value: string }[]>([]);
  
  let loading = $state(true);
  let saving = $state(false);
  let savedMessage = $state('');
  let error = $state('');
  let visibleValues = $state<Record<string, boolean>>({});

  async function loadData() {
    try {
      const projRes = await fetch(`/api/v1/projects/${encodeURIComponent(slug)}`, { credentials: 'include' });
      if (projRes.ok) {
        project = await projRes.json();
        const wsSlug = project.workspace_slug || project.WorkspaceSlug || project.workspace_id || project.WorkspaceID;
        if (wsSlug) {
          const [wsRes, varsRes] = await Promise.all([
            fetch(`/api/v1/workspaces/${encodeURIComponent(wsSlug)}`, { credentials: 'include' }),
            fetch(`/api/v1/workspaces/${encodeURIComponent(wsSlug)}/variables`, { credentials: 'include' })
          ]);
          if (wsRes.ok) workspace = await wsRes.json();
          if (varsRes.ok) {
            const d = await varsRes.json();
            globalVariables = d.variables ?? [];
            envGroups = d.groups ?? [];
          }
        }
      }
    } catch (e: any) {
      error = 'Failed to load project environment variables and groups';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadData();
  });

  function toggleGroupLink(groupId: string) {
    const projId = project?.id || project?.ID || slug;
    envGroups = envGroups.map(g => {
      if (g.id === groupId) {
        const linked = g.linkedProjectIds || [];
        const isLinked = linked.includes(projId);
        const newLinked = isLinked ? linked.filter((id: string) => id !== projId) : [...linked, projId];
        return { ...g, linkedProjectIds: newLinked };
      }
      return g;
    });
  }

  async function saveLinkedGroups() {
    const wsSlug = project?.workspace_slug || workspace?.slug || slug;
    if (!wsSlug) return;
    saving = true;
    error = '';
    savedMessage = '';
    try {
      const res = await fetch(`/api/v1/workspaces/${encodeURIComponent(wsSlug)}/variables`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          variables: globalVariables,
          groups: envGroups
        })
      });
      if (res.ok) {
        savedMessage = 'Project environment groups updated successfully!';
        setTimeout(() => { savedMessage = ''; }, 4000);
      } else {
        const d = await res.json().catch(() => ({}));
        error = d.error || 'Failed to update linked groups';
      }
    } catch (e: any) {
      error = e.message;
    } finally {
      saving = false;
    }
  }

  const projId = $derived(project?.id || project?.ID || slug);
  const linkedGroups = $derived(envGroups.filter(g => (g.linkedProjectIds || []).includes(projId)));
  const unlinkedGroups = $derived(envGroups.filter(g => !(g.linkedProjectIds || []).includes(projId)));

  // Aggregate all effective variables inherited by this project from linked groups + workspace globals
  const effectiveVariables = $derived.by(() => {
    const map = new Map<string, { value: string; isSecret: boolean; source: string }>();
    for (const v of globalVariables) {
      if (v.key) map.set(v.key, { value: v.value, isSecret: false, source: 'Workspace Global' });
    }
    for (const g of linkedGroups) {
      for (const v of (g.variables || [])) {
        if (v.key) {
          map.set(v.key, { value: v.value, isSecret: !!v.isSecret, source: `Group: ${g.name}` });
        }
      }
    }
    return Array.from(map.entries()).map(([key, data]) => ({ key, ...data }));
  });
</script>

<svelte:head>
  <title>Environment Groups - {project?.name || slug} - kloudsPanel</title>
</svelte:head>

<div class="page-header" style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 1rem;">
  <div>
    <div class="page-breadcrumbs">
      <a href="/projects/{slug}">{project?.name || slug}</a>
      <span>/</span>
      <span>Environment Groups</span>
    </div>
    <h1 class="page-title" style="margin: 0; font-size: 1.5rem; font-weight: 600;">Environment Groups</h1>
    <p class="page-subtitle" style="margin-top: 4px;">Link workspace shared environment variable groups to inject credentials across all services in {project?.name || slug}.</p>
  </div>
  <button 
    class="btn btn-primary" 
    onclick={saveLinkedGroups}
    disabled={saving}
    style="display: flex; align-items: center; gap: 6px;"
  >
    {#if saving}
      <Loader2 size={15} class="animate-spin" /> Saving...
    {:else}
      <Save size={15} /> Save Linked Groups
    {/if}
  </button>
</div>

{#if savedMessage}
  <div style="background: rgba(34,197,94,0.1); border: 1px solid rgba(34,197,94,0.3); border-radius: var(--radius-md); padding: 0.75rem 1.25rem; margin-bottom: 1.25rem; color: #16a34a; font-weight: 600; font-size: 0.875rem; display: flex; align-items: center; gap: 8px;">
    <Check size={18} /> {savedMessage}
  </div>
{/if}

{#if error}
  <div style="background: var(--color-danger-subtle); border: 1px solid var(--color-danger); border-radius: var(--radius-md); padding: 0.75rem 1.25rem; margin-bottom: 1.25rem; color: var(--color-danger); font-size: 0.875rem; display: flex; align-items: center; gap: 8px;">
    <AlertTriangle size={18} /> {error}
  </div>
{/if}

{#if loading}
  <div class="empty-state">
    <Loader2 size={36} class="animate-spin text-muted" style="margin-bottom:1rem;" />
    <p>Loading project environment groups...</p>
  </div>
{:else}
  <!-- Section 1: Workspace Environment Groups Attachment -->
  <div class="card" style="padding: 1.25rem; margin-bottom: 1.5rem;">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; border-bottom: 1px solid var(--color-border); padding-bottom: 0.75rem;">
      <div>
        <h3 style="margin: 0; font-size: 1rem; font-weight: 600; color: var(--color-ink); display: flex; align-items: center; gap: 6px;">
          <Link2 size={16} style="color: var(--color-accent);" /> Workspace Groups Linked to this Project
        </h3>
        <p class="text-xs text-muted" style="margin: 2px 0 0 0;">
          Toggle groups on or off to attach them to services deployed in this project.
        </p>
      </div>
      {#if workspace?.slug}
        <a 
          href="/workspaces/{workspace.slug}/variables" 
          class="btn btn-secondary" 
          style="font-size: 0.75rem; padding: 4px 10px; min-height: 28px;"
        >
          Manage All Workspace Groups
        </a>
      {/if}
    </div>

    {#if envGroups.length === 0}
      <div style="text-align: center; padding: 2rem 1rem; color: var(--color-ink-muted);">
        <Key size={28} style="margin: 0 auto 0.5rem auto; opacity: 0.5;" />
        <p class="text-xs" style="margin: 0 0 0.75rem 0;">No Environment Groups created in this workspace yet.</p>
        {#if workspace?.slug}
          <a href="/workspaces/{workspace.slug}/variables" class="btn btn-secondary" style="font-size: 0.75rem;">
            Create First Env Group
          </a>
        {/if}
      </div>
    {:else}
      <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1rem;">
        {#each envGroups as group}
          {@const isLinked = (group.linkedProjectIds || []).includes(projId)}
          <div 
            class="card"
            style="
              padding: 1rem; 
              border: 1.5px solid {isLinked ? 'var(--color-accent)' : 'var(--color-border)'}; 
              background: {isLinked ? 'var(--color-surface-subtle)' : 'var(--color-surface)'};
              border-radius: var(--radius-md);
              display: flex;
              flex-direction: column;
              justify-content: space-between;
              gap: 0.75rem;
            "
          >
            <div>
              <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 4px;">
                <span style="font-weight: 600; font-size: 0.9rem; color: var(--color-ink);">{group.name}</span>
                {#if isLinked}
                  <span class="badge badge-running" style="font-size: 0.68rem; display: flex; align-items: center; gap: 3px;">
                    <Check size={11} /> Linked
                  </span>
                {:else}
                  <span class="badge" style="background: var(--color-surface-subtle); color: var(--color-ink-muted); font-size: 0.68rem;">
                    Not Linked
                  </span>
                {/if}
              </div>
              <p class="text-xs text-muted" style="margin: 0 0 6px 0; min-height: 18px;">
                {group.description || 'Shared environment credentials'}
              </p>
              <div class="text-xs font-mono" style="color: var(--color-accent); font-size: 0.72rem;">
                {(group.variables || []).length} variable{(group.variables || []).length === 1 ? '' : 's'}
              </div>
            </div>

            <button 
              type="button" 
              class="btn {isLinked ? 'btn-secondary' : 'btn-primary'}"
              style="width: 100%; font-size: 0.75rem; padding: 4px 10px; min-height: 28px;"
              onclick={() => toggleGroupLink(group.id)}
            >
              {isLinked ? 'Detach from Project' : 'Link to this Project'}
            </button>
          </div>
        {/each}
      </div>
    {/if}
  </div>

  <!-- Section 2: Effective Inherited Variables Preview -->
  <div class="card" style="padding: 1.25rem;">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; border-bottom: 1px solid var(--color-border); padding-bottom: 0.75rem;">
      <div>
        <h3 style="margin: 0; font-size: 1rem; font-weight: 600; color: var(--color-ink); display: flex; align-items: center; gap: 6px;">
          <Key size={16} style="color: #38bdf8;" /> Effective Variables for Services ({effectiveVariables.length})
        </h3>
        <p class="text-xs text-muted" style="margin: 2px 0 0 0;">
          All runtime environment variables automatically injected into every service build and deployment in {project?.name || slug}.
        </p>
      </div>
    </div>

    {#if effectiveVariables.length === 0}
      <div style="text-align: center; padding: 2rem; color: var(--color-ink-muted); font-size: 0.8125rem;">
        No variables inherited. Link an Environment Group above to inject shared credentials.
      </div>
    {:else}
      <div class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th style="width: 35%;">Key</th>
              <th style="width: 40%;">Value</th>
              <th style="width: 25%;">Inherited From</th>
            </tr>
          </thead>
          <tbody>
            {#each effectiveVariables as item}
              <tr>
                <td>
                  <span class="font-mono text-xs" style="font-weight: 600; color: var(--color-ink);">
                    {item.key}
                  </span>
                </td>
                <td>
                  <div style="display: flex; align-items: center; gap: 8px;">
                    {#if item.isSecret && !visibleValues[item.key]}
                      <span class="font-mono text-xs text-muted">••••••••••••••••</span>
                    {:else}
                      <span class="font-mono text-xs" style="color: var(--color-ink); word-break: break-all;">
                        {item.value}
                      </span>
                    {/if}
                    {#if item.isSecret}
                      <button 
                        type="button" 
                        class="btn btn-secondary" 
                        style="padding: 2px 6px; min-height: 22px; border: none; color: var(--color-ink-muted);"
                        onclick={() => visibleValues[item.key] = !visibleValues[item.key]}
                        title={visibleValues[item.key] ? 'Hide secret' : 'Show secret'}
                      >
                        {#if visibleValues[item.key]}
                          <EyeOff size={13} />
                        {:else}
                          <Eye size={13} />
                        {/if}
                      </button>
                    {/if}
                  </div>
                </td>
                <td>
                  <span class="badge" style="background: rgba(56,189,248,0.12); color: #38bdf8; font-size: 0.72rem;">
                    {item.source}
                  </span>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>
{/if}
