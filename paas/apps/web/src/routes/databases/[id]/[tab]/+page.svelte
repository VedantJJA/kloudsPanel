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
    Wrench,
    Network,
    Table,
    Link,
    Search,
    ZoomIn,
    ZoomOut,
    Maximize2,
    Move,
    Layers,
    Share2,
    Plus
  } from 'lucide-svelte';

  const { id, tab } = $derived($page.params);
  const tabs = ['overview', 'query', 'visualizer', 'logs', 'settings'];

  let database = $state<any>(null);
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

  // Database Schema & ER Diagram state
  let schemaData = $state<any>(null);
  let schemaLoading = $state(false);
  let searchTable = $state('');
  let zoomLevel = $state(1);
  let panOffset = $state({ x: 50, y: 50 });
  let isPanning = $state(false);
  let panStart = $state({ x: 0, y: 0 });
  let tablePositions = $state<Record<string, { x: number; y: number }>>({});
  let activeDragTable = $state<string | null>(null);
  let dragOffset = $state({ x: 0, y: 0 });
  let selectedTable = $state<string | null>(null);

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

  async function loadSchema() {
    schemaLoading = true;
    try {
      const res = await fetch(`/api/v1/databases/${id}/schema`, { credentials: 'include' });
      if (res.ok) {
        schemaData = await res.json();
        autoArrangeTables(schemaData.tables || []);
      }
    } catch (e) {
      console.error(e);
    } finally {
      schemaLoading = false;
    }
  }

  function autoArrangeTables(tables: any[]) {
    const cols = Math.ceil(Math.sqrt(tables.length || 1)) || 3;
    const colWidth = 320;
    const rowHeight = 340;
    const pos: Record<string, { x: number; y: number }> = {};
    tables.forEach((t, i) => {
      if (!tablePositions[t.name]) {
        const col = i % cols;
        const row = Math.floor(i / cols);
        pos[t.name] = {
          x: 60 + col * colWidth,
          y: 60 + row * rowHeight
        };
      } else {
        pos[t.name] = tablePositions[t.name];
      }
    });
    tablePositions = pos;
  }

  function handleCanvasMouseDown(e: MouseEvent) {
    if ((e.target as HTMLElement).closest('.table-node') || (e.target as HTMLElement).closest('.canvas-toolbar') || (e.target as HTMLElement).closest('button') || (e.target as HTMLElement).closest('input')) {
      return;
    }
    isPanning = true;
    panStart = { x: e.clientX - panOffset.x, y: e.clientY - panOffset.y };
  }

  function handleCanvasMouseMove(e: MouseEvent) {
    if (isPanning) {
      panOffset = {
        x: e.clientX - panStart.x,
        y: e.clientY - panStart.y
      };
    } else if (activeDragTable) {
      tablePositions = {
        ...tablePositions,
        [activeDragTable]: {
          x: Math.max(0, (e.clientX - dragOffset.x - panOffset.x) / zoomLevel),
          y: Math.max(0, (e.clientY - dragOffset.y - panOffset.y) / zoomLevel)
        }
      };
    }
  }

  function handleCanvasMouseUp() {
    isPanning = false;
    activeDragTable = null;
  }

  function handleTableMouseDown(e: MouseEvent, tableName: string) {
    e.stopPropagation();
    activeDragTable = tableName;
    const currentPos = tablePositions[tableName] || { x: 100, y: 100 };
    dragOffset = {
      x: e.clientX - (currentPos.x * zoomLevel + panOffset.x),
      y: e.clientY - (currentPos.y * zoomLevel + panOffset.y)
    };
    selectedTable = tableName;
  }

  function zoomIn() {
    zoomLevel = Math.min(2.0, Number((zoomLevel + 0.15).toFixed(2)));
  }

  function zoomOut() {
    zoomLevel = Math.max(0.4, Number((zoomLevel - 0.15).toFixed(2)));
  }

  function resetView() {
    zoomLevel = 1;
    panOffset = { x: 50, y: 50 };
    if (schemaData?.tables) {
      autoArrangeTables(schemaData.tables);
    }
  }

  function calculateConnectorPath(rel: any) {
    const fromPos = tablePositions[rel.from_table];
    const toPos = tablePositions[rel.to_table];
    if (!fromPos || !toPos) return '';

    const width = 270;
    const fromX = fromPos.x < toPos.x ? fromPos.x + width : fromPos.x;
    const fromY = fromPos.y + 45;
    const toX = fromPos.x < toPos.x ? toPos.x : toPos.x + width;
    const toY = toPos.y + 45;

    const dx = Math.abs(toX - fromX) * 0.5;
    const cx1 = fromPos.x < toPos.x ? fromX + dx : fromX - dx;
    const cx2 = fromPos.x < toPos.x ? toX - dx : toX + dx;

    return `M ${fromX} ${fromY} C ${cx1} ${fromY}, ${cx2} ${toY}, ${toX} ${toY}`;
  }

  async function createSampleEcommerceSchema() {
    if (!confirm('Load sample E-Commerce schema (Users, Categories, Products, Orders, OrderItems)?')) return;
    actionLoading = true;
    try {
      let sampleSql = '';
      if (database?.engine === 'mysql') {
        sampleSql = `
          CREATE TABLE IF NOT EXISTS users (
            id INT AUTO_INCREMENT PRIMARY KEY,
            email VARCHAR(255) NOT NULL UNIQUE,
            full_name VARCHAR(100) NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
          );
          CREATE TABLE IF NOT EXISTS categories (
            id INT AUTO_INCREMENT PRIMARY KEY,
            name VARCHAR(100) NOT NULL,
            slug VARCHAR(100) NOT NULL UNIQUE
          );
          CREATE TABLE IF NOT EXISTS products (
            id INT AUTO_INCREMENT PRIMARY KEY,
            category_id INT NOT NULL,
            name VARCHAR(255) NOT NULL,
            price DECIMAL(10,2) NOT NULL,
            stock INT DEFAULT 0,
            FOREIGN KEY (category_id) REFERENCES categories(id) ON DELETE CASCADE
          );
          CREATE TABLE IF NOT EXISTS orders (
            id INT AUTO_INCREMENT PRIMARY KEY,
            user_id INT NOT NULL,
            total_amount DECIMAL(10,2) NOT NULL,
            status VARCHAR(50) DEFAULT 'pending',
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
          );
          CREATE TABLE IF NOT EXISTS order_items (
            id INT AUTO_INCREMENT PRIMARY KEY,
            order_id INT NOT NULL,
            product_id INT NOT NULL,
            quantity INT NOT NULL,
            price DECIMAL(10,2) NOT NULL,
            FOREIGN KEY (order_id) REFERENCES orders(id) ON DELETE CASCADE,
            FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE
          );
        `;
      } else {
        sampleSql = `
          CREATE TABLE IF NOT EXISTS users (
            id BIGSERIAL PRIMARY KEY,
            email VARCHAR(255) NOT NULL UNIQUE,
            full_name VARCHAR(100) NOT NULL,
            created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
          );
          CREATE TABLE IF NOT EXISTS categories (
            id BIGSERIAL PRIMARY KEY,
            name VARCHAR(100) NOT NULL,
            slug VARCHAR(100) NOT NULL UNIQUE
          );
          CREATE TABLE IF NOT EXISTS products (
            id BIGSERIAL PRIMARY KEY,
            category_id BIGINT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
            name VARCHAR(255) NOT NULL,
            price NUMERIC(10,2) NOT NULL,
            stock INT DEFAULT 0
          );
          CREATE TABLE IF NOT EXISTS orders (
            id BIGSERIAL PRIMARY KEY,
            user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            total_amount NUMERIC(10,2) NOT NULL,
            status VARCHAR(50) DEFAULT 'pending',
            created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
          );
          CREATE TABLE IF NOT EXISTS order_items (
            id BIGSERIAL PRIMARY KEY,
            order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
            product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
            quantity INT NOT NULL,
            price NUMERIC(10,2) NOT NULL
          );
        `;
      }
      const res = await fetch(`/api/v1/databases/${id}/query`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ query: sampleSql })
      });
      if (res.ok) {
        await loadSchema();
      }
    } catch (e: any) {
      alert('Failed to create sample schema: ' + e.message);
    } finally {
      actionLoading = false;
    }
  }

  function statusClass(status: string) {
    switch (status?.toLowerCase()) {
      case 'ready':
      case 'running':
        return 'badge badge-running';
      case 'deploying':
      case 'building':
      case 'starting':
      case 'restarting':
      case 'provisioning':
        return 'badge badge-building';
      case 'failed':
      case 'error':
      case 'dead':
        return 'badge badge-failed';
      case 'stopped':
      case 'paused':
      case 'exited':
        return 'badge badge-stopped';
      default:
        return 'badge badge-pending';
    }
  }

  onMount(() => {
    loadDatabase();
    pollInterval = setInterval(() => {
      if (tab === 'logs') {
        fetchLogs();
      }
      const st = (database?.runtime_status || '').toLowerCase();
      if (st === 'restarting' || st === 'provisioning' || st === 'starting') {
        loadDatabase();
      }
    }, 2500);
  });

  $effect(() => {
    if (tab === 'visualizer' && !schemaData && !schemaLoading) {
      loadSchema();
    }
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
      internalConnectionUri: internalUri,
      externalConnectionUri: externalUri,
      externalHost: externalHost,
      externalPort: externalPort
    };
  });

  const cliCommand = $derived.by(() => {
    const engine = database?.engine || 'postgres';
    const m = parsedMeta;
    switch (engine) {
      case 'postgres':
        return `psql "${m.externalConnectionUri}"`;
      case 'mysql':
        return `mysql -h ${m.externalHost} -P ${m.externalPort} -u ${m.username} -p ${m.databaseName}`;
      case 'redis':
        return `redis-cli -h ${m.externalHost} -p ${m.externalPort} -a "${m.password}"`;
      case 'mongodb':
        return `mongosh "${m.externalConnectionUri}"`;
      case 'clickhouse':
        return `clickhouse-client --host ${m.externalHost} --port ${m.externalPort} --user ${m.username} --password "${m.password}" --database ${m.databaseName}`;
      default:
        return `psql "${m.externalConnectionUri}"`;
    }
  });

  async function copyText(text: string, fieldName: string = 'general') {
    try {
      await navigator.clipboard.writeText(text);
      copied = true;
      copiedField = fieldName;
      setTimeout(() => {
        copied = false;
        copiedField = null;
      }, 2000);
    } catch (e) {
      console.error(e);
    }
  }

  async function restartDatabase() {
    actionLoading = true;
    try {
      const res = await fetch(`/api/v1/databases/${id}/restart`, { method: 'POST', credentials: 'include' });
      if (res.ok) {
        await loadDatabase();
      }
    } finally {
      actionLoading = false;
    }
  }

  async function deleteDatabase() {
    if (!confirm(`Are you sure you want to permanently delete database "${database?.name}"? All data inside will be permanently lost.`)) return;
    actionLoading = true;
    try {
      const res = await fetch(`/api/v1/databases/${id}`, { method: 'DELETE', credentials: 'include' });
      if (res.ok) {
        goto('/databases');
      } else {
        alert('Failed to delete database');
      }
    } finally {
      actionLoading = false;
    }
  }

  async function runQuery() {
    if (!queryText.trim()) return;
    queryLoading = true;
    queryResult = null;
    try {
      const res = await fetch(`/api/v1/databases/${id}/query`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ query: queryText })
      });
      const data = await res.json();
      if (res.ok) {
        queryResult = data;
      } else {
        queryResult = { error: data.error || 'Failed to execute query' };
      }
    } catch (e: any) {
      queryResult = { error: e.message };
    } finally {
      queryLoading = false;
    }
  }

  function setTemplateQuery(q: string) {
    queryText = q;
  }

  const filteredTables = $derived.by(() => {
    if (!schemaData?.tables) return [];
    if (!searchTable.trim()) return schemaData.tables;
    const q = searchTable.toLowerCase().trim();
    return schemaData.tables.filter((t: any) => 
      t.name.toLowerCase().includes(q) || 
      t.columns?.some((c: any) => c.name.toLowerCase().includes(q))
    );
  });
