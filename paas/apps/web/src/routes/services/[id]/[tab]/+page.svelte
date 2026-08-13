<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import LogViewer from '$lib/components/logs/LogViewer.svelte';
  import { Loader2, ExternalLink, Square, Rocket } from 'lucide-svelte';

  const { id, tab } = $derived($page.params);
  const tabs = ['overview', 'deployments', 'logs', 'terminal', 'variables', 'domains', 'scale', 'resources', 'settings'];

  let service = $state<any>(null);
  let deployments = $state<any[]>([]);
  let loading = $state(true);

  onMount(async () => {
    try {
      const res = await fetch(`/api/v1/services/${id}`, { credentials: 'include' });
      if (!res.ok) { goto('/workspaces'); return; }
      service = await res.json();
      const depRes = await fetch(`/api/v1/services/${id}/deployments`, { credentials: 'include' });
      deployments = (await depRes.json()).deployments ?? [];
    } finally {
      loading = false;
    }
  });

  const statusColor = (s: string) => {
    const m: Record<string, string> = {
      running: '#065f46', healthy: '#065f46', stopped: '#64748b',
      failed: '#991b1b', building: '#92400e', deploying: '#4c1d95', draft: '#64748b',
    };
    return m[s] ?? '#16202a';
  };
</script>

<svelte:head>
  <title>{service?.name ?? 'Service'} — kloudsPanel</title>
</svelte:head>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={48} /></div>
    <p>Loading…</p>
  </div>
{:else}
  <!-- Service header -->
  <div class="page-header">
    <div style="flex:1;min-width:0">
      <p class="text-xs text-muted" style="margin-bottom:0.25rem">
        <a href="/workspaces">Workspaces</a> / Project /
      </p>
      <div style="display:flex;align-items:center;gap:1rem;flex-wrap:wrap">
        <h1 class="page-title" style="margin:0">{service?.name}</h1>
        <span class="badge badge-{service?.runtime_status}">{service?.runtime_status}</span>
      </div>
      {#if service?.domain}
        <a href="https://{service.domain}" target="_blank" rel="noopener" class="text-xs"
           style="color:var(--color-accent);margin-top:0.25rem;display:inline-block">
          <ExternalLink size={12} style="display:inline;vertical-align:middle" /> {service.domain}
        </a>
      {/if}
    </div>
    <div style="display:flex;gap:0.5rem">
      <button class="btn btn-secondary"><Square size={14} fill="currentColor" /> Stop</button>
      <button class="btn btn-primary"><Rocket size={14} /> Deploy</button>
    </div>
  </div>

  <!-- Tabs -->
  <div style="display:flex;gap:0;border-bottom:2px solid var(--color-border);margin-bottom:1.5rem;overflow-x:auto">
    {#each tabs as t}
      <a
        href="/services/{id}/{t}"
        style="
          padding:0.625rem 1rem;font-size:0.875rem;font-weight:500;
          color:{tab === t ? 'var(--color-accent)' : 'var(--color-ink-secondary)'};
          border-bottom:2px solid {tab === t ? 'var(--color-accent)' : 'transparent'};
          margin-bottom:-2px;white-space:nowrap;text-decoration:none;
          transition:color 0.15s;
        "
      >{t.charAt(0).toUpperCase() + t.slice(1)}</a>
    {/each}
  </div>

  <!-- Tab content -->
  {#if tab === 'overview'}
    <div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:1rem;margin-bottom:1.5rem">
      {#each [
        { label: 'Kind', value: service?.kind },
        { label: 'Status', value: service?.runtime_status },
        { label: 'Port', value: service?.internal_port ?? '—' },
        { label: 'Auto Deploy', value: service?.auto_deploy ? 'Yes' : 'No' },
      ] as stat}
        <div class="card" style="padding:1rem">
          <div class="text-xs text-muted" style="margin-bottom:0.25rem">{stat.label}</div>
          <div style="font-size:1rem;font-weight:600">{stat.value}</div>
        </div>
      {/each}
    </div>
    <!-- Latest deployment -->
    {#if deployments.length > 0}
      <h3 style="font-size:0.9375rem;margin-bottom:0.75rem">Latest Deployment</h3>
      <div class="card">
        <div style="display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:0.5rem">
          <div>
            <span class="font-mono text-xs">{deployments[0].id}</span>
            <span class="badge badge-{deployments[0].status}" style="margin-left:0.5rem">{deployments[0].status}</span>
          </div>
          <span class="text-xs text-muted">{deployments[0].trigger} • seq #{deployments[0].sequence}</span>
        </div>
      </div>
    {/if}

  {:else if tab === 'logs'}
    <LogViewer serviceId={id as string} deploymentId={deployments[0]?.id as string | undefined} />

  {:else if tab === 'deployments'}
    <div class="table-wrapper">
      <table>
        <thead><tr><th>#</th><th>Status</th><th>Trigger</th><th>Driver</th><th>Started</th></tr></thead>
        <tbody>
          {#each deployments as dep}
            <tr>
              <td class="font-mono text-xs">{dep.sequence}</td>
              <td><span class="badge badge-{dep.status}">{dep.status}</span></td>
              <td>{dep.trigger}</td>
              <td>{dep.build_driver}</td>
              <td class="text-xs text-muted">{dep.started_at ?? '—'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>

  {:else if tab === 'variables'}
    <div class="card">
      <div class="card-header">
        <h3>Environment Variables</h3>
        <button class="btn btn-secondary" style="font-size:0.8125rem;padding:4px 12px;min-height:32px">+ Add Variable</button>
      </div>
      <p class="text-sm text-muted">No environment variables configured.</p>
    </div>

  {:else if tab === 'domains'}
    <div class="card">
      <div class="card-header"><h3>Custom Domains</h3></div>
      <p class="text-sm text-muted">Configure custom domain routing for this service.</p>
    </div>

  {:else if tab === 'scale'}
    <div class="card">
      <div class="card-header"><h3>Scale Configuration</h3></div>
      <div style="display:flex;align-items:center;justify-content:space-between">
        <div>
          <div style="font-weight:500">Scale to Zero</div>
          <div class="text-sm text-muted">Pause this service when idle to save resources</div>
        </div>
        <label style="position:relative;width:44px;height:24px;cursor:pointer">
          <input type="checkbox" style="opacity:0;width:0;height:0" />
          <span style="
            position:absolute;inset:0;background:var(--color-border);border-radius:999px;
            transition:0.2s;
          "></span>
        </label>
      </div>
    </div>

  {:else if tab === 'settings'}
    <div class="card">
      <div class="card-header"><h3 style="color:var(--color-danger)">Danger Zone</h3></div>
      <div style="display:flex;align-items:center;justify-content:space-between">
        <div>
          <div style="font-weight:500">Delete Service</div>
          <div class="text-sm text-muted">Permanently delete this service and all deployments.</div>
        </div>
        <button class="btn btn-danger" style="font-size:0.875rem">Delete Service</button>
      </div>
    </div>

  {:else}
    <p class="text-muted">Tab content coming soon.</p>
  {/if}
{/if}
