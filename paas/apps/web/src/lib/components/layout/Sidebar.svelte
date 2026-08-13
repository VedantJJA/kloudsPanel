<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import {
    Home,
    Database,
    Settings,
    Users,
    Activity,
    ClipboardList,
    LogOut
  } from 'lucide-svelte';

  let sidebarOpen = $state(false);

  type NavItem = {
    label: string;
    href: string;
    icon: any;
    section?: string;
  };

  const navItems: NavItem[] = [
    { label: 'Workspaces', href: '/workspaces', icon: Home },
    { label: 'Databases', href: '/databases', icon: Database },
    { section: 'Administration', label: '', href: '', icon: null },
    { label: 'Platform', href: '/admin/setup', icon: Settings },
    { label: 'Users', href: '/admin/users', icon: Users },
    { label: 'Telemetry', href: '/admin/telemetry', icon: Activity },
    { label: 'Audit Log', href: '/admin/audit', icon: ClipboardList },
  ];

  function isActive(href: string): boolean {
    if (href === '/workspaces') {
      return $page.url.pathname === '/workspaces' || $page.url.pathname.startsWith('/workspaces/') || $page.url.pathname.startsWith('/projects/') || $page.url.pathname.startsWith('/services/');
    }
    return $page.url.pathname.startsWith(href);
  }

  async function handleLogout() {
    await fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'include' });
    goto('/login');
  }
</script>

<nav class="sidebar" class:open={sidebarOpen} aria-label="Main navigation">
  <!-- Logo -->
  <div class="sidebar-logo">
    <div class="sidebar-logo-mark" aria-hidden="true">K</div>
    <span class="sidebar-logo-text">kloudsPanel</span>
  </div>

  <!-- Nav items -->
  <div class="sidebar-nav" role="list">
    {#each navItems as item}
      {@const Icon = item.icon}
      {#if item.section}
        <div class="nav-section">{item.section}</div>
      {:else}
        <a
          href={item.href}
          class="nav-item"
          class:active={isActive(item.href)}
          aria-current={isActive(item.href) ? 'page' : undefined}
        >
          <span class="nav-item-icon" aria-hidden="true"><Icon size={20} /></span>
          {item.label}
        </a>
      {/if}
    {/each}
  </div>

  <!-- Footer: User account -->
  <div class="sidebar-footer">
    <button
      class="nav-item nav-item-logout"
      style="width:100%;color:rgba(234,241,250,0.6);font-size:0.8rem"
      onclick={handleLogout}
      aria-label="Sign out"
    >
      <span aria-hidden="true"><LogOut size={20} /></span>
      Sign Out
    </button>
  </div>
</nav>

<!-- Mobile overlay -->
<button
  class="sidebar-overlay"
  style="display:{sidebarOpen ? 'block' : 'none'};position:fixed;inset:0;z-index:99;background:rgba(11,31,58,0.5);border:none;cursor:pointer"
  onclick={() => (sidebarOpen = false)}
  aria-label="Close navigation"
></button>

<style>
  .sidebar-overlay { display: none; }
  @media (max-width: 960px) {
    .sidebar-overlay { display: block; }
  }
</style>
