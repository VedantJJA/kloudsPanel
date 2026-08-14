<script lang="ts">
  let { serviceId, deploymentId }: { serviceId: string; deploymentId?: string } = $props();

  let logs = $state<Array<{stream?: string, message?: string, emitted_at?: string, timestamp?: string}>>([]);
  let loading = $state(true);
  let pollInterval: any = null;

  async function fetchLogs(isInitial = false) {
    if (isInitial && logs.length === 0) loading = true;
    try {
      let fetched = false;
      if (deploymentId) {
        const depRes = await fetch(`/api/v1/deployments/${deploymentId}/logs`, { credentials: 'include' });
        if (depRes.ok) {
          const data = await depRes.json();
          if (data.entries && data.entries.length > 0) {
            logs = data.entries;
            fetched = true;
          }
        }
      }
      if (!fetched && serviceId) {
        const svcRes = await fetch(`/api/v1/services/${serviceId}/logs`, { credentials: 'include' });
        if (svcRes.ok) {
          const data = await svcRes.json();
          logs = data.entries ?? [];
        }
      }
    } catch (e) {
      console.error(e);
    } finally {
      if (isInitial) loading = false;
    }
  }

  function formatTime(ts?: string): string {
    if (!ts) return '';
    if (ts.includes('T')) {
      return ts.slice(11, 19);
    }
    return ts;
  }

  $effect(() => {
    fetchLogs(true);
    pollInterval = setInterval(() => {
      fetchLogs(false);
    }, 2000);

    return () => {
      if (pollInterval) clearInterval(pollInterval);
    };
  });
</script>

<div class="log-viewer" id="log-viewer" role="log" aria-label="Deployment logs" aria-live="polite">
  {#if loading}
    <div style="display:flex; align-items:center; gap:0.5rem; color:#8b949e;">
      <span class="log-line-system">Loading deployment logs…</span>
    </div>
  {:else if logs.length === 0}
    <span class="log-line-system">No log entries recorded yet. Trigger a deployment to view real console output.</span>
  {:else}
    {#each logs as entry}
      {@const timeStr = formatTime(entry.emitted_at || entry.timestamp)}
      <div class="log-line-{entry.stream || 'stdout'}" style="margin-bottom:0.25rem; word-break:break-word; white-space:pre-wrap;">
        {#if timeStr}
          <span style="opacity:0.45; margin-right:0.75rem; font-size:0.75rem; font-family:var(--font-mono); user-select:none;">{timeStr}</span>
        {/if}
        <span>{entry.message}</span>
      </div>
    {/each}
  {/if}
</div>

<style>
  .log-viewer {
    min-height: 380px;
    max-height: 560px;
    background: #090d13;
    border: 1px solid #21262d;
    border-radius: var(--radius-md);
    padding: var(--sp-4);
    font-family: var(--font-mono);
    font-size: 0.8125rem;
    line-height: 1.65;
    color: #e6edf3;
    overflow-y: auto;
  }

  :global(.log-line-system) {
    color: #79c0ff;
  }

  :global(.log-line-build) {
    color: #d2a8ff;
  }

  :global(.log-line-stdout) {
    color: #e6edf3;
  }

  :global(.log-line-stderr) {
    color: #ff7b72;
  }
</style>
