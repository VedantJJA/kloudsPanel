<script lang="ts">
  import { page } from '$app/stores';
  import { Menu, X, Plus, Rocket, Layers, Database, Shield, Sun, Moon, Monitor } from 'lucide-svelte';
  import { isMobileNavOpen, toggleMobileNav } from '$lib/stores/ui';
  import { theme } from '$lib/stores/theme';
  import { activeWorkspaceSlug } from '$lib/stores/workspace';

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

    <a href="/" class="mobile-logo-link" onclick={() => isMobileNavOpen.set(false)}>
      <div class="sidebar-logo-mark" style="width:28px; height:28px; font-size:0.875rem;" aria-hidden="true">K</div>
      <span class="mobile-brand-title">kloudsPanel</span>
    </a>
  </div>

  <div style="display: flex; align-items: center; gap: 0.5rem;">
    <!-- Mobile Theme Toggle Button -->
    <button
      type="button"
      class="mobile-menu-btn"
      onclick={() => theme.toggle()}
      aria-label="Toggle dark and light mode"
      title={`Theme: ${$theme}`}
      style="min-width: 32px; min-height: 32px; padding: 4px;"
    >
      {#if $theme === 'dark'}
        <Moon size={18} style="color: var(--color-accent);" />
      {:else if $theme === 'light'}
        <Sun size={18} style="color: #f59e0b;" />
      {:else}
        <Monitor size={18} />
      {/if}
    </button>

    {#if pageCategory !== 'kloudsPanel'}
      <span class="badge" style="background: rgba(255, 255, 255, 0.08); color: #ffffff; font-size: 0.7rem; font-weight: 600; text-transform: uppercase;">
        {pageCategory}
      </span>
    {/if}
    
    <a 
      href={$activeWorkspaceSlug ? `/workspaces/${$activeWorkspaceSlug}` : '/'} 
      class="btn btn-primary" 
      style="padding: 4px 10px; min-height: 30px; font-size: 0.75rem; border-radius: var(--radius-md);"
      title="Projects"
      onclick={() => isMobileNavOpen.set(false)}
    >
      <Layers size={14} /> Projects
    </a>
  </div>
</header>
