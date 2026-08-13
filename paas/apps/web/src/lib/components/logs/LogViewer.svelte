<script lang="ts">
  let { serviceId, deploymentId }: { serviceId: string; deploymentId?: string } = $props();

  let logs = $state<Array<{stream: string, message: string, emitted_at: string}>>([]);
  let loading = $state(true);
  let ws: WebSocket | null = null;

  $effect(() => {
    if (!deploymentId) { loading = false; return; }

    // Initial fetch
    fetch(`/api/v1/deployments/${deploymentId}/logs`, { credentials: 'include' })
      .then(r => r.json())
      .then((data: { entries?: typeof logs }) => { logs = data.entries ?? []; loading = false; })
      .catch(() => { loading = false; });

    // WebSocket for live stream
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    ws = new WebSocket(`${proto}://${location.host}/api/v1/ws/deployments/${deploymentId}/logs`);
    ws.onmessage = (e: MessageEvent) => {
      try {
        const entry = JSON.parse(e.data as string);
        logs = [...logs, entry as (typeof logs)[0]].slice(-5000);
      } catch {}
    };

    return () => ws?.close();
  });
</script>

<div class="log-viewer" id="log-viewer" role="log" aria-label="Deployment logs" aria-live="polite">
  {#if !deploymentId}
    <span class="log-line-system">No deployment selected.</span>
  {:else if loading}
    <span class="log-line-system">Loading logs…</span>
  {:else if logs.length === 0}
    <span class="log-line-system">No log entries yet.</span>
  {:else}
    {#each logs as entry}
      <div class="log-line-{entry.stream}">
        <span style="opacity:0.4;margin-right:0.75rem;font-size:0.75rem">{entry.emitted_at.slice(11,23)}</span>{entry.message}
      </div>
    {/each}
  {/if}
</div>

<style>
  .log-viewer {
    min-height: 300px;
  }
</style>
