<script lang="ts">
  import { tick, onDestroy } from 'svelte';

  let { serviceId, deploymentId }: { serviceId: string; deploymentId?: string } = $props();

  let logs = $state<Array<{stream?: string, message?: string, emitted_at?: string, timestamp?: string}>>([]);
  let loading = $state(true);
  let isPolling = $state(true);
  let pollTimeout: any = null;
  let viewerEl = $state<HTMLDivElement | null>(null);
  let autoScroll = $state(true);
  let unchangedCount = 0;
  let lastLogCount = 0;

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

  function isTerminalLog(entries: Array<{message?: string}>): boolean {
    if (!entries || entries.length === 0) return false;
    const lastFew = entries.slice(-6);
    for (const e of lastFew) {
      const msg = (e.message || '').toLowerCase();
      if (
        msg.includes('deployment completed') ||
        msg.includes('deployment succeeded') ||
        msg.includes('deployment failed') ||
        msg.includes('build failed') ||
        msg.includes('container is healthy') ||
        msg.includes('service started successfully') ||
        msg.includes('exit status 1') ||
        msg.includes('returned a non-zero code')
      ) {
        return true;
      }
    }
    return false;
  }

  async function fetchLogs(isInitial = false): Promise<boolean> {
    if (isInitial && logs.length === 0) loading = true;
    let isFinished = false;
    try {
      let fetched = false;
      if (deploymentId) {
        const depRes = await fetch(`/api/v1/deployments/${deploymentId}/logs`, { credentials: 'include' });
        if (depRes.ok) {
          const data = await depRes.json();
          if (data.entries && Array.isArray(data.entries)) {
            logs = data.entries;
            fetched = true;
            if (isTerminalLog(data.entries)) {
              isFinished = true;
            }
          }
        }
      }
      if (!fetched && serviceId) {
        const svcRes = await fetch(`/api/v1/services/${serviceId}/logs`, { credentials: 'include' });
        if (svcRes.ok) {
          const data = await svcRes.json();
          logs = data.entries ?? [];
          if (isTerminalLog(logs)) {
            isFinished = true;
          }
        }
      }

      if (logs.length === lastLogCount) {
        unchangedCount++;
      } else {
        unchangedCount = 0;
        lastLogCount = logs.length;
      }

      scrollToBottom();
    } catch (e) {
      console.error(e);
    } finally {
      if (isInitial) loading = false;
    }
    return isFinished;
  }

  async function scheduleNextPoll() {
    if (pollTimeout) clearTimeout(pollTimeout);
    if (!isPolling) return;

    if (typeof document !== 'undefined' && document.hidden) {
      // Inactive tab: poll every 12 seconds
      pollTimeout = setTimeout(scheduleNextPoll, 12000);
      return;
    }

    const finished = await fetchLogs(false);
    if (finished || unchangedCount > 8) {
      // Deployment has concluded or output has stabilized
      isPolling = false;
      return;
    }

    const delay = unchangedCount > 3 ? 5000 : 2500;
    pollTimeout = setTimeout(scheduleNextPoll, delay);
  }

  function handleVisibilityChange() {
    if (typeof document !== 'undefined' && !document.hidden && isPolling) {
      scheduleNextPoll();
    }
  }

  function formatTime(ts?: string): string {
    if (!ts) return '';
    if (ts.includes('T')) {
      return ts.slice(11, 19);
    }
    return ts;
  }

  function cleanLogMessage(msg?: string): string {
    if (!msg) return '';
    // Strip raw ANSI escape sequences (e.g. [91m, \x1b[0m, etc.)
    return msg
      .replace(/\x1b\[[0-9;]*[a-zA-Z]/g, '')
      .replace(/\[[0-9]{1,2}m/g, '')
      .replace(/\x1b/g, '');
  }

  function getLineType(stream?: string, message?: string): 'error' | 'warning' | 'success' | 'build' | 'system' | 'stdout' {
    const raw = (message || '').trim();
    const clean = cleanLogMessage(raw);
    const lower = clean.toLowerCase();

    // Check for errors
    if (
      lower.includes('[error]') ||
      lower.includes('error:') ||
      lower.includes('build failed') ||
      lower.includes('failed to') ||
      lower.includes('cannot find module') ||
      lower.includes('err_pnpm') ||
      lower.includes('exit status 1') ||
      clean.startsWith('✘ [ERROR]') ||
      clean.startsWith('Error:') ||
      (stream === 'stderr' && !lower.includes('warn'))
    ) {
      if (lower.includes('warning') || lower.includes('[warn]') || lower.includes('deprecated')) {
        return 'warning';
      }
      return 'error';
    }

    // Check for warnings
    if (
      lower.includes('[warn]') ||
      lower.includes('warning:') ||
      lower.includes('warn:') ||
      lower.includes('deprecated') ||
      lower.includes('deprecation')
    ) {
      return 'warning';
    }

    // Check for success markers
    if (
      lower.includes('done in') ||
      lower.includes('successfully') ||
      lower.includes('deployment completed') ||
      lower.includes('container is healthy') ||
      lower.includes('checkout complete')
    ) {
      return 'success';
    }

    if (stream === 'build') return 'build';
    if (stream === 'system') return 'system';
    return 'stdout';
  }

  $effect(() => {
    isPolling = true;
    unchangedCount = 0;
    lastLogCount = 0;
    fetchLogs(true).then((finished) => {
      if (!finished) {
        pollTimeout = setTimeout(scheduleNextPoll, 2500);
      } else {
        isPolling = false;
      }
    });

    if (typeof document !== 'undefined') {
      document.addEventListener('visibilitychange', handleVisibilityChange);
    }

    return () => {
      if (pollTimeout) clearTimeout(pollTimeout);
      if (typeof document !== 'undefined') {
        document.removeEventListener('visibilitychange', handleVisibilityChange);
      }
    };
  });

  onDestroy(() => {
    if (pollTimeout) clearTimeout(pollTimeout);
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
      {@const lineType = getLineType(entry.stream, entry.message)}
      {@const displayMessage = cleanLogMessage(entry.message)}
      <div class="log-entry-row log-type-{lineType}">
        {#if timeStr}
          <span class="log-timestamp">{timeStr}</span>
        {/if}
        {#if lineType === 'error'}
          <span class="log-badge log-badge-error">ERR</span>
        {:else if lineType === 'warning'}
          <span class="log-badge log-badge-warn">WARN</span>
        {/if}
        <span class="log-text">{displayMessage}</span>
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

  .log-entry-row {
    display: flex;
    align-items: flex-start;
    gap: 0.5rem;
    padding: 2px 6px;
    margin: 1px 0;
    border-radius: 4px;
    word-break: break-word;
    white-space: pre-wrap;
    transition: background 0.15s ease;
  }

  .log-entry-row:hover {
    background: rgba(255, 255, 255, 0.03);
  }

  .log-timestamp {
    opacity: 0.4;
    font-size: 0.72rem;
    font-family: var(--font-mono);
    user-select: none;
    flex-shrink: 0;
    margin-top: 1px;
  }

  .log-badge {
    font-size: 0.65rem;
    font-weight: 700;
    padding: 0 4px;
    border-radius: 3px;
    line-height: 1.4;
    user-select: none;
    flex-shrink: 0;
    margin-top: 2px;
    letter-spacing: 0.04em;
  }

  .log-badge-error {
    background: rgba(248, 81, 73, 0.25);
    color: #ff7b72;
    border: 1px solid rgba(248, 81, 73, 0.5);
  }

  .log-badge-warn {
    background: rgba(210, 153, 34, 0.2);
    color: #e3b341;
    border: 1px solid rgba(210, 153, 34, 0.4);
  }

  .log-text {
    flex: 1;
  }

  .log-type-error {
    color: #ff7b72;
    background: rgba(248, 81, 73, 0.08);
    border-left: 2px solid #f85149;
  }

  .log-type-warning {
    color: #e3b341;
    background: rgba(210, 153, 34, 0.06);
    border-left: 2px solid #d29922;
  }

  .log-type-success {
    color: #7ee787;
  }

  .log-type-build {
    color: #d2a8ff;
  }

  .log-type-system {
    color: #79c0ff;
  }

  .log-type-stdout {
    color: #e6edf3;
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
</style>
