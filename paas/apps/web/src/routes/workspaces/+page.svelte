<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { Loader2 } from 'lucide-svelte';
  import { loadWorkspaces, activeWorkspaceSlug } from '$lib/stores/workspace';

  onMount(async () => {
    try {
      const list = await loadWorkspaces();
      if (list && list.length > 0) {
        const target = $activeWorkspaceSlug && list.some(w => (w.slug || (w as any).Slug) === $activeWorkspaceSlug)
          ? $activeWorkspaceSlug
          : (list[0].slug || (list[0] as any).Slug || list[0].id);
        goto(`/workspaces/${target}`, { replaceState: true });
      } else {
        goto('/workspaces/new', { replaceState: true });
      }
    } catch (e) {
      console.error(e);
      goto('/workspaces/new', { replaceState: true });
    }
  });
</script>

<svelte:head>
  <title>Workspaces - kloudsPanel</title>
</svelte:head>

<div class="empty-state" style="padding: 4rem 1rem;">
  <div class="animate-spin text-muted" style="margin-bottom:1rem">
    <Loader2 size={36} />
  </div>
  <p>Opening workspace...</p>
</div>
