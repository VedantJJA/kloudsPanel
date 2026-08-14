<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount, onDestroy } from 'svelte';
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
    Cpu,
    Play,
    Code,
    Clock,
    AlertCircle,
    Download,
    Zap,
    Sparkles,
    Wrench
  } from 'lucide-svelte';

  const { id, tab } = $derived($page.params);
  const tabs = ['overview', 'query', 'logs', 'settings'];

  let database = $state<any>(null);
  let dbAidEnabled = $state(true);
  let loading = $state(true);
  let actionLoading = $state(false);
  let copied = $state(false);
  let copiedField = $state<string | null>(null);

  // Live Logs state
  let logs = $state<any[]>([]);
  let pollInterval: any = null;

  // SQL Query Console state
  let queryText = $state('SELECT NOW();');
  let queryLoading = $state(false);
  let queryResult = $state<{
    columns?: string[];
    rows?: string[][];
    rowCount?: number;
    durationMs?: number;
    rawOutput?: string;
    error?: string;
  } | null>(null);

  async function loadDatabase() {
    try {
      const res = await fetch(`/api/v1/databases/${id}`, { credentials: 'include' });
      if (!res.ok) { goto('/databases'); return; }
      database = await res.json();

      // Default sample query per engine
      if (database?.engine === 'mysql') {
        queryText = 'SELECT NOW(), VERSION();';
      } else if (database?.engine === 'redis') {
        queryText = 'PING';
      } else if (database?.engine === 'mongodb') {
        queryText = 'db.stats()';
      } else if (database?.engine === 'clickhouse') {
        queryText = 'SELECT version(), currentDatabase(), now();';
      } else {
        queryText = 'SELECT NOW() as current_time, version();';
      }

      await fetchLogs();
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  async function fetchLogs() {
    try {
      const logsRes = await fetch(`/api/v1/databases/${id}/logs`, { credentials: 'include' });
      if (logsRes.ok) {
        const d = await logsRes.json();
        logs = d.entries ?? [];
      }
    } catch {}
  }

  async function checkDbAidStatus() {
    try {
      const res = await fetch('/api/v1/admin/settings', { credentials: 'include' });
      if (res.ok) {
        const d = await res.json();
        if (d.settings?.db_aid_enabled !== undefined) {
          dbAidEnabled = d.settings.db_aid_enabled;
        }
      }
    } catch {}
  }

  let dbAidSaving = $state(false);

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

  function executeAidCommand(cmd: string) {
    queryText = cmd;
    if (tab !== 'query') {
      goto(`/databases/${id}/query`);
    }
    setTimeout(() => {
      runQuery();
    }, 150);
  }

  onMount(() => {
    loadDatabase();
    checkDbAidStatus();
    pollInterval = setInterval(() => {
      if (tab === 'logs') {
        fetchLogs();
      }
    }, 2500);
  });

  onDestroy(() => {
    if (pollInterval) clearInterval(pollInterval);
  });

  // Extract parsed credentials from resource_json
  const parsedMeta = $derived.by(() => {
    let raw: any = {};
    try {
      if (database?.resource_json || database?.ResourceJSON) {
        raw = JSON.parse(database.resource_json || database.ResourceJSON);
      }
    } catch {}

    const engine = database?.engine || 'postgres';
    const internalHost = database?.internal_hostname || `paas-db-${database?.name}`;
    const internalPort = database?.internal_port || (engine === 'mysql' ? 3306 : engine === 'redis' ? 6379 : 5432);
    const externalPort = raw.externalPort || 15432;
    const externalHost = raw.externalHost || (typeof window !== 'undefined' ? window.location.hostname : 'yourdomain.com');
    const dbName = raw.databaseName || database?.database_name || database?.name || 'app';
    const user = raw.username || (engine === 'postgres' ? 'postgres' : 'root');
    const pass = raw.password || '••••••••';
    const internalUri = raw.internalConnectionUri || raw.connectionUri || `${engine}://${user}:${pass}@${internalHost}:${internalPort}/${dbName}`;
    const externalUri = raw.externalConnectionUri || `${engine}://${user}:${pass}@${externalHost}:${externalPort}/${dbName}`;

    return {
      username: user,
      password: pass,
      databaseName: dbName,
      internalHost,
      internalPort,
      externalHost,
      externalPort,
      internalConnectionUri: internalUri,
      externalConnectionUri: externalUri,
      connectionUri: externalUri,
      psqlCommand: raw.psqlCommand || (engine === 'postgres' ? `psql "${externalUri}?sslmode=disable"` : `mysql -h ${externalHost} -P ${externalPort} -u ${user} -p ${dbName}`)
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

  async function runQuery() {
    if (!queryText.trim() || queryLoading) return;
    queryLoading = true;
    queryResult = null;

    try {
      const res = await fetch(`/api/v1/databases/${id}/query`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ query: queryText.trim() })
      });

      const data = await res.json();
      if (!res.ok) {
        queryResult = {
          error: data.error || 'Query execution failed',
          durationMs: data.durationMs || 0
        };
      } else {
        queryResult = data;
      }
    } catch (e: any) {
      queryResult = {
        error: 'Network / API Error: ' + e.message,
        durationMs: 0
      };
    } finally {
      queryLoading = false;
    }
  }

  function setTemplateQuery(q: string) {
    queryText = q;
    runQuery();
  }

  async function restartDatabase() {
    if (!confirm('Are you sure you want to restart this database container?')) return;
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
    if (!confirm(`Are you sure you want to permanently delete database "${database?.name || id}"? All stored data and tables will be erased.`)) return;
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
    const extUri = parsedMeta.externalConnectionUri;
    const extHost = parsedMeta.externalHost;
    const extPort = parsedMeta.externalPort;
    const user = parsedMeta.username;
    const db = parsedMeta.databaseName;

    switch (eng) {
      case 'postgres': {
        const uri = extUri?.includes('sslmode=') ? extUri : (extUri?.includes('?') ? `${extUri}&sslmode=disable` : `${extUri}?sslmode=disable`);
        return `psql "${uri}"`;
      }
      case 'mysql':
        return `mysql -h ${extHost} -P ${extPort} -u ${user} -p ${db}`;
      case 'redis':
        return `redis-cli -h ${extHost} -p ${extPort} -a "${parsedMeta.password}"`;
      case 'mongodb':
        return `mongosh "${extUri}"`;
      case 'clickhouse':
        return `clickhouse-client --host ${extHost} --port ${extPort} --user ${user} --database ${db}`;
      default:
        return `${eng} "${extUri}"`;
    }
  });
</script>

<svelte:head>
  <title>{database?.name || 'Database'} - kloudsPanel</title>
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
        Public: <span class="font-mono">{parsedMeta.externalHost}:{parsedMeta.externalPort}</span> • Internal: <span class="font-mono">{database?.internal_hostname}:{database?.internal_port}</span>
      </div>
    </div>

    <div style="display:flex; gap:0.5rem; align-items:center;">
      <button class="btn btn-secondary" onclick={restartDatabase} disabled={actionLoading}>
        {#if actionLoading}<Loader2 size={14} class="animate-spin" />{:else}<RefreshCw size={14} />{/if}
        Restart
      </button>
      <button class="btn btn-primary" onclick={() => copyText(parsedMeta.externalConnectionUri)}>
        {#if copied}<Check size={14} /> Copied!{:else}<Copy size={14} /> Copy External URI{/if}
      </button>
    </div>
  </div>

  <!-- Tabs -->
  <div class="tabs-bar" style="display:flex; gap:0; border-bottom:2px solid var(--color-border); margin-bottom:1.5rem; overflow-x:auto;">
    {#each tabs as t}
      <a
        href="/databases/{id}/{t}"
        style="
          padding:0.625rem 1.25rem; font-size:0.875rem; font-weight:500;
          color:{tab === t ? 'var(--color-accent)' : 'var(--color-ink-secondary)'};
          border-bottom:2px solid {tab === t ? 'var(--color-accent)' : 'transparent'};
          margin-bottom:-2px; white-space:nowrap; text-decoration:none;
          transition:color 0.15s;
          display: flex; align-items: center; gap: 6px;
        "
      >
        {#if t === 'overview'}<Database size={15} />{/if}
        {#if t === 'query'}<Terminal size={15} />{/if}
        {#if t === 'logs'}<Code size={15} />{/if}
        {#if t === 'settings'}<Settings size={15} />{/if}
        {t === 'query' ? 'SQL / Query Console' : t.charAt(0).toUpperCase() + t.slice(1)}
      </a>
    {/each}
  </div>

  <!-- Tab Contents -->
  {#if tab === 'overview'}
    <!-- External Public Connection String Banner -->
    <div class="card" style="margin-bottom:1.5rem; background: var(--color-surface); border: 1.5px solid var(--color-accent); border-radius: var(--radius-lg); padding: 1.25rem;">
      <div style="display:flex; justify-content:space-between; align-items:flex-start; margin-bottom: 0.75rem; flex-wrap:wrap; gap:0.5rem;">
        <div>
          <div style="display:flex; align-items:center; gap:0.5rem;">
            <span class="badge" style="background:rgba(0,166,166,0.15); color:var(--color-accent); font-weight:700;">🌐 External / Public Access</span>
            <h3 style="margin:0; font-size:1.05rem;">Public Connection URI</h3>
          </div>
          <p class="text-xs text-muted" style="margin-top:0.35rem;">Use this URI to connect from your local laptop (<span class="font-mono">psql</span>, DBeaver, TablePlus, pgAdmin, Prisma Studio, scripts).</p>
        </div>
        <button class="btn btn-primary" style="font-size:0.8125rem; padding:5px 12px;" onclick={() => copyText(parsedMeta.externalConnectionUri, 'ext_uri')}>
          {#if copiedField === 'ext_uri'}<Check size={13} /> Copied!{:else}<Copy size={13} /> Copy Public URI{/if}
        </button>
      </div>
      <div style="background: #0d1117; color: #79c0ff; font-family: var(--font-mono); font-size: 0.875rem; padding: 0.85rem 1rem; border-radius: var(--radius-md); word-break: break-all; margin-bottom: 1rem;">
        {parsedMeta.externalConnectionUri}
      </div>

      <!-- Quick PSQL / CLI Command -->
      <div style="background: rgba(0,0,0,0.03); border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: 0.75rem 1rem; display: flex; align-items: center; justify-content: space-between; gap: 1rem; flex-wrap: wrap;">
        <div style="font-size: 0.8125rem; font-weight: 600; color: var(--color-ink);">
          Terminal Command: <span class="font-mono" style="color: #059669; font-weight: normal; margin-left: 6px;">{cliCommand}</span>
        </div>
        <button class="btn btn-secondary" style="font-size:0.75rem; padding:3px 10px; min-height:28px;" onclick={() => copyText(cliCommand, 'cli')}>
          {#if copiedField === 'cli'}<Check size={12} /> Copied CLI{:else}<Copy size={12} /> Copy Command{/if}
        </button>
      </div>
    </div>

    <!-- Internal Connection URI Card -->
    <div class="card" style="margin-bottom:1.5rem; background: var(--color-surface); border: 1px solid var(--color-border);">
      <div class="card-header" style="display:flex; justify-content:space-between; align-items:center;">
        <div>
          <div style="display:flex; align-items:center; gap:0.5rem;">
            <span class="badge" style="background:#f1f5f9; color:#475569; font-weight:600;">🔒 Private Platform Network</span>
            <h3 style="margin:0; font-size:0.9375rem;">Internal Connection URI</h3>
          </div>
          <p class="text-xs text-muted" style="margin-top:0.25rem;">Use inside web services & background workers running on kloudsPanel.</p>
        </div>
        <button class="btn btn-secondary" style="font-size:0.75rem; padding:4px 10px; min-height:30px;" onclick={() => copyText(parsedMeta.internalConnectionUri, 'int_uri')}>
          {#if copiedField === 'int_uri'}<Check size={12} /> Copied{:else}<Copy size={12} /> Copy Internal URI{/if}
        </button>
      </div>
      <div style="background: #0d1117; color: #a5d6ff; font-family: var(--font-mono); font-size: 0.8125rem; padding: 0.75rem 1rem; border-radius: var(--radius-md); word-break: break-all;">
        {parsedMeta.internalConnectionUri}
      </div>
    </div>

    <!-- Credentials Table Grid -->
    <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(200px, 1fr)); gap:1rem; margin-bottom:1.5rem;">
      <div class="card" style="padding:1rem;">
        <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Public Host / IP</div>
        <div style="display:flex; justify-content:space-between; align-items:center;">
          <span class="font-mono text-sm" style="font-weight:600;">{parsedMeta.externalHost}</span>
          <button class="btn btn-secondary" style="padding:2px 6px; min-height:24px; border:none;" onclick={() => copyText(parsedMeta.externalHost, 'pubhost')}>
            {#if copiedField === 'pubhost'}<Check size={12} />{:else}<Copy size={12} />{/if}
          </button>
        </div>
      </div>

      <div class="card" style="padding:1rem;">
        <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Public Port</div>
        <div style="display:flex; justify-content:space-between; align-items:center;">
          <span class="font-mono text-sm" style="font-weight:600;">:{parsedMeta.externalPort}</span>
          <button class="btn btn-secondary" style="padding:2px 6px; min-height:24px; border:none;" onclick={() => copyText(String(parsedMeta.externalPort), 'pubport')}>
            {#if copiedField === 'pubport'}<Check size={12} />{:else}<Copy size={12} />{/if}
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
          <span class="font-mono text-sm" style="font-weight:600;">{parsedMeta.password}</span>
          <button class="btn btn-secondary" style="padding:2px 6px; min-height:24px; border:none;" onclick={() => copyText(parsedMeta.password, 'pass')}>
            {#if copiedField === 'pass'}<Check size={12} />{:else}<Copy size={12} />{/if}
          </button>
        </div>
      </div>

      <div class="card" style="padding:1rem;">
        <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Database Name</div>
        <div style="display:flex; justify-content:space-between; align-items:center;">
          <span class="font-mono text-sm" style="font-weight:600;">{parsedMeta.databaseName}</span>
          <button class="btn btn-secondary" style="padding:2px 6px; min-height:24px; border:none;" onclick={() => copyText(parsedMeta.databaseName, 'dbname')}>
            {#if copiedField === 'dbname'}<Check size={12} />{:else}<Copy size={12} />{/if}
          </button>
        </div>
      </div>
    </div>

    <!-- Database Aid: 1-Click Command Abstractions -->
    {#if dbAidEnabled}
      <div class="card" style="margin-bottom:1.5rem; background: linear-gradient(135deg, rgba(0,166,166,0.05) 0%, rgba(15,23,42,0.02) 100%); border: 1px solid rgba(0,166,166,0.3); border-radius: var(--radius-lg); padding: 1.25rem;">
        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:0.85rem; flex-wrap:wrap; gap:0.5rem;">
          <div style="display:flex; align-items:center; gap:0.6rem;">
            <div style="width:28px; height:28px; border-radius:var(--radius-sm); background:var(--color-accent); color:#fff; display:flex; align-items:center; justify-content:center;">
              <Zap size={15} />
            </div>
            <div>
              <div style="font-weight:700; font-size:0.9375rem; color:var(--color-ink);">Database Aid • 1-Click Command Abstractions</div>
              <p class="text-xs text-muted" style="margin:0;">Pre-built database operations and diagnostic tools for {database?.engine?.toUpperCase()}.</p>
            </div>
          </div>
          <span class="badge badge-running" style="font-size:0.7rem;">Active</span>
        </div>

        <div style="display:flex; gap:0.5rem; flex-wrap:wrap;">
          {#if database?.engine === 'postgres'}
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("SELECT pid, usename, state, age(clock_timestamp(), query_start) AS duration, query FROM pg_stat_activity WHERE state != 'idle' LIMIT 20;")}>
              <Activity size={13} style="color:var(--color-accent);" /> Active Connections
            </button>
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("SELECT relname AS table_name, pg_size_pretty(pg_total_relation_size(relid)) AS total_size, n_live_tup AS estimated_rows FROM pg_catalog.pg_statio_user_tables ORDER BY pg_total_relation_size(relid) DESC;")}>
              <HardDrive size={13} style="color:#0284c7;" /> Table Sizes & Disk
            </button>
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("VACUUM ANALYZE;")}>
              <Sparkles size={13} style="color:#059669;" /> Optimize & Vacuum
            </button>
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("SELECT now() - pg_postmaster_start_time() AS server_uptime, version();")}>
              <Clock size={13} style="color:#d97706;" /> Server Uptime
            </button>
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("SELECT locktype, relation::regclass, mode, granted FROM pg_locks WHERE NOT granted;")}>
              <ShieldAlert size={13} style="color:#ef4444;" /> Check Locks
            </button>
          {:else if database?.engine === 'mysql'}
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("SHOW FULL PROCESSLIST;")}>
              <Activity size={13} style="color:var(--color-accent);" /> Active Processlist
            </button>
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("SHOW TABLE STATUS;")}>
              <HardDrive size={13} style="color:#0284c7;" /> Table Status & Engine
            </button>
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("SHOW STATUS LIKE 'Threads%';")}>
              <Zap size={13} style="color:#059669;" /> Thread Metrics
            </button>
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("SHOW STATUS LIKE 'Uptime%';")}>
              <Clock size={13} style="color:#d97706;" /> Server Uptime
            </button>
          {:else if database?.engine === 'redis'}
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("INFO memory")}>
              <HardDrive size={13} style="color:#0284c7;" /> Memory Statistics
            </button>
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("CLIENT LIST")}>
              <Activity size={13} style="color:var(--color-accent);" /> Connected Clients
            </button>
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("DBSIZE")}>
              <Database size={13} style="color:#059669;" /> Total Keys (DBSIZE)
            </button>
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("SLOWLOG GET 10")}>
              <Clock size={13} style="color:#d97706;" /> Slow Log (Top 10)
            </button>
          {:else if database?.engine === 'mongodb'}
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("db.serverStatus()")}>
              <Activity size={13} style="color:var(--color-accent);" /> Server Status
            </button>
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("db.stats()")}>
              <HardDrive size={13} style="color:#0284c7;" /> DB Stats
            </button>
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("db.currentOp()")}>
              <Zap size={13} style="color:#059669;" /> Current Operations
            </button>
          {:else}
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:5px 12px; background:var(--color-surface);" onclick={() => executeAidCommand("SELECT 1;")}>
              <Activity size={13} style="color:var(--color-accent);" /> Ping Database
            </button>
          {/if}
        </div>
      </div>
    {/if}

  {:else if tab === 'query'}
    <!-- SQL / Query Console -->
    <div class="card" style="margin-bottom:1.5rem; padding: 1.25rem; background: var(--color-surface); border: 1px solid var(--color-border);">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; flex-wrap: wrap; gap: 0.75rem;">
        <div>
          <div style="font-weight: 700; font-size: 1rem; display: flex; align-items: center; gap: 6px;">
            <Terminal size={18} style="color: var(--color-accent);" /> Interactive {database?.engine?.toUpperCase()} Query Console
          </div>
          <p class="text-xs text-muted" style="margin-top: 2px;">
            Execute queries directly against container <span class="font-mono">{database?.internal_hostname}</span>
          </p>
        </div>

        <!-- Preset Query Shortcuts -->
        <div style="display: flex; gap: 0.35rem; flex-wrap: wrap;">
          {#if database?.engine === 'postgres'}
            <button type="button" class="btn btn-secondary" style="font-size: 0.75rem; padding: 3px 8px;" onclick={() => setTemplateQuery('SELECT NOW(), version();')}>
              Server Time
            </button>
            <button type="button" class="btn btn-secondary" style="font-size: 0.75rem; padding: 3px 8px;" onclick={() => setTemplateQuery("SELECT table_name FROM information_schema.tables WHERE table_schema='public';")}>
              List Tables
            </button>
          {:else if database?.engine === 'mysql'}
            <button type="button" class="btn btn-secondary" style="font-size: 0.75rem; padding: 3px 8px;" onclick={() => setTemplateQuery('SHOW TABLES;')}>
              Show Tables
            </button>
            <button type="button" class="btn btn-secondary" style="font-size: 0.75rem; padding: 3px 8px;" onclick={() => setTemplateQuery('SHOW STATUS LIKE "%Threads%";')}>
              Threads Status
            </button>
          {:else if database?.engine === 'redis'}
            <button type="button" class="btn btn-secondary" style="font-size: 0.75rem; padding: 3px 8px;" onclick={() => setTemplateQuery('INFO')}>
              INFO
            </button>
            <button type="button" class="btn btn-secondary" style="font-size: 0.75rem; padding: 3px 8px;" onclick={() => setTemplateQuery('KEYS *')}>
              KEYS *
            </button>
          {/if}
        </div>
      </div>

      <!-- Editor Area -->
      <div style="position: relative; margin-bottom: 1rem;">
        <textarea
          rows={5}
          class="form-input font-mono"
          style="width: 100%; font-size: 0.875rem; line-height: 1.45; background: #0d1117; color: #58a6ff; border-radius: var(--radius-md); padding: 0.85rem; border: 1px solid #30363d; resize: vertical;"
          bind:value={queryText}
          placeholder="Enter query here (e.g. SELECT * FROM users;)"
          onkeydown={(e) => {
            if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
              e.preventDefault();
              runQuery();
            }
          }}
        ></textarea>
      </div>

      <!-- Execution Action Bar -->
      <div style="display: flex; justify-content: space-between; align-items: center;">
        <div class="text-xs text-muted">
          Press <kbd style="background: rgba(0,0,0,0.06); padding: 2px 5px; border-radius: 3px; font-family: var(--font-mono);">Ctrl + Enter</kbd> to execute query
        </div>
        <button
          type="button"
          class="btn btn-primary"
          style="display: flex; align-items: center; gap: 6px; padding: 6px 16px;"
          onclick={runQuery}
          disabled={queryLoading}
        >
          {#if queryLoading}
            <Loader2 size={14} class="animate-spin" /> Executing…
          {:else}
            <Play size={14} fill="currentColor" /> Run Query
          {/if}
        </button>
      </div>
    </div>

    <!-- Query Results Section -->
    {#if queryResult}
      <div class="card" style="padding: 1.25rem; background: var(--color-surface); border: 1px solid var(--color-border); margin-bottom: 2rem; max-width: 100%; overflow: hidden; box-sizing: border-box;">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.85rem; flex-wrap: wrap; gap: 0.5rem;">
          <div style="display: flex; align-items: center; gap: 0.75rem;">
            <div style="font-weight: 700; font-size: 0.9375rem;">Execution Results</div>
            {#if queryResult.durationMs !== undefined}
              <span class="badge" style="background: rgba(0,166,166,0.1); color: var(--color-accent); font-size: 0.75rem;">
                <Clock size={11} style="margin-right: 3px;" /> {queryResult.durationMs}ms
              </span>
            {/if}
            {#if queryResult.rowCount !== undefined}
              <span class="badge" style="background: rgba(16,185,129,0.1); color: #059669; font-size: 0.75rem;">
                {queryResult.rowCount} rows
              </span>
            {/if}
          </div>
        </div>

        {#if queryResult.error}
          <div style="background: #fef2f2; border: 1px solid #fecaca; border-radius: var(--radius-md); padding: 0.85rem 1rem; color: #991b1b; font-size: 0.875rem; max-width: 100%; overflow: hidden;">
            <div style="display: flex; align-items: center; gap: 6px; font-weight: 700; margin-bottom: 4px;">
              <AlertCircle size={16} /> Query Error
            </div>
            <pre style="margin: 0; font-family: var(--font-mono); font-size: 0.8125rem; white-space: pre-wrap; word-break: break-word; overflow-x: auto; max-height: 300px;">{queryResult.error}</pre>
          </div>
        {:else if queryResult.columns && queryResult.columns.length > 0}
          <div style="overflow-x: auto; overflow-y: auto; border: 1px solid var(--color-border); border-radius: var(--radius-md); max-height: 420px; max-width: 100%; width: 100%; box-sizing: border-box;">
            <table class="table" style="margin: 0; font-size: 0.8125rem; width: 100%; border-collapse: collapse;">
              <thead style="position: sticky; top: 0; background: var(--color-surface-sunken); z-index: 2;">
                <tr>
                  {#each queryResult.columns as col}
                    <th style="padding: 8px 12px; font-weight: 700; font-family: var(--font-mono); white-space: nowrap; border-bottom: 1px solid var(--color-border);">{col}</th>
                  {/each}
                </tr>
              </thead>
              <tbody>
                {#if queryResult.rows && queryResult.rows.length > 0}
                  {#each queryResult.rows as row}
                    <tr>
                      {#each row as cell}
                        <td style="padding: 7px 12px; font-family: var(--font-mono); max-width: 600px; overflow-wrap: break-word; word-break: break-word; white-space: pre-wrap; font-size: 0.8125rem; border-bottom: 1px solid rgba(0,0,0,0.05);">
                          {cell === '' ? '<NULL>' : cell}
                        </td>
                      {/each}
                    </tr>
                  {/each}
                {:else}
                  <tr>
                    <td colspan={queryResult.columns.length} style="text-align: center; padding: 1.5rem; color: var(--color-ink-muted);">
                      Query executed successfully. 0 rows returned.
                    </td>
                  </tr>
                {/if}
              </tbody>
            </table>
          </div>
        {:else if queryResult.rawOutput}
          <div style="max-height: 420px; overflow-x: auto; overflow-y: auto; max-width: 100%; width: 100%; border-radius: var(--radius-md); border: 1px solid #30363d; background: #0d1117; box-sizing: border-box;">
            <pre style="background: transparent; color: #c9d1d9; font-family: var(--font-mono); padding: 1rem; font-size: 0.8125rem; white-space: pre-wrap; word-break: break-word; margin: 0;">{queryResult.rawOutput}</pre>
          </div>
        {/if}
      </div>
    {/if}

  {:else if tab === 'logs'}
    <div class="card" style="padding:0; overflow:hidden; border: 1px solid var(--color-border);">
      <div class="card-header" style="padding:0.85rem 1.25rem; margin:0; border-bottom:1px solid var(--color-border); display: flex; justify-content: space-between; align-items: center;">
        <h3 style="margin:0; font-size:0.9375rem;">Live Container Logs ({database?.internal_hostname})</h3>
        <span class="text-xs text-muted" style="display: flex; align-items: center; gap: 4px;">
          <span style="display: inline-block; width: 6px; height: 6px; border-radius: 50%; background: #10b981;"></span> Auto-polling
        </span>
      </div>
      <div class="log-viewer" style="border-radius:0; min-height:380px; max-height: 520px; overflow-y: auto; background: #0d1117; padding: 0.85rem;">
        {#if logs.length > 0}
          {#each logs as log}
            <div class="log-line-{log.stream}" style="font-family: var(--font-mono); font-size: 0.8125rem; padding: 2px 0; line-height: 1.45; color: {log.stream === 'stderr' ? '#f87171' : log.stream === 'system' ? '#38bdf8' : '#e2e8f0'};">
              <span style="opacity:0.4; margin-right:0.75rem;">{log.timestamp || '00:00:00'}</span>{log.message}
            </div>
          {/each}
        {:else}
          <div style="color: #64748b; font-family: var(--font-mono); font-size: 0.8125rem; padding: 1rem;">
            No logs captured yet from container.
          </div>
        {/if}
      </div>
    </div>

  {:else if tab === 'settings'}
    <!-- Database Assist Mode Settings -->
    <div class="card" style="margin-bottom:1.5rem; background: var(--color-surface); border: 1px solid var(--color-border); padding: 1.25rem;">
      <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:1rem;">
        <div style="display:flex; align-items:flex-start; gap:0.85rem; max-width:600px;">
          <div style="width:38px; height:38px; border-radius:var(--radius-md); background:rgba(0,166,166,0.12); color:var(--color-accent); display:flex; align-items:center; justify-content:center; flex-shrink:0;">
            <Zap size={20} />
          </div>
          <div>
            <div style="font-weight:700; font-size:0.9375rem; color:var(--color-ink); display:flex; align-items:center; gap:8px;">
              Assist Mode • 1-Click Database Command Abstractions
              {#if dbAidEnabled}
                <span class="badge badge-running" style="font-size:0.7rem;">Active</span>
              {:else}
                <span class="badge badge-stopped" style="font-size:0.7rem;">Disabled</span>
              {/if}
            </div>
            <p class="text-xs text-muted" style="margin:3px 0 0 0; line-height:1.45;">
              Controls whether 1-click execution buttons for table statistics, vacuuming, active connection checks, and diagnostics are displayed.
            </p>
          </div>
        </div>

        <button 
          type="button" 
          class="btn {dbAidEnabled ? 'btn-primary' : 'btn-secondary'}" 
          onclick={toggleDbAid}
          disabled={dbAidSaving}
          style="padding:6px 16px; font-weight:600; font-size:0.8125rem;"
        >
          {#if dbAidSaving}
            <Loader2 size={13} class="animate-spin" /> Updating…
          {:else if dbAidEnabled}
            Assist Mode: ON (Click to Disable)
          {:else}
            Assist Mode: OFF (Click to Enable)
          {/if}
        </button>
      </div>
    </div>

    <div class="card" style="border-color:#fca5a5; margin-bottom:1.5rem; background: var(--color-surface);">
      <div class="card-header" style="border-bottom-color:#fee2e2;">
        <h3 style="color:var(--color-danger); margin:0;">Danger Zone</h3>
      </div>
      <div style="display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:1rem; padding:0.5rem 0;">
        <div>
          <div style="font-weight:600; color:var(--color-ink);">Delete this Database</div>
          <div class="text-sm text-muted">Permanently erase this database instance, its Docker container, and stored data.</div>
        </div>
        <button class="btn btn-danger" onclick={deleteDatabase} disabled={actionLoading}>
          <Trash2 size={16} /> Delete Database
        </button>
      </div>
    </div>
  {/if}
{/if}
