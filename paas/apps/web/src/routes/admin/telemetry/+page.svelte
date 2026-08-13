<script lang="ts">
  import { onMount } from 'svelte';

  type Metric = {
    cpu_percent: number;
    memory_total_bytes: number;
    memory_used_bytes: number;
    storage_total_bytes: number;
    storage_used_bytes: number;
    load1: number;
  };

  let metrics = $state<Metric | null>(null);
  let loading = $state(true);
  let refreshInterval: ReturnType<typeof setInterval>;

  const fmt = (bytes: number) => {
    const gb = bytes / 1024 / 1024 / 1024;
    return gb >= 1 ? `${gb.toFixed(1)} GB` : `${(bytes / 1024 / 1024).toFixed(0)} MB`;
  };

  const pct = (used: number, total: number) => total > 0 ? Math.round((used / total) * 100) : 0;

  const barClass = (p: number) => p >= 90 ? 'critical' : p >= 80 ? 'warn' : '';

  async function fetchMetrics() {
    try {
      const res = await fetch('/api/v1/admin/telemetry', { credentials: 'include' });
      if (res.ok) metrics = (await res.json()).host ?? null;
    } catch {}
    loading = false;
  }

  onMount(() => {
    fetchMetrics();
    refreshInterval = setInterval(fetchMetrics, 10000);
    return () => clearInterval(refreshInterval);
  });
</script>

<svelte:head>
  <title>Telemetry — kloudsPanel</title>
</svelte:head>

<div class="page-header">
  <div>
    <h1 class="page-title">Platform Telemetry</h1>
    <p class="page-subtitle">Real-time host and container capacity metrics</p>
  </div>
  <button class="btn btn-secondary" onclick={fetchMetrics}>↻ Refresh</button>
</div>

{#if loading}
  <div class="empty-state"><div style="opacity:0.4;font-size:2rem">⏳</div><p>Loading metrics…</p></div>
{:else if !metrics}
  <div class="empty-state">
    <div class="empty-state-icon">📡</div>
    <h3>No metrics available</h3>
    <p>The agent must be running to collect host metrics.</p>
  </div>
{:else}
  <!-- Capacity strip -->
  <div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(280px,1fr));gap:1rem;margin-bottom:2rem">
    {#each [
      {
        label: 'CPU Utilization',
        icon: '⚡',
        value: `${metrics.cpu_percent.toFixed(1)}%`,
        sub: `Load avg: ${metrics.load1?.toFixed(2) ?? '—'}`,
        pct: metrics.cpu_percent,
      },
      {
        label: 'Memory',
        icon: '🧠',
        value: `${fmt(metrics.memory_used_bytes)} / ${fmt(metrics.memory_total_bytes)}`,
        sub: `${pct(metrics.memory_used_bytes, metrics.memory_total_bytes)}% used`,
        pct: pct(metrics.memory_used_bytes, metrics.memory_total_bytes),
      },
      {
        label: 'Storage',
        icon: '💾',
        value: `${fmt(metrics.storage_used_bytes)} / ${fmt(metrics.storage_total_bytes)}`,
        sub: `${pct(metrics.storage_used_bytes, metrics.storage_total_bytes)}% used`,
        pct: pct(metrics.storage_used_bytes, metrics.storage_total_bytes),
      },
    ] as m}
      <div class="card">
        <div style="display:flex;align-items:center;gap:0.75rem;margin-bottom:0.75rem">
          <span style="font-size:1.5rem">{m.icon}</span>
          <div>
            <div class="text-xs text-muted">{m.label}</div>
            <div style="font-size:1.125rem;font-weight:700">{m.value}</div>
          </div>
        </div>
        <div class="capacity-bar">
          <div
            class="capacity-bar-fill {barClass(m.pct)}"
            style="width:{Math.min(m.pct,100)}%"
            role="progressbar"
            aria-valuenow={m.pct}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-label="{m.label} {m.pct}%"
          ></div>
        </div>
        <div class="text-xs text-muted" style="margin-top:0.5rem">{m.sub}</div>
        {#if m.pct >= 90}
          <div style="color:var(--color-danger);font-size:0.8125rem;font-weight:600;margin-top:0.5rem">
            ⚠ Critical — deployment may fail
          </div>
        {:else if m.pct >= 80}
          <div style="color:var(--color-warning);font-size:0.8125rem;font-weight:600;margin-top:0.5rem">
            ⚠ Warning — approaching capacity
          </div>
        {/if}
      </div>
    {/each}
  </div>

  <!-- Placeholder trend charts -->
  <div class="card" style="margin-bottom:1rem">
    <div class="card-header">
      <h3>Trends</h3>
      <div style="display:flex;gap:0.5rem">
        {#each ['1m', '15m', '60m'] as t}
          <button class="btn btn-secondary" style="padding:4px 10px;min-height:28px;font-size:0.75rem">{t}</button>
        {/each}
      </div>
    </div>
    <div style="height:120px;background:var(--color-canvas);border-radius:var(--radius-md);display:flex;align-items:center;justify-content:center;color:var(--color-ink-secondary);font-size:0.875rem">
      Chart rendering requires time-series data from agent
    </div>
  </div>
{/if}
