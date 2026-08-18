<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { loadWorkspaces, activeWorkspaceSlug } from '$lib/stores/workspace';

  onMount(async () => {
    await loadWorkspaces();
    if ($activeWorkspaceSlug) {
      goto(`/workspaces/${$activeWorkspaceSlug}`);
    } else {
      goto('/workspaces');
    }
  });
</script>
