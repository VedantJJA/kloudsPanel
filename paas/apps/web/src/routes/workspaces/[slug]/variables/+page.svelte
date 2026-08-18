<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { 
    Key, 
    Plus, 
    Trash2, 
    Save, 
    Eye, 
    EyeOff, 
    Loader2, 
    Info, 
    FolderKanban, 
    Link2, 
    Edit2, 
    Check, 
    X,
    Sparkles,
    Lock
  } from 'lucide-svelte';

  const slug = $derived($page.params.slug);
  let workspace = $state<any>(null);
  let projects = $state<any[]>([]);
  let envGroups = $state<any[]>([]);
  let globalVariables = $state<{ key: string; value: string }[]>([]);
  
  let loading = $state(true);
  let saving = $state(false);
  let savedMessage = $state('');
  let error = $state('');

  // Active modal/editor state
  let showEditModal = $state(false);
  let isNewGroup = $state(true);
  let editingGroupId = $state('');
  let editingGroupName = $state('');
  let editingGroupDescription = $state('');
  let editingGroupLinkedProjects = $state<string[]>([]);
  let editingGroupVariables = $state<{ key: string; value: string; isSecret?: boolean }[]>([]);
  let visibleModalValues = $state<Record<number, boolean>>({});

  async function loadData() {
    try {
      const [wsRes, varsRes] = await Promise.all([
        fetch(`/api/v1/workspaces/${slug}`, { credentials: 'include' }),
        fetch(`/api/v1/workspaces/${slug}/variables`, { credentials: 'include' })
      ]);

      if (wsRes.ok) {
        workspace = await wsRes.json();
        const wsId = workspace.id || workspace.ID;
        if (wsId) {
          const projRes = await fetch(`/api/v1/projects?workspaceId=${wsId}`, { credentials: 'include' });
          if (projRes.ok) {
            const projData = await projRes.json();
            projects = projData.projects ?? [];
          }
        }
      }

      if (varsRes.ok) {
        const d = await varsRes.json();
        globalVariables = d.variables ?? [];
        envGroups = d.groups ?? [];
      }
    } catch (e: any) {
      error = 'Failed to load environment variables';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadData();
  });

  function openCreateGroupModal() {
    isNewGroup = true;
    editingGroupId = '';
    editingGroupName = '';
    editingGroupDescription = '';
    editingGroupLinkedProjects = [];
    editingGroupVariables = [{ key: '', value: '', isSecret: false }];
    visibleModalValues = {};
    showEditModal = true;
  }

  function openEditGroupModal(group: any) {
    isNewGroup = false;
    editingGroupId = group.id;
    editingGroupName = group.name;
    editingGroupDescription = group.description || '';
    editingGroupLinkedProjects = [...(group.linkedProjectIds || [])];
    editingGroupVariables = (group.variables || []).map((v: any) => ({
      key: v.key || '',
      value: v.value || '',
      isSecret: !!v.isSecret
    }));
    if (editingGroupVariables.length === 0) {
      editingGroupVariables = [{ key: '', value: '', isSecret: false }];
    }
    visibleModalValues = {};
    showEditModal = true;
  }

  function addModalVariable() {
    editingGroupVariables = [...editingGroupVariables, { key: '', value: '', isSecret: false }];
  }

  function removeModalVariable(index: number) {
    editingGroupVariables = editingGroupVariables.filter((_, i) => i !== index);
  }

  function toggleProjectLink(projId: string) {
    if (editingGroupLinkedProjects.includes(projId)) {
      editingGroupLinkedProjects = editingGroupLinkedProjects.filter(p => p !== projId);
    } else {
      editingGroupLinkedProjects = [...editingGroupLinkedProjects, projId];
    }
  }

  function toggleAllProjects() {
    if (editingGroupLinkedProjects.length === projects.length) {
      editingGroupLinkedProjects = [];
    } else {
      editingGroupLinkedProjects = projects.map(p => p.id || p.ID);
    }
  }

  async function handleSaveGroup(e: Event) {
    e.preventDefault();
    if (!editingGroupName.trim()) return;
    saving = true;
    error = '';

    const validVars = editingGroupVariables
      .map(v => ({ key: v.key.trim().toUpperCase(), value: v.value.trim(), isSecret: v.isSecret }))
      .filter(v => v.key !== '');

    try {
      const res = await fetch(`/api/v1/workspaces/${slug}/env-groups`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          id: editingGroupId || undefined,
          name: editingGroupName.trim(),
          description: editingGroupDescription.trim(),
          linkedProjectIds: editingGroupLinkedProjects,
          variables: validVars
        })
      });

      if (res.ok) {
        showEditModal = false;
        savedMessage = `Environment variable set "${editingGroupName}" saved successfully.`;
        setTimeout(() => savedMessage = '', 4000);
        await loadData();
      } else {
        const d = await res.json().catch(() => ({}));
        error = d.error || 'Failed to save environment variable set';
      }
    } catch (e: any) {
      error = 'Network error: ' + e.message;
    } finally {
      saving = false;
    }
  }

  async function deleteGroup(groupId: string, groupName: string) {
    if (!confirm(`Are you sure you want to delete variable set "${groupName}"? Projects linked to this set will lose access to its environment variables on their next deployment.`)) return;
    try {
      const res = await fetch(`/api/v1/workspaces/${slug}/env-groups/${groupId}`, {
        method: 'DELETE',
        credentials: 'include'
      });
      if (res.ok) {
        savedMessage = `Variable set "${groupName}" deleted.`;
        setTimeout(() => savedMessage = '', 3000);
        await loadData();
      }
    } catch {}
  }
