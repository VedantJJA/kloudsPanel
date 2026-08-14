<script lang="ts">
  import { onMount } from 'svelte';
  import { 
    FolderGit2, 
    Check, 
    Loader2, 
    Plus, 
    Trash2, 
    Key, 
    Globe, 
    ShieldCheck, 
    Copy, 
    ExternalLink, 
    Sparkles, 
    ArrowRight,
    Lock,
    RefreshCw
  } from 'lucide-svelte';

  type ProviderKey = 'github' | 'gitlab' | 'bitbucket';

  let activeTab = $state<ProviderKey>('github');
  let rootDomain = $state('');

  // OAuth Credentials
  let githubClientId = $state('');
  let githubClientSecret = $state('');
  let gitlabClientId = $state('');
  let gitlabClientSecret = $state('');
  let bitbucketClientId = $state('');
  let bitbucketClientSecret = $state('');

  let savingOAuth = $state(false);
  let oauthSaved = $state(false);
  let copiedCallback = $state(false);

  // Integrations List
  let gitIntegrations = $state<any[]>([]);
  let oauthEnabledMap = $state<Record<string, boolean>>({});

  // Manual Token Fallback
  let showManualToken = $state(false);
  let manualUsername = $state('');
  let manualToken = $state('');
  let savingManual = $state(false);
  let manualSaved = $state(false);
  let hostDomain = $derived(
    rootDomain || (typeof window !== 'undefined' ? window.location.host : 'yourdomain.com')
  );

  let currentCallbackUrl = $derived(
    `https://${hostDomain}/api/v1/integrations/git/${activeTab}/callback`
  );

  let currentConnected = $derived(
    gitIntegrations.find(g => g.provider === activeTab && g.connected)
  );

  let currentHasKeys = $derived(
    activeTab === 'github' ? !!githubClientId :
    activeTab === 'gitlab' ? !!gitlabClientId :
    !!bitbucketClientId
  );

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

  let current = $derived(providerMeta[activeTab]);

  async function loadData() {
    try {
      const [settingsRes, gitRes] = await Promise.all([
        fetch('/api/v1/admin/settings', { credentials: 'include' }),
        fetch('/api/v1/integrations/git', { credentials: 'include' })
      ]);

      if (settingsRes.ok) {
        const data = await settingsRes.json();
        rootDomain = data.settings?.root_domain ?? '';
        githubClientId = data.settings?.github_client_id ?? '';
        githubClientSecret = data.settings?.github_client_secret ?? '';
        gitlabClientId = data.settings?.gitlab_client_id ?? '';
        gitlabClientSecret = data.settings?.gitlab_client_secret ?? '';
        bitbucketClientId = data.settings?.bitbucket_client_id ?? '';
        bitbucketClientSecret = data.settings?.bitbucket_client_secret ?? '';
      }

      if (gitRes.ok) {
        const gitData = await gitRes.json();
        gitIntegrations = gitData.integrations ?? [];
        oauthEnabledMap = gitData.oauthEnabled ?? {};
      }
    } catch {}
  }

  onMount(() => {
    loadData();
  });

  async function saveOAuth(e: Event) {
    e.preventDefault();
    savingOAuth = true;
    oauthSaved = false;

    try {
      const payload: Record<string, string> = {};
      if (activeTab === 'github') {
        payload.github_client_id = githubClientId;
        payload.github_client_secret = githubClientSecret;
      } else if (activeTab === 'gitlab') {
        payload.gitlab_client_id = gitlabClientId;
        payload.gitlab_client_secret = gitlabClientSecret;
      } else if (activeTab === 'bitbucket') {
        payload.bitbucket_client_id = bitbucketClientId;
        payload.bitbucket_client_secret = bitbucketClientSecret;
      }

      const res = await fetch('/api/v1/admin/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify(payload)
      });

      if (res.ok) {
        oauthSaved = true;
        await loadData();
        setTimeout(() => oauthSaved = false, 3000);
      }
    } finally {
      savingOAuth = false;
    }
  }

  async function authorizeProvider(provider: ProviderKey) {
    window.location.href = `/api/v1/integrations/git/${provider}/authorize?return_to=${encodeURIComponent('/admin/git-providers')}`;
  }

  async function disconnectProvider(provider: string) {
    if (!confirm(`Disconnect ${provider}?`)) return;
    await fetch(`/api/v1/integrations/git/${provider}`, { method: 'DELETE', credentials: 'include' });
    await loadData();
  }

  async function copyCallback() {
    navigator.clipboard.writeText(currentCallbackUrl);
    copiedCallback = true;
    setTimeout(() => copiedCallback = false, 2000);
  }

  async function saveManualToken(e: Event) {
    e.preventDefault();
    savingManual = true;
    manualSaved = false;
    try {
      const res = await fetch('/api/v1/integrations/git', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          provider: activeTab,
          username: manualUsername,
          token: manualToken
        })
      });
      if (res.ok) {
        manualSaved = true;
        manualToken = '';
        await loadData();
        setTimeout(() => manualSaved = false, 3000);
      }
    } finally {
      savingManual = false;
    }
  }
