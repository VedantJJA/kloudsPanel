<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Settings, Save, Trash2, Loader2, Check, AlertTriangle, FolderKanban, ShieldAlert } from 'lucide-svelte';
  import DeleteConfirmationModal from '$lib/components/modals/DeleteConfirmationModal.svelte';

  const slug = $derived($page.params.slug);
  let project = $state<any>(null);
  let services = $state<any[]>([]);
  let databases = $state<any[]>([]);
  let name = $state('');
  let description = $state('');

  let loading = $state(true);
  let saving = $state(false);
  let savedMessage = $state('');
  let error = $state('');

  // Delete modal state
  let showDeleteModal = $state(false);
  let deleting = $state(false);

  async function loadData() {
    try {
      const targetSlug = slug || '';
      const [projRes, svcRes, dbRes] = await Promise.all([
        fetch(`/api/v1/projects/${encodeURIComponent(targetSlug)}`, { credentials: 'include' }),
        fetch(`/api/v1/services?projectId=${encodeURIComponent(targetSlug)}`, { credentials: 'include' }),
        fetch(`/api/v1/databases?projectId=${encodeURIComponent(targetSlug)}`, { credentials: 'include' })
      ]);
      if (projRes.ok) {
        project = await projRes.json();
        name = project.name || project.Name || '';
        description = project.description || project.Description || '';
      }
      if (svcRes.ok) {
        services = (await svcRes.json()).services ?? [];
      }
      if (dbRes.ok) {
        databases = (await dbRes.json()).databases ?? [];
      }
    } catch (e: any) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadData();
  });

  async function handleUpdateProject(e: Event) {
    e.preventDefault();
    if (!name.trim() || saving) return;
    saving = true;
    error = '';
    savedMessage = '';
    try {
      const projId = project?.id || project?.ID || slug;
      const res = await fetch(`/api/v1/projects/${encodeURIComponent(projId)}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          name: name.trim(),
          description: description.trim()
        })
      });
      if (res.ok) {
        project = await res.json();
        savedMessage = 'Project settings saved successfully!';
        setTimeout(() => { savedMessage = ''; }, 4000);
      } else {
        const d = await res.json().catch(() => ({}));
        error = d.error || 'Failed to save project settings';
      }
    } catch (e: any) {
      error = e.message;
    } finally {
      saving = false;
    }
  }

  async function executeDeleteProject() {
    const id = project?.id || project?.ID || slug;
    deleting = true;
    try {
      const res = await fetch(`/api/v1/projects/${encodeURIComponent(id)}`, {
        method: 'DELETE',
        credentials: 'include'
      });
      if (res.ok) {
        const wsSlug = project?.workspace_slug || project?.WorkspaceSlug || 'personal';
        goto(`/workspaces/${wsSlug}`);
      } else {
        const d = await res.json().catch(() => ({}));
        alert('Failed to delete project: ' + (d.error || d.detail || res.statusText));
      }
    } catch (e: any) {
      alert('Error deleting project: ' + e.message);
    } finally {
      deleting = false;
      showDeleteModal = false;
    }
  }

  const activeResourceCount = $derived(services.length + databases.length);
</script>

<svelte:head>
  <title>Settings - {project?.name || slug} - kloudsPanel</title>
</svelte:head>

<div class="page-header">
  <div>
    <div class="page-breadcrumbs">
      {#if project?.workspace_slug || project?.WorkspaceSlug || project?.workspace_id || project?.WorkspaceID}
        <a href="/workspaces/{project.workspace_slug || project.WorkspaceSlug || project.workspace_id || project.WorkspaceID}">
          {project.workspace_name || project.WorkspaceName || 'Workspace'}
        </a>
        <span>/</span>
      {/if}
      <a href="/projects/{slug}">{project?.name || slug}</a>
      <span>/</span>
      <span>Settings</span>
    </div>
    <h1 class="page-title" style="margin: 0; font-size: 1.5rem; font-weight: 600;">Project Settings</h1>
    <p class="page-subtitle" style="margin-top: 4px;">Manage project general configuration, metadata, and lifecycle options.</p>
  </div>
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
    <p>Loading project settings...</p>
  </div>
{:else}
  <!-- General Project Info Card -->
  <div class="card" style="padding: 1.5rem; margin-bottom: 1.5rem; max-width: 680px;">
    <h3 style="margin: 0 0 1rem 0; font-size: 1.05rem; font-weight: 600; color: var(--color-ink);">General Configuration</h3>
    <form onsubmit={handleUpdateProject}>
      <div class="form-group">
        <label for="proj-name" class="form-label">Project Name</label>
        <input 
          id="proj-name" 
          type="text" 
          class="form-input" 
          bind:value={name} 
          required 
        />
      </div>

      <div class="form-group">
        <label for="proj-slug" class="form-label">Project Slug</label>
        <input 
          id="proj-slug" 
          type="text" 
          class="form-input font-mono" 
          value={project?.slug || slug} 
          disabled 
        />
        <p class="text-xs text-muted" style="margin-top: 4px;">Slugs are permanently assigned on creation and used in DNS subdomains.</p>
      </div>

      <div class="form-group">
        <label for="proj-desc" class="form-label">Description</label>
        <textarea 
          id="proj-desc" 
          class="form-input" 
          rows="3" 
          bind:value={description} 
          placeholder="Brief description of this project's purpose"
        ></textarea>
      </div>

      <div style="display: flex; justify-content: flex-end; margin-top: 1.25rem;">
        <button 
          type="submit" 
          class="btn btn-primary" 
          disabled={saving}
          style="display: flex; align-items: center; gap: 6px;"
        >
          {#if saving}
            <Loader2 size={15} class="animate-spin" /> Saving...
          {:else}
            <Save size={15} /> Save Changes
          {/if}
        </button>
      </div>
    </form>
  </div>

  <!-- Danger Zone -->
  <div class="card" style="padding: 1.5rem; max-width: 680px; border-color: rgba(239, 68, 68, 0.35);">
    <div style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 1rem;">
      <div>
        <h3 style="margin: 0 0 4px 0; font-size: 1.05rem; font-weight: 600; color: var(--color-danger); display: flex; align-items: center; gap: 6px;">
          <ShieldAlert size={18} /> Delete Project
        </h3>
        <p class="text-xs text-muted" style="margin: 0; max-width: 440px;">
          Permanently destroy this project, stopping all {services.length} running service{services.length === 1 ? '' : 's'} and {databases.length} database{databases.length === 1 ? '' : 's'}, wiping their containers, storage volumes, and reverse proxy routing.
        </p>
      </div>
      <button 
        type="button" 
        class="btn btn-danger" 
        onclick={() => showDeleteModal = true}
        style="display: flex; align-items: center; gap: 6px;"
      >
        <Trash2 size={15} /> Delete Project
      </button>
    </div>
  </div>
{/if}

<!-- Two-Step Delete Confirmation Modal -->
<DeleteConfirmationModal
  show={showDeleteModal}
  title="Delete Project"
  entityType="project"
  entityName={project?.name || project?.Name || slug || 'Project'}
  servicesCount={services.length}
  databasesCount={databases.length}
  loading={deleting}
  onConfirm={executeDeleteProject}
  onCancel={() => showDeleteModal = false}
/>
