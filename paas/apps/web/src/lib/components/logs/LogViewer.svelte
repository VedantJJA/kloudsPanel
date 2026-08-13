<script lang="ts">
  let { serviceId, deploymentId }: { serviceId: string; deploymentId?: string } = $props();

  let logs = $state<Array<{stream?: string, message?: string, emitted_at?: string, timestamp?: string}>>([]);
  let loading = $state(true);
  let ws: WebSocket | null = null;

  async function fetchLogs() {
    loading = true;
    try {
      const url = deploymentId 
        ? `/api/v1/deployments/${deploymentId}/logs` 
        : `/api/v1/services/${serviceId}/logs`;
      const r = await fetch(url, { credentials: 'include' });
      if (r.ok) {
        const data = await r.json();
        logs = data.entries ?? [];
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    fetchLogs();

    if (deploymentId) {
      const proto = location.protocol === 'https:' ? 'wss' : 'ws';
      try {
        ws = new WebSocket(`${proto}://${location.host}/api/v1/ws/deployments/${deploymentId}/logs`);
        ws.onmessage = (e: MessageEvent) => {
          try {
            const entry = JSON.parse(e.data as string);
            logs = [...logs, entry].slice(-5000);
          } catch {}
        };
      } catch {}
    }

    return () => ws?.close();
  });
</script>

<div class="log-viewer" id="log-viewer" role="log" aria-label="Deployment logs" aria-live="polite">
  {#if loading}
    <span class="log-line-system">Loading logs…</span>
  {:else if logs.length === 0}
    <span class="log-line-system">No log entries recorded yet. Trigger a deployment to view build and runtime output.</span>
  {:else}
    {#each logs as entry}
      {@const timeStr = (entry.emitted_at || entry.timestamp || '').slice(11, 19)}
      <div class="log-line-{entry.stream || 'stdout'}" style="margin-bottom:0.25rem;">
        {#if timeStr}
          <span style="opacity:0.4;margin-right:0.75rem;font-size:0.75rem;font-family:var(--font-mono)">{timeStr}</span>
        {/if}
        <span>{entry.message}</span>
      </div>
    {/each}
  {/if}
</div>

<style>
  .log-viewer {
    min-height: 360px;
    max-height: 520px;
    background: #0d1117;
    border-radius: var(--radius-md);
    padding: var(--sp-4);
    font-family: var(--font-mono);
    font-size: 0.8125rem;
    line-height: 1.7;
    color: #e6edf3;
    overflow-y: auto;
  }
</style>
