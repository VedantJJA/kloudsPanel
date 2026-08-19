<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { loadWorkspaces, activeWorkspaceSlug } from '$lib/stores/workspace';

  onMount(async () => {
    try {
      const list = await loadWorkspaces();
      if ($activeWorkspaceSlug) {
        goto(`/workspaces/${$activeWorkspaceSlug}`, { replaceState: true });
      } else if (list && list.length > 0) {
        goto(`/workspaces/${list[0].slug || (list[0] as any).Slug || list[0].id}`, { replaceState: true });
      } else {
        goto('/workspaces/new', { replaceState: true });
      }
    } catch {
      goto('/workspaces/new', { replaceState: true });
    }
  });
</script>
