<script lang="ts">
  import { onMount } from 'svelte';
  import { Settings, FolderGit2, Check, Loader2, Plus, Trash2, Key, Globe, ShieldCheck, Copy, ExternalLink, Sparkles } from 'lucide-svelte';

  let rootDomain = $state('');
  let acmeEmail = $state('');
  let dnsMode = $state('http-01');
  let saving = $state(false);
  let saved = $state(false);
  let error = $state('');

  // OAuth App Credentials
  let githubClientId = $state('');
  let githubClientSecret = $state('');
  let savingOAuth = $state(false);
  let oauthSaved = $state(false);
  let copiedCallback = $state(false);

  // Git Integrations
  let gitIntegrations = $state<any[]>([]);
  let selectedProvider = $state('github');
  let providerUsername = $state('');
  let providerToken = $state('');
  let savingGit = $state(false);
  let gitSaved = $state(false);

  let callbackUrl = $derived(`https://${rootDomain || 'klouds.online'}/api/v1/integrations/git/github/callback`);

  async function loadData() {
    try {
      const [res, gitRes] = await Promise.all([
        fetch('/api/v1/admin/settings', { credentials: 'include' }),
        fetch('/api/v1/integrations/git', { credentials: 'include' })
      ]);
      if (res.ok) {
        const data = await res.json();
        rootDomain = data.settings?.root_domain ?? '';
        acmeEmail = data.settings?.acme_email ?? '';
        dnsMode = data.settings?.dns_mode ?? 'http-01';
        githubClientId = data.settings?.github_client_id ?? '';
        githubClientSecret = data.settings?.github_client_secret ?? '';
      }
      if (gitRes.ok) {
        gitIntegrations = (await gitRes.json()).integrations ?? [];
      }
    } catch {}
  }

  onMount(() => {
    loadData();
  });

  async function handleSave(e: Event) {
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

  async function saveOAuthCredentials(e: Event) {
    e.preventDefault();
    savingOAuth = true;
    oauthSaved = false;
    try {
      const res = await fetch('/api/v1/admin/settings', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          github_client_id: githubClientId,
          github_client_secret: githubClientSecret
        })
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

  async function copyCallback() {
    navigator.clipboard.writeText(callbackUrl);
    copiedCallback = true;
    setTimeout(() => copiedCallback = false, 2000);
  }

  async function authorizeWithGitHub() {
    window.location.href = `/api/v1/integrations/git/github/authorize?return_to=${encodeURIComponent('/admin/setup')}`;
  }

  async function disconnectGit(provider: string) {
    if (!confirm(`Disconnect ${provider}?`)) return;
    await fetch(`/api/v1/integrations/git/${provider}`, { method: 'DELETE', credentials: 'include' });
    await loadData();
  }
</script>

<svelte:head>
  <title>Platform Setup - kloudsPanel</title>
</svelte:head>

<div class="page-header" style="margin-bottom: 1.5rem;">
  <div>
    <h1 class="page-title">Platform Setup</h1>
    <p class="page-subtitle">Configure root domain, TLS, networking, and 1-Click GitHub App authorization - main admin only</p>
  </div>
</div>

<!-- 1-Click GitHub App Authorization Card (Primary) -->
<div class="card" style="margin-bottom: 2rem; padding: 1.5rem; border: 2px solid var(--color-accent); background: var(--color-surface);">
  <div style="display:flex; justify-content:space-between; align-items:flex-start; flex-wrap:wrap; gap:1rem; margin-bottom:1.25rem;">
    <div style="display:flex; align-items:center; gap:0.75rem;">
      <div style="width:44px; height:44px; border-radius:var(--radius-md); background:#24292f; color:#fff; display:flex; align-items:center; justify-content:center;">
        <FolderGit2 size={24} />
      </div>
      <div>
        <div style="font-size:1.1rem; font-weight:700; color:var(--color-ink); display:flex; align-items:center; gap:6px;">
          GitHub 1-Click App Authorization
          <span class="badge badge-running" style="font-size:0.7rem;">Official Flow</span>
        </div>
        <p class="text-xs text-muted" style="margin:2px 0 0 0;">
          Allow kloudsPanel to authorize with GitHub like Vercel and Render without using personal access tokens.
        </p>
      </div>
    </div>

    <!-- Live Status / Connect Button -->
    {#if gitIntegrations.find(g => g.provider === 'github' && g.connected)}
      {@const gh = gitIntegrations.find(g => g.provider === 'github')}
      <div style="display:flex; align-items:center; gap:0.75rem; background:rgba(16,185,129,0.1); border:1px solid #10b981; border-radius:var(--radius-md); padding:6px 12px;">
        <ShieldCheck size={18} style="color:#059669;" />
        <span style="font-size:0.875rem; font-weight:600; color:#065f46;">
          Connected as @{gh.username}
        </span>
        <button 
          type="button" 
          class="btn btn-secondary" 
          style="padding:2px 8px; font-size:0.75rem; color:var(--color-error); border:none;"
          onclick={() => disconnectGit('github')}
        >
          Disconnect
        </button>
      </div>
    {:else if githubClientId}
      <button 
        type="button" 
        class="btn btn-primary" 
        style="padding:8px 18px; font-size:0.875rem; background:#24292f; border-color:transparent; display:flex; align-items:center; gap:8px;"
        onclick={authorizeWithGitHub}
      >
        <FolderGit2 size={16} /> Authorize with GitHub (1-Click)
      </button>
    {/if}
  </div>

  {#if oauthSaved}
    <div style="background:#d1fae5;border:1px solid #6ee7b7;color:#065f46;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem">
      ✓ GitHub OAuth credentials saved. You can now click "Authorize with GitHub" above!
    </div>
  {/if}

  <!-- Configuration Steps -->
  <div style="background:rgba(0,0,0,0.02); padding:1.25rem; border-radius:var(--radius-md); border:1px solid var(--color-border); margin-bottom:1.25rem;">
    <div style="font-weight:700; font-size:0.875rem; margin-bottom:0.6rem;">Quick 30-Second Setup on GitHub:</div>
    <ol style="margin:0 0 1rem 1.25rem; padding:0; font-size:0.8125rem; line-height:1.6; color:var(--color-ink);">
      <li>Go to <strong><a href="https://github.com/settings/developers" target="_blank" rel="noreferrer" style="color:var(--color-accent-dim);">GitHub Developer Settings → OAuth Apps</a></strong> and click <strong>"New OAuth App"</strong>.</li>
      <li>Set <strong>Homepage URL</strong> to <code class="font-mono">https://{rootDomain || 'klouds.online'}</code>.</li>
      <li>
        Set <strong>Authorization callback URL</strong> to:
        <div style="display:flex; align-items:center; gap:0.5rem; margin-top:4px;">
          <input type="text" readonly value={callbackUrl} class="form-input font-mono text-xs" style="max-width:480px;" />
          <button type="button" class="btn btn-secondary" style="padding:4px 8px; font-size:0.75rem;" onclick={copyCallback}>
            {#if copiedCallback}<Check size={14} style="color:var(--color-success);" /> Copied{:else}<Copy size={14} /> Copy{/if}
          </button>
        </div>
      </li>
      <li>Click <strong>Register application</strong>, then copy your <strong>Client ID</strong> and generate a <strong>Client Secret</strong> below.</li>
    </ol>

    <!-- Credentials Form -->
    <form onsubmit={saveOAuthCredentials} style="display:grid; grid-template-columns:repeat(auto-fit, minmax(280px, 1fr)) auto; gap:0.75rem; align-items:flex-end;">
      <div class="form-group" style="margin:0;">
        <label class="form-label" for="gh-client-id" style="font-size:0.8125rem;">GitHub Client ID</label>
        <input 
          id="gh-client-id" 
          type="text" 
          class="form-input font-mono" 
          placeholder="e.g. Iv1.8a2b3c4d5e6f7g8h" 
          bind:value={githubClientId} 
          required 
        />
      </div>

      <div class="form-group" style="margin:0;">
        <label class="form-label" for="gh-client-secret" style="font-size:0.8125rem;">GitHub Client Secret</label>
        <input 
          id="gh-client-secret" 
          type="password" 
          class="form-input font-mono" 
          placeholder="••••••••••••••••••••••••••••••••" 
          bind:value={githubClientSecret} 
          required 
        />
      </div>

      <button type="submit" class="btn btn-primary" style="height:38px; padding:0 16px; font-size:0.8125rem;" disabled={savingOAuth || !githubClientId || !githubClientSecret}>
        {#if savingOAuth}<Loader2 size={14} class="animate-spin" /> Saving...{:else}<Key size={14} /> Save App Keys{/if}
      </button>
    </form>
  </div>
</div>

<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(360px, 1fr)); gap: 1.5rem; margin-bottom: 3rem;">
  <!-- Domain & TLS Configuration -->
  <div class="card">
    <div class="card-header">
      <h3 style="margin:0;">Root Domain & TLS</h3>
      <p class="text-xs text-muted" style="margin-top:0.25rem;">Master domain and Let's Encrypt certificate settings.</p>
    </div>

    {#if saved}
      <div style="background:#d1fae5;border:1px solid #6ee7b7;color:#065f46;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem">
        ✓ Configuration saved.
      </div>
    {/if}
    {#if error}
      <div style="background:#fee2e2;border:1px solid #fca5a5;color:#991b1b;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem">
        {error}
      </div>
    {/if}

    <form onsubmit={handleSave}>
      <div class="form-group">
        <label class="form-label" for="rootDomain">Root Domain</label>
        <input id="rootDomain" type="text" class="form-input" bind:value={rootDomain}
          placeholder="yourdomain.com" required />
        <p class="text-xs text-muted" style="margin-top:0.25rem">
          Control plane: <strong>https://{rootDomain || 'yourdomain.com'}</strong>
        </p>
      </div>

      <div class="form-group">
        <label class="form-label" for="acmeEmail">ACME Email</label>
        <input id="acmeEmail" type="email" class="form-input" bind:value={acmeEmail}
          placeholder="admin@yourdomain.com" required/>
        <p class="text-xs text-muted" style="margin-top:0.25rem">Used for Let's Encrypt certificate renewals.</p>
      </div>

      <div class="form-group">
        <label class="form-label" for="dnsMode">DNS Challenge Mode</label>
        <select id="dnsMode" class="form-select" bind:value={dnsMode}>
          <option value="http-01">HTTP-01 (standard standalone server)</option>
          <option value="dns-01">DNS-01 (wildcard certificates)</option>
        </select>
      </div>

      <div style="background:#fef3c7;border:1px solid #fbbf24;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.8125rem;margin-bottom:1.25rem;">
        ⚠ <strong>DNS Requirement:</strong> Ensure <code>*.{rootDomain || 'yourdomain.com'}</code> resolves to this server's IP.
      </div>

      <button type="submit" class="btn btn-primary" disabled={saving}>
        {#if saving}Saving & Verifying...{:else}Save Domain Config{/if}
      </button>
    </form>
  </div>

  <!-- Connected Git Accounts -->
  <div class="card">
    <div class="card-header">
      <h3 style="margin:0;">Connected Git Accounts</h3>
      <p class="text-xs text-muted" style="margin-top:0.25rem;">Active provider authorizations for your account.</p>
    </div>

    <!-- Connected Providers List -->
    <div style="display:flex; flex-direction:column; gap:0.75rem;">
      {#each gitIntegrations as item}
        <div style="display:flex; align-items:center; justify-content:space-between; padding:0.85rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-surface);">
          <div style="display:flex; align-items:center; gap:0.75rem;">
            <FolderGit2 size={20} style="color:var(--color-accent);" />
            <div>
              <div style="font-weight:600; text-transform:capitalize; font-size:0.9375rem;">{item.provider}</div>
              <div class="text-xs text-muted">
                {#if item.connected}
                  Connected as <span class="font-mono" style="color:var(--color-ink); font-weight:600;">@{item.username}</span>
                {:else}
                  Not connected
                {/if}
              </div>
            </div>
          </div>

          {#if item.connected}
            <button class="btn btn-secondary" style="padding:4px 8px; color:var(--color-error); font-size:0.75rem;" onclick={() => disconnectGit(item.provider)}>
              Disconnect
            </button>
          {:else if item.provider === 'github' && githubClientId}
            <button class="btn btn-primary" style="padding:4px 10px; font-size:0.75rem; background:#24292f; border-color:transparent;" onclick={authorizeWithGitHub}>
              Connect
            </button>
          {:else}
            <span class="badge" style="background:#f1f5f9; color:#475569;">Ready</span>
          {/if}
        </div>
      {/each}
    </div>
  </div>
</div>