</script>

<svelte:head>
  <title>{database?.name || 'Database'} - kloudsPanel</title>
</svelte:head>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={48} /></div>
    <p>Loading database details...</p>
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
        <span class={statusClass(database?.runtime_status || 'provisioning')}>
          {#if (database?.runtime_status || '').toLowerCase() === 'restarting' || (database?.runtime_status || '').toLowerCase() === 'starting' || (database?.runtime_status || '').toLowerCase() === 'provisioning'}
            <Loader2 size={12} class="animate-spin" style="margin-right: 3px;" />
          {/if}
          {database?.runtime_status || 'provisioning'}
        </span>
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
        {#if t === 'visualizer'}<Network size={15} />{/if}
        {#if t === 'logs'}<Code size={15} />{/if}
        {#if t === 'settings'}<Settings size={15} />{/if}
        {t === 'query' ? 'SQL / Query Studio' : t === 'visualizer' ? 'ER Diagram & Schema' : t.charAt(0).toUpperCase() + t.slice(1)}
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
            <button type="button" class="btn btn-secondary" style="font-size: 0.75rem; padding: 3px 8px;" onclick={() => setTemplateQuery('DBSIZE')}>
              DBSIZE
            </button>
          {:else if database?.engine === 'mongodb'}
            <button type="button" class="btn btn-secondary" style="font-size: 0.75rem; padding: 3px 8px;" onclick={() => setTemplateQuery('db.stats()')}>
              DB Stats
            </button>
            <button type="button" class="btn btn-secondary" style="font-size: 0.75rem; padding: 3px 8px;" onclick={() => setTemplateQuery('db.getCollectionNames()')}>
              Collections
            </button>
          {/if}
        </div>
      </div>

      <div style="position: relative; margin-bottom: 0.85rem;">
        <textarea
          class="form-input font-mono"
          bind:value={queryText}
          rows="4"
          style="width: 100%; resize: vertical; font-size: 0.875rem; background: #0d1117; color: #58a6ff; border-color: #30363d; border-radius: var(--radius-md); padding: 0.75rem 1rem;"
          placeholder="Enter SQL query or command..."
        ></textarea>
      </div>

      <div style="display: flex; justify-content: space-between; align-items: center; flex-wrap: wrap; gap: 0.5rem;">
        <div class="text-xs text-muted">
          Tip: Select queries return formatted tabular output.
        </div>
        <button
          type="button"
          class="btn btn-primary"
          style="display: inline-flex; align-items: center; gap: 6px; padding: 6px 18px; font-weight: 600;"
          onclick={runQuery}
          disabled={queryLoading}
        >
          {#if queryLoading}
            <Loader2 size={15} class="animate-spin" /> Executing...
          {:else}
            <Play size={15} /> Execute Query
          {/if}
        </button>
      </div>
    </div>

    <!-- Query Results Container -->
    {#if queryResult}
      <div class="card" style="margin-bottom: 1.5rem; padding: 1.25rem; background: var(--color-surface); border: 1px solid var(--color-border);">
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

  <!-- ══════════════════════════════════════════════════════════════════════════ -->
  <!-- TAB 3: ER DIAGRAM & SCHEMA VISUALIZER                                      -->
  <!-- ══════════════════════════════════════════════════════════════════════════ -->
  {:else if tab === 'visualizer'}
    <div class="card" style="padding: 1.25rem; margin-bottom: 1.5rem; background: var(--color-surface); border: 1px solid var(--color-border);">
      <!-- Visualizer Control Toolbar -->
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; flex-wrap: wrap; gap: 0.75rem;">
        <div style="display: flex; align-items: center; gap: 0.75rem; flex-wrap: wrap;">
          <div style="font-weight: 700; font-size: 1.05rem; display: flex; align-items: center; gap: 6px;">
            <Network size={18} style="color: var(--color-accent);" /> ER Diagram & Schema Visualizer
          </div>
          {#if schemaData}
            <span class="badge" style="background: rgba(0,166,166,0.1); color: var(--color-accent); font-weight: 600;">
              {schemaData.table_count || 0} Tables
            </span>
            <span class="badge" style="background: rgba(16,185,129,0.1); color: #059669; font-weight: 600;">
              {schemaData.relationships?.length || 0} Relations
            </span>
          {/if}
        </div>

        <!-- Controls: Search, Zoom, Reset, Refresh -->
        <div style="display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap;">
          <div style="position: relative; width: 180px;">
            <input 
              type="text" 
              class="form-input" 
              style="padding: 4px 8px 4px 28px; font-size: 0.8125rem; height: 30px;"
              placeholder="Search tables..."
              bind:value={searchTable}
            />
            <Search size={14} style="position: absolute; left: 8px; top: 8px; color: var(--color-ink-muted);" />
          </div>

          <div style="display: flex; align-items: center; background: var(--color-surface-sunken); border-radius: var(--radius-md); border: 1px solid var(--color-border); padding: 2px;">
            <button class="btn btn-secondary" style="padding: 3px 8px; height: 26px; border: none;" onclick={zoomOut} title="Zoom Out">
              <ZoomOut size={13} />
            </button>
            <span style="font-family: var(--font-mono); font-size: 0.75rem; padding: 0 6px; min-width: 44px; text-align: center; font-weight: 600;">
              {Math.round(zoomLevel * 100)}%
            </span>
            <button class="btn btn-secondary" style="padding: 3px 8px; height: 26px; border: none;" onclick={zoomIn} title="Zoom In">
              <ZoomIn size={13} />
            </button>
          </div>

          <button class="btn btn-secondary" style="padding: 4px 10px; height: 30px; font-size: 0.8125rem;" onclick={resetView} title="Reset view and rearrange layout">
            <Maximize2 size={13} /> Reset Grid
          </button>

          <button class="btn btn-secondary" style="padding: 4px 10px; height: 30px; font-size: 0.8125rem;" onclick={loadSchema} disabled={schemaLoading}>
            {#if schemaLoading}<Loader2 size={13} class="animate-spin" />{:else}<RefreshCw size={13} />{/if} Refresh
          </button>
        </div>
      </div>

      <!-- Interactive Canvas Viewport -->
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div 
        class="er-canvas-container"
        style="
          position: relative;
          height: 600px;
          background: #090d16;
          border-radius: var(--radius-md);
          border: 1px solid #1e293b;
          overflow: hidden;
          cursor: {isPanning ? 'grabbing' : 'grab'};
          user-select: none;
          background-image: radial-gradient(rgba(255,255,255,0.08) 1px, transparent 1px);
          background-size: 24px 24px;
        "
        onmousedown={handleCanvasMouseDown}
        onmousemove={handleCanvasMouseMove}
        onmouseup={handleCanvasMouseUp}
      >
        <!-- Canvas Instructions overlay -->
        <div style="position: absolute; bottom: 12px; left: 12px; z-index: 10; pointer-events: none; background: rgba(15,23,42,0.85); border: 1px solid rgba(255,255,255,0.1); border-radius: 6px; padding: 4px 10px; font-size: 0.75rem; color: #94a3b8; display: flex; align-items: center; gap: 8px;">
          <Move size={12} /> Drag canvas to pan • Drag table headers to reposition • Click tables to inspect
        </div>

        {#if schemaLoading}
          <div style="display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100%; color: #94a3b8;">
            <Loader2 size={36} class="animate-spin" style="color: var(--color-accent); margin-bottom: 0.75rem;" />
            <p style="font-size: 0.875rem;">Inspecting database schema & relationships...</p>
          </div>
        {:else if !schemaData || (schemaData.tables && schemaData.tables.length === 0)}
          <!-- Empty State with 1-click sample schema creator -->
          <div style="display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100%; text-align: center; padding: 2rem;">
            <div style="width: 56px; height: 56px; border-radius: 50%; background: rgba(0,166,166,0.12); color: var(--color-accent); display: flex; align-items: center; justify-content: center; margin-bottom: 1rem;">
              <Network size={28} />
            </div>
            <h3 style="color: #fff; margin: 0 0 0.5rem 0; font-size: 1.15rem;">No Tables Found in Database</h3>
            <p style="color: #94a3b8; font-size: 0.875rem; max-width: 460px; margin-bottom: 1.25rem; line-height: 1.5;">
              Database <span class="font-mono" style="color:#7dd3fc;">{parsedMeta.databaseName}</span> has no tables created yet. You can load a sample schema or create tables via SQL Studio.
            </p>
            <div style="display: flex; gap: 0.75rem; flex-wrap: wrap; justify-content: center;">
              <button 
                class="btn btn-primary" 
                style="font-weight: 600; font-size: 0.875rem; padding: 8px 18px; display: inline-flex; align-items: center; gap: 6px;"
                onclick={createSampleEcommerceSchema}
                disabled={actionLoading}
              >
                {#if actionLoading}<Loader2 size={14} class="animate-spin" />{:else}<Sparkles size={14} />{/if}
                Load Sample E-Commerce Schema
              </button>
              <a 
                href="/databases/{id}/query" 
                class="btn btn-secondary" 
                style="background: rgba(255,255,255,0.06); color: #fff; border-color: rgba(255,255,255,0.15); font-size: 0.875rem; padding: 8px 18px; text-decoration: none; display: inline-flex; align-items: center; gap: 6px;"
              >
                <Terminal size={14} /> Open SQL Studio
              </a>
            </div>
          </div>
        {:else}
          <!-- Canvas Graph Inner Transform Container -->
          <div 
            class="canvas-world"
            style="
              position: absolute;
              top: 0;
              left: 0;
              transform-origin: 0 0;
              transform: translate({panOffset.x}px, {panOffset.y}px) scale({zoomLevel});
              width: 3000px;
              height: 3000px;
            "
          >
            <!-- SVG Relationships Curves Layer -->
            <svg style="position: absolute; top: 0; left: 0; width: 3000px; height: 3000px; pointer-events: none; overflow: visible;">
              <defs>
                <marker id="arrow" viewBox="0 0 10 10" refX="6" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
                  <path d="M 0 1 L 10 5 L 0 9 z" fill="#0284c7" />
                </marker>
                <linearGradient id="relGradient" x1="0%" y1="0%" x2="100%" y2="0%">
                  <stop offset="0%" stop-color="#0284c7" />
                  <stop offset="100%" stop-color="#38bdf8" />
                </linearGradient>
              </defs>

              {#if schemaData?.relationships}
                {#each schemaData.relationships as rel}
                  {@const path = calculateConnectorPath(rel)}
                  {#if path}
                    <path
                      d={path}
                      fill="none"
                      stroke="url(#relGradient)"
                      stroke-width="2.5"
                      stroke-dasharray="6,3"
                      marker-end="url(#arrow)"
                      style="opacity: 0.85; transition: stroke-width 0.2s;"
                    />
                  {/if}
                {/each}
              {/if}
            </svg>

            <!-- Table Nodes -->
            {#each filteredTables as table}
              {@const pos = tablePositions[table.name] || { x: 60, y: 60 }}
              <!-- svelte-ignore a11y_no_static_element_interactions -->
              <!-- svelte-ignore a11y_click_events_have_key_events -->
              <div 
                class="table-node"
                style="
                  position: absolute;
                  left: {pos.x}px;
                  top: {pos.y}px;
                  width: 270px;
                  background: #0f172a;
                  border: 1.5px solid {selectedTable === table.name ? 'var(--color-accent)' : '#334155'};
                  border-radius: 8px;
                  box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.6), 0 8px 10px -6px rgba(0, 0, 0, 0.6);
                  overflow: hidden;
                  z-index: {selectedTable === table.name ? 5 : 2};
                  transition: border-color 0.15s, box-shadow 0.15s;
                "
                onclick={() => selectedTable = table.name}
              >
                <!-- Table Header (Draggable) -->
                <!-- svelte-ignore a11y_no_static_element_interactions -->
                <div 
                  style="
                    background: #1e293b;
                    padding: 8px 12px;
                    display: flex;
                    align-items: center;
                    justify-content: space-between;
                    border-bottom: 1px solid #334155;
                    cursor: move;
                  "
                  onmousedown={(e) => handleTableMouseDown(e, table.name)}
                >
                  <div style="display: flex; align-items: center; gap: 7px; overflow: hidden;">
                    <Table size={14} style="color: var(--color-accent); flex-shrink: 0;" />
                    <span style="font-weight: 700; font-size: 0.875rem; color: #f8fafc; text-overflow: ellipsis; overflow: hidden; white-space: nowrap;">
                      {table.name}
                    </span>
                  </div>
                  <span style="font-size: 0.6875rem; background: rgba(255,255,255,0.08); color: #94a3b8; padding: 1px 6px; border-radius: 4px; font-weight: 600;">
                    {table.column_count || table.columns?.length || 0}
                  </span>
                </div>

                <!-- Column Items -->
                <div style="padding: 4px 0; max-height: 240px; overflow-y: auto; background: #0f172a;">
                  {#if table.columns && table.columns.length > 0}
                    {#each table.columns as col}
                      <div style="
                        display: flex;
                        align-items: center;
                        justify-content: space-between;
                        padding: 5px 12px;
                        font-size: 0.75rem;
                        border-bottom: 1px solid rgba(255,255,255,0.03);
                        gap: 6px;
                      ">
                        <div style="display: flex; align-items: center; gap: 6px; min-width: 0;">
                          {#if col.is_primary}
                            <span title="Primary Key" style="display:inline-flex;"><Key size={12} style="color: #f59e0b; flex-shrink: 0;" /></span>
                          {:else if col.is_foreign}
                            <span title="Foreign Key" style="display:inline-flex;"><Link size={12} style="color: #38bdf8; flex-shrink: 0;" /></span>
                          {:else}
                            <div style="width: 5px; height: 5px; border-radius: 50%; background: #475569; flex-shrink: 0; margin: 0 3px;"></div>
                          {/if}
                          <span style="
                            font-family: var(--font-mono);
                            font-weight: {col.is_primary ? 700 : 500};
                            color: {col.is_primary ? '#fbbf24' : col.is_foreign ? '#7dd3fc' : '#e2e8f0'};
                            text-overflow: ellipsis;
                            overflow: hidden;
                            white-space: nowrap;
                          ">
                            {col.name}
                          </span>
                        </div>

                        <span style="
                          font-family: var(--font-mono);
                          font-size: 0.6875rem;
                          color: #94a3b8;
                          background: rgba(255,255,255,0.05);
                          padding: 1px 5px;
                          border-radius: 3px;
                          white-space: nowrap;
                          flex-shrink: 0;
                        ">
                          {col.type}
                        </span>
                      </div>
                    {/each}
                  {:else}
                    <div style="padding: 8px 12px; color: #64748b; font-size: 0.75rem; text-align: center;">
                      No column metadata
                    </div>
                  {/if}
                </div>
              </div>
            {/each}
          </div>
        {/if}
      </div>
    </div>

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
    <div class="card" style="border-color:#fca5a5; margin-bottom:1.5rem; background: var(--color-surface);">
      <div class="card-header" style="border-bottom-color:#fee2e2;">
        <h3 style="color:var(--color-danger); margin:0;">Danger Zone</h3>
      </div>
      <div style="display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:1rem; padding:0.5rem 0;">
        <div>
          <div style="font-weight:600; color:var(--color-ink);">Delete this Database</div>
          <div class="text-sm text-muted">Permanently erase this database instance, its Docker container, and stored data volume.</div>
        </div>
        <button class="btn btn-danger" onclick={deleteDatabase} disabled={actionLoading}>
          <Trash2 size={16} /> Delete Database
        </button>
      </div>
    </div>
  {/if}
{/if}
