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
    ChevronsUpDown,
    Check,
    Plus,
    PanelLeftClose,
    PanelLeftOpen,
    FolderGit2,
    FolderOpen,
    Box,
    X,
    Sun,
    Moon,
    Monitor,
    Building2,
    Briefcase
  } from 'lucide-svelte';
  import { isMobileNavOpen, closeMobileNav } from '$lib/stores/ui';
  import { theme } from '$lib/stores/theme';
  import Logo from '$lib/components/Logo.svelte';
  import {
    workspaces,
    activeWorkspace,
    activeWorkspaceSlug,
    setActiveWorkspace,
    loadWorkspaces,
    createNewWorkspace,
    type Workspace
  } from '$lib/stores/workspace';

  let isCollapsed = $state(false);
  let isWorkspaceDropdownOpen = $state(false);
  let showNewWorkspaceModal = $state(false);
  let newWorkspaceName = $state('');
  let creatingWorkspace = $state(false);

  // Detect context
  const pathname = $derived($page.url.pathname);
  const isServiceContext = $derived(pathname.startsWith('/services/') && !pathname.endsWith('/services/new'));
  const isDatabaseContext = $derived(pathname.startsWith('/databases/') && pathname !== '/databases/new');
  const isProjectContext = $derived(
    pathname.startsWith('/projects/') && 
    !pathname.includes('/projects/new') && 
    !isServiceContext && 
    !isDatabaseContext
  );
  const isWorkspaceContext = $derived(
    pathname.startsWith('/workspaces/') && 
    pathname !== '/workspaces/new' && 
    !pathname.includes('/projects/new') && 
    !isServiceContext && 
    !isDatabaseContext &&
    !isProjectContext
  );

  // Extract entity ID from route params
  const currentServiceId = $derived(isServiceContext ? pathname.split('/')[2] : null);
  const currentDatabaseId = $derived(isDatabaseContext ? pathname.split('/')[2] : null);
  const currentProjectSlug = $derived(isProjectContext ? pathname.split('/')[2] : null);
  const currentWorkspaceSlug = $derived(isWorkspaceContext ? pathname.split('/')[2] : null);

  let currentService = $state<any>(null);
  let currentDatabase = $state<any>(null);
  let currentProject = $state<any>(null);
  let currentUser = $state<any>(null);

  const isAdmin = $derived(
    currentUser?.isAdmin === true || 
    currentUser?.isMainAdmin === true || 
    currentUser?.platform_role === 'main_admin' || 
    currentUser?.platform_role === 'admin' || 
    currentUser?.platformRole === 'main_admin' || 
    currentUser?.platformRole === 'admin'
  );

  const targetWsSlug = $derived(
    currentWorkspaceSlug || 
    $activeWorkspaceSlug || 
    currentProject?.workspace_slug || 
    'personal'
  );

  onMount(() => {
    try {
      const saved = localStorage.getItem('klouds_sidebar_collapsed');
      if (saved === 'true') {
        isCollapsed = true;
        document.querySelector('.app-shell')?.classList.add('sidebar-collapsed');
      }
    } catch {}

    loadWorkspaces(currentWorkspaceSlug || undefined);

    fetch('/api/v1/auth/me', { credentials: 'include' })
      .then(r => r.ok ? r.json() : null)
      .then(data => { if (data) currentUser = data; })
      .catch(() => {});
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

  function handleSelectWorkspace(ws: Workspace) {
    setActiveWorkspace(ws);
    isWorkspaceDropdownOpen = false;
    closeMobileNav();
    const s = ws.slug || ws.Slug;
    if (s) {
      goto(`/workspaces/${s}`);
    }
  }

  async function handleCreateWorkspaceSubmit(e: Event) {
    e.preventDefault();
    if (!newWorkspaceName.trim() || creatingWorkspace) return;
    creatingWorkspace = true;
    try {
      const created = await createNewWorkspace(newWorkspaceName.trim());
      if (created) {
        newWorkspaceName = '';
        showNewWorkspaceModal = false;
        isWorkspaceDropdownOpen = false;
        const s = created.slug || created.Slug;
        if (s) {
          goto(`/workspaces/${s}`);
        }
      }
    } finally {
      creatingWorkspace = false;
    }
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
        .then(data => {
          if (data) {
            currentDatabase = data;
            const projId = data.project_id || data.projectId;
            if (projId && (!currentProject || currentProject.id !== projId)) {
              fetch(`/api/v1/projects/${projId}`, { credentials: 'include' })
                .then(pr => pr.ok ? pr.json() : null)
                .then(pdata => { if (pdata) currentProject = pdata; })
                .catch(() => {});
            }
          }
        })
        .catch(() => {});
    } else {
      currentDatabase = null;
    }
  });

  $effect(() => {
    if (currentProjectSlug) {
      fetch(`/api/v1/projects/${currentProjectSlug}`, { credentials: 'include' })
        .then(r => r.ok ? r.json() : null)
        .then(data => { if (data) currentProject = data; })
        .catch(() => {});
    } else {
      currentProject = null;
    }
  });

  $effect(() => {
    if (currentWorkspaceSlug && $workspaces.length > 0) {
      const match = $workspaces.find(w => (w.slug || w.Slug) === currentWorkspaceSlug || (w.id || w.ID) === currentWorkspaceSlug);
      if (match && (match.slug || match.Slug) !== $activeWorkspaceSlug) {
        setActiveWorkspace(match);
      }
    }
  });

  type NavItem = {
    label: string;
    href: string;
    icon: any;
    section?: string;
  };

  const defaultNavItems = $derived.by<NavItem[]>(() => {
    const items: NavItem[] = [];
    if ($workspaces.length > 0) {
      const s = $activeWorkspaceSlug || $workspaces[0]?.slug || ($workspaces[0] as any)?.Slug || 'personal';
      items.push(
        { label: 'Projects', href: `/workspaces/${s}`, icon: FolderOpen },
        { label: 'Databases', href: `/workspaces/${s}/databases`, icon: Database },
        { label: 'Shared Env Vars', href: `/workspaces/${s}/variables`, icon: Key },
        { label: 'Members', href: `/workspaces/${s}/members`, icon: Users },
        { label: 'Settings', href: `/workspaces/${s}/settings`, icon: Settings },
      );
    } else {
      items.push(
        { label: 'New Workspace', href: '/workspaces/new', icon: Plus },
      );
    }
    if (isAdmin) {
      items.push(
        { section: 'Administration', label: '', href: '', icon: null },
        { label: 'Containers & Host', href: '/admin/containers', icon: Box },
        { label: 'Settings & Setup', href: '/admin/setup', icon: Settings },
        { label: 'Users', href: '/admin/users', icon: Users },
        { label: 'Telemetry', href: '/admin/telemetry', icon: Activity },
        { label: 'Audit Log', href: '/admin/audit', icon: ClipboardList },
      );
    }
    return items;
  });

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

  const projectTabs = $derived([
    { label: 'Services & Overview', href: `/projects/${currentProjectSlug}`, icon: LayoutDashboard },
    { label: 'Databases', href: `/projects/${currentProjectSlug}/databases`, icon: Database },
    { label: 'Shared Env Groups', href: `/projects/${currentProjectSlug}/variables`, icon: Key },
    { label: 'Settings', href: `/projects/${currentProjectSlug}/settings`, icon: Settings },
  ]);

  const workspaceTabs = $derived([
    { label: 'Projects', href: `/workspaces/${targetWsSlug}`, icon: FolderOpen },
    { label: 'Databases', href: `/workspaces/${targetWsSlug}/databases`, icon: Database },
    { label: 'Shared Env Vars', href: `/workspaces/${targetWsSlug}/variables`, icon: Key },
    { label: 'Members', href: `/workspaces/${targetWsSlug}/members`, icon: Users },
    { label: 'Settings', href: `/workspaces/${targetWsSlug}/settings`, icon: Settings },
  ]);

  function isTabActive(href: string): boolean {
    return pathname === href || (href.endsWith('/overview') && pathname === href.replace('/overview', ''));
  }

  function isWorkspaceTabActive(href: string): boolean {
    if (href === `/workspaces/${targetWsSlug}`) {
      return pathname === href;
    }
    return pathname.startsWith(href);
  }

  function isDefaultActive(href: string): boolean {
    if (href === `/workspaces/${targetWsSlug}`) {
      return pathname === href || pathname === '/workspaces';
    }
    return pathname.startsWith(href);
  }

  async function handleLogout() {
    await fetch('/api/v1/auth/logout', { method: 'POST', credentials: 'include' });
    goto('/login');
  }
