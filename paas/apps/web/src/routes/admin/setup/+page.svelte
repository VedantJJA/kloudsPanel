<script lang="ts">
  import { onMount } from 'svelte';

  let rootDomain = $state('');
  let acmeEmail = $state('');
  let dnsMode = $state('http-01');
  let saving = $state(false);
  let saved = $state(false);
  let error = $state('');

  onMount(async () => {
    const res = await fetch('/api/v1/admin/platform', { credentials: 'include' });
    const data = await res.json();
    rootDomain = data.settings?.root_domain ?? '';
    acmeEmail = data.settings?.acme_email ?? '';
    dnsMode = data.settings?.dns_mode ?? 'http-01';
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
</script>

<svelte:head>
  <title>Platform Setup — kloudsPanel</title>
</svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Platform Setup</h1>
    <p class="page-subtitle">Configure root domain, TLS, and networking — main admin only</p>
  </div>
</div>

<div class="card" style="max-width:600px">
  <div class="card-header">
    <h3>Domain & TLS Configuration</h3>
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
        Control plane will be at <strong>https://{rootDomain || 'yourdomain.com'}</strong>.
        Apps at <strong>https://appname.{rootDomain || 'yourdomain.com'}</strong>.
      </p>
    </div>

    <div class="form-group">
      <label class="form-label" for="acmeEmail">ACME Email</label>
      <input id="acmeEmail" type="email" class="form-input" bind:value={acmeEmail}
        placeholder="admin@yourdomain.com" required/>
      <p class="text-xs text-muted" style="margin-top:0.25rem">Used for Let's Encrypt certificate notifications.</p>
    </div>

    <div class="form-group">
      <label class="form-label" for="dnsMode">DNS Challenge Mode</label>
      <select id="dnsMode" class="form-select" bind:value={dnsMode}>
        <option value="http-01">HTTP-01 (recommended for most setups)</option>
        <option value="dns-01">DNS-01 (required for wildcard certs)</option>
      </select>
    </div>

    <div style="
      background:#fef3c7;border:1px solid #fbbf24;border-radius:var(--radius-md);
      padding:0.875rem 1rem;font-size:0.875rem;margin-bottom:1.25rem;
    ">
      ⚠ <strong>Important:</strong> Ensure your DNS record <code>*.{rootDomain || 'yourdomain.com'}</code>
      points to this server's IP before saving.
    </div>

    <button type="submit" class="btn btn-primary" disabled={saving}>
      {#if saving}Saving & Verifying…{:else}Save Configuration{/if}
    </button>
  </form>
</div>
