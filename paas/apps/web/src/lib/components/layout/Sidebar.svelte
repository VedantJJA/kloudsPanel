<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';

  let sidebarOpen = $state(false);

  type NavItem = {
    label: string;
    href: string;
    icon: string;
    section?: string;
  };

  const navItems: NavItem[] = [
    { label: 'Workspaces', href: '/workspaces', icon: '🏠' },
    { label: 'Databases', href: '/databases', icon: '🗄️' },
    { section: 'Administration', label: '', href: '', icon: '' },
    { label: 'Platform', href: '/admin', icon: '⚙️' },
    { label: 'Users', href: '/admin/users', icon: '👥' },
    { label: 'Telemetry', href: '/admin/telemetry', icon: '📊' },
    { label: 'Audit Log', href: '/admin/audit', icon: '📋' },
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
      {#if item.section}
        <span class="sidebar-section-label">{item.section}</span>
      {:else}
        <a
          href={item.href}
          class="nav-item"
          class:active={isActive(item.href)}
          aria-current={isActive(item.href) ? 'page' : undefined}
        >
          <span class="nav-item-icon" aria-hidden="true">{item.icon}</span>
          {item.label}
        </a>
      {/if}
    {/each}
  </div>

  <!-- Footer: User account -->
  <div class="sidebar-footer">
    <button
      class="nav-item"
      style="width:100%;color:rgba(234,241,250,0.6);font-size:0.8rem"
      onclick={handleLogout}
      aria-label="Sign out"
    >
      <span aria-hidden="true">🚪</span>
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