</script>

<svelte:head>
  <title>Shared Variables - {workspace?.name || slug} - kloudsPanel</title>
</svelte:head>

<div class="page-header">
  <div>
    <div class="page-breadcrumbs">
      <a href="/workspaces/{slug}">{workspace?.name || slug}</a>
      <span>/</span>
      <span>Shared Environment Variables</span>
    </div>
    <h1 class="page-title">Shared Environment Variable Sets</h1>
    <p class="page-subtitle">Group shared variables and link them across projects in this workspace</p>
  </div>
  <button class="btn btn-primary" onclick={openCreateGroupModal}>
    <Plus size={16} /> New Variable Set
  </button>
</div>

{#if savedMessage}
  <div style="background: rgba(16,185,129,0.1); border: 1px solid rgba(16,185,129,0.3); color: #10b981; border-radius: var(--radius-md); padding: 0.75rem 1rem; font-size: 0.8125rem; margin-bottom: 1.25rem; display: flex; align-items: center; gap: 8px;">
    <Check size={16} /> {savedMessage}
  </div>
{/if}

{#if error}
  <div style="background: rgba(239,68,68,0.1); border: 1px solid rgba(239,68,68,0.3); color: #ef4444; border-radius: var(--radius-md); padding: 0.75rem 1rem; font-size: 0.8125rem; margin-bottom: 1.25rem;">
    {error}
  </div>
{/if}



{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom: 1rem"><Loader2 size={36} /></div>
    <p>Loading environment variable sets...</p>
  </div>
{:else if envGroups.length === 0}
  <div class="empty-state">
    <div class="empty-state-icon"><Key size={40} /></div>
    <h3>No Environment Variable Sets</h3>
    <p>Create a shared set of environment variables (e.g. "Staging Keys", "Database Secrets") and link them to your projects.</p>
    <button class="btn btn-primary" style="margin-top: 1rem;" onclick={openCreateGroupModal}>
      <Plus size={15} /> Create First Variable Set
    </button>
  </div>
{:else}
  <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 380px), 1fr)); gap: 1.25rem;">
    {#each envGroups as group}
      {@const varCount = (group.variables || []).length}
      {@const linkedIds = group.linkedProjectIds || []}
      {@const linkedProjects = projects.filter(p => linkedIds.includes(p.id || p.ID))}
      <div class="card" style="display: flex; flex-direction: column; justify-content: space-between; border: 1px solid var(--color-border); padding: 1.25rem;">
        <div>
          <div style="display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 0.5rem; gap: 0.5rem;">
            <div style="display: flex; align-items: center; gap: 8px;">
              <div style="width: 32px; height: 32px; border-radius: var(--radius-sm); background: rgba(0,166,166,0.12); color: var(--color-accent); display: flex; align-items: center; justify-content: center; flex-shrink: 0;">
                <Key size={16} />
              </div>
              <div>
                <h3 style="margin: 0; font-size: 1rem; color: var(--color-ink);">{group.name}</h3>
                {#if group.description}
                  <p class="text-xs text-muted" style="margin: 2px 0 0 0;">{group.description}</p>
                {/if}
              </div>
            </div>
            <span class="badge" style="background: rgba(255,255,255,0.06); font-size: 0.7rem;">
              {varCount} Variable{varCount === 1 ? '' : 's'}
            </span>
          </div>

          <!-- Variable Keys Preview -->
          <div style="margin: 1rem 0; display: flex; flex-wrap: wrap; gap: 4px; max-height: 80px; overflow: hidden;">
            {#if varCount === 0}
              <span class="text-xs text-muted">No variables added yet</span>
            {:else}
              {#each (group.variables || []).slice(0, 6) as v}
                <span class="badge font-mono" style="background: rgba(0,0,0,0.03); border: 1px solid var(--color-border); font-size: 0.7rem;">
                  {v.key}
                </span>
              {/each}
              {#if varCount > 6}
                <span class="badge" style="background: transparent; font-size: 0.7rem; color: var(--color-ink-muted);">
                  +{varCount - 6} more
                </span>
              {/if}
            {/if}
          </div>

          <!-- Linked Projects -->
          <div style="border-top: 1px solid var(--color-border); padding-top: 0.75rem; margin-top: 0.75rem;">
            <div style="font-size: 0.7rem; font-weight: 700; color: var(--color-ink-muted); text-transform: uppercase; letter-spacing: 0.04em; margin-bottom: 0.4rem; display: flex; align-items: center; gap: 4px;">
              <Link2 size={12} /> Linked Projects ({linkedProjects.length})
            </div>
            <div style="display: flex; flex-wrap: wrap; gap: 4px;">
              {#if linkedProjects.length === 0}
                <span class="text-xs text-muted">Not linked to any projects</span>
              {:else}
                {#each linkedProjects as proj}
                  <span class="badge" style="background: rgba(59,130,246,0.1); color: #3b82f6; font-size: 0.7rem;">
                    {proj.name || proj.Name}
                  </span>
                {/each}
              {/if}
            </div>
          </div>
        </div>

        <div style="display: flex; justify-content: flex-end; gap: 0.5rem; border-top: 1px solid var(--color-border); padding-top: 0.85rem; margin-top: 1rem;">
          <button 
            type="button" 
            class="btn btn-secondary" 
            style="padding: 4px 10px; font-size: 0.75rem; display: flex; align-items: center; gap: 4px;"
            onclick={() => openEditGroupModal(group)}
          >
            <Edit2 size={12} /> Edit & Link Projects
          </button>
          <button 
            type="button" 
            class="btn btn-secondary" 
            style="padding: 4px 8px; color: #ef4444;"
            onclick={() => deleteGroup(group.id, group.name)}
            title="Delete Variable Set"
          >
            <Trash2 size={13} />
          </button>
        </div>
      </div>
    {/each}
  </div>
{/if}

<!-- Modal: Create / Edit Environment Variable Set -->
{#if showEditModal}
  <div 
    class="modal-backdrop"
    style="position: fixed; inset: 0; background: rgba(0,0,0,0.65); backdrop-filter: blur(4px); z-index: 1000; display: flex; align-items: center; justify-content: center; padding: 1rem;"
    onclick={(e) => { if (e.target === e.currentTarget) showEditModal = false; }}
    role="presentation"
  >
    <div 
      class="modal-card" 
      style="width: 100%; max-width: 680px; max-height: 90vh; display: flex; flex-direction: column; background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius-lg); padding: 1.5rem; box-shadow: 0 20px 25px -5px rgba(0,0,0,0.3);"
    >
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; border-bottom: 1px solid var(--color-border); padding-bottom: 0.75rem;">
        <div style="display: flex; align-items: center; gap: 8px;">
          <Key size={18} style="color: var(--color-accent);" />
          <h3 style="margin: 0; font-size: 1.15rem; color: var(--color-ink);">
            {isNewGroup ? 'Create Environment Variable Set' : `Edit "${editingGroupName}"`}
          </h3>
        </div>
        <button 
          type="button" 
          class="btn btn-secondary" 
          style="padding: 4px; min-height: 28px; width: 28px; height: 28px; border: none;"
          onclick={() => showEditModal = false}
        >
          <X size={16} />
        </button>
      </div>

      <form onsubmit={handleSaveGroup} style="overflow-y: auto; flex: 1; padding-right: 4px;">
        <!-- Set Name & Description -->
        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; margin-bottom: 1.25rem;">
          <div class="form-group" style="margin: 0;">
            <label for="group-name" class="form-label">Set Name</label>
            <input
              id="group-name"
              type="text"
              class="form-input"
              placeholder="e.g. Production Secrets, Global API Keys"
              bind:value={editingGroupName}
              required
            />
          </div>
          <div class="form-group" style="margin: 0;">
            <label for="group-desc" class="form-label">Description (Optional)</label>
            <input
              id="group-desc"
              type="text"
              class="form-input"
              placeholder="e.g. Injected into all production services"
              bind:value={editingGroupDescription}
            />
          </div>
        </div>

        <!-- Project Linking Selector -->
        <div style="background: rgba(0,0,0,0.02); border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: 1rem; margin-bottom: 1.25rem;">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem;">
            <div style="font-weight: 700; font-size: 0.8125rem; color: var(--color-ink); display: flex; align-items: center; gap: 6px;">
              <Link2 size={14} style="color: var(--color-accent);" /> Link to Projects in this Workspace:
            </div>
            {#if projects.length > 0}
              <button 
                type="button" 
                class="btn btn-secondary" 
                style="padding: 2px 8px; font-size: 0.7rem; min-height: 22px;"
                onclick={toggleAllProjects}
              >
                {editingGroupLinkedProjects.length === projects.length ? 'Deselect All' : 'Select All'}
              </button>
            {/if}
          </div>

          {#if projects.length === 0}
            <p class="text-xs text-muted" style="margin: 0;">No projects exist in this workspace yet. Create a project first to link variables.</p>
          {:else}
            <div style="display: flex; flex-wrap: wrap; gap: 0.5rem; margin-top: 0.5rem;">
              {#each projects as p}
                {@const pid = p.id || p.ID}
                {@const isLinked = editingGroupLinkedProjects.includes(pid)}
                <button
                  type="button"
                  class="badge"
                  style="
                    padding: 6px 12px; 
                    font-size: 0.78125rem; 
                    cursor: pointer; 
                    display: flex; 
                    align-items: center; 
                    gap: 6px; 
                    border-radius: var(--radius-sm);
                    background: {isLinked ? 'rgba(0,166,166,0.15)' : 'var(--color-surface)'};
                    color: {isLinked ? 'var(--color-accent)' : 'var(--color-ink)'};
                    border: 1px solid {isLinked ? 'var(--color-accent)' : 'var(--color-border)'};
                  "
                  onclick={() => toggleProjectLink(pid)}
                >
                  {#if isLinked}
                    <Check size={13} style="color: var(--color-accent);" />
                  {:else}
                    <FolderKanban size={13} style="color: var(--color-ink-muted);" />
                  {/if}
                  {p.name || p.Name}
                </button>
              {/each}
            </div>
          {/if}
        </div>

        <!-- Variables List -->
        <div style="margin-bottom: 1.25rem;">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.5rem;">
            <div style="font-weight: 700; font-size: 0.8125rem; color: var(--color-ink);">
              Variables ({editingGroupVariables.length})
            </div>
            <button 
              type="button" 
              class="btn btn-secondary" 
              style="padding: 2px 8px; font-size: 0.72rem; min-height: 24px; display: flex; align-items: center; gap: 4px;"
              onclick={addModalVariable}
            >
              <Plus size={12} /> Add Variable
            </button>
          </div>

          <div style="display: flex; flex-direction: column; gap: 0.5rem;">
            {#each editingGroupVariables as v, idx}
              <div style="display: grid; grid-template-columns: 1fr 2fr auto auto; gap: 0.5rem; align-items: center; background: var(--color-surface); padding: 6px 8px; border-radius: var(--radius-sm); border: 1px solid var(--color-border);">
                <input
                  type="text"
                  class="form-input font-mono text-xs"
                  placeholder="KEY_NAME"
                  bind:value={v.key}
                  style="text-transform: uppercase;"
                />
                <input
                  type={visibleModalValues[idx] ? 'text' : 'password'}
                  class="form-input font-mono text-xs"
                  placeholder="value"
                  bind:value={v.value}
                />
                <button
                  type="button"
                  class="btn btn-secondary"
                  style="padding: 4px 6px; min-height: 26px;"
                  onclick={() => visibleModalValues[idx] = !visibleModalValues[idx]}
                  title={visibleModalValues[idx] ? 'Hide value' : 'Show value'}
                >
                  {#if visibleModalValues[idx]}<EyeOff size={13} />{:else}<Eye size={13} />{/if}
                </button>
                <button
                  type="button"
                  class="btn btn-secondary"
                  style="padding: 4px 6px; min-height: 26px; color: #ef4444;"
                  onclick={() => removeModalVariable(idx)}
                  title="Remove variable"
                >
                  <Trash2 size={13} />
                </button>
              </div>
            {/each}
          </div>
        </div>

        <div style="display: flex; justify-content: flex-end; gap: 0.5rem; border-top: 1px solid var(--color-border); padding-top: 1rem;">
          <button 
            type="button" 
            class="btn btn-secondary" 
            onclick={() => showEditModal = false}
          >
            Cancel
          </button>
          <button 
            type="submit" 
            class="btn btn-primary"
            disabled={saving || !editingGroupName.trim()}
          >
            {saving ? 'Saving...' : 'Save Variable Set'}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}
