<script lang="ts">
  import { onMount } from 'svelte';
  import {
    Settings,
    FolderGit2,
    Check,
    Loader2,
    Globe,
    ArrowRight,
    ShieldCheck,
    Database,
    Zap,
    Key,
    Lock,
    Server,
    Sliders,
    Shield,
    Save,
    ExternalLink,
    Activity,
    Users,
    HardDrive,
    Sparkles,
    Copy
  } from 'lucide-svelte';

  type SettingsTab = 'initial-setup' | 'general' | 'domain' | 'git' | 'security';
  type ProviderKey = 'github' | 'gitlab' | 'bitbucket';

  let activeTab = $state<SettingsTab>('initial-setup');
  let activeGitTab = $state<ProviderKey>('github');
  let copiedCallback = $state(false);

  const providerMeta = {
    github: {
      name: 'GitHub',
      color: '#24292f',
      devUrl: 'https://github.com/settings/developers',
      keyLabel: 'Client ID',
      secretLabel: 'Client Secret',
      scopes: 'repo, read:user, user:email'
    },
    gitlab: {
      name: 'GitLab',
      color: '#fc6d26',
      devUrl: 'https://gitlab.com/-/user_settings/applications',
      keyLabel: 'Application ID',
      secretLabel: 'Secret',
      scopes: 'read_user, read_api, read_repository'
    },
    bitbucket: {
      name: 'Bitbucket',
      color: '#0052cc',
      devUrl: 'https://bitbucket.org/account/settings/api/',
      keyLabel: 'Key (Client ID)',
      secretLabel: 'Secret',
      scopes: 'Account (Read), Repositories (Read)'
    }
  };

  // Platform state
  let rootDomain = $state('');
  let acmeEmail = $state('');
  let dnsMode = $state('http-01');
  let autoApprove = $state(false);

  // Storage Maintenance state
  let pruning = $state(false);
  let pruneResult = $state<any>(null);

  // OAuth credentials state
  let githubClientId = $state('');
  let githubClientSecret = $state('');
  let gitlabClientId = $state('');
  let gitlabClientSecret = $state('');
  let bitbucketClientId = $state('');
  let bitbucketClientSecret = $state('');

  let gitIntegrations = $state<any[]>([]);

  let hostDomain = $derived(
    rootDomain || (typeof window !== 'undefined' ? window.location.host : 'yourdomain.com')
  );

  let curMeta = $derived(providerMeta[activeGitTab]);
  let activeConn = $derived(gitIntegrations.find(g => g.provider === activeGitTab && g.connected));

  let currentCallbackUrl = $derived(
    `https://${hostDomain}/api/v1/integrations/git/${activeGitTab}/callback`
  );

  function copyCallback() {
    if (typeof navigator !== 'undefined') {
      navigator.clipboard.writeText(currentCallbackUrl);
      copiedCallback = true;
      setTimeout(() => copiedCallback = false, 2500);
    }
  }

  let loading = $state(true);
  let saving = $state(false);
  let saved = $state(false);
  let error = $state('');

  let autoApproveSaving = $state(false);
  let optimizingContainers = $state(false);
  let optimizeResult = $state<any>(null);

  async function handleOptimizeContainers() {
    if (!confirm('Scan Docker daemon for unmanaged or orphaned containers and purge them? Only unmanaged containers not found in your database will be terminated.')) {
      return;
    }
    optimizingContainers = true;
    optimizeResult = null;
    try {
      const res = await fetch('/api/v1/admin/maintenance/optimize-containers', {
        method: 'POST',
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        optimizeResult = data;
        setTimeout(() => { optimizeResult = null; }, 10000);
      } else {
        const err = await res.json().catch(() => ({}));
        alert(err.error || 'Failed to optimize containers');
      }
    } catch (e: any) {
      alert('Error during container optimization: ' + e.message);
    } finally {
      optimizingContainers = false;
    }
  }

  async function handleReclaimStorage() {
    if (!confirm('Are you sure you want to reclaim storage? This will prune BuildKit build caches, dangling Docker layers, ephemeral build containers, and old system logs. Running containers and database volumes are fully preserved.')) {
      return;
    }
    pruning = true;
    pruneResult = null;
    try {
      const res = await fetch('/api/v1/admin/maintenance/prune-storage', {
        method: 'POST',
        credentials: 'include'
      });
      if (res.ok) {
        const data = await res.json();
        pruneResult = data;
        setTimeout(() => { pruneResult = null; }, 8000);
      } else {
        const err = await res.json().catch(() => ({}));
        alert(err.error || 'Failed to reclaim storage');
      }
    } catch (e: any) {
      alert('Error during storage reclamation: ' + e.message);
    } finally {
      pruning = false;
    }
  }

  async function loadData() {
    try {
      const [res, gitRes] = await Promise.all([
        fetch('/api/v1/admin/settings', { credentials: 'include' }),
        fetch('/api/v1/integrations/git', { credentials: 'include' })
      ]);
      if (res.ok) {
        const data = await res.json();
        const s = data.settings || {};
        rootDomain = s.root_domain ?? '';
        acmeEmail = s.acme_email ?? '';
        dnsMode = s.dns_mode ?? 'http-01';
        autoApprove = s.auto_approve_users ?? false;
        githubClientId = s.github_client_id ?? '';
        githubClientSecret = s.github_client_secret ?? '';
        gitlabClientId = s.gitlab_client_id ?? '';
        gitlabClientSecret = s.gitlab_client_secret ?? '';
        bitbucketClientId = s.bitbucket_client_id ?? '';
        bitbucketClientSecret = s.bitbucket_client_secret ?? '';
      }
      if (gitRes.ok) {
        gitIntegrations = (await gitRes.json()).integrations ?? [];
      }
    } catch {} finally {
      loading = false;
    }
  }

  async function toggleAutoApprove() {
    autoApproveSaving = true;
    try {
      const nextVal = !autoApprove;
      const res = await fetch('/api/v1/admin/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ auto_approve_users: nextVal })
      });
      if (res.ok) {
        autoApprove = nextVal;
      }
    } catch {} finally {
      autoApproveSaving = false;
    }
  }

  async function handleSaveDomain(e: Event) {
    e.preventDefault();
    saving = true; error = ''; saved = false;
    try {
      const res = await fetch('/api/v1/admin/setup', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ root_domain: rootDomain, acme_email: acmeEmail, dns_mode: dnsMode }),
      });
      if (!res.ok) { error = (await res.json()).detail; return; }
      saved = true;
      setTimeout(() => saved = false, 3000);
    } catch { error = 'Network error'; }
    finally { saving = false; }
  }

  async function handleSaveGitOAuth(e: Event) {
    e.preventDefault();
    saving = true; error = ''; saved = false;
    try {
      const res = await fetch('/api/v1/admin/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          github_client_id: githubClientId,
          github_client_secret: githubClientSecret,
          gitlab_client_id: gitlabClientId,
          gitlab_client_secret: gitlabClientSecret,
          bitbucket_client_id: bitbucketClientId,
          bitbucket_client_secret: bitbucketClientSecret
        })
      });
      if (res.ok) {
        saved = true;
        setTimeout(() => saved = false, 3000);
      } else {
        error = 'Failed to save Git OAuth credentials';
      }
    } catch {
      error = 'Network error';
    } finally {
      saving = false;
    }
  }

  onMount(() => {
    loadData();
  });

  const tabs: Array<{ id: SettingsTab; label: string; icon: any; count?: number }> = [
    { id: 'initial-setup', label: 'Initial Setup Hub', icon: Sparkles },
    { id: 'general', label: 'General & Maintenance', icon: Sliders },
    { id: 'domain', label: 'Domain & SSL/TLS', icon: Globe },
    { id: 'git', label: 'Git Integrations', icon: FolderGit2 },
    { id: 'security', label: 'Security & Access', icon: ShieldCheck }
  ];
