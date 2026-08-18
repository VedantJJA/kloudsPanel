<script lang="ts">
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';

  onMount(() => {
    goto('/admin/setup?tab=git');
  });
</script>

<div style="text-align: center; padding: 3rem;">
  <p>Redirecting to Platform Settings & Setup...</p>
</div>
