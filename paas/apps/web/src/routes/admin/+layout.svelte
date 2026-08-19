<script lang="ts">
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { Loader2, ShieldAlert } from 'lucide-svelte';

  let { children } = $props();
  let loading = $state(true);
  let isAuthorized = $state(false);

  onMount(async () => {
    try {
      const res = await fetch('/api/v1/auth/me', { credentials: 'include' });
      if (!res.ok) {
        goto('/login');
        return;
      }
      const user = await res.json();
      const isAdmin = user.isAdmin === true || 
                      user.isMainAdmin === true || 
                      user.platform_role === 'main_admin' || 
                      user.platform_role === 'admin' || 
                      user.platformRole === 'main_admin' || 
                      user.platformRole === 'admin';
      if (!isAdmin) {
        goto('/');
        return;
      }
      isAuthorized = true;
    } catch {
      goto('/');
    } finally {
      loading = false;
    }
  });
</script>

{#if loading}
  <div class="empty-state" style="padding: 4rem 1rem;">
    <Loader2 size={32} class="animate-spin text-muted" style="margin-bottom: 0.75rem;" />
    <p class="text-xs text-muted">Verifying administrator permissions...</p>
  </div>
{:else if isAuthorized}
  {@render children()}
{:else}
  <div class="empty-state" style="padding: 4rem 1rem;">
    <ShieldAlert size={36} style="color: var(--color-error); margin-bottom: 0.75rem;" />
    <div style="font-weight: 700; color: var(--color-ink); margin-bottom: 0.25rem;">Access Restricted</div>
    <p class="text-xs text-muted">Administrator credentials required to view this section.</p>
  </div>
{/if}
