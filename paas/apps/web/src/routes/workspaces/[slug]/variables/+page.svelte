<script lang="ts">
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { Key, Plus, Trash2, Save, Eye, EyeOff, Loader2, Info } from 'lucide-svelte';

  const { slug } = $derived($page.params);
  let workspace = $state<any>(null);
  let variables = $state<{ key: string; value: string }[]>([]);
  let visibleValues = $state<Record<number, boolean>>({});
  let loading = $state(true);
  let saving = $state(false);
  let saved = $state(false);
  let error = $state('');

  async function loadData() {
    try {
      const [wsRes, varsRes] = await Promise.all([
        fetch(`/api/v1/workspaces/${slug}`, { credentials: 'include' }),
        fetch(`/api/v1/workspaces/${slug}/variables`, { credentials: 'include' })
      ]);
      if (wsRes.ok) {
        workspace = await wsRes.json();
      }
      if (varsRes.ok) {
        const d = await varsRes.json();
        variables = d.variables ?? [];
      }
    } catch (e: any) {
      error = 'Failed to load variables';
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadData();
  });

  function addVariable() {
    variables = [...variables, { key: '', value: '' }];
  }

  function removeVariable(index: number) {
    variables = variables.filter((_, i) => i !== index);
  }

  function toggleVisibility(index: number) {
    visibleValues[index] = !visibleValues[index];
  }

  async function saveVariables(e: Event) {
    e.preventDefault();
    saving = true;
    error = '';
    saved = false;

    // Filter empty keys
    const validVars = variables
      .map(v => ({ key: v.key.trim().toUpperCase(), value: v.value.trim() }))
      .filter(v => v.key !== '');

    try {
      const res = await fetch(`/api/v1/workspaces/${slug}/variables`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ variables: validVars })
      });

      if (res.ok) {
        saved = true;
        variables = validVars;
        setTimeout(() => saved = false, 3000);
      } else {
        const d = await res.json().catch(() => ({}));
        error = d.error || 'Failed to save environment variables';
      }
    } catch (e: any) {
      error = 'Network error: ' + e.message;
    } finally {
      saving = false;
    }
  }
</script>

<svelte:head>
  <title>Shared Variables - {workspace?.name || slug} - kloudsPanel</title>
</svelte:head>

<div class="page-header" style="margin-bottom: 1.5rem;">
  <div>
    <div class="page-breadcrumbs">
      <a href="/workspaces">Workspaces</a> /
      <a href="/workspaces/{slug}">{workspace?.name || slug}</a> /
      <span>Shared Variables</span>
    </div>
    <h1 class="page-title">Shared Environment Variables</h1>
    <p class="page-subtitle">These variables are automatically inherited by all projects and services in this workspace</p>
  </div>
</div>

<div class="card" style="padding: 1.5rem; background: var(--color-surface); border: 1px solid var(--color-border); margin-bottom: 2rem;">
  <div style="display: flex; align-items: center; gap: 0.5rem; background: rgba(0,166,166,0.08); border: 1px solid rgba(0,166,166,0.25); border-radius: var(--radius-md); padding: 0.85rem 1rem; margin-bottom: 1.5rem; font-size: 0.8125rem; color: var(--color-ink);">
    <Info size={18} style="color: var(--color-accent); flex-shrink: 0;" />
    <div>
      Shared variables provide a single source of truth for global API keys, database credentials, or environment flags across all services in this workspace. Service-specific variables with the same name will override workspace variables.
    </div>
  </div>

  {#if saved}
    <div style="background:#d1fae5;border:1px solid #6ee7b7;color:#065f46;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem">
      Shared environment variables updated successfully. Future deployments in this workspace will inherit these values.
    </div>
  {/if}

  {#if error}
    <div style="background:#fee2e2;border:1px solid #fca5a5;color:#991b1b;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem">
      {error}
    </div>
  {/if}

  {#if loading}
    <div style="text-align: center; padding: 2rem;">
      <Loader2 size={32} class="animate-spin text-muted" />
    </div>
  {:else}
    <form onsubmit={saveVariables}>
      <div style="display: flex; flex-direction: column; gap: 0.75rem; margin-bottom: 1.5rem;">
        {#if variables.length === 0}
          <div style="text-align: center; padding: 2rem; border: 1px dashed var(--color-border); border-radius: var(--radius-md); color: var(--color-ink-secondary);">
            No shared environment variables configured yet.
          </div>
        {:else}
          {#each variables as v, i}
            <div style="display: grid; grid-template-columns: 1fr 2fr auto auto; gap: 0.75rem; align-items: center;">
              <input
                type="text"
                class="form-input font-mono"
                style="font-size: 0.8125rem;"
                placeholder="VARIABLE_NAME"
                bind:value={v.key}
                required
              />

              <div style="position: relative;">
                {#if visibleValues[i]}
                  <input
                    type="text"
                    class="form-input font-mono"
                    style="font-size: 0.8125rem; padding-right: 2.25rem;"
                    placeholder="value"
                    bind:value={v.value}
                  />
                {:else}
                  <input
                    type="password"
                    class="form-input font-mono"
                    style="font-size: 0.8125rem; padding-right: 2.25rem;"
                    placeholder="••••••••••••"
                    bind:value={v.value}
                  />
                {/if}

                <button
                  type="button"
                  class="btn btn-secondary"
                  style="position: absolute; right: 4px; top: 4px; padding: 4px 6px; min-height: 28px; height: 28px; border: none; color: var(--color-ink-secondary);"
                  onclick={() => toggleVisibility(i)}
                  title={visibleValues[i] ? 'Hide value' : 'Show value'}
                >
                  {#if visibleValues[i]}
                    <EyeOff size={14} />
                  {:else}
                    <Eye size={14} />
                  {/if}
                </button>
              </div>

              <button
                type="button"
                class="btn btn-secondary"
                style="padding: 6px 10px; color: var(--color-error); border-color: var(--color-border); min-height: 36px;"
                onclick={() => removeVariable(i)}
                title="Delete variable"
              >
                <Trash2 size={15} />
              </button>
            </div>
          {/each}
        {/if}
      </div>

      <div style="display: flex; justify-content: space-between; align-items: center; border-top: 1px solid var(--color-border); padding-top: 1.25rem;">
        <button
          type="button"
          class="btn btn-secondary"
          style="display: flex; align-items: center; gap: 6px; font-size: 0.8125rem;"
          onclick={addVariable}
        >
          <Plus size={15} /> Add Shared Variable
        </button>

        <button
          type="submit"
          class="btn btn-primary"
          style="display: flex; align-items: center; gap: 6px; font-size: 0.8125rem; padding: 8px 20px;"
          disabled={saving}
        >
          {#if saving}
            <Loader2 size={14} class="animate-spin" /> Saving...
          {:else}
            <Save size={15} /> Save Variables
          {/if}
        </button>
      </div>
    </form>
  {/if}
</div>
