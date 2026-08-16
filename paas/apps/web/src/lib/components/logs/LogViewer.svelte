<script lang="ts">
  import { tick } from 'svelte';

  let { serviceId, deploymentId }: { serviceId: string; deploymentId?: string } = $props();

  let logs = $state<Array<{stream?: string, message?: string, emitted_at?: string, timestamp?: string}>>([]);
  let loading = $state(true);
  let pollInterval: any = null;
  let viewerEl = $state<HTMLDivElement | null>(null);
  let autoScroll = $state(true);

  function handleScroll() {
    if (!viewerEl) return;
    const { scrollTop, scrollHeight, clientHeight } = viewerEl;
    autoScroll = scrollHeight - (scrollTop + clientHeight) <= 40;
  }

  async function scrollToBottom() {
    if (!autoScroll || !viewerEl) return;
    await tick();
    if (viewerEl) {
      viewerEl.scrollTop = viewerEl.scrollHeight;
    }
  }

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
      scrollToBottom();
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

<div 
  bind:this={viewerEl}
  onscroll={handleScroll}
  class="log-viewer"
  id="log-viewer"
  role="log"
  aria-label="Deployment logs"
  aria-live="polite"
>
  {#if loading}
    <div style="display:flex; align-items:center; gap:0.5rem; color:#8b949e;">
      <span class="log-line-system">Loading deployment logs...</span>
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
    {#if !autoScroll}
      <button 
        type="button"
        class="scroll-bottom-btn"
        onclick={() => {
          autoScroll = true;
          if (viewerEl) viewerEl.scrollTop = viewerEl.scrollHeight;
        }}
      >
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <polyline points="6 9 12 15 18 9"></polyline>
        </svg>
        Scroll to bottom
      </button>
    {/if}
  {/if}
</div>

<style>
  .log-viewer {
    position: relative;
    min-height: 380px;
    max-height: 560px;
    background: #06070a;
    border: 1px solid var(--color-border);
    border-radius: var(--radius-md);
    padding: var(--sp-4);
    font-family: var(--font-mono);
    font-size: 0.8125rem;
    line-height: 1.65;
    color: #e6edf3;
    overflow-y: auto;
    scroll-behavior: smooth;
  }

  .scroll-bottom-btn {
    position: sticky;
    bottom: 0.5rem;
    left: 50%;
    transform: translateX(-50%);
    display: inline-flex;
    align-items: center;
    gap: 0.35rem;
    padding: 0.35rem 0.75rem;
    background: #1f293d;
    color: #e6edf3;
    border: 1px solid #30363d;
    border-radius: var(--radius-sm, 6px);
    font-size: 0.75rem;
    cursor: pointer;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.4);
    z-index: 10;
  }

  .scroll-bottom-btn:hover {
    background: #28354d;
    color: #ffffff;
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