</script>

<svelte:head>
  <title>Platform Settings & Setup - kloudsPanel</title>
</svelte:head>

<div class="page-header" style="margin-bottom: 1.25rem;">
  <div>
    <h1 class="page-title">Platform Settings & Setup</h1>
    <p class="page-subtitle">Configure root domain, TLS certificates, storage maintenance, and platform infrastructure</p>
  </div>
</div>

<!-- Modern Tab Bar -->
<div class="tabs-bar" style="display:flex; gap:0.25rem; border-bottom:2px solid var(--color-border); margin-bottom:1.5rem; overflow-x:auto;">
  {#each tabs as t}
    {@const Icon = t.icon}
    <button
      type="button"
      onclick={() => { activeTab = t.id; error = ''; saved = false; }}
      style="
        padding: 0.625rem 1.25rem;
        font-size: 0.875rem;
        font-weight: 600;
        color: {activeTab === t.id ? 'var(--color-accent)' : 'var(--color-ink-secondary)'};
        border-bottom: 2px solid {activeTab === t.id ? 'var(--color-accent)' : 'transparent'};
        margin-bottom: -2px;
        white-space: nowrap;
        background: transparent;
        border-top: none;
        border-left: none;
        border-right: none;
        cursor: pointer;
        transition: all 0.15s;
        display: flex;
        align-items: center;
        gap: 7px;
      "
    >
      <Icon size={16} />
      <span>{t.label}</span>
    </button>
  {/each}
</div>

{#if loading}
  <div style="text-align:center; padding:3rem 0;">
    <Loader2 size={36} class="animate-spin text-muted" />
    <p class="text-sm text-muted" style="margin-top:0.5rem;">Loading platform configuration...</p>
  </div>
{:else}
  <!-- --- TAB: INITIAL SETUP HUB ------------------------------------------- -->
  {#if activeTab === 'initial-setup'}
    <div style="display:flex; flex-direction:column; gap:1.5rem; max-width:960px;">

      <!-- Platform Welcome Banner -->
      <div class="card" style="background: var(--color-surface); border: 1px solid var(--color-border); padding: 1.5rem;">
        <div style="display:flex; align-items:flex-start; gap:1rem; flex-wrap:wrap;">
          <div style="width:40px; height:40px; border-radius:var(--radius-sm); background:var(--color-surface-subtle); border: 1px solid var(--color-border); color:#ffffff; display:flex; align-items:center; justify-content:center; flex-shrink:0;">
            <Sparkles size={20} />
          </div>
          <div style="flex:1; min-width:280px;">
            <h2 style="margin:0 0 0.25rem 0; font-size:1.25rem; font-weight:700; color:var(--color-ink);">Platform Foundation & Initial Setup</h2>
            <p class="text-xs text-muted" style="margin:0; line-height:1.6;">
              Welcome to kloudsPanel! Complete the foundation checklist below to enable automated HTTPS certificates, 1-click Git OAuth for all users, Nginx static site hosting, and database provisioning.
            </p>
          </div>
        </div>
      </div>

      <!-- Initial Setup Steps Grid -->
      <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(440px, 1fr)); gap:1.25rem;">

        <!-- Step 1: Root Domain & Traefik Ingress -->
        <div class="card" style="padding:1.25rem; border:1px solid var(--color-border); background:var(--color-surface); display:flex; flex-direction:column; justify-content:space-between;">
          <div>
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:0.75rem;">
              <div style="display:flex; align-items:center; gap:8px; font-weight:700; font-size:0.9375rem; color:var(--color-ink);">
                <div style="width:28px; height:28px; border-radius:50%; background:rgba(37,99,235,0.1); color:#2563eb; display:flex; align-items:center; justify-content:center; font-size:0.75rem; font-weight:800;">1</div>
                Domain & Ingress Gateway
              </div>
              {#if rootDomain}
                <span class="badge badge-running" style="font-size:0.6875rem;">Configured</span>
              {:else}
                <span class="badge badge-building" style="font-size:0.6875rem;">Setup Needed</span>
              {/if}
            </div>
            <p class="text-xs text-muted" style="margin:0 0 0.75rem 0; line-height:1.5;">
              Traefik reverse proxy handles automatic Let's Encrypt TLS certificates and subdomains for all user services.
            </p>
            <div style="background:rgba(0,0,0,0.02); border:1px solid var(--color-border); border-radius:var(--radius-sm); padding:0.625rem; font-size:0.8125rem; margin-bottom:0.75rem;">
              <div style="display:flex; justify-content:space-between; margin-bottom:2px;">
                <span class="text-muted">Root Domain:</span>
                <strong class="font-mono">{rootDomain || 'Not set'}</strong>
              </div>
              <div style="display:flex; justify-content:space-between;">
                <span class="text-muted">ACME Email:</span>
                <span class="font-mono text-xs">{acmeEmail || 'Not set'}</span>
              </div>
            </div>
          </div>
          <button 
            type="button" 
            class="btn btn-secondary" 
            style="width:100%; font-size:0.8125rem; display:flex; align-items:center; justify-content:center; gap:6px;"
            onclick={() => activeTab = 'domain'}
          >
            <Globe size={14} /> Configure Domain & SSL
          </button>
        </div>

        <!-- Step 2: Nginx Static Site Engine -->
        <div class="card" style="padding:1.25rem; border:1px solid var(--color-border); background:var(--color-surface); display:flex; flex-direction:column; justify-content:space-between;">
          <div>
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:0.75rem;">
              <div style="display:flex; align-items:center; gap:8px; font-weight:700; font-size:0.9375rem; color:var(--color-ink);">
                <div style="width:28px; height:28px; border-radius:50%; background:rgba(16,185,129,0.1); color:#10b981; display:flex; align-items:center; justify-content:center; font-size:0.75rem; font-weight:800;">2</div>
                Nginx Static Site Engine
              </div>
              <span class="badge badge-running" style="font-size:0.6875rem;">Active & Ready</span>
            </div>
            <p class="text-xs text-muted" style="margin:0 0 0.75rem 0; line-height:1.5;">
              Built-in Nginx container engine for hosting React, Vue, Vite, Svelte, Next.js static exports, and HTML/CSS/JS websites on port 80.
            </p>
            <div style="background:rgba(0,0,0,0.02); border:1px solid var(--color-border); border-radius:var(--radius-sm); padding:0.625rem; font-size:0.8125rem; margin-bottom:0.75rem; display:flex; flex-direction:column; gap:4px;">
              <div style="display:flex; align-items:center; gap:6px; color:#10b981; font-size:0.75rem; font-weight:600;">
                <Check size={13} /> SPA Fallback Routing (<code class="font-mono">try_files $uri /index.html</code>)
              </div>
              <div style="display:flex; align-items:center; gap:6px; color:#10b981; font-size:0.75rem; font-weight:600;">
                <Check size={13} /> Gzip Compression & Static Asset Caching
              </div>
              <div style="display:flex; align-items:center; gap:6px; color:#10b981; font-size:0.75rem; font-weight:600;">
                <Check size={13} /> Automatic <code class="font-mono">/api/</code> Backend Reverse Proxy
              </div>
            </div>
          </div>
          <div style="font-size:0.75rem; color:var(--color-ink-muted); text-align:center; padding:6px 0;">
            Automatically configured during static service deployments
          </div>
        </div>

        <!-- Step 3: Git 1-Click OAuth Providers -->
        <div class="card" style="padding:1.25rem; border:1px solid var(--color-border); background:var(--color-surface); display:flex; flex-direction:column; justify-content:space-between;">
          <div>
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:0.75rem;">
              <div style="display:flex; align-items:center; gap:8px; font-weight:700; font-size:0.9375rem; color:var(--color-ink);">
                <div style="width:28px; height:28px; border-radius:50%; background:rgba(139,92,246,0.1); color:#8b5cf6; display:flex; align-items:center; justify-content:center; font-size:0.75rem; font-weight:800;">3</div>
                Git 1-Click OAuth APIs
              </div>
              {#if githubClientId || gitlabClientId || bitbucketClientId}
                <span class="badge badge-running" style="font-size:0.6875rem;">OAuth Active</span>
              {:else}
                <span class="badge badge-building" style="font-size:0.6875rem;">Keys Needed</span>
              {/if}
            </div>
            <p class="text-xs text-muted" style="margin:0 0 0.75rem 0; line-height:1.5;">
              Provide the GitHub, GitLab, or Bitbucket OAuth app keys once. All users on the platform can then link their accounts with 1-click.
            </p>
            <div style="display:flex; gap:0.5rem; margin-bottom:0.75rem; flex-wrap:wrap;">
              <span class="badge {githubClientId ? 'badge-running' : 'badge-neutral'}" style="font-size:0.6875rem;">
                GitHub: {githubClientId ? 'Ready' : 'Not Set'}
              </span>
              <span class="badge {gitlabClientId ? 'badge-running' : 'badge-neutral'}" style="font-size:0.6875rem;">
                GitLab: {gitlabClientId ? 'Ready' : 'Not Set'}
              </span>
              <span class="badge {bitbucketClientId ? 'badge-running' : 'badge-neutral'}" style="font-size:0.6875rem;">
                Bitbucket: {bitbucketClientId ? 'Ready' : 'Not Set'}
              </span>
            </div>
          </div>
          <button 
            type="button" 
            class="btn btn-secondary" 
            style="width:100%; font-size:0.8125rem; display:flex; align-items:center; justify-content:center; gap:6px;"
            onclick={() => activeTab = 'git'}
          >
            <FolderGit2 size={14} /> Setup Git OAuth Providers
          </button>
        </div>

        <!-- Step 4: User Registration & Auto-Accept Policy -->
        <div class="card" style="padding:1.25rem; border:1px solid var(--color-border); background:var(--color-surface); display:flex; flex-direction:column; justify-content:space-between;">
          <div>
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:0.75rem;">
              <div style="display:flex; align-items:center; gap:8px; font-weight:700; font-size:0.9375rem; color:var(--color-ink);">
                <div style="width:28px; height:28px; border-radius:50%; background:rgba(234,88,12,0.1); color:#ea580c; display:flex; align-items:center; justify-content:center; font-size:0.75rem; font-weight:800;">4</div>
                Registration & Auto-Accept
              </div>
              {#if autoApprove}
                <span class="badge badge-running" style="font-size:0.6875rem;">Instant Access (ON)</span>
              {:else}
                <span class="badge badge-building" style="font-size:0.6875rem;">Approval Required (OFF)</span>
              {/if}
            </div>
            <p class="text-xs text-muted" style="margin:0 0 0.75rem 0; line-height:1.5;">
              Controls whether new signups receive instant platform access or require manual approval in the Admin Users tab.
            </p>
            <div style="background:rgba(0,0,0,0.02); border:1px solid var(--color-border); border-radius:var(--radius-sm); padding:0.625rem; font-size:0.8125rem; margin-bottom:0.75rem;">
              <span class="text-muted">Current Policy:</span> 
              <strong>{autoApprove ? 'New users gain immediate access upon registration.' : 'New users are placed in Pending status until approved.'}</strong>
            </div>
          </div>
          <button 
            type="button" 
            class="btn {autoApprove ? 'btn-secondary' : 'btn-primary'}" 
            style="width:100%; font-size:0.8125rem; display:flex; align-items:center; justify-content:center; gap:6px;"
            onclick={toggleAutoApprove}
            disabled={autoApproveSaving}
          >
            {#if autoApproveSaving}
              <Loader2 size={14} class="animate-spin" /> Updating...
            {:else if autoApprove}
              Switch to Require Approval (Recommended)
            {:else}
              Switch to Instant Auto-Accept
            {/if}
          </button>
        </div>

        <!-- Step 5: Multi-Engine Database Orchestrator -->
        <div class="card" style="padding:1.25rem; border:1px solid var(--color-border); background:var(--color-surface); display:flex; flex-direction:column; justify-content:space-between;">
          <div>
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:0.75rem;">
              <div style="display:flex; align-items:center; gap:8px; font-weight:700; font-size:0.9375rem; color:var(--color-ink);">
                <div style="width:28px; height:28px; border-radius:50%; background:rgba(14,165,233,0.1); color:#0ea5e9; display:flex; align-items:center; justify-content:center; font-size:0.75rem; font-weight:800;">5</div>
                Managed Databases Engine
              </div>
              <span class="badge badge-running" style="font-size:0.6875rem;">Ready</span>
            </div>
            <p class="text-xs text-muted" style="margin:0 0 0.75rem 0; line-height:1.5;">
              Deploy persistent database instances with automated credentials, private container networking, and health checks.
            </p>
            <div style="display:flex; gap:0.4rem; flex-wrap:wrap; margin-bottom:0.75rem;">
              <span class="badge badge-neutral" style="font-size:0.6875rem;">PostgreSQL</span>
              <span class="badge badge-neutral" style="font-size:0.6875rem;">MySQL</span>
              <span class="badge badge-neutral" style="font-size:0.6875rem;">Redis</span>
              <span class="badge badge-neutral" style="font-size:0.6875rem;">MongoDB</span>
              <span class="badge badge-neutral" style="font-size:0.6875rem;">ClickHouse</span>
            </div>
          </div>
          <a 
            href="/databases" 
            class="btn btn-secondary" 
            style="width:100%; font-size:0.8125rem; display:flex; align-items:center; justify-content:center; gap:6px;"
          >
            <Database size={14} /> Open Databases Manager
          </a>
        </div>

        <!-- Step 6: Docker Storage & Cache Maintenance -->
        <div class="card" style="padding:1.25rem; border:1px solid var(--color-border); background:var(--color-surface); display:flex; flex-direction:column; justify-content:space-between;">
          <div>
            <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:0.75rem;">
              <div style="display:flex; align-items:center; gap:8px; font-weight:700; font-size:0.9375rem; color:var(--color-ink);">
                <div style="width:28px; height:28px; border-radius:50%; background:rgba(239,68,68,0.1); color:#ef4444; display:flex; align-items:center; justify-content:center; font-size:0.75rem; font-weight:800;">6</div>
                Storage & Build Cache Maintenance
              </div>
              <span class="badge badge-neutral" style="font-size:0.6875rem;">1-Click</span>
            </div>
            <p class="text-xs text-muted" style="margin:0 0 0.75rem 0; line-height:1.5;">
              Safely clean up dangling BuildKit layers, ephemeral build caches, and old logs to reclaim host disk space.
            </p>
            {#if pruneResult}
              <div style="background:rgba(34,197,94,0.1); border:1px solid rgba(34,197,94,0.3); border-radius:var(--radius-sm); padding:0.5rem; font-size:0.75rem; color:#16a34a; margin-bottom:0.75rem;">
                <Check size={12} style="display:inline;" /> {pruneResult.message}
              </div>
            {/if}
          </div>
          <button 
            type="button" 
            class="btn btn-secondary" 
            style="width:100%; font-size:0.8125rem; display:flex; align-items:center; justify-content:center; gap:6px;"
            onclick={handleReclaimStorage}
            disabled={pruning}
          >
            {#if pruning}
              <Loader2 size={14} class="animate-spin" /> Cleaning...
            {:else}
              <Sparkles size={14} style="color:var(--color-accent);" /> Reclaim Storage Now
            {/if}
          </button>
        </div>

      </div>
    </div>

  <!-- --- TAB 1: GENERAL & MAINTENANCE ------------------------------------ -->
  {:else if activeTab === 'general'}
    <div style="display:flex; flex-direction:column; gap:1.5rem; max-width:840px;">

      <!-- User Auto-Approve Policy Card -->
      <div class="card" style="background: var(--color-surface); border: 1px solid var(--color-border); padding: 1.5rem;">
        <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:1rem;">
          <div style="display:flex; align-items:flex-start; gap:1rem; max-width:580px;">
            <div style="width:40px; height:40px; border-radius:var(--radius-sm); background:var(--color-surface-subtle); border: 1px solid var(--color-border); color:#ffffff; display:flex; align-items:center; justify-content:center; flex-shrink:0;">
              <Users size={20} />
            </div>
            <div>
              <div style="font-weight:700; font-size:1.05rem; color:var(--color-ink); display:flex; align-items:center; gap:8px;">
                User Registration Auto-Approval
                {#if autoApprove}
                  <span class="badge badge-running" style="font-size:0.75rem;">Instant Access</span>
                {:else}
                  <span class="badge badge-building" style="font-size:0.75rem;">Admin Approval Required</span>
                {/if}
              </div>
              <p class="text-xs text-muted" style="margin:4px 0 0 0; line-height:1.5;">
                When enabled, newly registered users gain instant platform access. When disabled, accounts remain in pending status until approved by a Main Admin.
              </p>
            </div>
          </div>

          <button 
            type="button" 
            class="btn {autoApprove ? 'btn-primary' : 'btn-secondary'}" 
            onclick={toggleAutoApprove}
            disabled={autoApproveSaving}
            style="padding:8px 20px; font-weight:600; font-size:0.8125rem;"
          >
            {#if autoApproveSaving}
              <Loader2 size={14} class="animate-spin" /> Updating...
            {:else if autoApprove}
              Auto-Approve: ON (Disable)
            {:else}
              Auto-Approve: OFF (Enable)
            {/if}
          </button>
        </div>
      </div>

      <!-- Storage Maintenance & Build Cache Pruning Card -->
      <div class="card" style="background: var(--color-surface); border: 1px solid var(--color-border); padding: 1.5rem;">
        {#if pruneResult}
          <div style="background: rgba(34,197,94,0.1); border: 1px solid rgba(34,197,94,0.3); border-radius: var(--radius-md); padding: 0.875rem 1.25rem; margin-bottom: 1.25rem; display: flex; align-items: center; justify-content: space-between;">
            <div style="display: flex; align-items: center; gap: 8px; color: #16a34a; font-weight: 600; font-size: 0.875rem;">
              <Check size={18} />
              {pruneResult.message}
            </div>
            <button class="btn btn-secondary" style="padding: 2px 8px; font-size: 0.75rem; min-height: 24px;" onclick={() => pruneResult = null}>Dismiss</button>
          </div>
        {/if}

        <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:1rem;">
          <div style="display:flex; align-items:flex-start; gap:1rem; max-width:580px;">
            <div style="width:44px; height:44px; border-radius:var(--radius-md); background:rgba(16,185,129,0.1); color:#10b981; display:flex; align-items:center; justify-content:center; flex-shrink:0;">
              <HardDrive size={22} />
            </div>
            <div>
              <div style="font-weight:700; font-size:1.05rem; color:var(--color-ink); display:flex; align-items:center; gap:8px;">
                Storage & Build Cache Maintenance
              </div>
              <p class="text-xs text-muted" style="margin:4px 0 0 0; line-height:1.5;">
                Prune BuildKit build cache, dangling image layers from previous builds, ephemeral build containers, and systemd journal logs. Running services and database data are completely protected.
              </p>
            </div>
          </div>

          <button 
            type="button" 
            class="btn btn-secondary" 
            onclick={handleReclaimStorage}
            disabled={pruning}
            style="padding:10px 22px; font-weight:700; font-size:0.875rem; display:flex; align-items:center; gap:6px;"
          >
            {#if pruning}
              <Loader2 size={16} class="animate-spin" /> Reclaiming Storage...
            {:else}
              <Sparkles size={16} style="color:var(--color-accent);" /> Reclaim Storage Now
            {/if}
          </button>
        </div>
      </div>

      <!-- Container & Resource Optimizer Card -->
      <div class="card" style="background: var(--color-surface); border: 1px solid var(--color-border); padding: 1.5rem;">
        {#if optimizeResult}
          <div style="background: rgba(34,197,94,0.1); border: 1px solid rgba(34,197,94,0.3); border-radius: var(--radius-md); padding: 0.875rem 1.25rem; margin-bottom: 1.25rem;">
            <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: optimizeResult.removed_containers?.length ? '0.5rem' : '0';">
              <div style="display: flex; align-items: center; gap: 8px; color: #16a34a; font-weight: 600; font-size: 0.875rem;">
                <Check size={18} />
                {optimizeResult.message}
              </div>
              <button class="btn btn-secondary" style="padding: 2px 8px; font-size: 0.75rem; min-height: 24px;" onclick={() => optimizeResult = null}>Dismiss</button>
            </div>
            {#if optimizeResult.removed_containers && optimizeResult.removed_containers.length > 0}
              <div style="font-size: 0.75rem; color: var(--color-ink-muted); margin-top: 4px;">
                Purged containers: <span class="font-mono text-xs" style="color: var(--color-ink);">{optimizeResult.removed_containers.join(', ')}</span>
              </div>
            {/if}
          </div>
        {/if}

        <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:1rem;">
          <div style="display:flex; align-items:flex-start; gap:1rem; max-width:580px;">
            <div style="width:44px; height:44px; border-radius:var(--radius-md); background:rgba(56,189,248,0.1); color:#38bdf8; display:flex; align-items:center; justify-content:center; flex-shrink:0;">
              <Zap size={22} />
            </div>
            <div>
              <div style="font-weight:700; font-size:1.05rem; color:var(--color-ink); display:flex; align-items:center; gap:8px;">
                Container & Resource Optimizer
              </div>
              <p class="text-xs text-muted" style="margin:4px 0 0 0; line-height:1.5;">
                Scan host Docker daemon for unmanaged or orphaned containers, ghost processes from deleted workspaces, and stale Traefik routing rules. Automatically purges unindexed containers and synchronizes reverse proxy routes.
              </p>
            </div>
          </div>

          <button 
            type="button" 
            class="btn btn-secondary" 
            onclick={handleOptimizeContainers}
            disabled={optimizingContainers}
            style="padding:10px 22px; font-weight:700; font-size:0.875rem; display:flex; align-items:center; gap:6px;"
          >
            {#if optimizingContainers}
              <Loader2 size={16} class="animate-spin" /> Scanning & Purging...
            {:else}
              <Zap size={16} style="color:#38bdf8;" /> Scan & Remove Orphan Containers
            {/if}
          </button>
        </div>
      </div>

      <!-- Platform Overview & Architecture Info -->
      <div class="card" style="background: var(--color-surface); border: 1px solid var(--color-border); padding: 1.5rem;">
        <h3 style="margin: 0 0 0.75rem 0; font-size: 1.05rem;">Platform Engine Status</h3>
        <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(220px, 1fr)); gap:1rem;">
          <div style="background:rgba(0,0,0,0.02); border:1px solid var(--color-border); border-radius:var(--radius-md); padding:1rem;">
            <div class="text-xs text-muted" style="margin-bottom:4px;">Reverse Proxy Engine</div>
            <div style="font-weight:700; font-size:0.9375rem; color:var(--color-ink);">Traefik v3 (Automated ACME)</div>
          </div>
          <div style="background:rgba(0,0,0,0.02); border:1px solid var(--color-border); border-radius:var(--radius-md); padding:1rem;">
            <div class="text-xs text-muted" style="margin-bottom:4px;">Container Runtime</div>
            <div style="font-weight:700; font-size:0.9375rem; color:var(--color-ink);">Docker Engine API</div>
          </div>
          <div style="background:rgba(0,0,0,0.02); border:1px solid var(--color-border); border-radius:var(--radius-md); padding:1rem;">
            <div class="text-xs text-muted" style="margin-bottom:4px;">Database Management</div>
            <div style="font-weight:700; font-size:0.9375rem; color:var(--color-ink);">Multi-Engine Orchestrator</div>
          </div>
        </div>
      </div>
    </div>

  <!-- --- TAB 2: DOMAIN & SSL/TLS ----------------------------------------- -->
  {:else if activeTab === 'domain'}
    <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(360px, 1fr)); gap:1.5rem; max-width:960px;">
      <!-- Domain & TLS Configuration -->
      <div class="card" style="padding:1.5rem;">
        <div class="card-header" style="margin-bottom:1.25rem;">
          <h3 style="margin:0;">Root Domain & ACME TLS</h3>
          <p class="text-xs text-muted" style="margin-top:0.25rem;">Master domain and automatic Let's Encrypt certificates.</p>
        </div>

        {#if saved}
          <div style="background:#d1fae5;border:1px solid #6ee7b7;color:#065f46;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem">
            Configuration saved successfully.
          </div>
        {/if}
        {#if error}
          <div style="background:#fee2e2;border:1px solid #fca5a5;color:#991b1b;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem">
            {error}
          </div>
        {/if}

        <form onsubmit={handleSaveDomain}>
          <div class="form-group">
            <label class="form-label" for="rootDomain">Root Domain</label>
            <input id="rootDomain" type="text" class="form-input" bind:value={rootDomain} placeholder="yourdomain.com" required />
            <p class="text-xs text-muted" style="margin-top:0.25rem">Projects and preview services default to <code>*.{rootDomain || 'yourdomain.com'}</code>.</p>
          </div>

          <div class="form-group">
            <label class="form-label" for="acmeEmail">ACME Let's Encrypt Email</label>
            <input id="acmeEmail" type="email" class="form-input" bind:value={acmeEmail} placeholder="admin@yourdomain.com" />
            <p class="text-xs text-muted" style="margin-top:0.25rem">Used for certificate expiration notices from Let's Encrypt.</p>
          </div>

          <div class="form-group">
            <label class="form-label" for="dnsMode">DNS Challenge Mode</label>
            <select id="dnsMode" class="form-select" bind:value={dnsMode}>
              <option value="http-01">HTTP-01 (standard standalone server)</option>
              <option value="dns-01">DNS-01 (wildcard certificates)</option>
            </select>
          </div>

          <div style="background:var(--color-surface-subtle);border:1px solid var(--color-border);border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.8125rem;margin-bottom:1.25rem;color:var(--color-ink);">
            <strong>DNS Requirement:</strong> Ensure <code>*.{rootDomain || 'yourdomain.com'}</code> A-Record resolves to this server's public IP.
          </div>

          <button type="submit" class="btn btn-primary" disabled={saving}>
            {#if saving}<Loader2 size={14} class="animate-spin" /> Saving and Verifying...{:else}<Save size={14} /> Save Domain Config{/if}
          </button>
        </form>
      </div>

      <!-- Domain Routing Guide -->
      <div class="card" style="padding:1.5rem;">
        <div class="card-header" style="margin-bottom:1.25rem;">
          <h3 style="margin:0;">Automatic Subdomain Routing</h3>
          <p class="text-xs text-muted" style="margin-top:0.25rem;">How kloudsPanel assigns hostnames to deployments.</p>
        </div>

        <div style="display:flex; flex-direction:column; gap:0.85rem; font-size:0.875rem;">
          <div style="padding:0.85rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-surface-subtle);">
            <div style="font-weight:600; color:var(--color-ink);">Platform Dashboard</div>
            <div class="font-mono text-xs" style="color:var(--color-ink); margin-top:2px;">https://{rootDomain || 'yourdomain.com'}</div>
          </div>
          <div style="padding:0.85rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-surface-subtle);">
            <div style="font-weight:600; color:var(--color-ink);">Web Services & Apps</div>
            <div class="font-mono text-xs" style="color:var(--color-ink); margin-top:2px;">https://[service-slug].{rootDomain || 'yourdomain.com'}</div>
          </div>
          <div style="padding:0.85rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-surface-subtle);">
            <div style="font-weight:600; color:var(--color-ink);">Custom Domains</div>
            <div class="text-xs text-muted" style="margin-top:2px;">Attach any CNAME or A-Record in each service's Domains tab.</div>
          </div>
        </div>
      </div>
    </div>

  <!-- --- TAB 3: GIT INTEGRATIONS ------------------------------------------- -->
  {:else if activeTab === 'git'}
    <div style="display:flex; flex-direction:column; gap:1.5rem; max-width:860px;">
      <!-- Provider Selector Tabs -->
      <div style="display:flex; gap:0.5rem; flex-wrap:wrap;">
        {#each (['github', 'gitlab', 'bitbucket'] as ProviderKey[]) as prov}
          {@const meta = providerMeta[prov]}
          {@const isConn = gitIntegrations.some(g => g.provider === prov && g.connected)}
          <button
            type="button"
            class="btn {activeGitTab === prov ? 'btn-primary' : 'btn-secondary'}"
            style="padding:8px 16px; font-size:0.875rem; font-weight:600; display:flex; align-items:center; gap:8px;"
            onclick={() => activeGitTab = prov}
          >
            <FolderGit2 size={16} />
            {meta.name}
            {#if isConn}
              <span class="badge badge-running" style="font-size:0.68rem; padding:1px 6px;">Connected</span>
            {/if}
          </button>
        {/each}
      </div>

      <!-- 30-Second Setup Guide Card -->
      <div class="card" style="background:var(--color-surface); border:1.5px solid var(--color-border); padding:1.5rem;">
        <div style="display:flex; justify-content:space-between; align-items:flex-start; margin-bottom:1.25rem; flex-wrap:wrap; gap:1rem;">
          <div>
            <h3 style="margin:0; font-size:1.1rem; color:var(--color-ink); display:flex; align-items:center; gap:8px;">
              <FolderGit2 size={20} style="color:var(--color-accent);" />
              30-Second {curMeta.name} Setup Guide
            </h3>
            <p class="text-xs text-muted" style="margin:4px 0 0 0;">
              Follow these quick steps to connect your {curMeta.name} account for 1-click repository imports and automated push-to-deploy webhooks.
            </p>
          </div>

          <a
            href={curMeta.devUrl}
            target="_blank"
            rel="noopener noreferrer"
            class="btn btn-secondary"
            style="font-size:0.8125rem; padding:6px 12px; display:flex; align-items:center; gap:6px;"
          >
            Open {curMeta.name} Developer Settings <ExternalLink size={13} />
          </a>
        </div>

        <!-- Step-by-Step Checklist -->
        <div style="display:flex; flex-direction:column; gap:1rem; margin-bottom:1.5rem; background:rgba(0,0,0,0.02); border:1px solid var(--color-border); border-radius:var(--radius-md); padding:1.25rem;">
          <div style="display:flex; gap:0.75rem; align-items:flex-start;">
            <div style="width:24px; height:24px; border-radius:50%; background:var(--color-accent); color:#ffffff; display:flex; align-items:center; justify-content:center; font-size:0.75rem; font-weight:700; flex-shrink:0;">
              1
            </div>
            <div style="font-size:0.875rem;">
              <div style="font-weight:600; color:var(--color-ink);">Open Developer Console</div>
              <div class="text-xs text-muted" style="margin-top:2px;">
                Go to <a href={curMeta.devUrl} target="_blank" rel="noopener noreferrer" style="color:var(--color-accent); text-decoration:underline;">{curMeta.devUrl}</a> and click <strong>"New OAuth App"</strong> or <strong>"New Application"</strong>.
              </div>
            </div>
          </div>

          <div style="display:flex; gap:0.75rem; align-items:flex-start;">
            <div style="width:24px; height:24px; border-radius:50%; background:var(--color-accent); color:#ffffff; display:flex; align-items:center; justify-content:center; font-size:0.75rem; font-weight:700; flex-shrink:0;">
              2
            </div>
            <div style="font-size:0.875rem; flex:1;">
              <div style="font-weight:600; color:var(--color-ink);">Enter Platform URLs</div>
              <div style="display:grid; grid-template-columns:140px 1fr auto; gap:0.5rem; align-items:center; margin-top:6px; background:var(--color-surface); padding:6px 10px; border-radius:var(--radius-sm); border:1px solid var(--color-border);">
                <span class="text-xs text-muted" style="font-weight:600;">Homepage URL:</span>
                <span class="font-mono text-xs" style="color:var(--color-ink);">https://{hostDomain}</span>
                <span></span>
              </div>
              <div style="display:grid; grid-template-columns:140px 1fr auto; gap:0.5rem; align-items:center; margin-top:4px; background:var(--color-surface); padding:6px 10px; border-radius:var(--radius-sm); border:1px solid var(--color-border);">
                <span class="text-xs text-muted" style="font-weight:600;">Callback URL:</span>
                <span class="font-mono text-xs" style="color:var(--color-ink); overflow:hidden; text-overflow:ellipsis;">{currentCallbackUrl}</span>
                <button
                  type="button"
                  class="btn btn-secondary"
                  style="padding:2px 8px; font-size:0.72rem; min-height:24px; display:flex; align-items:center; gap:4px;"
                  onclick={copyCallback}
                >
                  {#if copiedCallback}<Check size={12} style="color:#10b981;" /> Copied{:else}<Copy size={12} /> Copy{/if}
                </button>
              </div>
              <div class="text-xs text-muted" style="margin-top:4px;">
                Scopes/Permissions required: <span class="font-mono" style="color:var(--color-accent); font-weight:600;">{curMeta.scopes}</span>
              </div>
            </div>
          </div>

          <div style="display:flex; gap:0.75rem; align-items:flex-start;">
            <div style="width:24px; height:24px; border-radius:50%; background:var(--color-accent); color:#ffffff; display:flex; align-items:center; justify-content:center; font-size:0.75rem; font-weight:700; flex-shrink:0;">
              3
            </div>
            <div style="font-size:0.875rem;">
              <div style="font-weight:600; color:var(--color-ink);">Save Credentials Below</div>
              <div class="text-xs text-muted" style="margin-top:2px;">
                Copy your generated {curMeta.keyLabel} and {curMeta.secretLabel} from {curMeta.name} into the form below and click <strong>"Save OAuth Credentials"</strong>.
              </div>
            </div>
          </div>
        </div>

        {#if saved}
          <div style="background:#d1fae5;border:1px solid #6ee7b7;color:#065f46;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem;display:flex;align-items:center;gap:6px;">
            <Check size={16} /> OAuth credentials saved successfully.
          </div>
        {/if}
        {#if error}
          <div style="background:#fee2e2;border:1px solid #fca5a5;color:#991b1b;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem;">
            {error}
          </div>
        {/if}

        <!-- Credentials Form for Active Provider -->
        <form onsubmit={handleSaveGitOAuth}>
          {#if activeGitTab === 'github'}
            <div style="display:grid; grid-template-columns:1fr 1fr; gap:1rem; margin-bottom:1.25rem;">
              <div class="form-group" style="margin:0;">
                <label class="form-label text-xs" for="ghId">GitHub Client ID</label>
                <input id="ghId" type="text" class="form-input font-mono text-xs" bind:value={githubClientId} placeholder="e.g. Ov23li..." required />
              </div>
              <div class="form-group" style="margin:0;">
                <label class="form-label text-xs" for="ghSec">GitHub Client Secret</label>
                <input id="ghSec" type="password" class="form-input font-mono text-xs" bind:value={githubClientSecret} placeholder="Enter secret" required />
              </div>
            </div>
          {:else if activeGitTab === 'gitlab'}
            <div style="display:grid; grid-template-columns:1fr 1fr; gap:1rem; margin-bottom:1.25rem;">
              <div class="form-group" style="margin:0;">
                <label class="form-label text-xs" for="glId">GitLab Application ID</label>
                <input id="glId" type="text" class="form-input font-mono text-xs" bind:value={gitlabClientId} placeholder="e.g. gloas-..." required />
              </div>
              <div class="form-group" style="margin:0;">
                <label class="form-label text-xs" for="glSec">GitLab Secret</label>
                <input id="glSec" type="password" class="form-input font-mono text-xs" bind:value={gitlabClientSecret} placeholder="Enter secret" required />
              </div>
            </div>
          {:else if activeGitTab === 'bitbucket'}
            <div style="display:grid; grid-template-columns:1fr 1fr; gap:1rem; margin-bottom:1.25rem;">
              <div class="form-group" style="margin:0;">
                <label class="form-label text-xs" for="bbId">Bitbucket Key (Client ID)</label>
                <input id="bbId" type="text" class="form-input font-mono text-xs" bind:value={bitbucketClientId} placeholder="e.g. 69..." required />
              </div>
              <div class="form-group" style="margin:0;">
                <label class="form-label text-xs" for="bbSec">Bitbucket Secret</label>
                <input id="bbSec" type="password" class="form-input font-mono text-xs" bind:value={bitbucketClientSecret} placeholder="Enter secret" required />
              </div>
            </div>
          {/if}

          <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:1rem; border-top:1px solid var(--color-border); padding-top:1rem;">
            <button type="submit" class="btn btn-primary" disabled={saving}>
              {#if saving}<Loader2 size={14} class="animate-spin" /> Saving...{:else}<Save size={14} /> Save {curMeta.name} Credentials{/if}
            </button>

            <!-- 1-Click Connect Button -->
            {#if activeConn}
              <div style="display:flex; align-items:center; gap:8px;">
                <span class="badge badge-running" style="font-size:0.75rem;">Connected as @{activeConn.username}</span>
                <a href="/api/v1/integrations/git/{activeGitTab}/auth" class="btn btn-secondary" style="font-size:0.75rem; padding:4px 10px;">
                  Re-authenticate
                </a>
              </div>
            {:else}
              <a 
                href="/api/v1/integrations/git/{activeGitTab}/auth" 
                class="btn btn-secondary"
                style="font-size:0.8125rem; padding:7px 16px; display:flex; align-items:center; gap:6px;"
              >
                <FolderGit2 size={14} /> 1-Click Connect {curMeta.name} Account
              </a>
            {/if}
          </div>
        </form>
      </div>
    </div>

  <!-- --- TAB 4: SECURITY & ACCESS ----------------------------------------- -->
  {:else if activeTab === 'security'}
    <div style="display:flex; flex-direction:column; gap:1.5rem; max-width:800px;">
      <!-- Access Policy Card -->
      <div class="card" style="padding:1.5rem;">
        <div class="card-header" style="margin-bottom:1.25rem;">
          <h3 style="margin:0;">Access & Registration Policies</h3>
          <p class="text-xs text-muted" style="margin-top:0.25rem;">Control how users sign up and receive platform access.</p>
        </div>

        <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:1rem; padding:1rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-surface); margin-bottom:1.25rem;">
          <div>
            <div style="font-weight:600; font-size:0.9375rem; color:var(--color-ink);">Instant User Registration Approval</div>
            <div class="text-xs text-muted" style="margin-top:2px;">When turned off, new signups must be manually approved in the Users tab.</div>
          </div>

          <button 
            type="button" 
            class="btn {autoApprove ? 'btn-primary' : 'btn-secondary'}" 
            onclick={toggleAutoApprove}
            disabled={autoApproveSaving}
            style="padding:6px 16px; font-weight:600; font-size:0.8125rem;"
          >
            {#if autoApproveSaving}
              <Loader2 size={14} class="animate-spin" /> Updating...
            {:else if autoApprove}
              Enabled (Click to Require Approval)
            {:else}
              Approval Required (Click to Enable Instant)
            {/if}
          </button>
        </div>

        <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:1rem; padding:1rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-surface-subtle);">
          <div>
            <div style="font-weight:600; font-size:0.9375rem; color:var(--color-ink);">Audit & Compliance Logging</div>
            <div class="text-xs text-muted" style="margin-top:2px;">All deployment triggers, database modifications, and user approvals are recorded.</div>
          </div>
          <a href="/admin/audit" class="btn btn-secondary" style="font-size:0.8125rem; padding:6px 14px;">
            View Audit Log
          </a>
        </div>
      </div>

      <!-- Session & Cookie Security -->
      <div class="card" style="padding:1.5rem;">
        <div class="card-header" style="margin-bottom:1.25rem;">
          <h3 style="margin:0;">Session & Cookie Security</h3>
          <p class="text-xs text-muted" style="margin-top:0.25rem;">Platform session parameters and transport encryption.</p>
        </div>

        <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(220px, 1fr)); gap:1rem;">
          <div style="padding:0.85rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-surface-subtle);">
            <div class="text-xs text-muted">Cookie Flags</div>
            <div style="font-weight:600; font-size:0.875rem; color:var(--color-ink); margin-top:2px;">HttpOnly; SameSite=Lax</div>
          </div>
          <div style="padding:0.85rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-surface-subtle);">
            <div class="text-xs text-muted">Transport Security</div>
            <div style="font-weight:600; font-size:0.875rem; color:var(--color-ink); margin-top:2px;">TLS 1.2 / TLS 1.3 Strict</div>
          </div>
          <div style="padding:0.85rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-surface-subtle);">
            <div class="text-xs text-muted">Session Duration</div>
            <div style="font-weight:600; font-size:0.875rem; color:var(--color-ink); margin-top:2px;">7 Days Rolling</div>
          </div>
        </div>
      </div>
    </div>
  {/if}
{/if}
