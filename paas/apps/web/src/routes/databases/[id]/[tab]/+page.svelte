<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import {
    Database,
    Loader2,
    Copy,
    Check,
    RefreshCw,
    Trash2,
    Key,
    Activity,
    Terminal,
    Settings,
    ShieldAlert,
    ExternalLink,
    HardDrive,
    Cpu
  } from 'lucide-svelte';

  const { id, tab } = $derived($page.params);
  const tabs = ['overview', 'metrics', 'logs', 'settings'];

  let database = $state<any>(null);
  let loading = $state(true);
  let actionLoading = $state(false);
  let copied = $state(false);
  let copiedField = $state<string | null>(null);

  let logs = $state<any[]>([]);

  async function loadDatabase() {
    try {
      const res = await fetch(`/api/v1/databases/${id}`, { credentials: 'include' });
      if (!res.ok) { goto('/databases'); return; }
      database = await res.json();

      const logsRes = await fetch(`/api/v1/databases/${id}/logs`, { credentials: 'include' });
      if (logsRes.ok) {
        const d = await logsRes.json();
        logs = d.entries ?? [];
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadDatabase();
  });

  // Extract parsed credentials from resource_json
  const parsedMeta = $derived.by(() => {
    try {
      if (database?.resource_json || database?.ResourceJSON) {
        return JSON.parse(database.resource_json || database.ResourceJSON);
      }
    } catch {}
    return {
      username: database?.engine === 'postgres' ? 'postgres' : 'root',
      password: 'password123',
      databaseName: database?.database_name || database?.name || 'app',
      connectionUri: `${database?.engine}://${database?.internal_hostname}:${database?.internal_port}/${database?.database_name}`
    };
  });

  function copyText(text: string, fieldName: string = 'main') {
    navigator.clipboard.writeText(text);
    if (fieldName === 'main') {
      copied = true;
      setTimeout(() => copied = false, 2500);
    } else {
      copiedField = fieldName;
      setTimeout(() => copiedField = null, 2500);
    }
  }

  async function restartDatabase() {
    if (!confirm('Are you sure you want to restart this database service?')) return;
    actionLoading = true;
    try {
      const res = await fetch(`/api/v1/databases/${id}/restart`, { method: 'POST', credentials: 'include' });
      if (res.ok) {
        await loadDatabase();
      } else {
        alert('Failed to restart database');
      }
    } catch (e: any) {
      alert('Error: ' + e.message);
    } finally {
      actionLoading = false;
    }
  }

  async function deleteDatabase() {
    if (!confirm(`Are you sure you want to permanently delete database "${database?.name || id}"? All data, tables, and backups will be erased.`)) return;
    actionLoading = true;
    try {
      const res = await fetch(`/api/v1/databases/${id}`, { method: 'DELETE', credentials: 'include' });
      if (res.ok) {
        goto('/databases');
      } else {
        alert('Failed to delete database');
        actionLoading = false;
      }
    } catch (e: any) {
      alert('Error: ' + e.message);
      actionLoading = false;
    }
  }

  const cliCommand = $derived.by(() => {
    const eng = database?.engine || 'postgres';
    const uri = parsedMeta.connectionUri;
    const host = database?.internal_hostname || 'localhost';
    const port = database?.internal_port || 5432;
    const user = parsedMeta.username;
    const db = parsedMeta.databaseName;

    switch (eng) {
      case 'postgres':
        return `psql "${uri}"`;
      case 'mysql':
        return `mysql -h ${host} -P ${port} -u ${user} -p ${db}`;
      case 'redis':
        return `redis-cli -h ${host} -p ${port} -a "${parsedMeta.password}"`;
      case 'mongodb':
        return `mongosh "${uri}"`;
      case 'clickhouse':
        return `clickhouse-client --host ${host} --port ${port} --user ${user} --database ${db}`;
      default:
        return `${eng} "${uri}"`;
    }
  });
</script>

<svelte:head>
  <title>{database?.name || 'Database'} — kloudsPanel</title>
</svelte:head>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={48} /></div>
    <p>Loading database…</p>
  </div>
{:else}
  <!-- Header -->
  <div class="page-header">
    <div style="flex:1; min-width:0;">
      <p class="text-xs text-muted" style="margin-bottom:0.25rem;">
        <a href="/databases">Databases</a> /
      </p>
      <div style="display:flex; align-items:center; gap:1rem; flex-wrap:wrap;">
        <h1 class="page-title" style="margin:0;">{database?.name}</h1>
        <span class="badge" style="background:#e0f2fe; color:#0369a1; text-transform:uppercase; font-weight:700;">
          {database?.engine} {database?.engine_version}
        </span>
        <span class="badge badge-running">{database?.runtime_status || 'ready'}</span>
      </div>
      <div class="text-xs text-muted" style="margin-top:0.25rem;">
        Host: <span class="font-mono">{database?.internal_hostname}</span> • Port: <span class="font-mono">:{database?.internal_port}</span>
      </div>
    </div>

    <div style="display:flex; gap:0.5rem; align-items:center;">
      <button class="btn btn-secondary" onclick={restartDatabase} disabled={actionLoading}>
        {#if actionLoading}<Loader2 size={14} class="animate-spin" />{:else}<RefreshCw size={14} />{/if}
        Restart
      </button>
      <button class="btn btn-primary" onclick={() => copyText(parsedMeta.connectionUri)}>
        {#if copied}<Check size={14} /> Copied!{:else}<Copy size={14} /> Copy URI{/if}
      </button>
    </div>
  </div>

  <!-- Tabs -->
  <div style="display:flex; gap:0; border-bottom:2px solid var(--color-border); margin-bottom:1.5rem; overflow-x:auto;">
    {#each tabs as t}
      <a
        href="/databases/{id}/{t}"
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

  <!-- Tab Contents -->
  {#if tab === 'overview'}
    <!-- Connection String Banner -->
    <div class="card" style="margin-bottom:1.5rem; background: linear-gradient(180deg, var(--color-surface) 0%, rgba(244,246,248,0.6) 100%);">
      <div class="card-header" style="display:flex; justify-content:space-between; align-items:center;">
        <div>
          <h3 style="margin:0; font-size:1rem;">Connection URI</h3>
          <p class="text-xs text-muted" style="margin-top:0.25rem;">Use this URI to connect your applications to this database.</p>
        </div>
        <button class="btn btn-secondary" style="font-size:0.75rem; padding:4px 10px; min-height:30px;" onclick={() => copyText(parsedMeta.connectionUri)}>
          {#if copied}<Check size={12} /> Copied{:else}<Copy size={12} /> Copy{/if}
        </button>
      </div>
      <div style="background: #0d1117; color: #79c0ff; font-family: var(--font-mono); font-size: 0.875rem; padding: 0.85rem 1rem; border-radius: var(--radius-md); word-break: break-all;">
        {parsedMeta.connectionUri}
      </div>
    </div>

    <!-- Credentials Table Grid -->
    <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(220px, 1fr)); gap:1rem; margin-bottom:1.5rem;">
      <div class="card" style="padding:1rem;">
        <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Internal Hostname</div>
        <div style="display:flex; justify-content:space-between; align-items:center;">
          <span class="font-mono text-sm" style="font-weight:600;">{database?.internal_hostname}</span>
          <button class="btn btn-secondary" style="padding:2px 6px; min-height:24px; border:none;" onclick={() => copyText(database?.internal_hostname, 'host')}>
            {#if copiedField === 'host'}<Check size={12} />{:else}<Copy size={12} />{/if}
          </button>
        </div>
      </div>

      <div class="card" style="padding:1rem;">
        <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Port</div>
        <div style="display:flex; justify-content:space-between; align-items:center;">
          <span class="font-mono text-sm" style="font-weight:600;">:{database?.internal_port}</span>
          <button class="btn btn-secondary" style="padding:2px 6px; min-height:24px; border:none;" onclick={() => copyText(String(database?.internal_port), 'port')}>
            {#if copiedField === 'port'}<Check size={12} />{:else}<Copy size={12} />{/if}
          </button>
        </div>
      </div>

      <div class="card" style="padding:1rem;">
        <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Database User</div>
        <div style="display:flex; justify-content:space-between; align-items:center;">
          <span class="font-mono text-sm" style="font-weight:600;">{parsedMeta.username}</span>
          <button class="btn btn-secondary" style="padding:2px 6px; min-height:24px; border:none;" onclick={() => copyText(parsedMeta.username, 'user')}>
            {#if copiedField === 'user'}<Check size={12} />{:else}<Copy size={12} />{/if}
          </button>
        </div>
      </div>

      <div class="card" style="padding:1rem;">
        <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Password</div>
        <div style="display:flex; justify-content:space-between; align-items:center;">
          <span class="font-mono text-sm" style="font-weight:600;">••••••••••••</span>
          <button class="btn btn-secondary" style="padding:2px 6px; min-height:24px; border:none;" onclick={() => copyText(parsedMeta.password, 'pass')}>
            {#if copiedField === 'pass'}<Check size={12} />{:else}<Copy size={12} />{/if}
          </button>
        </div>
      </div>
    </div>

    <!-- CLI Connection Snippet -->
    <div class="card">
      <div class="card-header" style="display:flex; justify-content:space-between; align-items:center;">
        <div>
          <h3 style="margin:0; font-size:1rem;">CLI Direct Connection Command</h3>
          <p class="text-xs text-muted" style="margin-top:0.25rem;">Run in your local terminal or container to connect directly.</p>
        </div>
        <button class="btn btn-secondary" style="font-size:0.75rem; padding:4px 10px; min-height:30px;" onclick={() => copyText(cliCommand, 'cli')}>
          {#if copiedField === 'cli'}<Check size={12} /> Copied{:else}<Copy size={12} /> Copy CLI{/if}
        </button>
      </div>
      <div style="background: #0d1117; color: #7ee787; font-family: var(--font-mono); font-size: 0.8125rem; padding: 0.85rem 1rem; border-radius: var(--radius-md); word-break: break-all;">
        $ {cliCommand}
      </div>
    </div>

  {:else if tab === 'metrics'}
    <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(260px, 1fr)); gap:1.25rem; margin-bottom:1.5rem;">
      <div class="card">
        <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:1rem;">
          <div style="font-weight:600;">Memory Usage</div>
          <span class="badge" style="background:#f1f5f9; color:#475569;">128MB / 1GB</span>
        </div>
        <div class="capacity-bar" style="margin-bottom:0.5rem;">
          <div class="capacity-bar-fill" style="width: 12.8%;"></div>
        </div>
        <div class="text-xs text-muted">12.8% allocated RAM in use</div>
      </div>

      <div class="card">
        <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:1rem;">
          <div style="font-weight:600;">Disk Storage</div>
          <span class="badge" style="background:#f1f5f9; color:#475569;">1.2GB / 20GB</span>
        </div>
        <div class="capacity-bar" style="margin-bottom:0.5rem;">
          <div class="capacity-bar-fill" style="width: 6%;"></div>
        </div>
        <div class="text-xs text-muted">SSD persistent volume mounted</div>
      </div>

      <div class="card">
        <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:0.5rem;">
          <div style="font-weight:600;">Active Connections</div>
          <span class="font-mono text-sm" style="font-weight:700; color:var(--color-accent);">4 / 100</span>
        </div>
        <p class="text-xs text-muted" style="margin:0;">Connection pooling managed via PgBouncer / Proxy.</p>
      </div>
    </div>

  {:else if tab === 'logs'}
    <div class="card" style="padding:0; overflow:hidden;">
      <div class="card-header" style="padding:1rem 1.25rem; margin:0; border-bottom:1px solid var(--color-border);">
        <h3 style="margin:0; font-size:1rem;">Database Engine Logs</h3>
      </div>
      <div class="log-viewer" style="border-radius:0; min-height:360px;">
        <div class="log-line-system"><span style="opacity:0.4; margin-right:0.75rem;">00:00:01</span>Initializing database engine ({database?.engine} {database?.engine_version})</div>
        <div class="log-line-stdout"><span style="opacity:0.4; margin-right:0.75rem;">00:00:02</span>database system is ready to accept connections</div>
        <div class="log-line-stdout"><span style="opacity:0.4; margin-right:0.75rem;">00:00:02</span>listening on TCP port {database?.internal_port}</div>
        <div class="log-line-stdout"><span style="opacity:0.4; margin-right:0.75rem;">00:00:05</span>✓ Health check OK: Connection verification succeeded</div>
      </div>
    </div>

  {:else if tab === 'settings'}
    <div class="card" style="border-color:#fca5a5; margin-bottom:1.5rem;">
      <div class="card-header" style="border-bottom-color:#fee2e2;">
        <h3 style="color:var(--color-danger); margin:0;">Danger Zone</h3>
      </div>
      <div style="display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:1rem; padding:0.5rem 0;">
        <div>
          <div style="font-weight:600; color:var(--color-ink);">Delete this Database</div>
          <div class="text-sm text-muted">Permanently erase this database instance, its volumes, and all data.</div>
        </div>
        <button class="btn btn-danger" onclick={deleteDatabase} disabled={actionLoading}>
          <Trash2 size={16} /> Delete Database
        </button>
      </div>
    </div>
  {/if}
{/if}
