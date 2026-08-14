<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import {
    Home,
    Database,
    Settings,
    Users,
    Activity,
    ClipboardList,
    LogOut,
    ArrowLeft,
    Rocket,
    Terminal,
    Key,
    Globe,
    Cpu,
    LayoutDashboard,
    Layers,
    Server,
    ShieldAlert,
    ChevronLeft,
    ChevronRight,
    PanelLeftClose,
    PanelLeftOpen,
    FolderGit2,
    FolderOpen,
    Box
  } from 'lucide-svelte';

  let sidebarOpen = $state(false);
  let isCollapsed = $state(false);

  // Detect context
  const pathname = $derived($page.url.pathname);
  const isServiceContext = $derived(pathname.startsWith('/services/') && !pathname.endsWith('/services/new'));
  const isDatabaseContext = $derived(pathname.startsWith('/databases/') && pathname !== '/databases/new');
  const isWorkspaceContext = $derived(
    pathname.startsWith('/workspaces/') && 
    pathname !== '/workspaces/new' && 
    !pathname.includes('/projects/new') && 
    !isServiceContext && 
    !isDatabaseContext
  );

  // Extract entity ID from route params
  const currentServiceId = $derived(isServiceContext ? pathname.split('/')[2] : null);
  const currentDatabaseId = $derived(isDatabaseContext ? pathname.split('/')[2] : null);
  const currentWorkspaceSlug = $derived(isWorkspaceContext ? pathname.split('/')[2] : null);

  let currentService = $state<any>(null);
  let currentDatabase = $state<any>(null);
  let currentWorkspace = $state<any>(null);

  onMount(() => {
    try {
      const saved = localStorage.getItem('klouds_sidebar_collapsed');
      if (saved === 'true') {
        isCollapsed = true;
        document.querySelector('.app-shell')?.classList.add('sidebar-collapsed');
      }
    } catch {}
  });

  function toggleCollapse() {
    isCollapsed = !isCollapsed;
    try {
      localStorage.setItem('klouds_sidebar_collapsed', String(isCollapsed));
      if (isCollapsed) {
        document.querySelector('.app-shell')?.classList.add('sidebar-collapsed');
      } else {
        document.querySelector('.app-shell')?.classList.remove('sidebar-collapsed');
      }
    } catch {}
  }

  $effect(() => {
    if (currentServiceId) {
      fetch(`/api/v1/services/${currentServiceId}`, { credentials: 'include' })
        .then(r => r.ok ? r.json() : null)
        .then(data => { if (data) currentService = data; })
        .catch(() => {});
    } else {
      currentService = null;
    }
  });

  $effect(() => {
    if (currentDatabaseId) {
      fetch(`/api/v1/databases/${currentDatabaseId}`, { credentials: 'include' })
        .then(r => r.ok ? r.json() : null)
        .then(data => { if (data) currentDatabase = data; })
        .catch(() => {});
    } else {
      currentDatabase = null;
    }
  });

  $effect(() => {
    if (currentWorkspaceSlug) {
      fetch(`/api/v1/workspaces/${currentWorkspaceSlug}`, { credentials: 'include' })
        .then(r => r.ok ? r.json() : null)
        .then(data => { if (data) currentWorkspace = data; })
        .catch(() => {});
    } else {
      currentWorkspace = null;
    }
  });

  type NavItem = {
    label: string;
    href: string;
    icon: any;
    section?: string;
  };

  const defaultNavItems: NavItem[] = [
    { label: 'Workspaces', href: '/workspaces', icon: Home },
    { label: 'Databases', href: '/databases', icon: Database },
    { section: 'Administration', label: '', href: '', icon: null },
    { label: 'Platform', href: '/admin/setup', icon: Settings },
    { label: 'Git Providers', href: '/admin/git-providers', icon: FolderGit2 },
    { label: 'Users', href: '/admin/users', icon: Users },
    { label: 'Telemetry', href: '/admin/telemetry', icon: Activity },
    { label: 'Audit Log', href: '/admin/audit', icon: ClipboardList },
  ];

  const serviceTabs = $derived([
    { label: 'Overview', href: `/services/${currentServiceId}/overview`, icon: LayoutDashboard },
    { label: 'Deployments', href: `/services/${currentServiceId}/deployments`, icon: Rocket },
    { label: 'Logs', href: `/services/${currentServiceId}/logs`, icon: Terminal },
    { label: 'Environment', href: `/services/${currentServiceId}/variables`, icon: Key },
    { label: 'Domains', href: `/services/${currentServiceId}/domains`, icon: Globe },
    { label: 'Scale & Limits', href: `/services/${currentServiceId}/scale`, icon: Cpu },
    { label: 'Settings', href: `/services/${currentServiceId}/settings`, icon: Settings },
  ]);

  const databaseTabs = $derived([
    { label: 'Connection', href: `/databases/${currentDatabaseId}/overview`, icon: Database },
    { label: 'Metrics', href: `/databases/${currentDatabaseId}/metrics`, icon: Activity },
    { label: 'Logs & Queries', href: `/databases/${currentDatabaseId}/logs`, icon: Terminal },
    { label: 'Settings', href: `/databases/${currentDatabaseId}/settings`, icon: Settings },
  ]);

  const workspaceTabs = $derived([
    { label: 'Projects', href: `/workspaces/${currentWorkspaceSlug}`, icon: FolderOpen },
    { label: 'Shared Env Vars', href: `/workspaces/${currentWorkspaceSlug}/variables`, icon: Key },
    { label: 'Members', href: `/workspaces/${currentWorkspaceSlug}/members`, icon: Users },
    { label: 'Settings', href: `/workspaces/${currentWorkspaceSlug}/settings`, icon: Settings },
  ]);

  function isTabActive(href: string): boolean {
    return pathname === href || (href.endsWith('/overview') && pathname === href.replace('/overview', ''));
  }

  function isWorkspaceTabActive(href: string): boolean {
    if (href === `/workspaces/${currentWorkspaceSlug}`) {
      return pathname === href;
    }
    return pathname.startsWith(href);
  }

  function isDefaultActive(href: string): boolean {
    if (href === '/workspaces') {
      return pathname === '/workspaces' || pathname.startsWith('/workspaces/') || pathname.startsWith('/projects/');
    }
    if (href === '/databases') {
      return pathname === '/databases' || pathname === '/databases/new';
    }
    return pathname.startsWith(href);
  }

  async function handleLogout() {
    await fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'include' });
    goto('/login');
  }
