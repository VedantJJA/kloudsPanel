<script lang="ts">
  import { page } from '$app/stores';
  import { Menu, X, Plus, Rocket, Layers, Database, Shield } from 'lucide-svelte';
  import { isMobileNavOpen, toggleMobileNav } from '$lib/stores/ui';

  const pathname = $derived($page.url.pathname);
  
  const pageCategory = $derived(
    pathname.startsWith('/services/') ? 'Service' :
    pathname.startsWith('/databases/') ? 'Database' :
    pathname.startsWith('/workspaces/') ? 'Workspace' :
    pathname.startsWith('/admin/') ? 'Admin' :
    'kloudsPanel'
  );
</script>

<header class="mobile-top-header" aria-label="Mobile Navigation Bar">
  <div style="display: flex; align-items: center; gap: 0.75rem;">
    <button 
      type="button" 
      class="mobile-menu-btn" 
      onclick={toggleMobileNav}
      aria-label={$isMobileNavOpen ? 'Close Navigation Menu' : 'Open Navigation Menu'}
    >
      {#if $isMobileNavOpen}
        <X size={22} />
      {:else}
        <Menu size={22} />
      {/if}
    </button>

    <a href="/workspaces" class="mobile-logo-link" onclick={() => isMobileNavOpen.set(false)}>
      <div class="sidebar-logo-mark" style="width:28px; height:28px; font-size:0.875rem;" aria-hidden="true">K</div>
      <span class="mobile-brand-title">kloudsPanel</span>
    </a>
  </div>

  <div style="display: flex; align-items: center; gap: 0.5rem;">
    {#if pageCategory !== 'kloudsPanel'}
      <span class="badge" style="background: rgba(0, 166, 166, 0.12); color: var(--color-accent); font-size: 0.7rem; font-weight: 600; text-transform: uppercase;">
        {pageCategory}
      </span>
    {/if}
    
    <a 
      href="/workspaces" 
      class="btn btn-primary" 
      style="padding: 4px 10px; min-height: 32px; font-size: 0.75rem; border-radius: var(--radius-full);"
      title="Workspaces"
      onclick={() => isMobileNavOpen.set(false)}
    >
      <Layers size={14} /> Workspaces
    </a>
  </div>
</header>
