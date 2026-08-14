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
    Users
  } from 'lucide-svelte';

  type SettingsTab = 'assist' | 'domain' | 'git' | 'security';

  let activeTab = $state<SettingsTab>('assist');

  // Platform state
  let rootDomain = $state('');
  let acmeEmail = $state('');
  let dnsMode = $state('http-01');
  let autoApprove = $state(true);
  let dbAidEnabled = $state(true);

  // OAuth credentials state
  let githubClientId = $state('');
  let githubClientSecret = $state('');
  let gitlabClientId = $state('');
  let gitlabClientSecret = $state('');
  let bitbucketClientId = $state('');
  let bitbucketClientSecret = $state('');

  let gitIntegrations = $state<any[]>([]);

  let loading = $state(true);
  let saving = $state(false);
  let saved = $state(false);
  let error = $state('');

  let dbAidSaving = $state(false);
  let autoApproveSaving = $state(false);

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
        autoApprove = s.auto_approve_users ?? true;
        dbAidEnabled = s.db_aid_enabled ?? true;
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

  async function toggleDbAid() {
    dbAidSaving = true;
    try {
      const nextVal = !dbAidEnabled;
      const res = await fetch('/api/v1/admin/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ db_aid_enabled: nextVal })
      });
      if (res.ok) {
        dbAidEnabled = nextVal;
      }
    } catch {} finally {
      dbAidSaving = false;
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
    { id: 'assist', label: 'Assist & Preferences', icon: Zap },
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
    <p class="page-subtitle">Configure root domain, TLS certificates, database assist mode, and platform infrastructure</p>
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
    <p class="text-sm text-muted" style="margin-top:0.5rem;">Loading platform configuration…</p>
  </div>
{:else}
  <!-- ══════════════════════════════════════════════════════════════════════════ -->
  <!-- TAB 1: ASSIST MODE & PREFERENCES                                          -->
  <!-- ══════════════════════════════════════════════════════════════════════════ -->
  {#if activeTab === 'assist'}
    <div style="display:flex; flex-direction:column; gap:1.5rem; max-width:840px;">
      <!-- Database Assist Mode Card -->
      <div class="card" style="background: var(--color-surface); border: 1.5px solid var(--color-accent); padding: 1.5rem;">
        <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:1rem;">
          <div style="display:flex; align-items:flex-start; gap:1rem; max-width:580px;">
            <div style="width:44px; height:44px; border-radius:var(--radius-md); background:rgba(0,166,166,0.12); color:var(--color-accent); display:flex; align-items:center; justify-content:center; flex-shrink:0;">
              <Zap size={24} />
            </div>
            <div>
              <div style="font-weight:700; font-size:1.05rem; color:var(--color-ink); display:flex; align-items:center; gap:8px;">
                Database Assist Mode & Command Abstractions
                {#if dbAidEnabled}
                  <span class="badge badge-running" style="font-size:0.75rem;">Active</span>
                {:else}
                  <span class="badge badge-stopped" style="font-size:0.75rem;">Disabled</span>
                {/if}
              </div>
              <p class="text-xs text-muted" style="margin:4px 0 0 0; line-height:1.5;">
                When active, all PostgreSQL, MySQL, Redis, MongoDB, and ClickHouse instances expose 1-click execution buttons for table statistics, vacuuming, active processlists, and cache flushes directly in the console.
              </p>
            </div>
          </div>

          <button 
            type="button" 
            class="btn {dbAidEnabled ? 'btn-primary' : 'btn-secondary'}" 
            onclick={toggleDbAid}
            disabled={dbAidSaving}
            style="padding:10px 22px; font-weight:700; font-size:0.875rem;"
          >
            {#if dbAidSaving}
              <Loader2 size={16} class="animate-spin" /> Updating…
            {:else if dbAidEnabled}
              Assist Mode: ON (Disable)
            {:else}
              Assist Mode: OFF (Enable)
            {/if}
          </button>
        </div>
      </div>

      <!-- User Auto-Approve Policy Card -->
      <div class="card" style="background: var(--color-surface); border: 1px solid var(--color-border); padding: 1.5rem;">
        <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:1rem;">
          <div style="display:flex; align-items:flex-start; gap:1rem; max-width:580px;">
            <div style="width:44px; height:44px; border-radius:var(--radius-md); background:rgba(37,99,235,0.1); color:#2563eb; display:flex; align-items:center; justify-content:center; flex-shrink:0;">
              <Users size={22} />
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
            style="padding:10px 22px; font-weight:700; font-size:0.875rem;"
          >
            {#if autoApproveSaving}
              <Loader2 size={16} class="animate-spin" /> Updating…
            {:else if autoApprove}
              Auto-Approve: ON (Disable)
            {:else}
              Auto-Approve: OFF (Enable)
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

  <!-- ══════════════════════════════════════════════════════════════════════════ -->
  <!-- TAB 2: DOMAIN & SSL/TLS                                                    -->
  <!-- ══════════════════════════════════════════════════════════════════════════ -->
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

          <div style="background:#fef3c7;border:1px solid #fbbf24;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.8125rem;margin-bottom:1.25rem;">
            <strong>DNS Requirement:</strong> Ensure <code>*.{rootDomain || 'yourdomain.com'}</code> A-Record resolves to this server's public IP.
          </div>

          <button type="submit" class="btn btn-primary" disabled={saving}>
            {#if saving}<Loader2 size={14} class="animate-spin" /> Saving & Verifying…{:else}<Save size={14} /> Save Domain Config{/if}
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
          <div style="padding:0.85rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:rgba(0,0,0,0.02);">
            <div style="font-weight:600; color:var(--color-ink);">Platform Dashboard</div>
            <div class="font-mono text-xs" style="color:var(--color-accent); margin-top:2px;">https://{rootDomain || 'yourdomain.com'}</div>
          </div>
          <div style="padding:0.85rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:rgba(0,0,0,0.02);">
            <div style="font-weight:600; color:var(--color-ink);">Web Services & Apps</div>
            <div class="font-mono text-xs" style="color:var(--color-accent); margin-top:2px;">https://[service-slug].{rootDomain || 'yourdomain.com'}</div>
          </div>
          <div style="padding:0.85rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:rgba(0,0,0,0.02);">
            <div style="font-weight:600; color:var(--color-ink);">Custom Domains</div>
            <div class="text-xs text-muted" style="margin-top:2px;">Attach any CNAME or A-Record in each service's Domains tab.</div>
          </div>
        </div>
      </div>
    </div>

  <!-- ══════════════════════════════════════════════════════════════════════════ -->
  <!-- TAB 3: GIT INTEGRATIONS                                                    -->
  <!-- ══════════════════════════════════════════════════════════════════════════ -->
  {:else if activeTab === 'git'}
    <div style="display:flex; flex-direction:column; gap:1.5rem; max-width:860px;">
      <!-- Quick Link to Dedicated Git Providers Setup Page -->
      <div class="card" style="padding: 1.25rem 1.5rem; background: linear-gradient(135deg, rgba(0,166,166,0.08) 0%, rgba(11,31,58,0.04) 100%); border: 1px solid rgba(0,166,166,0.3); display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 1rem;">
        <div style="display: flex; align-items: center; gap: 1rem;">
          <div style="width: 44px; height: 44px; border-radius: var(--radius-md); background: var(--color-accent); color: #fff; display: flex; align-items: center; justify-content: center; flex-shrink: 0;">
            <FolderGit2 size={24} />
          </div>
          <div>
            <div style="font-weight: 700; font-size: 1rem; color: var(--color-ink);">Git Provider 1-Click Authorizations</div>
            <p class="text-xs text-muted" style="margin: 2px 0 0 0;">
              Connect GitHub, GitLab, and Bitbucket accounts to import repositories and enable automated git webhook deployments.
            </p>
          </div>
        </div>

        <a href="/admin/git-providers" class="btn btn-primary" style="display: flex; align-items: center; gap: 6px;">
          Open 1-Click Setup <ArrowRight size={15} />
        </a>
      </div>

      <!-- Connected Git Accounts Overview -->
      <div class="card" style="padding:1.5rem;">
        <div class="card-header" style="margin-bottom:1.25rem;">
          <h3 style="margin:0;">Active Git Integrations</h3>
          <p class="text-xs text-muted" style="margin-top:0.25rem;">Connected provider accounts for this platform instance.</p>
        </div>

        <!-- Connected Providers List -->
        <div style="display:flex; flex-direction:column; gap:0.75rem;">
          {#each ['github', 'gitlab', 'bitbucket'] as prov}
            {@const item = gitIntegrations.find(g => g.provider === prov) || { provider: prov, connected: false }}
            <div style="display:flex; align-items:center; justify-content:space-between; padding:0.85rem 1rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-surface);">
              <div style="display:flex; align-items:center; gap:0.75rem;">
                <FolderGit2 size={20} style="color:var(--color-accent);" />
                <div>
                  <div style="font-weight:600; text-transform:capitalize; font-size:0.9375rem;">{item.provider}</div>
                  <div class="text-xs text-muted">
                    {#if item.connected}
                      Connected as <span class="font-mono" style="color:var(--color-ink); font-weight:600;">@{item.username}</span>
                    {:else}
                      Not connected yet
                    {/if}
                  </div>
                </div>
              </div>

              {#if item.connected}
                <span class="badge badge-running" style="font-size:0.75rem;">Active</span>
              {:else}
                <a href="/admin/git-providers" class="btn btn-secondary" style="padding:4px 12px; font-size:0.75rem;">
                  Setup OAuth
                </a>
              {/if}
            </div>
          {/each}
        </div>
      </div>

      <!-- Direct OAuth Credentials Form -->
      <div class="card" style="padding:1.5rem;">
        <div class="card-header" style="margin-bottom:1.25rem;">
          <h3 style="margin:0;">Direct OAuth Application Credentials</h3>
          <p class="text-xs text-muted" style="margin-top:0.25rem;">Specify Client IDs and Secrets for GitHub, GitLab, and Bitbucket apps.</p>
        </div>

        {#if saved}
          <div style="background:#d1fae5;border:1px solid #6ee7b7;color:#065f46;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem">
            OAuth credentials saved.
          </div>
        {/if}
        {#if error}
          <div style="background:#fee2e2;border:1px solid #fca5a5;color:#991b1b;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem">
            {error}
          </div>
        {/if}

        <form onsubmit={handleSaveGitOAuth}>
          <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(320px, 1fr)); gap:1.25rem; margin-bottom:1.5rem;">
            <!-- GitHub -->
            <div style="padding:1rem; border:1px solid var(--color-border); border-radius:var(--radius-md);">
              <div style="font-weight:700; margin-bottom:0.75rem; font-size:0.9375rem;">GitHub OAuth App</div>
              <div class="form-group" style="margin-bottom:0.75rem;">
                <label class="form-label text-xs" for="ghId">Client ID</label>
                <input id="ghId" type="text" class="form-input font-mono text-xs" bind:value={githubClientId} placeholder="Iv1..." />
              </div>
              <div class="form-group" style="margin:0;">
                <label class="form-label text-xs" for="ghSec">Client Secret</label>
                <input id="ghSec" type="password" class="form-input font-mono text-xs" bind:value={githubClientSecret} placeholder="••••••••" />
              </div>
            </div>

            <!-- GitLab -->
            <div style="padding:1rem; border:1px solid var(--color-border); border-radius:var(--radius-md);">
              <div style="font-weight:700; margin-bottom:0.75rem; font-size:0.9375rem;">GitLab OAuth App</div>
              <div class="form-group" style="margin-bottom:0.75rem;">
                <label class="form-label text-xs" for="glId">Application ID</label>
                <input id="glId" type="text" class="form-input font-mono text-xs" bind:value={gitlabClientId} placeholder="App ID" />
              </div>
              <div class="form-group" style="margin:0;">
                <label class="form-label text-xs" for="glSec">Secret</label>
                <input id="glSec" type="password" class="form-input font-mono text-xs" bind:value={gitlabClientSecret} placeholder="••••••••" />
              </div>
            </div>

            <!-- Bitbucket -->
            <div style="padding:1rem; border:1px solid var(--color-border); border-radius:var(--radius-md);">
              <div style="font-weight:700; margin-bottom:0.75rem; font-size:0.9375rem;">Bitbucket Consumer</div>
              <div class="form-group" style="margin-bottom:0.75rem;">
                <label class="form-label text-xs" for="bbId">Key</label>
                <input id="bbId" type="text" class="form-input font-mono text-xs" bind:value={bitbucketClientId} placeholder="Consumer Key" />
              </div>
              <div class="form-group" style="margin:0;">
                <label class="form-label text-xs" for="bbSec">Secret</label>
                <input id="bbSec" type="password" class="form-input font-mono text-xs" bind:value={bitbucketClientSecret} placeholder="••••••••" />
              </div>
            </div>
          </div>

          <button type="submit" class="btn btn-primary" disabled={saving}>
            {#if saving}<Loader2 size={14} class="animate-spin" /> Saving…{:else}<Save size={14} /> Save OAuth Credentials{/if}
          </button>
        </form>
      </div>
    </div>

  <!-- ══════════════════════════════════════════════════════════════════════════ -->
  <!-- TAB 4: SECURITY & ACCESS                                                  -->
  <!-- ══════════════════════════════════════════════════════════════════════════ -->
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
              <Loader2 size={14} class="animate-spin" /> Updating…
            {:else if autoApprove}
              Enabled (Click to Require Approval)
            {:else}
              Approval Required (Click to Enable Instant)
            {/if}
          </button>
        </div>

        <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:1rem; padding:1rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:rgba(0,0,0,0.02);">
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
          <div style="padding:0.85rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:rgba(0,0,0,0.02);">
            <div class="text-xs text-muted">Cookie Flags</div>
            <div style="font-weight:600; font-size:0.875rem; color:var(--color-ink); margin-top:2px;">HttpOnly; SameSite=Lax</div>
          </div>
          <div style="padding:0.85rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:rgba(0,0,0,0.02);">
            <div class="text-xs text-muted">Transport Security</div>
            <div style="font-weight:600; font-size:0.875rem; color:var(--color-ink); margin-top:2px;">TLS 1.2 / TLS 1.3 Strict</div>
          </div>
          <div style="padding:0.85rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:rgba(0,0,0,0.02);">
            <div class="text-xs text-muted">Session Duration</div>
            <div style="font-weight:600; font-size:0.875rem; color:var(--color-ink); margin-top:2px;">7 Days Rolling</div>
          </div>
        </div>
      </div>
    </div>
  {/if}
{/if}
