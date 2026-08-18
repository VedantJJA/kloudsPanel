<script lang="ts">
  import { AlertTriangle, Trash2, X, Loader2, ShieldAlert } from 'lucide-svelte';

  interface Props {
    show: boolean;
    title: string;
    entityName: string;
    entityType: 'workspace' | 'project';
    projectsCount?: number;
    servicesCount?: number;
    databasesCount?: number;
    loading?: boolean;
    onConfirm: () => Promise<void> | void;
    onCancel: () => void;
  }

  let {
    show = false,
    title,
    entityName,
    entityType,
    projectsCount = 0,
    servicesCount = 0,
    databasesCount = 0,
    loading = false,
    onConfirm,
    onCancel
  }: Props = $props();

  let inputPhrase = $state('');

  const requiredPhrase = $derived(`sudo delete ${entityName}`.trim());
  const hasChildResources = $derived(
    projectsCount > 0 || servicesCount > 0 || databasesCount > 0
  );

  const isConfirmed = $derived.by(() => {
    if (!hasChildResources) return true;
    return inputPhrase.trim().toLowerCase() === requiredPhrase.toLowerCase();
  });

  $effect(() => {
    if (show) {
      inputPhrase = '';
    }
  });

  function handleSubmit(e: Event) {
    e.preventDefault();
    if (!isConfirmed || loading) return;
    onConfirm();
  }
</script>

{#if show}
  <div 
    class="modal-backdrop"
    style="position: fixed; inset: 0; background: rgba(0,0,0,0.7); backdrop-filter: blur(4px); z-index: 1100; display: flex; align-items: center; justify-content: center; padding: 1rem;"
    onclick={(e) => { if (e.target === e.currentTarget && !loading) onCancel(); }}
    role="presentation"
  >
    <div 
      class="modal-card"
      style="width: 100%; max-width: 520px; background: var(--color-surface); border: 1px solid rgba(239, 68, 68, 0.4); border-radius: var(--radius-lg); padding: 1.5rem; box-shadow: 0 25px 35px -5px rgba(0,0,0,0.4);"
    >
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; border-bottom: 1px solid var(--color-border); padding-bottom: 0.75rem;">
        <div style="display: flex; align-items: center; gap: 8px;">
          <div style="width: 32px; height: 32px; border-radius: var(--radius-sm); background: rgba(239,68,68,0.12); color: var(--color-danger); display: flex; align-items: center; justify-content: center; flex-shrink: 0;">
            <AlertTriangle size={18} />
          </div>
          <h3 style="margin: 0; font-size: 1.1rem; color: var(--color-ink); font-weight: 600;">{title}</h3>
        </div>
        <button 
          type="button" 
          class="btn btn-secondary" 
          style="padding: 4px; min-height: 28px; width: 28px; height: 28px; border: none;"
          onclick={onCancel}
          disabled={loading}
        >
          <X size={16} />
        </button>
      </div>

      <form onsubmit={handleSubmit}>
        <!-- Warning Message -->
        <div style="background: var(--color-danger-subtle); border: 1px solid rgba(239,68,68,0.25); border-radius: var(--radius-md); padding: 1rem; margin-bottom: 1.25rem; font-size: 0.8125rem; color: var(--color-ink);">
          <div style="font-weight: 600; color: var(--color-danger); margin-bottom: 4px; display: flex; align-items: center; gap: 6px;">
            <ShieldAlert size={15} /> Recursive Destruction Warning
          </div>
          {#if hasChildResources}
            <p style="margin: 0 0 6px 0; line-height: 1.45;">
              This {entityType} currently contains:
            </p>
            <ul style="margin: 0 0 8px 1.25rem; padding: 0; line-height: 1.45;">
              {#if projectsCount > 0}
                <li><strong>{projectsCount} Project{projectsCount === 1 ? '' : 's'}</strong></li>
              {/if}
              {#if servicesCount > 0}
                <li><strong>{servicesCount} Deployed Service{servicesCount === 1 ? '' : 's'}</strong></li>
              {/if}
              {#if databasesCount > 0}
                <li><strong>{databasesCount} Database Instance{databasesCount === 1 ? '' : 's'}</strong></li>
              {/if}
            </ul>
            <p style="margin: 0; font-size: 0.75rem; color: var(--color-danger);">
              Deleting this {entityType} will permanently terminate all associated Docker containers, remove all database persistent volumes, and purge all reverse proxy routing records immediately.
            </p>
          {:else}
            <p style="margin: 0; line-height: 1.45;">
              Are you sure you want to permanently delete this {entityType}? This action cannot be reversed.
            </p>
          {/if}
        </div>

        {#if hasChildResources}
          <div class="form-group" style="margin-bottom: 1.25rem;">
            <label for="sudo-confirm-input" class="form-label" style="font-size: 0.78125rem; text-transform: none; font-weight: 500; color: var(--color-ink);">
              To confirm recursive deletion, please type <span class="font-mono" style="color: var(--color-danger); font-weight: 600;">{requiredPhrase}</span> below:
            </label>
            <input
              id="sudo-confirm-input"
              type="text"
              class="form-input font-mono"
              placeholder={requiredPhrase}
              bind:value={inputPhrase}
              autocomplete="off"
              required
            />
          </div>
        {/if}

        <div style="display: flex; justify-content: flex-end; gap: 0.5rem; border-top: 1px solid var(--color-border); padding-top: 1rem;">
          <button 
            type="button" 
            class="btn btn-secondary" 
            onclick={onCancel}
            disabled={loading}
          >
            Cancel
          </button>
          <button 
            type="submit" 
            class="btn btn-danger"
            disabled={!isConfirmed || loading}
            style="display: flex; align-items: center; gap: 6px;"
          >
            {#if loading}
              <Loader2 size={14} class="animate-spin" /> Deleting Recursively...
            {:else}
              <Trash2 size={14} /> Permanently Delete {entityType === 'workspace' ? 'Workspace' : 'Project'}
            {/if}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}