</script>

<nav class="sidebar" class:open={$isMobileNavOpen} aria-label="Main navigation">
  <!-- Render-Style Active Workspace Switcher & Top Header -->
  <div class="sidebar-logo" style="display:flex; justify-content:space-between; align-items:center; width:100%; position:relative;">
    <div class="workspace-switcher-container" style="display:flex; align-items:center; gap:var(--sp-2); flex:1; min-width:0; position:relative;">
      <!-- Active Workspace Dropdown Button (Always available to switch and create) -->
      <button
        type="button"
        class="workspace-switcher-btn"
        onclick={() => isWorkspaceDropdownOpen = !isWorkspaceDropdownOpen}
        title={$activeWorkspace?.name || $activeWorkspace?.Name || ($workspaces.length === 0 ? 'No Workspace (Click to Create)' : 'Personal Workspace')}
        aria-expanded={isWorkspaceDropdownOpen}
        style="
          display: flex; 
          align-items: center; 
          gap: 8px; 
          background: rgba(255,255,255,0.04); 
          border: 1px solid rgba(255,255,255,0.08); 
          border-radius: var(--radius-md); 
          padding: 5px 8px; 
          cursor: pointer; 
          color: var(--color-ink); 
          width: 100%; 
          min-width: 0; 
          text-align: left;
          transition: background 0.15s ease, border-color 0.15s ease;
        "
      >
        <div style="flex-shrink: 0; display: flex; align-items: center;">
          <Logo size={20} />
        </div>
        {#if !isCollapsed}
          <div style="min-width: 0; flex: 1; overflow: hidden;">
            <div style="font-weight: 600; font-size: 0.8125rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--color-ink);">
              {$activeWorkspace?.name || $activeWorkspace?.Name || ($workspaces.length === 0 ? 'Select Workspace' : 'Personal Workspace')}
            </div>
            <div class="text-xs" style="color: var(--color-ink-muted); font-size: 0.68rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
              {$workspaces.length === 0 ? 'Click to create' : 'Active Workspace'}
            </div>
          </div>
          <ChevronsUpDown size={14} style="color: var(--color-ink-muted); flex-shrink: 0; margin-left: 2px;" />
        {/if}
      </button>

      <!-- Workspace Switcher Floating Dropdown Menu -->
      {#if isWorkspaceDropdownOpen}
        <div 
          class="workspace-dropdown-menu" 
          style="
            position: absolute; 
            top: 100%; 
            left: 0; 
            margin-top: 6px; 
            width: 240px; 
            background: var(--color-surface); 
            border: 1px solid var(--color-border); 
            border-radius: var(--radius-md); 
            box-shadow: 0 10px 25px -5px rgba(0,0,0,0.25), 0 8px 10px -6px rgba(0,0,0,0.25); 
            z-index: 999; 
            overflow: hidden;
            display: flex;
            flex-direction: column;
          "
        >
          <div style="padding: 8px 12px; font-size: 0.6875rem; text-transform: uppercase; font-weight: 600; color: var(--color-ink-muted); letter-spacing: 0.05em; border-bottom: 1px solid var(--color-border);">
            <span>Switch Workspace</span>
          </div>

          <div style="max-height: 220px; overflow-y: auto; padding: 4px;">
            {#if $workspaces.length === 0}
              <div style="padding: 14px 10px; text-align: center; font-size: 0.75rem; color: var(--color-ink-muted);">
                No workspaces created yet.
              </div>
            {:else}
              {#each $workspaces as ws}
                {@const isSelected = (ws.slug || ws.Slug) === $activeWorkspaceSlug}
                <button
                  type="button"
                  style="
                    width: 100%; 
                    display: flex; 
                    align-items: center; 
                    gap: 8px; 
                    padding: 7px 10px; 
                    border-radius: var(--radius-sm); 
                    border: none; 
                    background: {isSelected ? 'rgba(0,166,166,0.12)' : 'transparent'}; 
                    color: {isSelected ? 'var(--color-accent)' : 'var(--color-ink)'}; 
                    cursor: pointer; 
                    text-align: left;
                    font-size: 0.8125rem;
                    transition: background 0.1s ease;
                  "
                  onclick={() => handleSelectWorkspace(ws)}
                >
                  <div style="width: 22px; height: 22px; border-radius: var(--radius-xs); background: {isSelected ? 'var(--color-accent)' : 'rgba(255,255,255,0.08)'}; color: #ffffff; display: flex; align-items: center; justify-content: center; font-size: 0.7rem; font-weight: 700; flex-shrink: 0;">
                    {(ws.name || ws.Name || 'W').charAt(0).toUpperCase()}
                  </div>
                  <div style="min-width: 0; flex: 1; overflow: hidden;">
                    <div style="font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
                      {ws.name || ws.Name}
                    </div>
                    <div class="text-xs text-muted" style="font-size: 0.68rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
                      /{ws.slug || ws.Slug}
                    </div>
                  </div>
                  {#if isSelected}
                    <Check size={14} style="color: var(--color-accent); flex-shrink: 0;" />
                  {/if}
                </button>
              {/each}
            {/if}
          </div>

          <div style="border-top: 1px solid var(--color-border); padding: 4px;">
            <button
              type="button"
              style="
                width: 100%; 
                display: flex; 
                align-items: center; 
                gap: 6px; 
                padding: 7px 10px; 
                border-radius: var(--radius-sm); 
                border: none; 
                background: transparent; 
                color: var(--color-accent); 
                cursor: pointer; 
                font-size: 0.78125rem; 
                font-weight: 600;
              "
              onclick={() => { isWorkspaceDropdownOpen = false; showNewWorkspaceModal = true; }}
            >
              <Plus size={14} /> Create New Workspace
            </button>
          </div>
        </div>
      {/if}
    </div>

    <div style="display:flex; align-items:center; gap:4px; margin-left:6px;">
      <!-- Mobile Close Button (visible only on mobile) -->
      <button
        type="button"
        class="mobile-close-btn"
        onclick={closeMobileNav}
        aria-label="Close navigation"
      >
        <X size={18} />
      </button>

      <!-- Desktop Collapse Button -->
      <button
        type="button"
        class="desktop-collapse-btn btn btn-secondary"
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
  </div>

  <!-- Context-Aware Sidebar Content -->
  {#if isServiceContext && currentServiceId}
    <!-- Service Context Header -->
    <div style="padding: 0 var(--sp-2); margin-bottom: 0.75rem;">
      <a 
        href={currentService?.project_id ? `/projects/${currentService.project_id}` : `/workspaces/${targetWsSlug}`} 
        class="nav-item" 
        style="padding: 6px var(--sp-2); min-height: 32px; font-size: 0.8125rem; color: var(--color-ink-secondary);"
        title="Back to Project"
        onclick={closeMobileNav}
      >
        <ArrowLeft size={14} style="margin-right: 4px; flex-shrink:0;" /> 
        <span class="nav-item-text">Back to Project</span>
      </a>
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
          onclick={closeMobileNav}
        >
          <span class="nav-item-icon" aria-hidden="true"><Icon size={18} /></span>
          <span class="nav-item-text">{item.label}</span>
        </a>
      {/each}
    </div>

  {:else if isDatabaseContext && currentDatabaseId}
    <!-- Database Context Header -->
    <div style="padding: 0 var(--sp-2); margin-bottom: 0.75rem;">
      <a 
        href={currentProject ? `/projects/${currentProject.slug || currentProject.id}/databases` : `/workspaces/${targetWsSlug}/databases`} 
        class="nav-item" 
        style="padding: 6px var(--sp-2); min-height: 32px; font-size: 0.8125rem; color: var(--color-ink-secondary);"
        title={currentProject ? `Back to ${currentProject.name || 'Project'} Databases` : "Back to Databases"}
        onclick={closeMobileNav}
      >
        <ArrowLeft size={14} style="margin-right: 4px; flex-shrink:0;" /> 
        <span class="nav-item-text">{currentProject ? `${currentProject.name || 'Project'} Databases` : 'Project Databases'}</span>
      </a>
    </div>

    <!-- Database Specific Nav Tabs -->
    <div class="sidebar-nav" role="list">
      <div class="nav-section" style="font-size: 0.6875rem; color: var(--color-ink-muted); text-transform: uppercase; letter-spacing: 0.05em; padding: 0 var(--sp-2); margin-bottom: 0.5rem;">
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
          onclick={closeMobileNav}
        >
          <span class="nav-item-icon" aria-hidden="true"><Icon size={18} /></span>
          <span class="nav-item-text">{item.label}</span>
        </a>
      {/each}
    </div>

  {:else if isWorkspaceContext && currentWorkspaceSlug}
    <!-- Workspace Specific Nav Tabs -->
    <div class="sidebar-nav" role="list">
      <div class="nav-section" style="font-size: 0.6875rem; color: var(--color-ink-muted); text-transform: uppercase; letter-spacing: 0.05em; padding: 0 var(--sp-2); margin-bottom: 0.5rem;">
        Workspace Resources
      </div>
      {#each workspaceTabs as item}
        {@const Icon = item.icon}
        <a
          href={item.href}
          class="nav-item"
          class:active={isWorkspaceTabActive(item.href)}
          aria-current={isWorkspaceTabActive(item.href) ? 'page' : undefined}
          title={item.label}
          onclick={closeMobileNav}
        >
          <span class="nav-item-icon" aria-hidden="true"><Icon size={18} /></span>
          <span class="nav-item-text">{item.label}</span>
        </a>
      {/each}

      {#if isAdmin}
        <div class="nav-section" style="font-size: 0.6875rem; color: rgba(234,241,250,0.4); text-transform: uppercase; letter-spacing: 0.05em; padding: 0 var(--sp-2); margin: 1rem 0 0.5rem 0;">
          Administration
        </div>
        <a href="/admin/setup" class="nav-item" class:active={pathname.startsWith('/admin/setup')} onclick={closeMobileNav}>
          <span class="nav-item-icon"><Settings size={18} /></span>
          <span class="nav-item-text">Settings & Setup</span>
        </a>
        <a href="/admin/users" class="nav-item" class:active={pathname.startsWith('/admin/users')} onclick={closeMobileNav}>
          <span class="nav-item-icon"><Users size={18} /></span>
          <span class="nav-item-text">Users</span>
        </a>
        <a href="/admin/telemetry" class="nav-item" class:active={pathname.startsWith('/admin/telemetry')} onclick={closeMobileNav}>
          <span class="nav-item-icon"><Activity size={18} /></span>
          <span class="nav-item-text">Telemetry</span>
        </a>
        <a href="/admin/audit" class="nav-item" class:active={pathname.startsWith('/admin/audit')} onclick={closeMobileNav}>
          <span class="nav-item-icon"><ClipboardList size={18} /></span>
          <span class="nav-item-text">Audit Log</span>
        </a>
      {/if}
    </div>

  {:else if isProjectContext && currentProjectSlug}
    <!-- Project Context Header -->
    <div style="padding: 0 var(--sp-2); margin-bottom: 0.75rem;">
      <a 
        href={`/workspaces/${targetWsSlug}`} 
        class="nav-item" 
        style="padding: 6px var(--sp-2); min-height: 32px; font-size: 0.8125rem; color: var(--color-ink-secondary);"
        title="Back to Workspace"
        onclick={closeMobileNav}
      >
        <ArrowLeft size={14} style="margin-right: 4px; flex-shrink:0;" /> 
        <span class="nav-item-text">Back to Workspace</span>
      </a>
    </div>

    <!-- Project Specific Nav Tabs -->
    <div class="sidebar-nav" role="list">
      <div class="nav-section" style="font-size: 0.6875rem; color: rgba(234,241,250,0.4); text-transform: uppercase; letter-spacing: 0.05em; padding: 0 var(--sp-2); margin-bottom: 0.5rem;">
        Project Management
      </div>
      {#each projectTabs as item}
        {@const Icon = item.icon}
        {#if item.section}
          <div class="nav-section" style="font-size: 0.6875rem; color: rgba(234,241,250,0.4); text-transform: uppercase; letter-spacing: 0.05em; padding: 0 var(--sp-2); margin: 1rem 0 0.5rem 0;">
            {item.section}
          </div>
        {:else}
          <a
            href={item.href}
            class="nav-item"
            class:active={pathname === item.href || (item.href !== `/projects/${currentProjectSlug}` && pathname.startsWith(item.href)) || (item.href === `/projects/${currentProjectSlug}` && pathname.includes('/services/new'))}
            aria-current={pathname === item.href ? 'page' : undefined}
            title={item.label}
            onclick={closeMobileNav}
          >
            <span class="nav-item-icon" aria-hidden="true"><Icon size={18} /></span>
            <span class="nav-item-text">{item.label}</span>
          </a>
        {/if}
      {/each}
    </div>

  {:else}
    <!-- Default Active Workspace Navigation items -->
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
            onclick={closeMobileNav}
          >
            <span class="nav-item-icon" aria-hidden="true"><Icon size={20} /></span>
            <span class="nav-item-text">{item.label}</span>
          </a>
        {/if}
      {/each}
    </div>
  {/if}

  <!-- Footer: User account & Theme Toggle -->
  <div class="sidebar-footer" style="display:flex; flex-direction:column; gap:0.25rem;">
    <button
      type="button"
      class="nav-item"
      style="width:100%; color:rgba(234,241,250,0.75); font-size:0.8rem; cursor:pointer;"
      onclick={() => theme.toggle()}
      title={`Theme: ${$theme} (Click to toggle)`}
      aria-label="Toggle dark and light mode"
    >
      <span class="nav-item-icon" aria-hidden="true" style="display:flex; align-items:center;">
        {#if $theme === 'dark'}
          <Moon size={18} style="color: var(--color-accent);" />
        {:else if $theme === 'light'}
          <Sun size={18} style="color: #fbbf24;" />
        {:else}
          <Monitor size={18} />
        {/if}
      </span>
      <span class="nav-item-text">
        {$theme === 'dark' ? 'Dark Mode' : $theme === 'light' ? 'Light Mode' : 'System Theme'}
      </span>
    </button>

    <button
      class="nav-item nav-item-logout"
      style="width:100%; color:rgba(234,241,250,0.6); font-size:0.8rem;"
      onclick={() => { closeMobileNav(); handleLogout(); }}
      title="Sign Out"
      aria-label="Sign out"
    >
      <span aria-hidden="true"><LogOut size={18} /></span>
      <span class="nav-item-text">Sign Out</span>
    </button>
  </div>
</nav>

<!-- Modal: Create New Workspace Quick Dialog -->
{#if showNewWorkspaceModal}
  <div 
    class="modal-backdrop"
    style="position: fixed; inset: 0; background: rgba(0,0,0,0.6); backdrop-filter: blur(4px); z-index: 1000; display: flex; align-items: center; justify-content: center; padding: 1rem;"
    onclick={(e) => { if (e.target === e.currentTarget) showNewWorkspaceModal = false; }}
    role="presentation"
  >
    <div 
      class="modal-card" 
      style="width: 100%; max-width: 440px; background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius-lg); padding: 1.5rem; box-shadow: 0 20px 25px -5px rgba(0,0,0,0.3);"
    >
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem;">
        <h3 style="margin: 0; font-size: 1.15rem; color: var(--color-ink);">Create New Workspace</h3>
        <button 
          type="button" 
          class="btn btn-secondary" 
          style="padding: 4px; min-height: 28px; width: 28px; height: 28px; border: none;"
          onclick={() => showNewWorkspaceModal = false}
        >
          <X size={16} />
        </button>
      </div>

      <p class="text-xs text-muted" style="margin-bottom: 1.25rem;">
        Workspaces isolate projects, services, databases, and shared environment variables for teams or environments.
      </p>

      <form onsubmit={handleCreateWorkspaceSubmit}>
        <div class="form-group" style="margin-bottom: 1.25rem;">
          <label for="new-ws-name" class="form-label">Workspace Name</label>
          <input
            id="new-ws-name"
            type="text"
            class="form-input"
            placeholder="e.g. Staging, Acme Production, Personal"
            bind:value={newWorkspaceName}
            required
          />
        </div>

        <div style="display: flex; justify-content: flex-end; gap: 0.5rem;">
          <button 
            type="button" 
            class="btn btn-secondary" 
            onclick={() => showNewWorkspaceModal = false}
          >
            Cancel
          </button>
          <button 
            type="submit" 
            class="btn btn-primary"
            disabled={creatingWorkspace || !newWorkspaceName.trim()}
          >
            {creatingWorkspace ? 'Creating...' : 'Create Workspace'}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}

<!-- Mobile backdrop overlay -->
{#if $isMobileNavOpen}
  <div
    class="sidebar-backdrop"
    onclick={closeMobileNav}
    onkeydown={(e) => e.key === 'Escape' && closeMobileNav()}
    tabindex="0"
    role="button"
    aria-label="Close navigation overlay"
  ></div>
{/if}
