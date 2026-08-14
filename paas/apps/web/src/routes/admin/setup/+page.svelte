<script lang="ts">
  import { onMount } from 'svelte';
  import { Settings, FolderGit2, Check, Loader2, Globe, ArrowRight, ShieldCheck } from 'lucide-svelte';

  let rootDomain = $state('');
  let acmeEmail = $state('');
  let dnsMode = $state('http-01');
  let saving = $state(false);
  let saved = $state(false);
  let error = $state('');

  let gitIntegrations = $state<any[]>([]);

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
</script>

<svelte:head>
  <title>Platform Setup - kloudsPanel</title>
</svelte:head>

<div class="page-header" style="margin-bottom: 1.5rem;">
  <div>
    <h1 class="page-title">Platform Setup</h1>
    <p class="page-subtitle">Configure root domain, TLS certificates, and core infrastructure - main admin only</p>
  </div>
</div>

<!-- Quick Link to Dedicated Git Providers Setup Page -->
<div class="card" style="margin-bottom: 2rem; padding: 1.25rem 1.5rem; background: linear-gradient(135deg, rgba(0,166,166,0.08) 0%, rgba(11,31,58,0.04) 100%); border: 1px solid rgba(0,166,166,0.3); display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 1rem;">
  <div style="display: flex; align-items: center; gap: 1rem;">
    <div style="width: 44px; height: 44px; border-radius: var(--radius-md); background: var(--color-accent); color: #fff; display: flex; align-items: center; justify-content: center; flex-shrink: 0;">
      <FolderGit2 size={24} />
    </div>
    <div>
      <div style="font-weight: 700; font-size: 1rem; color: var(--color-ink);">Git Provider 1-Click Authorizations</div>
      <p class="text-xs text-muted" style="margin: 2px 0 0 0;">
        Configure dedicated 1-Click OAuth apps and authorizations for GitHub, GitLab, and Bitbucket.
      </p>
    </div>
  </div>

  <a href="/admin/git-providers" class="btn btn-primary" style="display: flex; align-items: center; gap: 6px;">
    Open Git Providers Setup <ArrowRight size={15} />
  </a>
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
        Configuration saved.
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
        <strong>DNS Requirement:</strong> Ensure <code>*.{rootDomain || 'yourdomain.com'}</code> resolves to this server's IP.
      </div>

      <button type="submit" class="btn btn-primary" disabled={saving}>
        {#if saving}Saving & Verifying...{:else}Save Domain Config{/if}
      </button>
    </form>
  </div>

  <!-- Connected Git Accounts Overview -->
  <div class="card">
    <div class="card-header">
      <h3 style="margin:0;">Active Git Authorizations</h3>
      <p class="text-xs text-muted" style="margin-top:0.25rem;">Live provider connections for this server.</p>
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
            <span class="badge badge-running" style="font-size:0.75rem;">Active</span>
          {:else}
            <a href="/admin/git-providers" class="btn btn-secondary" style="padding:4px 10px; font-size:0.75rem;">
              Setup
            </a>
          {/if}
        </div>
      {/each}
    </div>
  </div>
</div>
