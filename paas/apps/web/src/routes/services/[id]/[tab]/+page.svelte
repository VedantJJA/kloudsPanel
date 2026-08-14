<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount, onDestroy } from 'svelte';
  import LogViewer from '$lib/components/logs/LogViewer.svelte';
  import {
    Loader2,
    ExternalLink,
    Square,
    Play,
    Rocket,
    Trash2,
    Plus,
    X,
    Save,
    RefreshCw,
    Copy,
    Check,
    Globe,
    Server,
    Clock,
    Layers,
    ShieldCheck
  } from 'lucide-svelte';

  const { id, tab } = $derived($page.params);
  const tabs = ['overview', 'deployments', 'logs', 'variables', 'domains', 'scale', 'settings'];

  let service = $state<any>(null);
  let deployments = $state<any[]>([]);
  let loading = $state(true);
  let actionLoading = $state(false);
  let copiedUrl = $state(false);
  let bannerNotice = $state<{ type: 'success' | 'error'; message: string } | null>(null);
  let pollTimer: any = null;

  // Variables state
  let envVars = $state<Array<{ key: string; value: string }>>([]);
  let envSaving = $state(false);
  let envSuccess = $state(false);

  // Custom Domains state
  let customDomainsList = $state<any[]>([]);
  let newDomainInput = $state('');
  let domainSaving = $state(false);
  let domainNotice = $state<{ type: 'success' | 'error'; message: string } | null>(null);

  async function loadDomains() {
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/domains`, { credentials: 'include' });
      if (res.ok) {
        const d = await res.json();
        customDomainsList = d.domains ?? [];
      }
    } catch {}
  }

  async function addCustomDomain() {
    if (!newDomainInput.trim()) return;
    domainSaving = true;
    domainNotice = null;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/domains`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ domain: newDomainInput.trim() })
      });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        domainNotice = { type: 'error', message: d.error || 'Failed to add domain' };
      } else {
        const d = await res.json();
        customDomainsList = d.domains ?? [];
        newDomainInput = '';
        domainNotice = { type: 'success', message: 'Custom domain saved and TLS certificate configured with Let\'s Encrypt!' };
      }
    } catch (e: any) {
      domainNotice = { type: 'error', message: e.message || 'Network error' };
    } finally {
      domainSaving = false;
    }
  }

  async function removeCustomDomain(domainName: string) {
    if (!confirm(`Are you sure you want to remove ${domainName}?`)) return;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/domains/${encodeURIComponent(domainName)}`, {
        method: 'DELETE',
        credentials: 'include'
      });
      if (res.ok) {
        const d = await res.json();
        customDomainsList = d.domains ?? [];
        domainNotice = { type: 'success', message: `Domain ${domainName} removed.` };
      }
    } catch {}
  }

  async function loadService() {
    try {
      const res = await fetch(`/api/v1/services/${id}`, { credentials: 'include' });
      if (!res.ok) { goto('/workspaces'); return; }
      service = await res.json();

      loadDomains();
      
      // Parse existing env vars and service settings if present in resource_json
      try {
        if (service.resource_json || service.ResourceJSON) {
          const r = JSON.parse(service.resource_json || service.ResourceJSON);
          if (r.env && typeof r.env === 'object') {
            envVars = Object.entries(r.env).map(([k, v]) => ({ key: k, value: String(v) }));
          }
          if (!settingsDirty) {
            settingsName = service.name || service.Name || '';
            settingsPreset = r.presetId || service.kind || 'node';
            settingsBuildCmd = r.buildCommand || '';
            settingsStartCmd = r.startCommand || '';
            settingsRootDir = r.rootDirectory || r.rootDir || '.';
            settingsBranch = r.gitBranch || 'main';
            settingsRepoUrl = r.gitRepoUrl || '';
            settingsPort = service.internal_port || service.InternalPort || 80;
            settingsAutoDeploy = service.auto_deploy !== false;
          }
        }
      } catch {}

      const targetId = service?.id || id;
      const depRes = await fetch(`/api/v1/services/${targetId}/deployments`, { credentials: 'include' });
      if (depRes.ok) {
        deployments = (await depRes.json()).deployments ?? [];
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadService();
    pollTimer = setInterval(() => {
      loadService();
    }, 2500);
  });

  onDestroy(() => {
    if (pollTimer) clearInterval(pollTimer);
  });

  async function triggerDeploy() {
    actionLoading = true;
    bannerNotice = null;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/deploy`, { method: 'POST', credentials: 'include' });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        bannerNotice = { type: 'error', message: d.error || 'Failed to trigger deployment' };
      } else {
        bannerNotice = { type: 'success', message: 'Deployment initiated! Compiling and launching container in background.' };
        if (tab !== 'logs') {
          goto(`/services/${id}/logs`);
        }
      }
      await loadService();
    } catch (e: any) {
      bannerNotice = { type: 'error', message: 'Error: ' + e.message };
    } finally {
      actionLoading = false;
    }
  }

  async function stopService() {
    actionLoading = true;
    bannerNotice = null;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/stop`, { method: 'POST', credentials: 'include' });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        bannerNotice = { type: 'error', message: d.error || 'Failed to stop service' };
      } else {
        bannerNotice = { type: 'success', message: 'Service container stopped' };
      }
      await loadService();
    } catch (e: any) {
      bannerNotice = { type: 'error', message: 'Error: ' + e.message };
    } finally {
      actionLoading = false;
    }
  }

  async function startService() {
    actionLoading = true;
    bannerNotice = null;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/start`, { method: 'POST', credentials: 'include' });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        bannerNotice = { type: 'error', message: d.error || 'Failed to start service' };
      } else {
        bannerNotice = { type: 'success', message: 'Service container started' };
      }
      await loadService();
    } catch (e: any) {
      bannerNotice = { type: 'error', message: 'Error: ' + e.message };
    } finally {
      actionLoading = false;
    }
  }

  async function deleteService() {
    if (!confirm(`Are you sure you want to permanently delete "${service?.name || service?.Name}"? This action cannot be undone.`)) return;
    actionLoading = true;
    bannerNotice = null;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}`, { method: 'DELETE', credentials: 'include' });
      if (res.ok) {
        if (service?.project_id || service?.ProjectID) {
          goto(`/projects/${service.project_id || service.ProjectID}`);
        } else {
          goto('/workspaces');
        }
      } else {
        const d = await res.json().catch(() => ({}));
        bannerNotice = { type: 'error', message: d.error || 'Failed to delete service' };
        actionLoading = false;
      }
    } catch (e: any) {
      bannerNotice = { type: 'error', message: 'Error: ' + e.message };
      actionLoading = false;
    }
  }

  function copyEndpoint() {
    const url = service?.endpoint_url || `https://${service?.domain}`;
    if (url) {
      navigator.clipboard.writeText(url);
      copiedUrl = true;
      setTimeout(() => copiedUrl = false, 2500);
    }
  }

  function addEnv() {
    envVars = [...envVars, { key: '', value: '' }];
  }

  function removeEnv(index: number) {
    envVars = envVars.filter((_, i) => i !== index);
  }

  async function saveEnvVars() {
    envSaving = true;
    envSuccess = false;
    bannerNotice = null;
    try {
      const envMap: Record<string, string> = {};
      for (const item of envVars) {
        if (item.key.trim()) {
          envMap[item.key.trim()] = item.value;
        }
      }
      let currentR: any = {};
      try {
        currentR = JSON.parse(service.resource_json || service.ResourceJSON || '{}');
      } catch {}
      currentR.env = envMap;

      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ resourceJson: JSON.stringify(currentR) })
      });
      if (res.ok) {
        envSuccess = true;
        bannerNotice = { type: 'success', message: 'Environment variables saved successfully' };
        setTimeout(() => envSuccess = false, 3000);
      } else {
        const d = await res.json().catch(() => ({}));
        bannerNotice = { type: 'error', message: d.error || 'Failed to save environment variables' };
      }
    } catch (e: any) {
      bannerNotice = { type: 'error', message: 'Error: ' + e.message };
    } finally {
      envSaving = false;
    }
  }

  // Settings Tab State
  let settingsName = $state('');
  let settingsBuildCmd = $state('');
  let settingsStartCmd = $state('');
  let settingsRootDir = $state('.');
  let settingsBranch = $state('main');
  let settingsRepoUrl = $state('');
  let settingsPort = $state<number>(80);
  let settingsAutoDeploy = $state(true);
  let settingsPreset = $state('node');
  let settingsSaving = $state(false);
  let settingsSaved = $state(false);
  let settingsError = $state('');
  let settingsDirty = $state(false);

  async function saveServiceSettings(e: Event) {
    e.preventDefault();
    settingsSaving = true;
    settingsSaved = false;
    settingsError = '';
    bannerNotice = null;
    try {
      let currentR: any = {};
      try {
        currentR = JSON.parse(service.resource_json || service.ResourceJSON || '{}');
      } catch {}
      currentR.buildCommand = settingsBuildCmd;
      currentR.startCommand = settingsStartCmd;
      currentR.rootDirectory = settingsRootDir;
      currentR.gitBranch = settingsBranch;
      currentR.gitRepoUrl = settingsRepoUrl;
      currentR.presetId = settingsPreset;

      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          name: settingsName,
          internalPort: Number(settingsPort),
          autoDeploy: settingsAutoDeploy,
          resourceJson: JSON.stringify(currentR)
        })
      });

      if (res.ok) {
        settingsSaved = true;
        settingsDirty = false;
        bannerNotice = { type: 'success', message: 'Service settings saved successfully' };
        await loadService();
        setTimeout(() => settingsSaved = false, 3000);
      } else {
        const d = await res.json().catch(() => ({}));
        settingsError = d.error || 'Failed to update service settings';
      }
    } catch (e: any) {
      settingsError = 'Error: ' + e.message;
    } finally {
      settingsSaving = false;
    }
  }

  async function restartService() {
    actionLoading = true;
    bannerNotice = null;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/restart`, { method: 'POST', credentials: 'include' });
      if (res.ok) {
        bannerNotice = { type: 'success', message: 'Service container restarted successfully' };
      } else {
        const d = await res.json().catch(() => ({}));
        bannerNotice = { type: 'error', message: d.error || 'Failed to restart service' };
      }
      await loadService();
    } catch (e: any) {
      bannerNotice = { type: 'error', message: 'Error: ' + e.message };
    } finally {
      actionLoading = false;
    }
  }

  const isRunning = $derived((service?.runtime_status || service?.RuntimeStatus) === 'running');
  const statusBadge = $derived(service?.runtime_status || service?.RuntimeStatus || 'draft');
  const endpointUrl = $derived(service?.endpoint_url || (service?.domain ? `https://${service.domain}` : null));
</script>

<svelte:head>
  <title>{service?.name || service?.Name || 'Service'} - kloudsPanel</title>
</svelte:head>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={48} /></div>
    <p>Loading service...</p>
  </div>
{:else}
  {#if bannerNotice}
    <div style="margin-bottom: 1.25rem; padding: 0.75rem 1rem; border-radius: var(--radius-md); font-size: 0.875rem; display: flex; align-items: center; justify-content: space-between; gap: 0.75rem; background: {bannerNotice.type === 'error' ? 'rgba(239,68,68,0.1)' : 'rgba(16,185,129,0.1)'}; border: 1px solid {bannerNotice.type === 'error' ? 'rgba(239,68,68,0.3)' : 'rgba(16,185,129,0.3)'}; color: {bannerNotice.type === 'error' ? '#ef4444' : '#10b981'};">
      <span>{bannerNotice.message}</span>
      <button type="button" class="btn btn-secondary" style="padding: 2px 8px; height: auto; min-height: 0; font-size: 0.75rem;" onclick={() => bannerNotice = null}>✕</button>
    </div>
  {/if}

  <!-- Service header -->
  <div class="page-header">
    <div style="flex:1; min-width:0;">
      <p class="text-xs text-muted" style="margin-bottom:0.25rem;">
        <a href="/workspaces">Workspaces</a> /
        {#if service?.project_id || service?.ProjectID}
          <a href="/projects/{service.project_id || service.ProjectID}">{service.project_name || 'Project'}</a> /
        {/if}
      </p>
      <div style="display:flex; align-items:center; gap:0.75rem; flex-wrap:wrap;">
        <h1 class="page-title" style="margin:0;">{service?.name || service?.Name}</h1>
        <span class="badge badge-{statusBadge}">{statusBadge}</span>
        {#if endpointUrl && (service?.kind === 'web' || service?.kind === 'static')}
          <a 
            href={endpointUrl} 
            target="_blank" 
            rel="noopener noreferrer" 
            class="badge" 
            style="background: rgba(0,166,166,0.12); color: var(--color-accent); font-weight: 600; text-decoration: none; display: inline-flex; align-items: center; gap: 4px; padding: 4px 8px;"
          >
            <Globe size={12} /> {service.domain || endpointUrl.replace('https://', '')} <ExternalLink size={11} />
          </a>
        {/if}
      </div>
      <div class="text-xs text-muted" style="margin-top:0.25rem;">
        Internal port: <span class="font-mono">:{service?.internal_port || service?.InternalPort || 80}</span> • Kind: {service?.kind || service?.Kind || 'web'}
      </div>
    </div>
    <div style="display:flex; gap:0.5rem; align-items:center;">
      {#if isRunning}
        <button class="btn btn-secondary" onclick={stopService} disabled={actionLoading}>
          <Square size={14} fill="currentColor" /> Stop
        </button>
      {:else}
        <button class="btn btn-secondary" onclick={startService} disabled={actionLoading}>
          <Play size={14} fill="currentColor" /> Start
        </button>
      {/if}
      <button class="btn btn-primary" onclick={triggerDeploy} disabled={actionLoading}>
        {#if actionLoading}
          <Loader2 size={14} class="animate-spin" /> Deploying...
        {:else}
          <Rocket size={14} /> Deploy Now
        {/if}
      </button>
    </div>
  </div>

  <!-- Tabs -->
  <div class="tabs-bar" style="display:flex; gap:0; border-bottom:2px solid var(--color-border); margin-bottom:1.5rem; overflow-x:auto;">
    {#each tabs as t}
      <a
        href="/services/{id}/{t}"
        style="
          padding:0.625rem 1.25rem; font-size:0.875rem; font-weight:500;
          color:{tab === t ? 'var(--color-accent)' : 'var(--color-ink-secondary)'};
          border-bottom:2px solid {tab === t ? 'var(--color-accent)' : 'transparent'};
          margin-bottom:-2px; white-space:nowrap; text-decoration:none;
          transition:color 0.15s;
        "
      >{t.charAt(0).toUpperCase() + t.slice(1)}</a>
    {/each}
  </div>

  <!-- Tab content -->
  {#if tab === 'overview'}
    {@const parsedRes = (() => {
      try {
        return JSON.parse(service?.resource_json || service?.ResourceJSON || '{}');
      } catch {
        return {};
      }
    })()}

    <!-- Live URL & Public Ingress Banner -->
    {#if endpointUrl && (service?.kind === 'web' || service?.kind === 'static')}
      <div class="card" style="margin-bottom:1.5rem; background: linear-gradient(135deg, rgba(0,166,166,0.06) 0%, var(--color-surface) 100%); border: 1px solid rgba(0,166,166,0.3);">
        <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:1rem;">
          <div style="display:flex; align-items:center; gap:0.85rem;">
            <div style="display:flex; align-items:center; justify-content:center; width:44px; height:44px; border-radius:var(--radius-md); background:rgba(0,166,166,0.15); color:var(--color-accent);">
              <Globe size={24} />
            </div>
            <div>
              <div style="font-size:0.8125rem; font-weight:600; color:var(--color-accent); text-transform:uppercase; letter-spacing:0.04em;">
                Live Application Endpoint
              </div>
              <a 
                href={endpointUrl} 
                target="_blank" 
                rel="noopener noreferrer" 
                style="font-size:1.125rem; font-weight:700; color:var(--color-ink); text-decoration:none; display:inline-flex; align-items:center; gap:6px; margin-top:2px;"
              >
                {endpointUrl} <ExternalLink size={14} style="color:var(--color-accent);" />
              </a>
              <div class="text-xs text-muted" style="display:flex; align-items:center; gap:0.4rem; margin-top:0.25rem;">
                <ShieldCheck size={13} style="color:#059669;" /> SSL Enabled (Let's Encrypt) • Routing via Traefik Edge
              </div>
            </div>
          </div>

          <div style="display:flex; gap:0.5rem; align-items:center;">
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:6px 12px;" onclick={copyEndpoint}>
              {#if copiedUrl}<Check size={14} /> Copied!{:else}<Copy size={14} /> Copy URL{/if}
            </button>
            <a href={endpointUrl} target="_blank" rel="noopener noreferrer" class="btn btn-primary" style="font-size:0.8125rem; padding:6px 14px;">
              Open Site <ExternalLink size={14} />
            </a>
          </div>
        </div>
      </div>
    {/if}

    <!-- Stat cards grid -->
    <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(200px, 1fr)); gap:1rem; margin-bottom:1.5rem;">
      <div class="card" style="padding:1.25rem;">
        <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Service Kind</div>
        <div style="font-size:1.125rem; font-weight:600; text-transform:capitalize;">{service?.kind || service?.Kind || 'web'}</div>
      </div>
      <div class="card" style="padding:1.25rem;">
        <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Runtime Status</div>
        <div style="display:flex; align-items:center; gap:0.5rem; margin-top:0.25rem;">
          <span class="badge badge-{statusBadge}">{statusBadge}</span>
        </div>
      </div>

      {#if (service?.kind || service?.Kind) === 'cron'}
        <div class="card" style="padding:1.25rem;">
          <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Cron Schedule</div>
          <div style="font-size:1rem; font-weight:600; font-family:var(--font-mono)">{parsedRes.cronSchedule || '0 * * * *'}</div>
        </div>
      {:else if (service?.kind || service?.Kind) === 'worker'}
        <div class="card" style="padding:1.25rem;">
          <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Execution Model</div>
          <div style="font-size:1.125rem; font-weight:600;">Background Daemon</div>
        </div>
      {:else}
        <div class="card" style="padding:1.25rem;">
          <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Internal Port</div>
          <div style="font-size:1.125rem; font-weight:600; font-family:var(--font-mono)">:{service?.internal_port || service?.InternalPort || 80}</div>
        </div>
      {/if}

      <div class="card" style="padding:1.25rem;">
        <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Total Deployments</div>
        <div style="font-size:1.125rem; font-weight:600;">{deployments.length}</div>
      </div>
    </div>

    <!-- Latest deployment card -->
    <div class="card" style="margin-bottom:1.5rem;">
      <div class="card-header" style="display:flex; align-items:center; justify-content:space-between;">
        <h3 style="margin:0; font-size:1rem;">Latest Deployment</h3>
        <button class="btn btn-secondary" style="padding:4px 10px; font-size:0.75rem;" onclick={loadService}>
          <RefreshCw size={12} /> Refresh
        </button>
      </div>
      {#if deployments.length > 0}
        {@const dep = deployments[0]}
        <div style="display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:1rem;">
          <div>
            <div style="display:flex; align-items:center; gap:0.5rem;">
              <span class="font-mono text-sm" style="font-weight:600;">Sequence #{dep.sequence}</span>
              <span class="badge badge-{dep.status}">{dep.status}</span>
            </div>
            <p class="text-xs text-muted" style="margin:0.25rem 0 0 0;">
              Triggered by {dep.triggered_by || 'system'} via {dep.trigger} • Driver: {dep.build_driver}
            </p>
          </div>
          <a href="/services/{id}/logs" class="btn btn-secondary" style="font-size:0.8125rem;">
            View Logs →
          </a>
        </div>
      {:else}
        <div style="padding:1rem 0; text-align:center;">
          <p class="text-sm text-muted" style="margin-bottom:1rem;">No deployment has been executed yet.</p>
          <button class="btn btn-primary" onclick={triggerDeploy} disabled={actionLoading}>
            <Rocket size={14} /> Trigger Initial Deployment
          </button>
        </div>
      {/if}
    </div>

  {:else if tab === 'logs'}
    <div style="margin-bottom:1rem; display:flex; justify-content:space-between; align-items:center;">
      <h3 style="margin:0; font-size:1rem;">Real-Time Build & Runtime Logs</h3>
    </div>
    <LogViewer serviceId={service?.id || (id as string)} deploymentId={deployments[0]?.id || deployments[0]?.ID} />

  {:else if tab === 'deployments'}
    <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:1rem;">
      <h3 style="margin:0; font-size:1rem;">Deployment History ({deployments.length})</h3>
      <button class="btn btn-primary" style="padding:0.35rem 0.85rem; font-size:0.8125rem;" onclick={triggerDeploy} disabled={actionLoading}>
        <Rocket size={14} /> Trigger Deployment
      </button>
    </div>
    {#if deployments.length === 0}
      <div class="empty-state" style="padding:2rem; background:var(--color-surface); border:1px solid var(--color-border); border-radius:var(--radius-lg);">
        <p>No deployments recorded yet.</p>
        <button class="btn btn-primary mt-4" onclick={triggerDeploy}>Deploy Now</button>
      </div>
    {:else}
      <div class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th># Seq</th>
              <th>Status</th>
              <th>Trigger</th>
              <th>Driver</th>
              <th>Started At</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {#each deployments as dep}
              <tr>
                <td class="font-mono text-sm" style="font-weight:600;">#{dep.sequence}</td>
                <td><span class="badge badge-{dep.status}">{dep.status}</span></td>
                <td class="text-sm">{dep.trigger} ({dep.triggered_by || 'user'})</td>
                <td class="font-mono text-xs">{dep.build_driver}</td>
                <td class="text-xs text-muted">{(dep.started_at || dep.StartedAt || '-').slice(0, 19).replace('T', ' ')}</td>
                <td style="text-align:right;">
                  <a href="/services/{id}/logs" class="btn btn-secondary" style="padding:4px 10px; font-size:0.75rem;">
                    Logs
                  </a>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}

  {:else if tab === 'variables'}
    <div class="card">
      <div class="card-header" style="display:flex; align-items:center; justify-content:space-between;">
        <div>
          <h3 style="margin:0;">Environment Variables</h3>
          <p class="text-xs text-muted" style="margin-top:0.25rem;">Runtime environment variables injected into the container.</p>
        </div>
        <button class="btn btn-secondary" style="font-size:0.8125rem; padding:4px 12px; min-height:32px;" onclick={addEnv}>
          <Plus size={14} /> Add Variable
        </button>
      </div>

      {#if envVars.length === 0}
        <p class="text-sm text-muted" style="padding:1rem 0;">No environment variables configured. Click "+ Add Variable" to add one.</p>
      {:else}
        <div style="display:flex; flex-direction:column; gap:0.75rem; margin-bottom:1.5rem;">
          {#each envVars as env, i}
            <div style="display:flex; gap:0.75rem; align-items:center;">
              <input type="text" class="form-input font-mono text-sm" placeholder="VARIABLE_NAME" bind:value={env.key} style="flex:1;" />
              <span class="text-muted">=</span>
              <input type="text" class="form-input font-mono text-sm" placeholder="value" bind:value={env.value} style="flex:2;" />
              <button class="btn btn-secondary" style="padding:6px; color:var(--color-error);" onclick={() => removeEnv(i)} aria-label="Remove variable">
                <X size={16} />
              </button>
            </div>
          {/each}
        </div>
      {/if}

      {#if envSuccess}
        <div style="background:#d1fae5; border:1px solid #6ee7b7; color:#065f46; border-radius:var(--radius-md); padding:0.6rem 1rem; font-size:0.875rem; margin-bottom:1rem;">
          Environment variables saved successfully. Redeploy to apply changes.
        </div>
      {/if}

      <div style="display:flex; justify-content:flex-end; gap:0.75rem; padding-top:1rem; border-top:1px solid var(--color-border);">
        <button class="btn btn-primary" onclick={saveEnvVars} disabled={envSaving}>
          {#if envSaving}<Loader2 size={14} class="animate-spin" /> Saving...{:else}<Save size={14} /> Save Variables{/if}
        </button>
      </div>
    </div>

  {:else if tab === 'domains'}
    <div class="card" style="margin-bottom: 1.5rem;">
      <div class="card-header" style="display:flex; justify-content:space-between; align-items:center;">
        <div>
          <h3 style="margin:0; font-size:1.05rem;">Domain & TLS Configuration</h3>
          <div class="text-xs text-muted" style="margin-top:2px;">Attach custom domain names and automate Let's Encrypt TLS/SSL certificates</div>
        </div>
        <button class="btn btn-secondary" onclick={loadDomains} style="padding:4px 10px; font-size:0.75rem;">
          <RefreshCw size={14} /> Refresh
        </button>
      </div>

      {#if domainNotice}
        <div style="background:{domainNotice.type === 'success' ? '#d1fae5' : '#fee2e2'}; border:1px solid {domainNotice.type === 'success' ? '#6ee7b7' : '#fca5a5'}; color:{domainNotice.type === 'success' ? '#065f46' : '#991b1b'}; border-radius:var(--radius-md); padding:0.6rem 1rem; font-size:0.875rem; margin-bottom:1rem;">
          {domainNotice.message}
        </div>
      {/if}

      <!-- Add Domain Form -->
      <div style="background: rgba(0,0,0,0.02); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border); margin-bottom: 1.5rem;">
        <label for="new-domain-input" class="form-label" style="font-weight:600; margin-bottom:0.4rem;">Add Custom Domain</label>
        <div style="display:flex; gap:0.75rem; flex-wrap:wrap;">
          <input 
            id="new-domain-input"
            type="text" 
            class="form-input font-mono" 
            placeholder="e.g. app.yourdomain.com or mybrand.com" 
            bind:value={newDomainInput} 
            style="max-width:380px; flex:1;"
            onkeydown={(e) => { if (e.key === 'Enter') addCustomDomain(); }}
          />
          <button class="btn btn-primary" onclick={addCustomDomain} disabled={domainSaving || !newDomainInput.trim()}>
            {#if domainSaving}<Loader2 size={14} class="animate-spin" /> Saving...{:else}<Plus size={14} /> Add Domain{/if}
          </button>
        </div>
        <p class="text-xs text-muted" style="margin-top: 0.5rem; margin-bottom: 0;">
          Point your domain's CNAME record to <code class="font-mono">{service?.domain || (typeof window !== 'undefined' ? `${service?.slug}.${window.location.hostname}` : 'yourdomain.com')}</code> to complete verification.
        </p>
      </div>

      <!-- Configured Domains List -->
      <div style="font-weight:600; font-size:0.875rem; margin-bottom:0.75rem;">Configured Domains & SSL Status</div>
      <div class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th>Domain</th>
              <th>Type</th>
              <th>SSL / TLS Status</th>
              <th>DNS Target</th>
              <th style="text-align:right;">Actions</th>
            </tr>
          </thead>
          <tbody>
            <!-- Primary System Domain -->
            <tr>
              <td>
                <div style="display:flex; align-items:center; gap:0.5rem;">
                  <Globe size={16} style="color:var(--color-accent);" />
                  <a href="https://{service?.domain || (typeof window !== 'undefined' ? `${service?.slug}.${window.location.hostname}` : 'yourdomain.com')}" target="_blank" rel="noreferrer" class="font-mono text-sm" style="font-weight:600; color:var(--color-accent-dim);">
                    {service?.domain || (typeof window !== 'undefined' ? `${service?.slug}.${window.location.hostname}` : 'yourdomain.com')}
                  </a>
                  <ExternalLink size={12} style="color:var(--color-ink-muted);" />
                </div>
              </td>
              <td><span class="badge badge-running" style="font-size:0.7rem;">Primary (Auto)</span></td>
              <td>
                <span class="badge" style="background:#dcfce7; color:#15803d; font-size:0.7rem; display:inline-flex; align-items:center; gap:4px;">
                  <ShieldCheck size={12} /> Auto Let's Encrypt Active
                </span>
              </td>
              <td class="font-mono text-xs text-muted">Platform Ingress</td>
              <td style="text-align:right;" class="text-xs text-muted">Default</td>
            </tr>

            <!-- Custom Domains -->
            {#each customDomainsList.filter(d => !d.isPrimary) as d}
              <tr>
                <td>
                  <div style="display:flex; align-items:center; gap:0.5rem;">
                    <Globe size={16} style="color:#3b82f6;" />
                    <a href="https://{d.domain}" target="_blank" rel="noreferrer" class="font-mono text-sm" style="font-weight:600; color:var(--color-ink);">
                      {d.domain}
                    </a>
                    <ExternalLink size={12} style="color:var(--color-ink-muted);" />
                  </div>
                </td>
                <td><span class="badge" style="background:#e0f2fe; color:#0369a1; font-size:0.7rem;">Custom Domain</span></td>
                <td>
                  <span class="badge" style="background:#dcfce7; color:#15803d; font-size:0.7rem; display:inline-flex; align-items:center; gap:4px;">
                    <ShieldCheck size={12} /> TLS Active
                  </span>
                </td>
                <td class="font-mono text-xs" style="color:var(--color-ink-muted);">
                  CNAME → {service?.domain || (typeof window !== 'undefined' ? `${service?.slug}.${window.location.hostname}` : 'yourdomain.com')}
                </td>
                <td style="text-align:right;">
                  <button 
                    class="btn btn-secondary" 
                    style="padding:3px 8px; font-size:0.75rem; color:var(--color-error);"
                    onclick={() => removeCustomDomain(d.domain)}
                    aria-label="Remove domain"
                  >
                    <Trash2 size={13} /> Remove
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>

  {:else if tab === 'scale'}
    <div class="card">
      <div class="card-header">
        <h3 style="margin:0;">Scale & Resource Limits</h3>
      </div>
      <div style="display:flex; align-items:center; justify-content:space-between; padding:1rem 0; border-bottom:1px solid var(--color-border);">
        <div>
          <div style="font-weight:600;">Scale to Zero (Sablier)</div>
          <div class="text-sm text-muted">Automatically suspend this container when inactive to save RAM and CPU.</div>
        </div>
        <span class="badge badge-running">Enabled</span>
      </div>
      <div style="padding-top:1rem; display:flex; justify-content:space-between; align-items:center;">
        <div>
          <div style="font-weight:600;">Replicas</div>
          <div class="text-sm text-muted">Number of container instances running behind Traefik.</div>
        </div>
        <span class="font-mono text-sm" style="font-weight:600;">1 instance</span>
      </div>
    </div>

  {:else if tab === 'settings'}
    <div style="display: flex; flex-direction: column; gap: 1.5rem;">
      <!-- General & Build Configuration Form -->
      <form onsubmit={saveServiceSettings} class="card" style="padding: 1.5rem; background: var(--color-surface); border: 1px solid var(--color-border);">
        <div class="card-header" style="margin-bottom: 1.25rem; display: flex; justify-content: space-between; align-items: center;">
          <div>
            <h3 style="margin: 0; font-size: 1.05rem;">Build & Deployment Settings</h3>
            <p class="text-xs text-muted" style="margin: 2px 0 0 0;">Configure build pipeline, start commands, branches, and auto-deploy behavior.</p>
          </div>
          <button type="submit" class="btn btn-primary" style="display: flex; align-items: center; gap: 6px; font-size: 0.8125rem;" disabled={settingsSaving}>
            {#if settingsSaving}<Loader2 size={14} class="animate-spin" /> Saving...{:else}<Save size={14} /> Save Settings{/if}
          </button>
        </div>

        {#if settingsSaved}
          <div style="background:#d1fae5; border:1px solid #6ee7b7; color:#065f46; border-radius:var(--radius-md); padding:0.65rem 1rem; font-size:0.875rem; margin-bottom:1.25rem;">
            Service configuration updated successfully. Click "Trigger Deployment" to apply changes.
          </div>
        {/if}

        {#if settingsError}
          <div style="background:#fee2e2; border:1px solid #fca5a5; color:#991b1b; border-radius:var(--radius-md); padding:0.65rem 1rem; font-size:0.875rem; margin-bottom:1.25rem;">
            {settingsError}
          </div>
        {/if}

        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1.25rem; margin-bottom: 1.25rem;">
          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-name">Service Name</label>
            <input 
              id="settings-name" 
              type="text" 
              class="form-input" 
              bind:value={settingsName} 
              oninput={() => settingsDirty = true} 
              required 
            />
          </div>

          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-preset">Runtime / Framework Preset</label>
            <select 
              id="settings-preset" 
              class="form-select" 
              bind:value={settingsPreset} 
              onchange={() => settingsDirty = true}
            >
              <option value="node">Node.js (Next.js / Express / Nest / Remix)</option>
              <option value="python">Python (FastAPI / Flask / Django)</option>
              <option value="go">Go (Fiber / Gin / Chi)</option>
              <option value="rust">Rust (Actix / Axum / Cargo)</option>
              <option value="java">Java (Spring Boot / Maven / Gradle)</option>
              <option value="php">PHP (Laravel / Apache / Nginx)</option>
              <option value="ruby">Ruby on Rails</option>
              <option value="static">Static Site (SPA / React / Vite / HTML)</option>
              <option value="dockerfile">Custom Dockerfile</option>
            </select>
          </div>
        </div>

        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1.25rem; margin-bottom: 1.25rem;">
          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-build">Build Command</label>
            <input 
              id="settings-build" 
              type="text" 
              class="form-input font-mono text-sm" 
              placeholder="e.g. npm install && npm run build" 
              bind:value={settingsBuildCmd} 
              oninput={() => settingsDirty = true} 
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">Command executed inside the build sandbox to compile assets or install dependencies.</p>
          </div>

          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-start">Start / Run Command</label>
            <input 
              id="settings-start" 
              type="text" 
              class="form-input font-mono text-sm" 
              placeholder="e.g. npm start or node server.js" 
              bind:value={settingsStartCmd} 
              oninput={() => settingsDirty = true} 
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">Command executed when starting the runtime production container.</p>
          </div>
        </div>

        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 1.25rem; margin-bottom: 1.25rem;">
          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-root">Root Directory</label>
            <input 
              id="settings-root" 
              type="text" 
              class="form-input font-mono text-sm" 
              placeholder="." 
              bind:value={settingsRootDir} 
              oninput={() => settingsDirty = true} 
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">Subdirectory containing code for monorepos (defaults to repository root <code>.</code>).</p>
          </div>

          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-branch">Git Branch</label>
            <input 
              id="settings-branch" 
              type="text" 
              class="form-input font-mono text-sm" 
              placeholder="main" 
              bind:value={settingsBranch} 
              oninput={() => settingsDirty = true} 
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">Branch tracked for deployments.</p>
          </div>

          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-port">Internal Container Port</label>
            <input 
              id="settings-port" 
              type="number" 
              class="form-input font-mono text-sm" 
              placeholder="80" 
              bind:value={settingsPort} 
              oninput={() => settingsDirty = true} 
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">Port your application listens on inside the container.</p>
          </div>
        </div>

        {#if settingsRepoUrl}
          <div class="form-group" style="margin-bottom: 1.25rem;">
            <label class="form-label" for="settings-repo">Git Repository URL</label>
            <input 
              id="settings-repo" 
              type="text" 
              class="form-input font-mono text-sm" 
              bind:value={settingsRepoUrl} 
              oninput={() => settingsDirty = true} 
            />
          </div>
        {/if}

        <div style="display: flex; align-items: center; justify-content: space-between; padding: 1rem; background: rgba(0,0,0,0.02); border: 1px solid var(--color-border); border-radius: var(--radius-md);">
          <div>
            <div style="font-weight: 600; font-size: 0.875rem;">Auto-Deploy on Git Push</div>
            <div class="text-xs text-muted">Automatically build and deploy whenever new commits are pushed to the tracked branch.</div>
          </div>
          <label style="display: inline-flex; align-items: center; cursor: pointer;">
            <input 
              type="checkbox" 
              bind:checked={settingsAutoDeploy} 
              onchange={() => settingsDirty = true}
              style="width: 18px; height: 18px; accent-color: var(--color-accent);"
            />
          </label>
        </div>
      </form>

      <!-- Container Operations Card -->
      <div class="card" style="padding: 1.5rem; background: var(--color-surface); border: 1px solid var(--color-border);">
        <div class="card-header" style="margin-bottom: 1rem;">
          <h3 style="margin: 0; font-size: 1.05rem;">Service Operations & Lifecycle</h3>
        </div>

        <div style="display: flex; gap: 1rem; flex-wrap: wrap;">
          <button 
            type="button" 
            class="btn btn-secondary" 
            style="display: flex; align-items: center; gap: 6px; font-size: 0.8125rem;"
            onclick={restartService}
            disabled={actionLoading}
          >
            <RefreshCw size={14} class={actionLoading ? 'animate-spin' : ''} />
            Restart Container
          </button>

          <button 
            type="button" 
            class="btn btn-primary" 
            style="display: flex; align-items: center; gap: 6px; font-size: 0.8125rem;"
            onclick={triggerDeploy}
            disabled={actionLoading}
          >
            <Rocket size={14} />
            Trigger Rebuild & Deploy
          </button>
        </div>
      </div>

      <!-- Danger Zone Card -->
      <div class="card" style="padding: 1.5rem; background: var(--color-surface); border: 1px solid #fca5a5;">
        <div class="card-header" style="border-bottom-color: #fee2e2; margin-bottom: 1rem;">
          <h3 style="color: var(--color-danger); margin: 0; font-size: 1.05rem;">Danger Zone</h3>
        </div>
        <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 1rem;">
          <div>
            <div style="font-weight: 600; color: var(--color-ink);">Delete this Service</div>
            <div class="text-sm text-muted">Permanently delete this service, its configuration, SSL certificates, and all deployment history.</div>
          </div>
          <button class="btn btn-danger" onclick={deleteService} disabled={actionLoading}>
            <Trash2 size={16} /> Delete Service
          </button>
        </div>
      </div>
    </div>
  {/if}
{/if}
