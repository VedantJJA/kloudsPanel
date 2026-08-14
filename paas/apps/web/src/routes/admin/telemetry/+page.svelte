<script lang="ts">
  import { onMount } from 'svelte';
  import { Loader2, Zap, Brain, HardDrive, RefreshCw, Satellite, Activity, Terminal, Layers, ShieldCheck, ShieldAlert, Sparkles, Filter } from 'lucide-svelte';

  type Metric = {
    cpu_percent: number;
    memory_total_bytes: number;
    memory_used_bytes: number;
    storage_total_bytes: number;
    storage_used_bytes: number;
    load1: number;
    load5?: number;
    load15?: number;
    active_containers?: number;
  };

  type TrendPoint = {
    timestamp: string;
    cpu_percent: number;
    memory_used_percent: number;
    memory_used_bytes: number;
    memory_total_bytes: number;
    storage_used_percent: number;
    storage_used_bytes: number;
    storage_total_bytes: number;
    load1: number;
    load5: number;
    load15: number;
    active_containers: number;
  };

  type LogEntry = {
    timestamp: string;
    level: string;
    source: string;
    message: string;
  };

  let metrics = $state<Metric | null>(null);
  let trends = $state<TrendPoint[]>([]);
  let logs = $state<LogEntry[]>([]);
  let loading = $state(true);
  let refreshInterval: ReturnType<typeof setInterval>;
  let selectedTab = $state<'charts' | 'logs'>('charts');
  let logFilter = $state('');
  let levelFilter = $state('all');
  let autoRefresh = $state(true);

  const fmt = (bytes: number) => {
    const gb = bytes / 1024 / 1024 / 1024;
    return gb >= 1 ? `${gb.toFixed(1)} GB` : `${(bytes / 1024 / 1024).toFixed(0)} MB`;
  };

  const pct = (used: number, total: number) => total > 0 ? Math.round((used / total) * 100) : 0;
  const barClass = (p: number) => p >= 90 ? 'critical' : p >= 80 ? 'warn' : '';

  async function fetchTelemetry() {
    try {
      const res = await fetch('/api/v1/admin/telemetry', { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        metrics = data.host ?? null;
        if (data.trends && Array.isArray(data.trends)) {
          trends = data.trends;
        }
        if (data.logs && Array.isArray(data.logs)) {
          logs = data.logs;
        }
      }
    } catch {}
    loading = false;
  }

  // SVG Chart path generator
  function generateSvgPath(data: number[], width: number, height: number, maxVal = 100): string {
    if (!data || data.length === 0) return '';
    const step = width / Math.max(data.length - 1, 1);
    return data.map((val, i) => {
      const x = i * step;
      const normalized = Math.min(Math.max(val, 0), maxVal);
      const y = height - (normalized / maxVal) * (height - 10) - 5;
      return `${i === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`;
    }).join(' ');
  }

  function generateAreaPath(data: number[], width: number, height: number, maxVal = 100): string {
    const line = generateSvgPath(data, width, height, maxVal);
    if (!line) return '';
    const lastX = width;
    return `${line} L ${lastX} ${height} L 0 ${height} Z`;
  }

  onMount(() => {
    fetchTelemetry();
    refreshInterval = setInterval(() => {
      if (autoRefresh) fetchTelemetry();
    }, 4000);
    return () => clearInterval(refreshInterval);
  });

  let filteredLogs = $derived(
    logs.filter(l => {
      const matchesText = !logFilter || l.message.toLowerCase().includes(logFilter.toLowerCase()) || l.source.toLowerCase().includes(logFilter.toLowerCase());
      const matchesLevel = levelFilter === 'all' || l.level.toLowerCase() === levelFilter.toLowerCase();
      return matchesText && matchesLevel;
    })
  );

  let cpuValues = $derived(trends.map(t => t.cpu_percent));
  let memValues = $derived(trends.map(t => t.memory_used_percent));
  let loadValues = $derived(trends.map(t => t.load1 * 20)); // scaled for graph

  let avgCpu = $derived(cpuValues.length > 0 ? (cpuValues.reduce((a, b) => a + b, 0) / cpuValues.length).toFixed(1) : '0');
  let maxCpu = $derived(cpuValues.length > 0 ? Math.max(...cpuValues).toFixed(1) : '0');
  let avgMem = $derived(memValues.length > 0 ? (memValues.reduce((a, b) => a + b, 0) / memValues.length).toFixed(1) : '0');
</script>

<svelte:head>
  <title>Telemetry - kloudsPanel</title>
</svelte:head>

<div class="page-header" style="margin-bottom: 1.5rem;">
  <div>
    <div style="display:flex;align-items:center;gap:0.5rem">
      <h1 class="page-title">Platform Telemetry</h1>
      <span class="badge badge-running" style="font-size:0.75rem;padding:3px 8px;display:flex;align-items:center;gap:4px">
        <Activity size={12} class="animate-pulse" /> Live Telemetry Engine
      </span>
    </div>
    <p class="page-subtitle">Real-time host capacity, container resource trends, and system telemetry logs</p>
  </div>
  <div style="display:flex;align-items:center;gap:0.75rem">
    <button 
      class="btn {autoRefresh ? 'btn-primary' : 'btn-secondary'}" 
      onclick={() => autoRefresh = !autoRefresh}
      style="font-size:0.8125rem;padding:6px 12px"
    >
      <Activity size={14} /> {autoRefresh ? 'Live Streaming (4s)' : 'Stream Paused'}
    </button>
    <button class="btn btn-secondary" onclick={fetchTelemetry} style="font-size:0.8125rem;padding:6px 12px">
      <RefreshCw size={14} /> Refresh
    </button>
  </div>
</div>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin" style="opacity:0.4;margin-bottom:1rem"><Loader2 size={48} /></div>
    <p>Connecting to host telemetry daemon...</p>
  </div>
{:else if !metrics}
  <div class="empty-state">
    <div class="empty-state-icon"><Satellite size={48} /></div>
    <h3>No metrics available</h3>
    <p>The host agent must be running to stream capacity metrics.</p>
  </div>
{:else}
  <!-- Capacity Metric Cards -->
  <div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(240px,1fr));gap:1rem;margin-bottom:1.5rem">
    {#each [
      {
        label: 'CPU Utilization',
        icon: Zap,
        value: `${metrics.cpu_percent.toFixed(1)}%`,
        sub: `Load avg: ${metrics.load1?.toFixed(2) ?? '0.00'} (1m)`,
        pct: metrics.cpu_percent,
        color: 'var(--color-accent)'
      },
      {
        label: 'Memory Usage',
        icon: Brain,
        value: `${fmt(metrics.memory_used_bytes)} / ${fmt(metrics.memory_total_bytes)}`,
        sub: `${pct(metrics.memory_used_bytes, metrics.memory_total_bytes)}% allocated`,
        pct: pct(metrics.memory_used_bytes, metrics.memory_total_bytes),
        color: 'var(--color-primary)'
      },
      {
        label: 'Storage Capacity',
        icon: HardDrive,
        value: `${fmt(metrics.storage_used_bytes)} / ${fmt(metrics.storage_total_bytes)}`,
        sub: `${pct(metrics.storage_used_bytes, metrics.storage_total_bytes)}% disk used`,
        pct: pct(metrics.storage_used_bytes, metrics.storage_total_bytes),
        color: 'var(--color-success)'
      },
      {
        label: 'Active Containers',
        icon: Layers,
        value: `${metrics.active_containers ?? 0} Containers`,
        sub: `Network: platform-control`,
        pct: Math.min((metrics.active_containers ?? 0) * 10, 100),
        color: '#8b5cf6'
      }
    ] as m}
      {@const Icon = m.icon}
      <div class="card" style="padding:1.25rem;">
        <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:0.75rem">
          <div style="display:flex;align-items:center;gap:0.65rem">
            <div style="width:36px;height:36px;border-radius:var(--radius-md);background:rgba(0,166,166,0.1);color:var(--color-accent);display:flex;align-items:center;justify-content:center">
              <Icon size={20} />
            </div>
            <div>
              <div class="text-xs text-muted" style="font-weight:600">{m.label}</div>
              <div style="font-size:1.125rem;font-weight:700">{m.value}</div>
            </div>
          </div>
        </div>
        <div class="capacity-bar" style="height:6px;">
          <div
            class="capacity-bar-fill {barClass(m.pct)}"
            style="width:{Math.min(m.pct,100)}%"
            role="progressbar"
            aria-valuenow={m.pct}
            aria-valuemin={0}
            aria-valuemax={100}
          ></div>
        </div>
        <div class="text-xs text-muted" style="margin-top:0.5rem">{m.sub}</div>
        {#if m.pct >= 90}
          <div style="color:var(--color-danger);font-size:0.75rem;font-weight:600;margin-top:0.35rem;display:flex;align-items:center;gap:4px">
            <ShieldAlert size={12} /> Critical - deployment capacity low
          </div>
        {:else if m.pct >= 80}
          <div style="color:var(--color-warning);font-size:0.75rem;font-weight:600;margin-top:0.35rem;display:flex;align-items:center;gap:4px">
            <ShieldAlert size={12} /> Warning - high utilization
          </div>
        {/if}
      </div>
    {/each}
  </div>

  <!-- Tabs Navigation -->
  <div style="display:flex;align-items:center;justify-content:space-between;margin-bottom:1rem;border-bottom:1px solid var(--color-border);padding-bottom:0.5rem;">
    <div style="display:flex;gap:0.5rem">
      <button 
        class="btn {selectedTab === 'charts' ? 'btn-primary' : 'btn-secondary'}"
        style="padding:6px 16px;font-size:0.8125rem"
        onclick={() => selectedTab = 'charts'}
      >
        <Activity size={14} /> Real-Time Trends & Graphs
      </button>
      <button 
        class="btn {selectedTab === 'logs' ? 'btn-primary' : 'btn-secondary'}"
        style="padding:6px 16px;font-size:0.8125rem;display:flex;align-items:center;gap:6px"
        onclick={() => selectedTab = 'logs'}
      >
        <Terminal size={14} /> Telemetry Logs ({logs.length})
      </button>
    </div>
    <div class="text-xs text-muted">
      Buffer: {trends.length} points recorded
    </div>
  </div>

  <!-- TAB 1: Live Interactive Trend Graphs -->
  {#if selectedTab === 'charts'}
    <div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(450px,1fr));gap:1rem;margin-bottom:1.5rem">
      
      <!-- CPU Trend Chart -->
      <div class="card" style="padding:1.25rem;">
        <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:1rem">
          <div>
            <div style="font-weight:700;font-size:0.9375rem;display:flex;align-items:center;gap:6px">
              <Zap size={16} style="color:var(--color-accent);" /> CPU Utilization Trend (%)
            </div>
            <div class="text-xs text-muted">Live 60-sample time-series snapshot</div>
          </div>
          <div style="text-align:right">
            <span class="badge badge-running" style="font-size:0.75rem">Current: {metrics.cpu_percent.toFixed(1)}%</span>
            <div class="text-xs text-muted" style="margin-top:2px">Avg: {avgCpu}% • Peak: {maxCpu}%</div>
          </div>
        </div>

        <!-- SVG CPU Line/Area Graph -->
        <div style="background:rgba(0,0,0,0.03);border-radius:var(--radius-md);padding:0.75rem;border:1px solid var(--color-border);">
          <svg viewBox="0 0 500 120" style="width:100%;height:140px;overflow:visible;">
            <defs>
              <linearGradient id="cpuGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" stop-color="var(--color-accent)" stop-opacity="0.4" />
                <stop offset="100%" stop-color="var(--color-accent)" stop-opacity="0.0" />
              </linearGradient>
            </defs>
            <!-- Grid Lines -->
            <line x1="0" y1="10" x2="500" y2="10" stroke="var(--color-border)" stroke-dasharray="3,3" />
            <line x1="0" y1="60" x2="500" y2="60" stroke="var(--color-border)" stroke-dasharray="3,3" />
            <line x1="0" y1="110" x2="500" y2="110" stroke="var(--color-border)" stroke-dasharray="3,3" />
            <text x="5" y="18" fill="var(--color-ink-muted)" font-size="9">100%</text>
            <text x="5" y="68" fill="var(--color-ink-muted)" font-size="9">50%</text>
            <text x="5" y="116" fill="var(--color-ink-muted)" font-size="9">0%</text>
            <!-- Area & Line -->
            <path d={generateAreaPath(cpuValues, 500, 120, 100)} fill="url(#cpuGrad)" />
            <path d={generateSvgPath(cpuValues, 500, 120, 100)} fill="none" stroke="var(--color-accent)" stroke-width="2.5" stroke-linecap="round" />
          </svg>
        </div>
      </div>

      <!-- Memory Trend Chart -->
      <div class="card" style="padding:1.25rem;">
        <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:1rem">
          <div>
            <div style="font-weight:700;font-size:0.9375rem;display:flex;align-items:center;gap:6px">
              <Brain size={16} style="color:var(--color-primary);" /> Memory Usage Trend (%)
            </div>
            <div class="text-xs text-muted">Live host RAM allocation trend</div>
          </div>
          <div style="text-align:right">
            <span class="badge" style="background:#e0e7ff;color:#3730a3;font-size:0.75rem">Current: {pct(metrics.memory_used_bytes, metrics.memory_total_bytes)}%</span>
            <div class="text-xs text-muted" style="margin-top:2px">Avg: {avgMem}% • {fmt(metrics.memory_used_bytes)}</div>
          </div>
        </div>

        <!-- SVG Memory Line/Area Graph -->
        <div style="background:rgba(0,0,0,0.03);border-radius:var(--radius-md);padding:0.75rem;border:1px solid var(--color-border);">
          <svg viewBox="0 0 500 120" style="width:100%;height:140px;overflow:visible;">
            <defs>
              <linearGradient id="memGrad" x1="0%" y1="0%" x2="0%" y2="100%">
                <stop offset="0%" stop-color="#3b82f6" stop-opacity="0.35" />
                <stop offset="100%" stop-color="#3b82f6" stop-opacity="0.0" />
              </linearGradient>
            </defs>
            <!-- Grid Lines -->
            <line x1="0" y1="10" x2="500" y2="10" stroke="var(--color-border)" stroke-dasharray="3,3" />
            <line x1="0" y1="60" x2="500" y2="60" stroke="var(--color-border)" stroke-dasharray="3,3" />
            <line x1="0" y1="110" x2="500" y2="110" stroke="var(--color-border)" stroke-dasharray="3,3" />
            <text x="5" y="18" fill="var(--color-ink-muted)" font-size="9">100%</text>
            <text x="5" y="68" fill="var(--color-ink-muted)" font-size="9">50%</text>
            <text x="5" y="116" fill="var(--color-ink-muted)" font-size="9">0%</text>
            <!-- Area & Line -->
            <path d={generateAreaPath(memValues, 500, 120, 100)} fill="url(#memGrad)" />
            <path d={generateSvgPath(memValues, 500, 120, 100)} fill="none" stroke="#3b82f6" stroke-width="2.5" stroke-linecap="round" />
          </svg>
        </div>
      </div>

    </div>
  {/if}

  <!-- TAB 2: Live Telemetry Event Logs -->
  {#if selectedTab === 'logs' || selectedTab === 'charts'}
    <div class="card" style="padding:1.25rem;margin-bottom:1.5rem">
      <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:1rem;flex-wrap:wrap;gap:0.75rem">
        <div>
          <div style="font-weight:700;font-size:0.9375rem;display:flex;align-items:center;gap:6px">
            <Terminal size={16} style="color:var(--color-accent);" /> Live Telemetry & Capacity Events
          </div>
          <div class="text-xs text-muted">Real-time daemon log stream and resource snapshots</div>
        </div>

        <div style="display:flex;align-items:center;gap:0.5rem;">
          <div style="position:relative;">
            <input 
              type="text" 
              class="form-input" 
              placeholder="Filter telemetry logs..." 
              bind:value={logFilter}
              style="padding:4px 8px 4px 28px;font-size:0.75rem;min-height:30px;width:180px;"
            />
            <Filter size={12} style="position:absolute;left:8px;top:50%;transform:translateY(-50%);color:var(--color-ink-muted);" />
          </div>

          <select class="form-input" bind:value={levelFilter} style="padding:4px 8px;font-size:0.75rem;min-height:30px;width:90px;">
            <option value="all">All</option>
            <option value="info">Info</option>
            <option value="warn">Warn</option>
            <option value="system">System</option>
          </select>
        </div>
      </div>

      <!-- Log Terminal Display -->
      <div style="background:#0f172a;color:#f8fafc;border-radius:var(--radius-md);padding:1rem;font-family:var(--font-mono);font-size:0.8125rem;max-height:320px;overflow-y:auto;border:1px solid #1e293b;">
        {#if filteredLogs.length === 0}
          <div style="color:#64748b;text-align:center;padding:1.5rem 0;">
            No telemetry log entries match the filter criteria.
          </div>
        {:else}
          {#each filteredLogs as l}
            <div style="display:flex;align-items:flex-start;gap:0.75rem;padding:3px 0;border-bottom:1px solid rgba(255,255,255,0.04);">
              <span style="color:#64748b;font-size:0.75rem;flex-shrink:0;">[{l.timestamp}]</span>
              <span 
                style="
                  font-size:0.7rem;
                  padding:1px 5px;
                  border-radius:3px;
                  font-weight:700;
                  flex-shrink:0;
                  background: {l.level === 'warn' ? 'rgba(234,179,8,0.2)' : l.level === 'system' ? 'rgba(14,165,233,0.2)' : 'rgba(34,197,94,0.2)'};
                  color: {l.level === 'warn' ? '#facc15' : l.level === 'system' ? '#38bdf8' : '#4ade80'};
                "
              >
                {l.level.toUpperCase()}
              </span>
              <span style="color:#94a3b8;font-size:0.75rem;flex-shrink:0;">({l.source})</span>
              <span style="color:#e2e8f0;word-break:break-all;">{l.message}</span>
            </div>
          {/each}
        {/if}
      </div>
    </div>
  {/if}

{/if}