</script>

<svelte:head>
  <title>Git Providers Setup - kloudsPanel</title>
</svelte:head>

<div class="page-header" style="margin-bottom: 1.5rem;">
  <div>
    <h1 class="page-title">Git Providers Setup</h1>
    <p class="page-subtitle">Configure 1-Click OAuth authorization for GitHub, GitLab, and Bitbucket accounts</p>
  </div>
</div>

<!-- Provider Navigation Tabs -->
<div class="tabs-bar" style="display:flex; gap:0.75rem; margin-bottom:1.5rem; border-bottom:1px solid var(--color-border); padding-bottom:0.75rem; flex-wrap:wrap;">
  {#each (['github', 'gitlab', 'bitbucket'] as ProviderKey[]) as prov}
    {@const meta = providerMeta[prov]}
    {@const connected = gitIntegrations.find(g => g.provider === prov && g.connected)}
    {@const hasKeys = prov === 'github' ? !!githubClientId : prov === 'gitlab' ? !!gitlabClientId : !!bitbucketClientId}
    <button
      type="button"
      class="btn"
      style="padding:8px 16px; font-size:0.875rem; border-radius:var(--radius-md); font-weight:600; display:flex; align-items:center; gap:8px; border:1px solid {activeTab === prov ? 'var(--color-ink)' : 'var(--color-border)'}; background:{activeTab === prov ? 'var(--color-surface-sunken, #0f172a)' : 'var(--color-surface)'}; color:{activeTab === prov ? '#fff' : 'var(--color-ink)'};"
      onclick={() => activeTab = prov}
    >
      <span style="display:inline-block; width:10px; height:10px; border-radius:50%; background:{connected ? '#10b981' : hasKeys ? '#3b82f6' : '#94a3b8'};"></span>
      {meta.name}
      {#if connected}
        <span class="badge badge-running" style="font-size:0.6875rem; padding:1px 6px;">Connected</span>
      {/if}
    </button>
  {/each}
</div>

<!-- Active Provider Dedicated Setup View -->
<div class="card" style="padding:1.75rem; margin-bottom:2rem; border:1px solid var(--color-border); background:var(--color-surface);">
  <!-- Header & Status Action Bar -->
  <div style="display:flex; justify-content:space-between; align-items:flex-start; flex-wrap:wrap; gap:1rem; margin-bottom:1.5rem; padding-bottom:1.25rem; border-bottom:1px solid var(--color-border);">
    <div style="display:flex; align-items:center; gap:1rem;">
      <div style="width:48px; height:48px; border-radius:var(--radius-md); background:{current.color}; color:#fff; display:flex; align-items:center; justify-content:center;">
        <FolderGit2 size={26} />
      </div>
      <div>
        <div style="font-size:1.25rem; font-weight:700; color:var(--color-ink); display:flex; align-items:center; gap:8px;">
          {current.name} 1-Click Authorization
          {#if currentConnected}
            <span class="badge badge-running" style="font-size:0.75rem;">Active</span>
          {/if}
        </div>
        <p class="text-xs text-muted" style="margin:2px 0 0 0;">
          Enables seamless 1-click repository imports and automatic webhook deployments without manual tokens.
        </p>
      </div>
    </div>

    <!-- Active Status Actions -->
    <div>
      {#if currentConnected}
        <div style="display:flex; align-items:center; gap:0.75rem; background:rgba(16,185,129,0.1); border:1px solid #10b981; border-radius:var(--radius-md); padding:8px 14px;">
          {#if currentConnected.avatar_url}
            <img src={currentConnected.avatar_url} alt="" style="width:24px; height:24px; border-radius:50%;" />
          {:else}
            <ShieldCheck size={20} style="color:#059669;" />
          {/if}
          <div style="font-size:0.875rem; font-weight:600; color:#065f46;">
            Connected as @{currentConnected.username}
          </div>
          <button 
            type="button" 
            class="btn btn-secondary" 
            style="padding:3px 10px; font-size:0.75rem; color:var(--color-error); border:none; margin-left:4px;"
            onclick={() => disconnectProvider(activeTab)}
          >
            Disconnect
          </button>
        </div>
      {:else if currentHasKeys}
        <button 
          type="button" 
          class="btn btn-primary" 
          style="padding:10px 20px; font-size:0.9375rem; background:{current.color}; border-color:transparent; display:flex; align-items:center; gap:8px;"
          onclick={() => authorizeProvider(activeTab)}
        >
          <FolderGit2 size={18} /> Authorize with {current.name} (1-Click)
        </button>
      {/if}
    </div>
  </div>

  {#if oauthSaved}
    <div style="background:#d1fae5;border:1px solid #6ee7b7;color:#065f46;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem">
      ✓ {current.name} App credentials saved successfully. Click "Authorize with {current.name}" above to link your account!
    </div>
  {/if}

  <!-- Step-by-Step Setup Guide -->
  <div style="background:rgba(0,0,0,0.02); padding:1.25rem; border-radius:var(--radius-md); border:1px solid var(--color-border); margin-bottom:1.5rem;">
    <div style="font-weight:700; font-size:0.9375rem; margin-bottom:0.75rem;">30-Second Setup Instructions for {current.name}:</div>
    
    <ol style="margin:0 0 1rem 1.25rem; padding:0; font-size:0.875rem; line-height:1.7; color:var(--color-ink);">
      <li>
        Open <strong><a href={current.devUrl} target="_blank" rel="noreferrer" style="color:var(--color-accent-dim); text-decoration:underline;">{current.name} Developer Settings <ExternalLink size={12} style="display:inline;" /></a></strong> and create a new application/consumer.
      </li>
      <li>Set <strong>Application Name</strong> to <code class="font-mono">kloudsPanel</code>.</li>
      <li>Set <strong>Homepage URL</strong> to <code class="font-mono">https://{hostDomain}</code>.</li>
      <li>
        Set <strong>Authorization Callback URL / Redirect URI</strong> to:
        <div style="display:flex; align-items:center; gap:0.5rem; margin-top:6px; margin-bottom:6px;">
          <input type="text" readonly value={currentCallbackUrl} class="form-input font-mono text-xs" style="max-width:540px;" />
          <button type="button" class="btn btn-secondary" style="padding:4px 10px; font-size:0.75rem;" onclick={copyCallback}>
            {#if copiedCallback}<Check size={14} style="color:var(--color-success);" /> Copied{:else}<Copy size={14} /> Copy Callback URL{/if}
          </button>
        </div>
      </li>
      <li>Set required <strong>Permissions / Scopes</strong>: <code class="font-mono text-xs">{current.scopes}</code>.</li>
      <li>Save the application on {current.name}, then copy and paste your credentials below.</li>
    </ol>

    <!-- App Credentials Form -->
    <form onsubmit={saveOAuth} style="display:grid; grid-template-columns:repeat(auto-fit, minmax(240px, 1fr)); gap:0.75rem; align-items:flex-end; border-top:1px solid var(--color-border); padding-top:1rem;">
      <div class="form-group" style="margin:0;">
        <label class="form-label" for="app-client-id" style="font-size:0.8125rem;">{current.keyLabel}</label>
        {#if activeTab === 'github'}
          <input id="app-client-id" type="text" class="form-input font-mono" placeholder="e.g. Iv1.8a2b3c4d5e6f7g8h" bind:value={githubClientId} required />
        {:else if activeTab === 'gitlab'}
          <input id="app-client-id" type="text" class="form-input font-mono" placeholder="e.g. 8a2b3c4d5e6f7g8h9i0j..." bind:value={gitlabClientId} required />
        {:else}
          <input id="app-client-id" type="text" class="form-input font-mono" placeholder="e.g. bitbucket_consumer_key..." bind:value={bitbucketClientId} required />
        {/if}
      </div>

      <div class="form-group" style="margin:0;">
        <label class="form-label" for="app-client-secret" style="font-size:0.8125rem;">{current.secretLabel}</label>
        {#if activeTab === 'github'}
          <input id="app-client-secret" type="password" class="form-input font-mono" placeholder="••••••••••••••••••••••••••••••••" bind:value={githubClientSecret} required />
        {:else if activeTab === 'gitlab'}
          <input id="app-client-secret" type="password" class="form-input font-mono" placeholder="••••••••••••••••••••••••••••••••" bind:value={gitlabClientSecret} required />
        {:else}
          <input id="app-client-secret" type="password" class="form-input font-mono" placeholder="••••••••••••••••••••••••••••••••" bind:value={bitbucketClientSecret} required />
        {/if}
      </div>

      <button type="submit" class="btn btn-primary" style="height:38px; padding:0 18px; font-size:0.8125rem;" disabled={savingOAuth}>
        {#if savingOAuth}<Loader2 size={14} class="animate-spin" /> Saving...{:else}<Key size={14} /> Save {current.name} Keys{/if}
      </button>
    </form>
  </div>

  <!-- Manual Personal Access Token Fallback Accordion -->
  <div style="border-top:1px solid var(--color-border); padding-top:1.25rem;">
    <button 
      type="button" 
      class="btn btn-secondary" 
      style="border:none; padding:4px 0; font-size:0.8125rem; color:var(--color-ink-secondary); text-decoration:underline;"
      onclick={() => showManualToken = !showManualToken}
    >
      {showManualToken ? 'Hide' : 'Alternative'}: Connect via Personal Access Token / App Password
    </button>

    {#if showManualToken}
      <div style="margin-top:1rem; padding:1.25rem; background:var(--color-canvas); border-radius:var(--radius-md); border:1px solid var(--color-border);">
        {#if manualSaved}
          <div style="background:#d1fae5;border:1px solid #6ee7b7;color:#065f46;border-radius:var(--radius-md);padding:0.5rem 1rem;font-size:0.8125rem;margin-bottom:1rem">
            ✓ {current.name} token connected successfully.
          </div>
        {/if}

        <form onsubmit={saveManualToken} style="display:grid; grid-template-columns:repeat(auto-fit, minmax(200px, 1fr)); gap:0.75rem; align-items:flex-end;">
          <div class="form-group" style="margin:0;">
            <label class="form-label" for="manual-username" style="font-size:0.8125rem;">Username</label>
            <input id="manual-username" type="text" class="form-input" placeholder="e.g. your-git-username" bind:value={manualUsername} required />
          </div>

          <div class="form-group" style="margin:0;">
            <label class="form-label" for="manual-token" style="font-size:0.8125rem;">Personal Access Token</label>
            <input id="manual-token" type="password" class="form-input font-mono" placeholder="ghp_..., glpat-..., or App Password" bind:value={manualToken} required />
          </div>

          <button type="submit" class="btn btn-secondary" style="height:38px; font-size:0.8125rem;" disabled={savingManual || !manualUsername || !manualToken}>
            {#if savingManual}<Loader2 size={14} class="animate-spin" /> Saving...{:else}<Plus size={14} /> Link Token{/if}
          </button>
        </form>
      </div>
    {/if}
  </div>
</div>
