<script lang="ts">
  import { onMount } from 'svelte';
  import { Settings, FolderGit2, Check, Loader2, Plus, Trash2 } from 'lucide-svelte';

  let rootDomain = $state('');
  let acmeEmail = $state('');
  let dnsMode = $state('http-01');
  let saving = $state(false);
  let saved = $state(false);
  let error = $state('');

  // Git Integrations
  let gitIntegrations = $state<any[]>([]);
  let selectedProvider = $state('github');
  let providerUsername = $state('');
  let providerToken = $state('');
  let savingGit = $state(false);
  let gitSaved = $state(false);

  async function loadData() {
    try {
      const [res, gitRes] = await Promise.all([
        fetch('/api/v1/admin/platform', { credentials: 'include' }),
        fetch('/api/v1/integrations/git', { credentials: 'include' })
      ]);
      if (res.ok) {
        const data = await res.json();
        rootDomain = data.settings?.root_domain ?? '';
        acmeEmail = data.settings?.acme_email ?? '';
        dnsMode = data.settings?.dns_mode ?? 'http-01';
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
    } catch { error = 'Network error'; }
    finally { saving = false; }
  }

  async function saveGitProvider(e: Event) {
    e.preventDefault();
    savingGit = true;
    gitSaved = false;
    try {
      const res = await fetch('/api/v1/integrations/git', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          provider: selectedProvider,
          username: providerUsername,
          token: providerToken
        })
      });
      if (res.ok) {
        gitSaved = true;
        providerToken = '';
        await loadData();
        setTimeout(() => gitSaved = false, 3000);
      }
    } finally {
      savingGit = false;
    }
  }

  async function disconnectGit(provider: string) {
    if (!confirm(`Disconnect ${provider}?`)) return;
    await fetch(`/api/v1/integrations/git/${provider}`, { method: 'DELETE', credentials: 'include' });
    await loadData();
  }
</script>

<svelte:head>
  <title>Platform Setup — kloudsPanel</title>
</svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Platform Setup</h1>
    <p class="page-subtitle">Configure root domain, TLS, networking, and Git provider integrations — main admin only</p>
  </div>
</div>

<div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(360px, 1fr)); gap: 1.5rem; margin-bottom: 3rem;">
  <!-- Domain & TLS Configuration -->
  <div class="card">
    <div class="card-header">
      <h3 style="margin:0;">Domain & TLS Configuration</h3>
    </div>

    {#if saved}
      <div style="background:#d1fae5;border:1px solid #6ee7b7;color:#065f46;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem">
        ✓ Configuration saved. Traefik will reload within 30 seconds.
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
        {#if saving}Saving & Verifying…{:else}Save Domain Config{/if}
      </button>
    </form>
  </div>

  <!-- Git Integrations Card -->
  <div class="card">
    <div class="card-header">
      <h3 style="margin:0;">Git Provider Integrations</h3>
      <p class="text-xs text-muted" style="margin-top:0.25rem;">Connect GitHub, Bitbucket, and GitLab accounts.</p>
    </div>

    {#if gitSaved}
      <div style="background:#d1fae5;border:1px solid #6ee7b7;color:#065f46;border-radius:var(--radius-md);padding:0.75rem 1rem;font-size:0.875rem;margin-bottom:1.25rem">
        ✓ Git provider connected successfully.
      </div>
    {/if}

    <!-- Connected Providers List -->
    <div style="display:flex; flex-direction:column; gap:0.75rem; margin-bottom:1.5rem;">
      {#each gitIntegrations as item}
        <div style="display:flex; align-items:center; justify-content:space-between; padding:0.75rem; border:1px solid var(--color-border); border-radius:var(--radius-md); background:var(--color-surface);">
          <div style="display:flex; align-items:center; gap:0.75rem;">
            <FolderGit2 size={20} style="color:var(--color-accent);" />
            <div>
              <div style="font-weight:600; text-transform:capitalize; font-size:0.9375rem;">{item.provider}</div>
              <div class="text-xs text-muted">
                {#if item.connected}
                  Connected as <span class="font-mono" style="color:var(--color-ink); font-weight:600;">@{item.username}</span>
                {:else}
                  Not connected (Public cloning enabled)
                {/if}
              </div>
            </div>
          </div>

          {#if item.connected}
            <button class="btn btn-secondary" style="padding:4px 8px; color:var(--color-error); font-size:0.75rem;" onclick={() => disconnectGit(item.provider)}>
              Disconnect
            </button>
          {:else}
            <span class="badge" style="background:#f1f5f9; color:#475569;">Ready</span>
          {/if}
        </div>
      {/each}
    </div>

    <!-- Link Provider Form -->
    <form onsubmit={saveGitProvider} style="border-top:1px solid var(--color-border); padding-top:1.25rem;">
      <h4 style="margin:0 0 1rem 0; font-size:0.9375rem;">Link Account / Access Token</h4>
      
      <div style="display:grid; grid-template-columns:1fr 1fr; gap:0.75rem; margin-bottom:0.75rem;">
        <div class="form-group" style="margin:0;">
          <label class="form-label" for="git-prov-select">Provider</label>
          <select id="git-prov-select" class="form-select" bind:value={selectedProvider}>
            <option value="github">GitHub</option>
            <option value="bitbucket">Bitbucket</option>
            <option value="gitlab">GitLab</option>
          </select>
        </div>

        <div class="form-group" style="margin:0;">
          <label class="form-label" for="git-prov-user">Username</label>
          <input id="git-prov-user" type="text" class="form-input" placeholder="e.g. vedantjja" bind:value={providerUsername} required />
        </div>
      </div>

      <div class="form-group" style="margin-bottom:1rem;">
        <label class="form-label" for="git-prov-token">Personal Access Token / Password</label>
        <input id="git-prov-token" type="password" class="form-input font-mono text-xs" placeholder="ghp_... or Bitbucket token" bind:value={providerToken} required />
      </div>

      <button type="submit" class="btn btn-secondary" disabled={savingGit || !providerUsername || !providerToken}>
        {#if savingGit}<Loader2 size={14} class="animate-spin" /> Saving...{:else}<Plus size={14} /> Connect {selectedProvider}{/if}
      </button>
    </form>
  </div>
</div>