</script>

<nav class="sidebar" class:open={sidebarOpen} aria-label="Main navigation">
  <!-- Logo & Header with Collapse Toggle -->
  <div class="sidebar-logo" style="display:flex; justify-content:space-between; align-items:center; width:100%;">
    <div style="display:flex; align-items:center; gap:var(--sp-3);">
      <div class="sidebar-logo-mark" aria-hidden="true">K</div>
      {#if !isCollapsed}
        <span class="sidebar-logo-text">kloudsPanel</span>
      {/if}
    </div>

    <button
      type="button"
      class="btn btn-secondary"
      style="padding:4px; min-height:28px; width:28px; height:28px; border-radius:var(--radius-sm); border:none; color:rgba(234,241,250,0.6); display:flex; align-items:center; justify-content:center; background:rgba(255,255,255,0.06);"
      onclick={toggleCollapse}
      title={isCollapsed ? 'Expand Side Panel' : 'Contract Side Panel'}
      aria-label={isCollapsed ? 'Expand Side Panel' : 'Contract Side Panel'}
    >
      {#if isCollapsed}
        <ChevronRight size={16} />
      {:else}
        <ChevronLeft size={16} />
      {/if}
    </button>
  </div>

  <!-- Context-Aware Sidebar Content -->
  {#if isServiceContext && currentServiceId}
    <!-- Service Context Header -->
    <div style="padding: 0 var(--sp-2); margin-bottom: 1.25rem;">
      <a 
        href={currentService?.project_id ? `/projects/${currentService.project_id}` : '/workspaces'} 
        class="nav-item" 
        style="padding: 6px var(--sp-2); min-height: 32px; font-size: 0.8125rem; color: rgba(234,241,250,0.6);"
        title="Back to Project"
      >
        <ArrowLeft size={14} style="margin-right: 4px; flex-shrink:0;" /> 
        <span class="nav-item-text">Back to Project</span>
      </a>
      <div class="context-header-details" style="margin-top: 0.75rem; padding: 0.75rem; background: rgba(234,241,250,0.06); border-radius: var(--radius-md); border: 1px solid rgba(234,241,250,0.1);">
        <div style="font-size: 0.875rem; font-weight: 600; color: #fff; text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">
          {currentService?.name || 'Service'}
        </div>
        <div style="display: flex; gap: 0.4rem; align-items: center; margin-top: 0.25rem;">
          <span style="font-size: 0.6875rem; text-transform: uppercase; background: rgba(0,166,166,0.3); color: #00e5e5; padding: 2px 6px; border-radius: 4px; font-weight: 600;">
            {currentService?.kind || 'web'}
          </span>
          <span style="font-size: 0.6875rem; color: rgba(234,241,250,0.5);">
            {currentService?.runtime_status || 'draft'}
          </span>
        </div>
      </div>
    </div>

    <!-- Service Specific Nav Tabs -->
    <div class="sidebar-nav" role="list">
      <div class="nav-section" style="font-size: 0.6875rem; color: rgba(234,241,250,0.4); text-transform: uppercase; letter-spacing: 0.05em; padding: 0 var(--sp-2); margin-bottom: 0.5rem;">
        Service Management
      </div>
      {#each serviceTabs as item}
        {@const Icon = item.icon}
        <a
          href={item.href}
          class="nav-item"
          class:active={isTabActive(item.href)}
          aria-current={isTabActive(item.href) ? 'page' : undefined}
          title={item.label}
        >
          <span class="nav-item-icon" aria-hidden="true"><Icon size={18} /></span>
          <span class="nav-item-text">{item.label}</span>
        </a>
      {/each}
    </div>

  {:else if isDatabaseContext && currentDatabaseId}
    <!-- Database Context Header -->
    <div style="padding: 0 var(--sp-2); margin-bottom: 1.25rem;">
      <a 
        href="/databases" 
        class="nav-item" 
        style="padding: 6px var(--sp-2); min-height: 32px; font-size: 0.8125rem; color: rgba(234,241,250,0.6);"
        title="Back to Databases"
      >
        <ArrowLeft size={14} style="margin-right: 4px; flex-shrink:0;" /> 
        <span class="nav-item-text">Back to Databases</span>
      </a>
      <div class="context-header-details" style="margin-top: 0.75rem; padding: 0.75rem; background: rgba(234,241,250,0.06); border-radius: var(--radius-md); border: 1px solid rgba(234,241,250,0.1);">
        <div style="font-size: 0.875rem; font-weight: 600; color: #fff; text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">
          {currentDatabase?.name || 'Database'}
        </div>
        <div style="display: flex; gap: 0.4rem; align-items: center; margin-top: 0.25rem;">
          <span style="font-size: 0.6875rem; text-transform: uppercase; background: rgba(3,105,161,0.4); color: #7dd3fc; padding: 2px 6px; border-radius: 4px; font-weight: 600;">
            {currentDatabase?.engine || 'postgres'}
          </span>
          <span style="font-size: 0.6875rem; color: rgba(234,241,250,0.5);">
            {currentDatabase?.runtime_status || 'ready'}
          </span>
        </div>
      </div>
    </div>

    <!-- Database Specific Nav Tabs -->
    <div class="sidebar-nav" role="list">
      <div class="nav-section" style="font-size: 0.6875rem; color: rgba(234,241,250,0.4); text-transform: uppercase; letter-spacing: 0.05em; padding: 0 var(--sp-2); margin-bottom: 0.5rem;">
        Database Management
      </div>
      {#each databaseTabs as item}
        {@const Icon = item.icon}
        <a
          href={item.href}
          class="nav-item"
          class:active={isTabActive(item.href)}
          aria-current={isTabActive(item.href) ? 'page' : undefined}
          title={item.label}
        >
          <span class="nav-item-icon" aria-hidden="true"><Icon size={18} /></span>
          <span class="nav-item-text">{item.label}</span>
        </a>
      {/each}
    </div>

  {:else if isWorkspaceContext && currentWorkspaceSlug}
    <!-- Workspace Context Header -->
    <div style="padding: 0 var(--sp-2); margin-bottom: 1.25rem;">
      <a 
        href="/workspaces" 
        class="nav-item" 
        style="padding: 6px var(--sp-2); min-height: 32px; font-size: 0.8125rem; color: rgba(234,241,250,0.6);"
        title="Back to All Workspaces"
      >
        <ArrowLeft size={14} style="margin-right: 4px; flex-shrink:0;" /> 
        <span class="nav-item-text">All Workspaces</span>
      </a>
      <div class="context-header-details" style="margin-top: 0.75rem; padding: 0.75rem; background: rgba(234,241,250,0.06); border-radius: var(--radius-md); border: 1px solid rgba(234,241,250,0.1);">
        <div style="font-size: 0.875rem; font-weight: 600; color: #fff; text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">
          {currentWorkspace?.name || currentWorkspace?.Name || currentWorkspaceSlug}
        </div>
        <div style="display: flex; gap: 0.4rem; align-items: center; margin-top: 0.25rem;">
          <span style="font-size: 0.6875rem; text-transform: uppercase; background: rgba(16,185,129,0.25); color: #34d399; padding: 2px 6px; border-radius: 4px; font-weight: 600;">
            Workspace
          </span>
          <span style="font-size: 0.6875rem; color: rgba(234,241,250,0.5);">
            {currentWorkspace?.status || 'active'}
          </span>
        </div>
      </div>
    </div>

    <!-- Workspace Specific Nav Tabs -->
    <div class="sidebar-nav" role="list">
      <div class="nav-section" style="font-size: 0.6875rem; color: rgba(234,241,250,0.4); text-transform: uppercase; letter-spacing: 0.05em; padding: 0 var(--sp-2); margin-bottom: 0.5rem;">
        Workspace Management
      </div>
      {#each workspaceTabs as item}
        {@const Icon = item.icon}
        <a
          href={item.href}
          class="nav-item"
          class:active={isWorkspaceTabActive(item.href)}
          aria-current={isWorkspaceTabActive(item.href) ? 'page' : undefined}
          title={item.label}
        >
          <span class="nav-item-icon" aria-hidden="true"><Icon size={18} /></span>
          <span class="nav-item-text">{item.label}</span>
        </a>
      {/each}
    </div>

  {:else}
    <!-- Default Global Nav items -->
    <div class="sidebar-nav" role="list">
      {#each defaultNavItems as item}
        {@const Icon = item.icon}
        {#if item.section}
          <div class="nav-section" style="font-size: 0.6875rem; color: rgba(234,241,250,0.4); text-transform: uppercase; letter-spacing: 0.05em; padding: 0 var(--sp-2); margin: 1rem 0 0.5rem 0;">
            {item.section}
          </div>
        {:else}
          <a
            href={item.href}
            class="nav-item"
            class:active={isDefaultActive(item.href)}
            aria-current={isDefaultActive(item.href) ? 'page' : undefined}
            title={item.label}
          >
            <span class="nav-item-icon" aria-hidden="true"><Icon size={20} /></span>
            <span class="nav-item-text">{item.label}</span>
          </a>
        {/if}
      {/each}
    </div>
  {/if}

  <!-- Footer: User account only -->
  <div class="sidebar-footer" style="display:flex; flex-direction:column; gap:0.25rem;">
    <button
      class="nav-item nav-item-logout"
      style="width:100%; color:rgba(234,241,250,0.6); font-size:0.8rem;"
      onclick={handleLogout}
      title="Sign Out"
      aria-label="Sign out"
    >
      <span aria-hidden="true"><LogOut size={18} /></span>
      <span class="nav-item-text">Sign Out</span>
    </button>
  </div>
</nav>

<!-- Mobile overlay -->
<button
  class="sidebar-overlay"
  style="display:{sidebarOpen ? 'block' : 'none'}; position:fixed; inset:0; z-index:99; background:rgba(11,31,58,0.5); border:none; cursor:pointer;"
  onclick={() => (sidebarOpen = false)}
  aria-label="Close navigation"
></button>

<style>
  .sidebar-overlay { display: none; }
  @media (max-width: 960px) {
    .sidebar-overlay { display: block; }
  }
</style>
