<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import { activeWorkspaceSlug, loadWorkspaces } from '$lib/stores/workspace';

  onMount(async () => {
    await loadWorkspaces();
    if ($activeWorkspaceSlug) {
      goto(`/workspaces/${$activeWorkspaceSlug}/databases`);
    } else {
      goto('/workspaces');
    }
  });
</script>

<div style="text-align: center; padding: 3rem;">
  <p>Redirecting to Workspace Databases...</p>
</div>
