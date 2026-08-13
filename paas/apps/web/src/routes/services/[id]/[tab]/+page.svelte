<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import LogViewer from '$lib/components/logs/LogViewer.svelte';
  import { Loader2, ExternalLink, Square, Play, Rocket, Trash2, Plus, X, Save, RefreshCw } from 'lucide-svelte';

  const { id, tab } = $derived($page.params);
  const tabs = ['overview', 'deployments', 'logs', 'variables', 'domains', 'scale', 'settings'];

  let service = $state<any>(null);
  let deployments = $state<any[]>([]);
  let loading = $state(true);
  let actionLoading = $state(false);

  // Variables state
  let envVars = $state<Array<{ key: string; value: string }>>([]);
  let envSaving = $state(false);
  let envSuccess = $state(false);

  async function loadService() {
    try {
      const res = await fetch(`/api/v1/services/${id}`, { credentials: 'include' });
      if (!res.ok) { goto('/workspaces'); return; }
      service = await res.json();
      
      // Parse existing env vars if present in resource_json
      try {
        if (service.resource_json || service.ResourceJSON) {
          const r = JSON.parse(service.resource_json || service.ResourceJSON);
          if (r.env && typeof r.env === 'object') {
            envVars = Object.entries(r.env).map(([k, v]) => ({ key: k, value: String(v) }));
          }
        }
      } catch {}

      const depRes = await fetch(`/api/v1/services/${id}/deployments`, { credentials: 'include' });
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
  });

  async function triggerDeploy() {
    actionLoading = true;
    try {
      const res = await fetch(`/api/v1/services/${id}/deploy`, { method: 'POST', credentials: 'include' });
      if (!res.ok) {
        alert('Failed to trigger deployment');
      }
      await loadService();
    } catch (e: any) {
      alert('Error: ' + e.message);
    } finally {
      actionLoading = false;
    }
  }

  async function stopService() {
    actionLoading = true;
    try {
      const res = await fetch(`/api/v1/services/${id}/stop`, { method: 'POST', credentials: 'include' });
      if (!res.ok) {
        alert('Failed to stop service');
      }
      await loadService();
    } catch (e: any) {
      alert('Error: ' + e.message);
    } finally {
      actionLoading = false;
    }
  }

  async function startService() {
    actionLoading = true;
    try {
      const res = await fetch(`/api/v1/services/${id}/start`, { method: 'POST', credentials: 'include' });
      if (!res.ok) {
        alert('Failed to start service');
      }
      await loadService();
    } catch (e: any) {
      alert('Error: ' + e.message);
    } finally {
      actionLoading = false;
    }
  }

  async function deleteService() {
    if (!confirm(`Are you sure you want to permanently delete service "${service?.name || service?.Name || id}"? This action cannot be undone.`)) return;
    actionLoading = true;
    try {
      const res = await fetch(`/api/v1/services/${id}`, { method: 'DELETE', credentials: 'include' });
      if (!res.ok) {
        alert('Failed to delete service');
        actionLoading = false;
        return;
      }
      const projId = service?.project_id || service?.ProjectID;
      if (projId) {
        goto(`/projects/${projId}`);
      } else {
        goto('/workspaces');
      }
    } catch (e: any) {
      alert('Error: ' + e.message);
      actionLoading = false;
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

      const res = await fetch(`/api/v1/services/${id}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ resourceJson: JSON.stringify(currentR) })
      });
      if (res.ok) {
        envSuccess = true;
        setTimeout(() => envSuccess = false, 3000);
      } else {
        alert('Failed to save variables');
      }
    } catch (e: any) {
      alert('Error: ' + e.message);
    } finally {
      envSaving = false;
    }
  }

  const isRunning = $derived((service?.runtime_status || service?.RuntimeStatus) === 'running');
  const statusBadge = $derived(service?.runtime_status || service?.RuntimeStatus || 'draft');
</script>

<svelte:head>
  <title>{service?.name || service?.Name || 'Service'} — kloudsPanel</title>
</svelte:head>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={48} /></div>
    <p>Loading service…</p>
  </div>
{:else}
  <!-- Service header -->
  <div class="page-header">
    <div style="flex:1; min-width:0;">
      <p class="text-xs text-muted" style="margin-bottom:0.25rem;">
        <a href="/workspaces">Workspaces</a> /
        {#if service?.project_id || service?.ProjectID}
          <a href="/projects/{service.project_id || service.ProjectID}">Project</a> /
        {/if}
      </p>
      <div style="display:flex; align-items:center; gap:1rem; flex-wrap:wrap;">
        <h1 class="page-title" style="margin:0;">{service?.name || service?.Name}</h1>
        <span class="badge badge-{statusBadge}">{statusBadge}</span>
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
  <div style="display:flex; gap:0; border-bottom:2px solid var(--color-border); margin-bottom:1.5rem; overflow-x:auto;">
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
      <div class="card" style="padding:1.25rem;">
        <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Internal Port</div>
        <div style="font-size:1.125rem; font-weight:600; font-family:var(--font-mono)">:{service?.internal_port || service?.InternalPort || 80}</div>
      </div>
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
    <LogViewer serviceId={id as string} deploymentId={deployments[0]?.id || deployments[0]?.ID} />

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
                <td class="text-xs text-muted">{(dep.started_at || dep.StartedAt || '—').slice(0, 19).replace('T', ' ')}</td>
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
          ✓ Environment variables saved successfully. Redeploy to apply changes.
        </div>
      {/if}

      <div style="display:flex; justify-content:flex-end; gap:0.75rem; padding-top:1rem; border-top:1px solid var(--color-border);">
        <button class="btn btn-primary" onclick={saveEnvVars} disabled={envSaving}>
          {#if envSaving}<Loader2 size={14} class="animate-spin" /> Saving...{:else}<Save size={14} /> Save Variables{/if}
        </button>
      </div>
    </div>

  {:else if tab === 'domains'}
    <div class="card">
      <div class="card-header">
        <h3 style="margin:0;">Custom Domains</h3>
      </div>
      <p class="text-sm text-muted">Direct traffic from your custom domains to this service.</p>
      <div style="display:flex; gap:0.75rem; margin-top:1rem;">
        <input type="text" class="form-input" placeholder="app.yourdomain.com" style="max-width:320px;" />
        <button class="btn btn-primary">Add Domain</button>
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
    <div class="card" style="border-color:#fca5a5;">
      <div class="card-header" style="border-bottom-color:#fee2e2;">
        <h3 style="color:var(--color-danger); margin:0;">Danger Zone</h3>
      </div>
      <div style="display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:1rem; padding:0.5rem 0;">
        <div>
          <div style="font-weight:600; color:var(--color-ink);">Delete this Service</div>
          <div class="text-sm text-muted">Permanently delete this service, its configuration, and all deployment history.</div>
        </div>
        <button class="btn btn-danger" onclick={deleteService} disabled={actionLoading}>
          <Trash2 size={16} /> Delete Service
        </button>
      </div>
    </div>
  {/if}
{/if}
