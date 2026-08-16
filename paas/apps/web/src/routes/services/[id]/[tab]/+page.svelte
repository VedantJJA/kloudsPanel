<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount, onDestroy } from 'svelte';
  import LogViewer from '$lib/components/logs/LogViewer.svelte';
  import {
    Loader2,
    ExternalLink,
    Square,
    Play,
    Rocket,
    Trash2,
    Plus,
    X,
    Save,
    RefreshCw,
    Copy,
    Check,
    Globe,
    Server,
    Clock,
    Layers,
    ShieldCheck,
    ArrowUp,
    ArrowDown,
    Sliders,
    Sparkles,
    FileText,
    Code,
    Download,
    ArrowRight
  } from 'lucide-svelte';
  import FrameworkIcon from '$lib/components/icons/FrameworkIcon.svelte';

  const id = $derived($page.params.id);
  const tab = $derived($page.params.tab);

  let service = $state<any>(null);
  let deployments = $state<any[]>([]);
  let loading = $state(true);
  let actionLoading = $state(false);
  let copiedUrl = $state(false);
  let bannerNotice = $state<{ type: 'success' | 'error'; message: string } | null>(null);
  let pollTimer: any = null;

  // Derive visible tabs based on service kind:
  // - 'routes' (Redirect & Rewrite Rules) only applies to static sites (like Render)
  // - 'domains' and 'scale' don't apply to workers/crons (no public endpoint)
  const serviceKind = $derived(service?.kind || service?.Kind || 'web');
  const tabs = $derived.by(() => {
    const base = ['overview', 'deployments', 'logs', 'variables'];
    if (serviceKind === 'web' || serviceKind === 'static') {
      base.push('domains');
    }
    if (serviceKind === 'static') {
      base.push('routes');
    }
    if (serviceKind === 'web' || serviceKind === 'static') {
      base.push('scale');
    }
    base.push('settings');
    return base;
  });

  // Variables state
  let envVars = $state<Array<{ key: string; value: string }>>([]);
  let envMode = $state<'form' | 'raw'>('form');
  let rawEnvText = $state('');
  let envDirty = $state(false);
  let envInitialLoaded = $state(false);
  let envSaving = $state(false);
  let envSuccess = $state(false);
  let blueprintImportLoading = $state(false);
  let blueprintNotice = $state<{ type: 'success' | 'error'; message: string } | null>(null);

  // Custom Domains state
  let customDomainsList = $state<any[]>([]);
  let newDomainInput = $state('');
  let domainSaving = $state(false);
  let domainNotice = $state<{ type: 'success' | 'error'; message: string } | null>(null);

  // Redirect and Rewrite Rules state
  let serviceRoutes = $state<Array<{ type: string; source: string; destination: string }>>([]);
  let routesDirty = $state(false);
  let routesInitialLoaded = $state(false);
  let routesSaving = $state(false);
  let routesNotice = $state<{ type: 'success' | 'error'; message: string } | null>(null);

  async function loadRoutes(force = false) {
    if (!force && (routesDirty || routesInitialLoaded)) return;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/routes`, { credentials: 'include' });
      if (res.ok) {
        const d = await res.json();
        serviceRoutes = (d.routes ?? []).map((r: any) => ({
          type: (r.type && r.type.startsWith('redirect')) ? 'redirect' : 'rewrite',
          source: r.source || '',
          destination: r.destination || ''
        }));
        routesInitialLoaded = true;
        routesDirty = false;
      }
    } catch {}
  }

  function addRule() {
    routesDirty = true;
    serviceRoutes = [
      ...serviceRoutes,
      { type: 'rewrite', source: '', destination: '' }
    ];
  }

  function removeRule(idx: number) {
    routesDirty = true;
    serviceRoutes = serviceRoutes.filter((_, i) => i !== idx);
  }

  function moveRule(idx: number, dir: number) {
    const targetIdx = idx + dir;
    if (targetIdx < 0 || targetIdx >= serviceRoutes.length) return;
    routesDirty = true;
    const item = serviceRoutes[idx];
    const newArr = [...serviceRoutes];
    newArr.splice(idx, 1);
    newArr.splice(targetIdx, 0, item);
    serviceRoutes = newArr;
  }

  async function saveRules() {
    routesSaving = true;
    routesNotice = null;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/routes`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ routes: serviceRoutes })
      });
      if (res.ok) {
        const d = await res.json();
        serviceRoutes = d.routes ?? serviceRoutes;
        routesDirty = false;
        routesInitialLoaded = true;
        routesNotice = { type: 'success', message: 'Redirect and Rewrite rules saved successfully and applied live.' };
      } else {
        const d = await res.json().catch(() => ({}));
        routesNotice = { type: 'error', message: d.error || 'Failed to save rules' };
      }
    } catch (e: any) {
      routesNotice = { type: 'error', message: e.message || 'Network error' };
    } finally {
      routesSaving = false;
    }
  }

  let testSimPath = $state('');
  let liveTestLoading = $state(false);
  let liveTestResult = $state<{ status: number; statusText: string; ok: boolean; timeMs: number; finalUrl: string } | null>(null);

  function joinUrlPath(base: string, sub: string): string {
    if (!sub) return base;
    const cleanBase = base.replace(/\/+$/, '');
    const cleanSub = sub.replace(/^\/+/, '');
    if (!cleanSub) return cleanBase;
    return `${cleanBase}/${cleanSub}`;
  }

  function simulateRuleMatch(rawInput: string, rules: Array<{ type: string; source: string; destination: string }>) {
    let input = rawInput.trim();
    if (!input) {
      return { matched: false, ruleIndex: 0, action: '', actionLabel: '', destination: '', explanation: '' };
    }

    let pathname = input;
    let query = '';

    // Handle full URL inputs (e.g. https://my-app.onrender.com/api/v1/auth?token=123)
    if (input.startsWith('http://') || input.startsWith('https://')) {
      try {
        const u = new URL(input);
        pathname = u.pathname || '/';
        query = u.search || '';
      } catch {
        const qIdx = input.indexOf('?');
        if (qIdx !== -1) {
          pathname = input.slice(0, qIdx);
          query = input.slice(qIdx);
        }
      }
    } else {
      const qIdx = input.indexOf('?');
      if (qIdx !== -1) {
        pathname = input.slice(0, qIdx);
        query = input.slice(qIdx);
      }
      if (!pathname.startsWith('/')) {
        pathname = '/' + pathname;
      }
    }

    const getLabel = (t: string) => {
      if (t === 'redirect' || t.startsWith('redirect')) return 'Redirect';
      return 'Rewrite';
    };

    for (let i = 0; i < rules.length; i++) {
      const rule = rules[i];
      const src = (rule.source || '').trim();
      const dest = (rule.destination || '').trim();
      if (!src || !dest) continue;

      // 1. Root wildcard: /* or * or /
      if (src === '/*' || src === '*' || src === '/') {
        let finalDest = dest;
        if (dest.endsWith('/*')) {
          const rest = pathname.startsWith('/') ? pathname.slice(1) : pathname;
          finalDest = joinUrlPath(dest.slice(0, -2), rest);
        } else if (dest.includes('$1')) {
          finalDest = dest.replace('$1', pathname.replace(/^\/+/, ''));
        }
        return {
          matched: true,
          ruleIndex: i + 1,
          action: rule.type,
          actionLabel: getLabel(rule.type),
          destination: finalDest + query,
          explanation: `Matched rule #${i + 1} (${src} -> ${dest})`
        };
      }

      // 2. Prefix wildcard: e.g. /api/* or /api*
      if (src.endsWith('/*')) {
        const prefix = src.slice(0, -2);
        if (pathname === prefix || pathname.startsWith(prefix + '/') || pathname.startsWith(prefix)) {
          const rest = pathname.slice(prefix.length);
          let finalDest = dest;
          if (dest.endsWith('/*')) {
            const baseDest = dest.slice(0, -2);
            finalDest = joinUrlPath(baseDest, rest);
          } else if (dest.includes('$1')) {
            finalDest = dest.replace('$1', rest.replace(/^\/+/, ''));
          } else {
            finalDest = joinUrlPath(dest, rest);
          }
          return {
            matched: true,
            ruleIndex: i + 1,
            action: rule.type,
            actionLabel: getLabel(rule.type),
            destination: finalDest + query,
            explanation: `Matched prefix wildcard #${i + 1} (${src} -> ${dest})`
          };
        }
      }

      // 3. Parameterized pattern matching (e.g. /users/:id or /posts/:cat/:id)
      if (src.includes(':')) {
        const srcSegments = src.split('/').filter(Boolean);
        const pathSegments = pathname.split('/').filter(Boolean);
        if (srcSegments.length === pathSegments.length) {
          let paramMatch = true;
          const params: Record<string, string> = {};
          const capturedVals: string[] = [];

          for (let s = 0; s < srcSegments.length; s++) {
            if (srcSegments[s].startsWith(':')) {
              const paramName = srcSegments[s].slice(1);
              params[paramName] = pathSegments[s];
              capturedVals.push(pathSegments[s]);
            } else if (srcSegments[s] !== pathSegments[s]) {
              paramMatch = false;
              break;
            }
          }

          if (paramMatch) {
            let finalDest = dest;
            for (let c = 0; c < capturedVals.length; c++) {
              finalDest = finalDest.replace(new RegExp(`\\$${c + 1}`, 'g'), capturedVals[c]);
            }
            for (const [pName, pVal] of Object.entries(params)) {
              finalDest = finalDest.replace(new RegExp(`:${pName}`, 'g'), pVal);
            }
            return {
              matched: true,
              ruleIndex: i + 1,
              action: rule.type,
              actionLabel: getLabel(rule.type),
              destination: finalDest + query,
              explanation: `Matched parameterized pattern #${i + 1} (${src} -> ${dest})`
            };
          }
        }
      }

      // 4. Exact path match: e.g. /old-page -> /new-page
      if (pathname === src || pathname === src + '/') {
        return {
          matched: true,
          ruleIndex: i + 1,
          action: rule.type,
          actionLabel: getLabel(rule.type),
          destination: dest + query,
          explanation: `Matched exact rule #${i + 1} (${src} -> ${dest})`
        };
      }
    }

    return {
      matched: false,
      ruleIndex: 0,
      action: '',
      actionLabel: '',
      destination: '',
      explanation: 'No matching redirect/rewrite rule. Request is handled by standard static asset routing / SPA fallback.'
    };
  }

  async function testLiveRequest(targetPath: string) {
    if (!targetPath.trim()) return;
    liveTestLoading = true;
    liveTestResult = null;
    const startTime = performance.now();
    try {
      const match = simulateRuleMatch(targetPath.trim(), serviceRoutes);
      let fetchUrl = targetPath.trim();
      if (match.matched && match.destination.startsWith('http')) {
        fetchUrl = match.destination;
      } else if (endpointUrl) {
        const p = targetPath.trim().startsWith('/') ? targetPath.trim() : '/' + targetPath.trim();
        fetchUrl = `${endpointUrl}${p}`;
      }
      const res = await fetch(fetchUrl, { method: 'GET', mode: 'no-cors' });
      const elapsed = Math.round(performance.now() - startTime);
      liveTestResult = {
        status: res.status || 200,
        statusText: res.statusText || 'OK',
        ok: res.ok !== false,
        timeMs: elapsed,
        finalUrl: fetchUrl
      };
    } catch (e: any) {
      const elapsed = Math.round(performance.now() - startTime);
      liveTestResult = {
        status: 0,
        statusText: e.message || 'Request Failed',
        ok: false,
        timeMs: elapsed,
        finalUrl: targetPath.trim()
      };
    } finally {
      liveTestLoading = false;
    }
  }

  async function loadDomains() {
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/domains`, { credentials: 'include' });
      if (res.ok) {
        const d = await res.json();
        customDomainsList = d.domains ?? [];
      }
    } catch {}
  }

  async function addCustomDomain() {
    if (!newDomainInput.trim()) return;
    domainSaving = true;
    domainNotice = null;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/domains`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ domain: newDomainInput.trim() })
      });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        domainNotice = { type: 'error', message: d.error || 'Failed to add domain' };
      } else {
        const d = await res.json();
        customDomainsList = d.domains ?? [];
        newDomainInput = '';
        domainNotice = { type: 'success', message: 'Custom domain saved and TLS certificate configured with Let\'s Encrypt!' };
      }
    } catch (e: any) {
      domainNotice = { type: 'error', message: e.message || 'Network error' };
    } finally {
      domainSaving = false;
    }
  }

  async function removeCustomDomain(domainName: string) {
    if (!confirm(`Are you sure you want to remove ${domainName}?`)) return;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/domains/${encodeURIComponent(domainName)}`, {
        method: 'DELETE',
        credentials: 'include'
      });
      if (res.ok) {
        const d = await res.json();
        customDomainsList = d.domains ?? [];
        domainNotice = { type: 'success', message: `Domain ${domainName} removed.` };
      }
    } catch {}
  }

  async function loadService() {
    try {
      const res = await fetch(`/api/v1/services/${id}`, { credentials: 'include' });
      if (!res.ok) { goto('/workspaces'); return; }
      service = await res.json();

      // Redirect to overview if user is on a tab that doesn't apply to this service kind
      const kind = service.kind || service.Kind || 'web';
      const inapplicable =
        (tab === 'routes' && kind !== 'static') ||
        (tab === 'domains' && kind !== 'web' && kind !== 'static') ||
        (tab === 'scale' && kind !== 'web' && kind !== 'static');
      if (inapplicable) {
        goto(`/services/${id}/overview`, { replaceState: true });
        return;
      }

      loadDomains();
      loadRoutes();
      
      // Parse existing env vars and service settings if present in resource_json
      try {
        if (service.resource_json || service.ResourceJSON) {
          const r = JSON.parse(service.resource_json || service.ResourceJSON);
          if (!envDirty && (!envInitialLoaded || tab !== 'variables')) {
            if (r.env && typeof r.env === 'object') {
              envVars = Object.entries(r.env).map(([k, v]) => ({ key: k, value: String(v) }));
              envInitialLoaded = true;
            }
          }
          if (!settingsDirty) {
            settingsName = service.name || service.Name || '';
            settingsPreset = r.presetId || service.kind || 'node';
            settingsRuntimeVersion = r.runtimeVersion || 'auto';
            settingsMemoryLimit = r.mem_limit || '512m';
            settingsCPULimit = r.cpu_limit || '1.0';
            settingsBuildCmd = r.buildCommand || '';
            settingsStartCmd = r.startCommand || '';
            settingsRootDir = r.rootDirectory || r.rootDir || '.';
            settingsBranch = r.gitBranch || 'main';
            settingsRepoUrl = r.gitRepoUrl || '';
            settingsPort = service.internal_port || service.InternalPort || 80;
            settingsAutoDeploy = service.auto_deploy !== false;
          }
        }
      } catch {}

      const targetId = service?.id || id;
      const depRes = await fetch(`/api/v1/services/${targetId}/deployments`, { credentials: 'include' });
      if (depRes.ok) {
        deployments = (await depRes.json()).deployments ?? [];
      }
    } catch (e) {
      console.error(e);
    } finally {
      loading = false;
    }
  }

  onMount(() => {
    loadService();
    pollTimer = setInterval(() => {
      loadService();
    }, 2500);
  });

  onDestroy(() => {
    if (pollTimer) clearInterval(pollTimer);
  });

  async function triggerDeploy() {
    actionLoading = true;
    bannerNotice = null;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/deploy`, { method: 'POST', credentials: 'include' });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        bannerNotice = { type: 'error', message: d.error || 'Failed to trigger deployment' };
      } else {
        bannerNotice = { type: 'success', message: 'Deployment initiated! Compiling and launching container in background.' };
        if (tab !== 'logs') {
          goto(`/services/${id}/logs`);
        }
      }
      await loadService();
    } catch (e: any) {
      bannerNotice = { type: 'error', message: 'Error: ' + e.message };
    } finally {
      actionLoading = false;
    }
  }

  async function stopService() {
    actionLoading = true;
    bannerNotice = null;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/stop`, { method: 'POST', credentials: 'include' });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        bannerNotice = { type: 'error', message: d.error || 'Failed to stop service' };
      } else {
        bannerNotice = { type: 'success', message: 'Service container stopped' };
      }
      await loadService();
    } catch (e: any) {
      bannerNotice = { type: 'error', message: 'Error: ' + e.message };
    } finally {
      actionLoading = false;
    }
  }

  async function startService() {
    actionLoading = true;
    bannerNotice = null;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/start`, { method: 'POST', credentials: 'include' });
      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        bannerNotice = { type: 'error', message: d.error || 'Failed to start service' };
      } else {
        bannerNotice = { type: 'success', message: 'Service container started' };
      }
      await loadService();
    } catch (e: any) {
      bannerNotice = { type: 'error', message: 'Error: ' + e.message };
    } finally {
      actionLoading = false;
    }
  }

  async function deleteService() {
    if (!confirm(`Are you sure you want to permanently delete "${service?.name || service?.Name}"? This action cannot be undone.`)) return;
    actionLoading = true;
    bannerNotice = null;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}`, { method: 'DELETE', credentials: 'include' });
      if (res.ok) {
        if (service?.project_id || service?.ProjectID) {
          goto(`/projects/${service.project_id || service.ProjectID}`);
        } else {
          goto('/workspaces');
        }
      } else {
        const d = await res.json().catch(() => ({}));
        bannerNotice = { type: 'error', message: d.error || 'Failed to delete service' };
        actionLoading = false;
      }
    } catch (e: any) {
      bannerNotice = { type: 'error', message: 'Error: ' + e.message };
      actionLoading = false;
    }
  }

  function copyEndpoint() {
    const url = service?.endpoint_url || `https://${service?.domain}`;
    if (url) {
      navigator.clipboard.writeText(url);
      copiedUrl = true;
      setTimeout(() => copiedUrl = false, 2500);
    }
  }

  function syncEnvToRaw() {
    rawEnvText = envVars.map(e => `${e.key}=${e.value}`).join('\n');
  }

  function syncRawToEnv() {
    const lines = rawEnvText.split('\n');
    const parsed: Array<{ key: string; value: string }> = [];
    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed || trimmed.startsWith('#')) continue;
      const eqIdx = trimmed.indexOf('=');
      if (eqIdx > 0) {
        const key = trimmed.slice(0, eqIdx).trim();
        let val = trimmed.slice(eqIdx + 1).trim();
        if ((val.startsWith('"') && val.endsWith('"')) || (val.startsWith("'") && val.endsWith("'"))) {
          val = val.slice(1, -1);
        }
        parsed.push({ key, value: val });
      }
    }
    envVars = parsed;
    envDirty = true;
  }

  function switchEnvMode(mode: 'form' | 'raw') {
    if (mode === 'raw') {
      syncEnvToRaw();
    } else {
      syncRawToEnv();
    }
    envMode = mode;
  }

  function addEnv() {
    envDirty = true;
    envVars = [...envVars, { key: '', value: '' }];
    syncEnvToRaw();
  }

  function removeEnv(index: number) {
    envDirty = true;
    envVars = envVars.filter((_, i) => i !== index);
    syncEnvToRaw();
  }

  async function importBlueprintEnv() {
    blueprintImportLoading = true;
    blueprintNotice = null;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/blueprint`, { credentials: 'include' });
      if (res.ok) {
        const d = await res.json();
        if (d.envVars && Object.keys(d.envVars).length > 0) {
          const newEntries = Object.entries(d.envVars).map(([k, v]) => ({ key: k, value: String(v) }));
          for (const item of newEntries) {
            const existingIdx = envVars.findIndex(e => e.key === item.key);
            if (existingIdx >= 0) {
              envVars[existingIdx].value = item.value;
            } else {
              envVars.push(item);
            }
          }
          envDirty = true;
          syncEnvToRaw();
          blueprintNotice = {
            type: 'success',
            message: `Imported ${Object.keys(d.envVars).length} environment variables from ${d.blueprintSource || 'blueprint'}. Click "Save & Redeploy" to apply.`
          };
        } else {
          blueprintNotice = {
            type: 'error',
            message: `No environment variables found in ${d.blueprintSource || 'blueprint'}.`
          };
        }
      } else {
        const d = await res.json().catch(() => ({}));
        blueprintNotice = { type: 'error', message: d.error || 'Failed to fetch blueprint from repository' };
      }
    } catch (e: any) {
      blueprintNotice = { type: 'error', message: 'Error: ' + e.message };
    } finally {
      blueprintImportLoading = false;
    }
  }

  async function importBlueprintSettings() {
    blueprintImportLoading = true;
    bannerNotice = null;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/blueprint`, { credentials: 'include' });
      if (res.ok) {
        const d = await res.json();
        if (d.service) {
          if (d.service.build_command) settingsBuildCmd = d.service.build_command;
          if (d.service.start_command) settingsStartCmd = d.service.start_command;
          if (d.service.root_dir) settingsRootDir = d.service.root_dir;
          if (d.service.internal_port) settingsPort = d.service.internal_port;
          if (d.service.preset) settingsPreset = d.service.preset;
          settingsDirty = true;
          bannerNotice = {
            type: 'success',
            message: `Synced build, start, and root directory settings from ${d.blueprintSource || 'blueprint'}. Click "Save Settings" to apply.`
          };
        }
      } else {
        const d = await res.json().catch(() => ({}));
        bannerNotice = { type: 'error', message: d.error || 'Failed to fetch blueprint from repository' };
      }
    } catch (e: any) {
      bannerNotice = { type: 'error', message: 'Error: ' + e.message };
    } finally {
      blueprintImportLoading = false;
    }
  }

  async function importBlueprintRoutes() {
    blueprintImportLoading = true;
    routesNotice = null;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/blueprint`, { credentials: 'include' });
      if (res.ok) {
        const d = await res.json();
        if (d.routes && d.routes.length > 0) {
          serviceRoutes = d.routes.map((r: any) => ({
            type: (r.type && r.type.startsWith('redirect')) ? 'redirect' : 'rewrite',
            source: r.source || '',
            destination: r.destination || ''
          }));
          routesDirty = true;
          routesNotice = {
            type: 'success',
            message: `Imported ${d.routes.length} redirect/rewrite rules from ${d.blueprintSource || 'blueprint'}. Click "Save Changes" to apply.`
          };
        } else {
          routesNotice = { type: 'error', message: `No routes found in ${d.blueprintSource || 'blueprint'}.` };
        }
      } else {
        const d = await res.json().catch(() => ({}));
        routesNotice = { type: 'error', message: d.error || 'Failed to fetch blueprint routes' };
      }
    } catch (e: any) {
      routesNotice = { type: 'error', message: 'Error: ' + e.message };
    } finally {
      blueprintImportLoading = false;
    }
  }

  async function saveEnvVars(redeploy: boolean = true) {
    if (envMode === 'raw') {
      syncRawToEnv();
    }
    envSaving = true;
    envSuccess = false;
    bannerNotice = null;
    try {
      const envMap: Record<string, string> = {};
      for (const item of envVars) {
        if (item.key.trim()) {
          envMap[item.key.trim()] = item.value;
        }
      }
      let currentR: any = {};
      try {
        currentR = JSON.parse(service.resource_json || service.ResourceJSON || '{}');
      } catch {}
      currentR.env = envMap;

      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({ resourceJson: JSON.stringify(currentR) })
      });
      if (res.ok) {
        envDirty = false;
        envInitialLoaded = true;
        envSuccess = true;
        if (redeploy) {
          bannerNotice = { type: 'success', message: 'Environment variables saved! Initiating redeployment with latest variables...' };
          await triggerDeploy();
        } else {
          bannerNotice = { type: 'success', message: 'Environment variables saved successfully' };
          setTimeout(() => envSuccess = false, 3000);
        }
      } else {
        const d = await res.json().catch(() => ({}));
        bannerNotice = { type: 'error', message: d.error || 'Failed to save environment variables' };
      }
    } catch (e: any) {
      bannerNotice = { type: 'error', message: 'Error: ' + e.message };
    } finally {
      envSaving = false;
    }
  }

  // Settings Tab State
  let settingsName = $state('');
  let settingsBuildCmd = $state('');
  let settingsStartCmd = $state('');
  let settingsRootDir = $state('.');
  let settingsBranch = $state('main');
  let settingsRepoUrl = $state('');
  let settingsPort = $state<number>(80);
  let settingsAutoDeploy = $state(true);
  let settingsPreset = $state('node');
  let settingsRuntimeVersion = $state('auto');
  let settingsMemoryLimit = $state('512m');
  let settingsCPULimit = $state('1.0');
  let settingsSaving = $state(false);
  let settingsSaved = $state(false);
  let settingsError = $state('');
  let settingsDirty = $state(false);

  async function saveServiceSettings(e: Event) {
    e.preventDefault();
    settingsSaving = true;
    settingsSaved = false;
    settingsError = '';
    bannerNotice = null;
    try {
      let currentR: any = {};
      try {
        currentR = JSON.parse(service.resource_json || service.ResourceJSON || '{}');
      } catch {}
      currentR.buildCommand = settingsBuildCmd;
      currentR.startCommand = settingsStartCmd;
      currentR.rootDirectory = settingsRootDir;
      currentR.gitBranch = settingsBranch;
      currentR.gitRepoUrl = settingsRepoUrl;
      currentR.presetId = settingsPreset;
      currentR.runtimeVersion = settingsRuntimeVersion === 'auto' ? '' : settingsRuntimeVersion;
      currentR.mem_limit = settingsMemoryLimit;
      currentR.cpu_limit = settingsCPULimit;

      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          name: settingsName,
          internalPort: Number(settingsPort),
          autoDeploy: settingsAutoDeploy,
          resourceJson: JSON.stringify(currentR)
        })
      });

      if (res.ok) {
        settingsSaved = true;
        settingsDirty = false;
        bannerNotice = { type: 'success', message: 'Service settings saved successfully' };
        await loadService();
        setTimeout(() => settingsSaved = false, 3000);
      } else {
        const d = await res.json().catch(() => ({}));
        settingsError = d.error || 'Failed to update service settings';
      }
    } catch (e: any) {
      settingsError = 'Error: ' + e.message;
    } finally {
      settingsSaving = false;
    }
  }

  async function restartService() {
    actionLoading = true;
    bannerNotice = null;
    try {
      const targetId = service?.id || id;
      const res = await fetch(`/api/v1/services/${targetId}/restart`, { method: 'POST', credentials: 'include' });
      if (res.ok) {
        bannerNotice = { type: 'success', message: 'Service container restarted successfully' };
      } else {
        const d = await res.json().catch(() => ({}));
        bannerNotice = { type: 'error', message: d.error || 'Failed to restart service' };
      }
      await loadService();
    } catch (e: any) {
      bannerNotice = { type: 'error', message: 'Error: ' + e.message };
    } finally {
      actionLoading = false;
    }
  }

  const isRunning = $derived((service?.runtime_status || service?.RuntimeStatus) === 'running');
  const statusBadge = $derived(service?.runtime_status || service?.RuntimeStatus || 'draft');
  const endpointUrl = $derived(service?.endpoint_url || (service?.domain ? `https://${service.domain}` : null));
  const parsedResource = $derived.by(() => {
    try {
      if (service?.resource_json || service?.ResourceJSON) {
        return JSON.parse(service.resource_json || service.ResourceJSON);
      }
    } catch {}
    return {};
  });
</script>

<svelte:head>
  <title>{service?.name || service?.Name || 'Service'} - kloudsPanel</title>
</svelte:head>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={48} /></div>
    <p>Loading service...</p>
  </div>
{:else}
  {#if bannerNotice}
    <div style="margin-bottom: 1.25rem; padding: 0.75rem 1rem; border-radius: var(--radius-md); font-size: 0.875rem; display: flex; align-items: center; justify-content: space-between; gap: 0.75rem; background: {bannerNotice.type === 'error' ? 'var(--color-danger-subtle)' : 'var(--color-success-subtle)'}; border: 1px solid {bannerNotice.type === 'error' ? 'rgba(248,113,113,0.3)' : 'rgba(52,211,153,0.3)'}; color: {bannerNotice.type === 'error' ? 'var(--color-danger)' : 'var(--color-success)'};">
      <span>{bannerNotice.message}</span>
      <button type="button" class="btn btn-secondary" style="padding: 2px 8px; height: auto; min-height: 0; font-size: 0.75rem;" onclick={() => bannerNotice = null}>
        <X size={13} />
      </button>
    </div>
  {/if}

  <!-- Service header -->
  <div class="page-header">
    <div style="flex:1; min-width:0;">
      <p class="text-xs text-muted" style="margin-bottom:0.25rem;">
        <a href="/workspaces">Workspaces</a> /
        {#if service?.project_id || service?.ProjectID}
          <a href="/projects/{service.project_id || service.ProjectID}">{service.project_name || 'Project'}</a> /
        {/if}
      </p>
      <div style="display:flex; align-items:center; gap:0.75rem; flex-wrap:wrap;">
        <FrameworkIcon name={parsedResource.presetId || service?.kind || 'node'} size={24} />
        <h1 class="page-title" style="margin:0;">{service?.name || service?.Name}</h1>
        <span class="badge badge-{statusBadge}">{statusBadge}</span>
        {#if endpointUrl && (service?.kind === 'web' || service?.kind === 'static')}
          <a 
            href={endpointUrl} 
            target="_blank" 
            rel="noopener noreferrer" 
            class="badge" 
            style="background: var(--color-surface-subtle); border: 1px solid var(--color-border); color: #ffffff; font-weight: 600; text-decoration: none; display: inline-flex; align-items: center; gap: 4px; padding: 3px 8px;"
          >
            <Globe size={12} /> {service.domain || endpointUrl.replace('https://', '')} <ExternalLink size={11} />
          </a>
        {/if}
      </div>
      <div class="text-xs text-muted" style="margin-top:0.25rem;">
        Internal port: <span class="font-mono">:{service?.internal_port || service?.InternalPort || 80}</span> | Kind: {service?.kind || service?.Kind || 'web'}
      </div>
    </div>
    <div style="display:flex; gap:0.5rem; align-items:center;">
      {#if isRunning}
        <button class="btn btn-secondary" onclick={stopService} disabled={actionLoading}>
          <Square size={14} fill="currentColor" /> Stop
        </button>
      {:else}
        <button class="btn btn-secondary" onclick={startService} disabled={actionLoading}>
          <Play size={14} fill="currentColor" /> Start
        </button>
      {/if}
      <button class="btn btn-primary" onclick={triggerDeploy} disabled={actionLoading}>
        {#if actionLoading}
          <Loader2 size={14} class="animate-spin" /> Deploying...
        {:else}
          <Rocket size={14} /> Deploy Now
        {/if}
      </button>
    </div>
  </div>

  <!-- Tabs -->
  <div class="tabs-bar" style="display:flex; gap:0; border-bottom:2px solid var(--color-border); margin-bottom:1.5rem; overflow-x:auto;">
    {#each tabs as t}
      <a
        href="/services/{id}/{t}"
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

  <!-- Tab content -->
  {#if tab === 'overview'}
    {@const parsedRes = (() => {
      try {
        return JSON.parse(service?.resource_json || service?.ResourceJSON || '{}');
      } catch {
        return {};
      }
    })()}

    <!-- Live URL & Public Ingress Banner -->
    {#if endpointUrl && (service?.kind === 'web' || service?.kind === 'static')}
      <div class="card" style="margin-bottom:1.5rem;">
        <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:1rem;">
          <div style="display:flex; align-items:center; gap:0.85rem;">
            <div style="display:flex; align-items:center; justify-content:center; width:44px; height:44px; border-radius:var(--radius-md); background:rgba(0,166,166,0.15); color:var(--color-accent);">
              <Globe size={24} />
            </div>
            <div>
              <div style="font-size:0.8125rem; font-weight:600; color:var(--color-accent); text-transform:uppercase; letter-spacing:0.04em;">
                Live Application Endpoint
              </div>
              <a 
                href={endpointUrl} 
                target="_blank" 
                rel="noopener noreferrer" 
                style="font-size:1.125rem; font-weight:700; color:var(--color-ink); text-decoration:none; display:inline-flex; align-items:center; gap:6px; margin-top:2px;"
              >
                {endpointUrl} <ExternalLink size={14} style="color:var(--color-accent);" />
              </a>
              <div class="text-xs text-muted" style="display:flex; align-items:center; gap:0.4rem; margin-top:0.25rem;">
                <ShieldCheck size={13} style="color:#34d399;" /> SSL Enabled (Let's Encrypt) | Routing via Traefik Edge
              </div>
            </div>
          </div>

          <div style="display:flex; gap:0.5rem; align-items:center;">
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:6px 12px;" onclick={copyEndpoint}>
              {#if copiedUrl}<Check size={14} /> Copied!{:else}<Copy size={14} /> Copy URL{/if}
            </button>
            <a href={endpointUrl} target="_blank" rel="noopener noreferrer" class="btn btn-primary" style="font-size:0.8125rem; padding:6px 14px;">
              Open Site <ExternalLink size={14} />
            </a>
          </div>
        </div>
      </div>
    {/if}

    <!-- Stat cards grid -->
    <div style="display:grid; grid-template-columns:repeat(auto-fit, minmax(200px, 1fr)); gap:1rem; margin-bottom:1.5rem;">
      <div class="card" style="padding:1.25rem;">
        <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Framework / Preset</div>
        <div style="font-size:1.125rem; font-weight:600; text-transform:capitalize;">{parsedRes.presetId || service?.kind || 'node'}</div>
      </div>

      <div class="card" style="padding:1.25rem;">
        <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Runtime Status</div>
        <div style="font-size:1.125rem; font-weight:600; text-transform:capitalize;">{service?.runtime_status || service?.RuntimeStatus || 'draft'}</div>
      </div>

      {#if service?.kind === 'web' || service?.kind === 'static'}
        <div class="card" style="padding:1.25rem;">
          <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Internal Port</div>
          <div style="font-size:1.125rem; font-weight:600; font-family:var(--font-mono)">:{service?.internal_port || service?.InternalPort || 80}</div>
        </div>
      {:else}
        <div class="card" style="padding:1.25rem;">
          <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Internal Port</div>
          <div style="font-size:1.125rem; font-weight:600; font-family:var(--font-mono)">:{service?.internal_port || service?.InternalPort || 80}</div>
        </div>
      {/if}

      <div class="card" style="padding:1.25rem;">
        <div class="text-xs text-muted" style="margin-bottom:0.25rem;">Total Deployments</div>
        <div style="font-size:1.125rem; font-weight:600;">{deployments.length}</div>
      </div>
    </div>

    <!-- Latest deployment card -->
    <div class="card" style="margin-bottom:1.5rem;">
      <div class="card-header" style="display:flex; align-items:center; justify-content:space-between;">
        <h3 style="margin:0; font-size:1rem;">Latest Deployment</h3>
        <button class="btn btn-secondary" style="padding:4px 10px; font-size:0.75rem;" onclick={loadService}>
          <RefreshCw size={12} /> Refresh
        </button>
      </div>
      {#if deployments.length > 0}
        {@const dep = deployments[0]}
        <div style="display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:1rem;">
          <div>
            <div style="display:flex; align-items:center; gap:0.5rem;">
              <span class="font-mono text-sm" style="font-weight:600;">Sequence #{dep.sequence}</span>
              <span class="badge badge-{dep.status}">{dep.status}</span>
            </div>
            <p class="text-xs text-muted" style="margin:0.25rem 0 0 0;">
              Triggered by {dep.triggered_by || 'system'} via {dep.trigger} | Driver: {dep.build_driver}
            </p>
          </div>
          <a href="/services/{id}/logs" class="btn btn-secondary" style="font-size:0.8125rem;">
            View Logs <ArrowRight size={13} />
          </a>
        </div>
      {:else}
        <div style="padding:1rem 0; text-align:center;">
          <p class="text-sm text-muted" style="margin-bottom:1rem;">No deployment has been executed yet.</p>
          <button class="btn btn-primary" onclick={triggerDeploy} disabled={actionLoading}>
            <Rocket size={14} /> Trigger Initial Deployment
          </button>
        </div>
      {/if}
    </div>

  {:else if tab === 'logs'}
    <div style="margin-bottom:1rem; display:flex; justify-content:space-between; align-items:center;">
      <h3 style="margin:0; font-size:1rem;">Real-Time Build & Runtime Logs</h3>
    </div>
    <LogViewer serviceId={service?.id || (id as string)} deploymentId={deployments[0]?.id || deployments[0]?.ID} />

  {:else if tab === 'deployments'}
    <div style="display:flex; align-items:center; justify-content:space-between; margin-bottom:1rem;">
      <h3 style="margin:0; font-size:1rem;">Deployment History ({deployments.length})</h3>
      <button class="btn btn-primary" style="padding:0.35rem 0.85rem; font-size:0.8125rem;" onclick={triggerDeploy} disabled={actionLoading}>
        <Rocket size={14} /> Trigger Deployment
      </button>
    </div>
    {#if deployments.length === 0}
      <div class="empty-state" style="padding:2rem; background:var(--color-surface); border:1px solid var(--color-border); border-radius:var(--radius-lg);">
        <p>No deployments recorded yet.</p>
        <button class="btn btn-primary mt-4" onclick={triggerDeploy}>Deploy Now</button>
      </div>
    {:else}
      <div class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th># Seq</th>
              <th>Status</th>
              <th>Trigger</th>
              <th>Driver</th>
              <th>Started At</th>
              <th>Actions</th>
            </tr>
          </thead>
          <tbody>
            {#each deployments as dep}
              <tr>
                <td class="font-mono text-sm" style="font-weight:600;">#{dep.sequence}</td>
                <td><span class="badge badge-{dep.status}">{dep.status}</span></td>
                <td class="text-sm">{dep.trigger} ({dep.triggered_by || 'user'})</td>
                <td class="font-mono text-xs">{dep.build_driver}</td>
                <td class="text-xs text-muted">{(dep.started_at || dep.StartedAt || '-').slice(0, 19).replace('T', ' ')}</td>
                <td style="text-align:right;">
                  <a href="/services/{id}/logs" class="btn btn-secondary" style="padding:4px 10px; font-size:0.75rem;">
                    Logs
                  </a>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}

  {:else if tab === 'variables'}
    <div class="card">
      <div class="card-header" style="display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:0.75rem;">
        <div>
          <h3 style="margin:0; font-size:1.05rem;">Environment Variables</h3>
          <p class="text-xs text-muted" style="margin-top:0.25rem;">Runtime environment variables injected into the container. Directly editable anytime.</p>
        </div>
        <div style="display:flex; align-items:center; gap:0.5rem; flex-wrap:wrap;">
          <!-- View Switcher -->
          <div style="display:inline-flex; background:var(--color-canvas); border:1px solid var(--color-border); border-radius:var(--radius-sm); padding:2px;">
            <button
              type="button"
              class="btn"
              style="padding:3px 10px; font-size:0.75rem; font-weight:{envMode === 'form' ? '700' : '500'}; background:{envMode === 'form' ? 'var(--color-surface)' : 'transparent'}; box-shadow:{envMode === 'form' ? 'var(--shadow-sm)' : 'none'}; border:none;"
              onclick={() => switchEnvMode('form')}
            >
              <FileText size={12} style="margin-right:4px;" /> Key-Value Editor
            </button>
            <button
              type="button"
              class="btn"
              style="padding:3px 10px; font-size:0.75rem; font-weight:{envMode === 'raw' ? '700' : '500'}; background:{envMode === 'raw' ? 'var(--color-surface)' : 'transparent'}; box-shadow:{envMode === 'raw' ? 'var(--shadow-sm)' : 'none'}; border:none;"
              onclick={() => switchEnvMode('raw')}
            >
              <Code size={12} style="margin-right:4px;" /> Raw .ENV
            </button>
          </div>

          <!-- Import from Blueprint button -->
          <button 
            type="button"
            class="btn btn-secondary" 
            style="font-size:0.8125rem; padding:4px 12px; min-height:32px; display:inline-flex; align-items:center; gap:6px;" 
            onclick={importBlueprintEnv}
            disabled={blueprintImportLoading}
            title="Import environment variables declared in repository klouds.yaml / render.yaml"
          >
            {#if blueprintImportLoading}
              <Loader2 size={13} class="animate-spin" /> Fetching...
            {:else}
              <Sparkles size={13} style="color:var(--color-accent);" /> Import from Blueprint
            {/if}
          </button>

          {#if envMode === 'form'}
            <button class="btn btn-secondary" style="font-size:0.8125rem; padding:4px 12px; min-height:32px;" onclick={addEnv}>
              <Plus size={14} /> Add Variable
            </button>
          {/if}
        </div>
      </div>

      {#if blueprintNotice}
        <div style="background:{blueprintNotice.type === 'success' ? 'var(--color-success-subtle)' : 'var(--color-danger-subtle)'}; border:1px solid {blueprintNotice.type === 'success' ? 'rgba(52,211,153,0.3)' : 'rgba(248,113,113,0.3)'}; color:{blueprintNotice.type === 'success' ? 'var(--color-success)' : 'var(--color-danger)'}; border-radius:var(--radius-md); padding:0.6rem 1rem; font-size:0.875rem; margin-bottom:1.25rem; display:flex; justify-content:space-between; align-items:center;">
          <span>{blueprintNotice.message}</span>
          <button type="button" class="btn btn-secondary" style="padding:2px 8px; font-size:0.75rem; height:auto; min-height:0;" onclick={() => blueprintNotice = null}>
            <X size={13} />
          </button>
        </div>
      {/if}

      <!-- Direct Editing Canvas -->
      {#if envMode === 'form'}
        {#if envVars.length === 0}
          <div style="text-align:center; padding:2rem 1rem; background:rgba(0,0,0,0.02); border:1px dashed var(--color-border); border-radius:var(--radius-md); margin-bottom:1.5rem;">
            <p class="text-sm text-muted" style="margin-bottom:0.75rem;">No environment variables configured yet.</p>
            <div style="display:flex; justify-content:center; gap:0.5rem;">
              <button type="button" class="btn btn-primary" style="font-size:0.8125rem;" onclick={addEnv}>
                <Plus size={14} /> Add Variable
              </button>
              <button type="button" class="btn btn-secondary" style="font-size:0.8125rem;" onclick={importBlueprintEnv} disabled={blueprintImportLoading}>
                <Sparkles size={14} /> Import from Blueprint
              </button>
            </div>
          </div>
        {:else}
          <div style="display:flex; flex-direction:column; gap:0.75rem; margin-bottom:1.5rem;">
            {#each envVars as env, i}
              <div style="display:flex; gap:0.75rem; align-items:center;">
                <input 
                  type="text" 
                  class="form-input font-mono text-sm" 
                  placeholder="VARIABLE_NAME" 
                  bind:value={env.key} 
                  oninput={() => { envDirty = true; syncEnvToRaw(); }}
                  style="flex:1;" 
                />
                <span class="text-muted" style="font-weight:700;">=</span>
                <input 
                  type="text" 
                  class="form-input font-mono text-sm" 
                  placeholder="value" 
                  bind:value={env.value} 
                  oninput={() => { envDirty = true; syncEnvToRaw(); }}
                  style="flex:2;" 
                />
                <button 
                  type="button"
                  class="btn btn-secondary" 
                  style="padding:6px; color:var(--color-error); border:none;" 
                  onclick={() => removeEnv(i)} 
                  aria-label="Remove variable"
                  title="Remove variable"
                >
                  <Trash2 size={16} />
                </button>
              </div>
            {/each}
          </div>
        {/if}
      {:else}
        <!-- Raw .ENV Mode -->
        <div style="margin-bottom:1.5rem;">
          <textarea
            class="form-input font-mono text-xs"
            rows="10"
            placeholder="KEY=value&#10;PORT=5000&#10;NODE_ENV=production&#10;VITE_API_URL=https://api.example.com"
            bind:value={rawEnvText}
            oninput={() => { envDirty = true; }}
            style="width:100%; resize:vertical; line-height:1.6; padding:0.85rem;"
          ></textarea>
          <p class="text-xs text-muted" style="margin-top:0.4rem;">
            Each line must follow the standard <code>KEY=VALUE</code> syntax. Lines starting with <code>#</code> are ignored.
          </p>
        </div>
      {/if}

      {#if envSuccess}
        <div style="background:#d1fae5; border:1px solid #6ee7b7; color:#065f46; border-radius:var(--radius-md); padding:0.6rem 1rem; font-size:0.875rem; margin-bottom:1rem;">
          Environment variables saved successfully.
        </div>
      {/if}

      <!-- Action Bar with Save & Redeploy -->
      <div style="display:flex; justify-content:space-between; align-items:center; flex-wrap:wrap; gap:0.75rem; padding-top:1.25rem; border-top:1px solid var(--color-border);">
        <span class="text-xs text-muted">
          {#if envDirty}
            <span style="color:#fbbf24; font-weight:600;">Unsaved changes</span>
          {:else}
            <span>All environment variables synced</span>
          {/if}
        </span>
        <div style="display:flex; gap:0.6rem; align-items:center;">
          <button 
            type="button" 
            class="btn btn-secondary" 
            style="font-size:0.8125rem; display:inline-flex; align-items:center; gap:6px; padding:7px 14px;"
            onclick={() => saveEnvVars(false)} 
            disabled={envSaving}
          >
            {#if envSaving}<Loader2 size={14} class="animate-spin" /> Saving...{:else}<Save size={14} /> Save Only{/if}
          </button>
          <button 
            type="button" 
            class="btn btn-primary" 
            style="font-size:0.8125rem; display:inline-flex; align-items:center; gap:6px; padding:7px 18px;"
            onclick={() => saveEnvVars(true)} 
            disabled={envSaving}
          >
            {#if envSaving}
              <Loader2 size={14} class="animate-spin" /> Deploying...
            {:else}
              <Rocket size={14} /> Save & Redeploy
            {/if}
          </button>
        </div>
      </div>
    </div>

  {:else if tab === 'domains'}
    <div class="card" style="margin-bottom: 1.5rem;">
      <div class="card-header" style="display:flex; justify-content:space-between; align-items:center;">
        <div>
          <h3 style="margin:0; font-size:1.05rem;">Domain & TLS Configuration</h3>
          <div class="text-xs text-muted" style="margin-top:2px;">Attach custom domain names and automate Let's Encrypt TLS/SSL certificates</div>
        </div>
        <button class="btn btn-secondary" onclick={loadDomains} style="padding:4px 10px; font-size:0.75rem;">
          <RefreshCw size={14} /> Refresh
        </button>
      </div>

      {#if domainNotice}
        <div style="background:{domainNotice.type === 'success' ? '#d1fae5' : '#fee2e2'}; border:1px solid {domainNotice.type === 'success' ? '#6ee7b7' : '#fca5a5'}; color:{domainNotice.type === 'success' ? '#065f46' : '#991b1b'}; border-radius:var(--radius-md); padding:0.6rem 1rem; font-size:0.875rem; margin-bottom:1rem;">
          {domainNotice.message}
        </div>
      {/if}

      <!-- Add Domain Form -->
      <div style="background: rgba(0,0,0,0.02); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border); margin-bottom: 1.5rem;">
        <label for="new-domain-input" class="form-label" style="font-weight:600; margin-bottom:0.4rem;">Add Custom Domain</label>
        <div style="display:flex; gap:0.75rem; flex-wrap:wrap;">
          <input 
            id="new-domain-input"
            type="text" 
            class="form-input font-mono" 
            placeholder="e.g. app.yourdomain.com or mybrand.com" 
            bind:value={newDomainInput} 
            style="max-width:380px; flex:1;"
            onkeydown={(e) => { if (e.key === 'Enter') addCustomDomain(); }}
          />
          <button class="btn btn-primary" onclick={addCustomDomain} disabled={domainSaving || !newDomainInput.trim()}>
            {#if domainSaving}<Loader2 size={14} class="animate-spin" /> Saving...{:else}<Plus size={14} /> Add Domain{/if}
          </button>
        </div>
        <p class="text-xs text-muted" style="margin-top: 0.5rem; margin-bottom: 0;">
          Point your domain's CNAME record to <code class="font-mono">{service?.domain || (typeof window !== 'undefined' ? `${service?.slug}.${window.location.hostname}` : 'yourdomain.com')}</code> to complete verification.
        </p>
      </div>

      <!-- Configured Domains List -->
      <div style="font-weight:600; font-size:0.875rem; margin-bottom:0.75rem;">Configured Domains & SSL Status</div>
      <div class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th>Domain</th>
              <th>Type</th>
              <th>SSL / TLS Status</th>
              <th>DNS Target</th>
              <th style="text-align:right;">Actions</th>
            </tr>
          </thead>
          <tbody>
            <!-- Primary System Domain -->
            <tr>
              <td>
                <div style="display:flex; align-items:center; gap:0.5rem;">
                  <Globe size={16} style="color:var(--color-accent);" />
                  <a href="https://{service?.domain || (typeof window !== 'undefined' ? `${service?.slug}.${window.location.hostname}` : 'yourdomain.com')}" target="_blank" rel="noreferrer" class="font-mono text-sm" style="font-weight:600; color:var(--color-accent-dim);">
                    {service?.domain || (typeof window !== 'undefined' ? `${service?.slug}.${window.location.hostname}` : 'yourdomain.com')}
                  </a>
                  <ExternalLink size={12} style="color:var(--color-ink-muted);" />
                </div>
              </td>
              <td><span class="badge badge-running" style="font-size:0.7rem;">Primary (Auto)</span></td>
              <td>
                <span class="badge" style="background:#dcfce7; color:#15803d; font-size:0.7rem; display:inline-flex; align-items:center; gap:4px;">
                  <ShieldCheck size={12} /> Auto Let's Encrypt Active
                </span>
              </td>
              <td class="font-mono text-xs text-muted">Platform Ingress</td>
              <td style="text-align:right;" class="text-xs text-muted">Default</td>
            </tr>

            <!-- Custom Domains -->
            {#each customDomainsList.filter(d => !d.isPrimary) as d}
              <tr>
                <td>
                  <div style="display:flex; align-items:center; gap:0.5rem;">
                    <Globe size={16} style="color:#3b82f6;" />
                    <a href="https://{d.domain}" target="_blank" rel="noreferrer" class="font-mono text-sm" style="font-weight:600; color:var(--color-ink);">
                      {d.domain}
                    </a>
                    <ExternalLink size={12} style="color:var(--color-ink-muted);" />
                  </div>
                </td>
                <td><span class="badge" style="background:#e0f2fe; color:#0369a1; font-size:0.7rem;">Custom Domain</span></td>
                <td>
                  <span class="badge" style="background:#dcfce7; color:#15803d; font-size:0.7rem; display:inline-flex; align-items:center; gap:4px;">
                    <ShieldCheck size={12} /> TLS Active
                  </span>
                </td>
                <td class="font-mono text-xs" style="color:var(--color-ink-muted);">
                  CNAME -&gt; {service?.domain || (typeof window !== 'undefined' ? `${service?.slug}.${window.location.hostname}` : 'yourdomain.com')}
                </td>
                <td style="text-align:right;">
                  <button 
                    class="btn btn-secondary" 
                    style="padding:3px 8px; font-size:0.75rem; color:var(--color-error);"
                    onclick={() => removeCustomDomain(d.domain)}
                    aria-label="Remove domain"
                  >
                    <Trash2 size={13} /> Remove
                  </button>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>

  {:else if tab === 'routes'}
    <div style="max-width: 860px;">
      <div class="card" style="padding: 1.5rem; background: var(--color-surface); border: 1px solid var(--color-border); border-radius: var(--radius-md);">
        <div style="margin-bottom: 1.5rem;">
          <h2 style="font-size: 1.125rem; font-weight: 700; color: var(--color-ink); margin: 0 0 0.4rem 0;">
            Redirect and Rewrite Rules
          </h2>
          <p class="text-xs text-muted" style="line-height: 1.5; margin: 0;">
            Add <span style="color: var(--color-accent); font-weight: 600;">Redirect or Rewrite Rules</span> to modify requests to your site. Rules are processed at the network edge with zero downtime and take effect instantly without changing environment variables or rebuilding.
          </p>
        </div>

        {#if routesNotice}
          <div style="padding: 0.75rem 1rem; border-radius: var(--radius-sm); margin-bottom: 1.25rem; font-size: 0.8125rem; background: {routesNotice.type === 'success' ? 'rgba(16,185,129,0.1)' : 'rgba(239,68,68,0.1)'}; color: {routesNotice.type === 'success' ? '#065f46' : '#991b1b'}; border: 1px solid {routesNotice.type === 'success' ? '#10b981' : '#ef4444'};">
            {routesNotice.message}
          </div>
        {/if}

        {#if serviceRoutes.length === 0}
          <div style="text-align: center; padding: 2.5rem 1rem; background: var(--color-canvas); border-radius: var(--radius-sm); border: 1px dashed var(--color-border); margin-bottom: 1.25rem;">
            <Sliders size={28} class="text-muted" style="margin-bottom: 0.5rem;" />
            <div style="font-size: 0.875rem; font-weight: 600; color: var(--color-ink);">No Redirect or Rewrite Rules defined</div>
            <div class="text-xs text-muted" style="margin-top: 0.25rem;">Add rules to proxy API paths or redirect legacy URLs seamlessly.</div>
          </div>
        {:else}
          <div style="display: flex; flex-direction: column; gap: 1rem; margin-bottom: 1.5rem;">
            {#each serviceRoutes as rule, idx}
              <div style="position: relative; padding: 1.25rem; border: 1px solid var(--color-border); border-radius: var(--radius-sm); background: var(--color-canvas);">
                <!-- Reorder & Delete controls -->
                <div style="position: absolute; left: 12px; top: 20px; display: flex; flex-direction: column; gap: 6px;">
                  <button
                    type="button"
                    class="btn-icon"
                    disabled={idx === 0}
                    onclick={() => moveRule(idx, -1)}
                    style="opacity: {idx === 0 ? 0.25 : 0.7}; padding: 2px; cursor: {idx === 0 ? 'not-allowed' : 'pointer'}; background: transparent; border: none; color: var(--color-ink);"
                    title="Move up"
                  >
                    <ArrowUp size={15} />
                  </button>
                  <button
                    type="button"
                    class="btn-icon"
                    disabled={idx === serviceRoutes.length - 1}
                    onclick={() => moveRule(idx, 1)}
                    style="opacity: {idx === serviceRoutes.length - 1 ? 0.25 : 0.7}; padding: 2px; cursor: {idx === serviceRoutes.length - 1 ? 'not-allowed' : 'pointer'}; background: transparent; border: none; color: var(--color-ink);"
                    title="Move down"
                  >
                    <ArrowDown size={15} />
                  </button>
                </div>

                <div style="position: absolute; right: 14px; top: 14px;">
                  <button
                    type="button"
                    onclick={() => removeRule(idx)}
                    style="background: transparent; border: none; color: #ef4444; cursor: pointer; padding: 4px; border-radius: 4px; display: flex; align-items: center;"
                    title="Delete rule"
                  >
                    <Trash2 size={16} />
                  </button>
                </div>

                <!-- Rule inputs container matching Render UI -->
                <div style="margin-left: 32px; margin-right: 32px; display: flex; flex-direction: column; gap: 0.85rem;">
                  <div>
                    <label for={`rule-source-${idx}`} class="form-label" style="font-size: 0.75rem; font-weight: 600; text-transform: none; color: var(--color-ink); margin-bottom: 4px; display: block;">Source</label>
                    <input
                      id={`rule-source-${idx}`}
                      type="text"
                      class="form-input font-mono text-xs"
                      placeholder="/api/*"
                      bind:value={rule.source}
                      oninput={() => { routesDirty = true; }}
                      style="width: 100%; height: 34px;"
                    />
                  </div>

                  <div>
                    <label for={`rule-dest-${idx}`} class="form-label" style="font-size: 0.75rem; font-weight: 600; text-transform: none; color: var(--color-ink); margin-bottom: 4px; display: block;">Destination</label>
                    <input
                      id={`rule-dest-${idx}`}
                      type="text"
                      class="form-input font-mono text-xs"
                      placeholder="https://api.example.com/api/*"
                      bind:value={rule.destination}
                      oninput={() => { routesDirty = true; }}
                      style="width: 100%; height: 34px;"
                    />
                  </div>

                  <div>
                    <label for={`rule-action-${idx}`} class="form-label" style="font-size: 0.75rem; font-weight: 600; text-transform: none; color: var(--color-ink); margin-bottom: 4px; display: block;">Action</label>
                    <select
                      id={`rule-action-${idx}`}
                      class="form-select text-xs"
                      bind:value={rule.type}
                      onchange={() => { routesDirty = true; }}
                      style="width: 100%; height: 34px;"
                    >
                      <option value="rewrite">Rewrite</option>
                      <option value="redirect">Redirect</option>
                    </select>
                  </div>
                </div>
              </div>
            {/each}
          </div>
        {/if}

        <!-- Bottom action bar -->
        <div style="display: flex; justify-content: space-between; align-items: center; gap: 0.75rem; flex-wrap: wrap;">
          <button
            type="button"
            class="btn btn-secondary"
            style="font-size: 0.8125rem; display: flex; align-items: center; gap: 6px; padding: 7px 14px;"
            onclick={importBlueprintRoutes}
            disabled={blueprintImportLoading}
            title="Import redirect and rewrite rules from repository klouds.yaml"
          >
            {#if blueprintImportLoading}
              <Loader2 size={14} class="animate-spin" />
            {:else}
              <Sparkles size={14} style="color:var(--color-accent);" />
            {/if}
            Import Routes from Blueprint
          </button>
          <div style="display: flex; gap: 0.6rem; align-items: center;">
            <button
              type="button"
              class="btn btn-secondary"
              style="font-size: 0.8125rem; display: flex; align-items: center; gap: 6px; padding: 7px 14px;"
              onclick={addRule}
            >
              <Plus size={14} /> Add Rule
            </button>
            <button
              type="button"
              class="btn btn-primary"
              style="font-size: 0.8125rem; display: flex; align-items: center; gap: 6px; padding: 7px 18px;"
              onclick={saveRules}
              disabled={routesSaving}
            >
              {#if routesSaving}
                <Loader2 size={14} class="animate-spin" /> Saving...
              {:else}
                <Save size={14} /> Save Changes
              {/if}
            </button>
          </div>
        </div>

        <!-- Interactive Test Path Simulator -->
        <div style="margin-top: 2rem; padding-top: 1.5rem; border-top: 1px solid var(--color-border);">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.35rem; flex-wrap: wrap; gap: 0.5rem;">
            <h4 style="font-size: 0.925rem; font-weight: 700; color: var(--color-ink); margin: 0;">
              Test Path Simulator
            </h4>
            <div style="display: flex; gap: 0.35rem; align-items: center; flex-wrap: wrap;">
              <span class="text-xs text-muted" style="font-size: 0.72rem;">Quick Test:</span>
              <button 
                type="button" 
                class="btn btn-secondary" 
                style="padding: 2px 8px; font-size: 0.72rem; height: auto; min-height: 0;"
                onclick={() => { testSimPath = '/api/login?ref=dashboard'; }}
              >
                /api/login
              </button>
              <button 
                type="button" 
                class="btn btn-secondary" 
                style="padding: 2px 8px; font-size: 0.72rem; height: auto; min-height: 0;"
                onclick={() => { testSimPath = '/api/v1/users?page=1'; }}
              >
                /api/v1/users
              </button>
              <button 
                type="button" 
                class="btn btn-secondary" 
                style="padding: 2px 8px; font-size: 0.72rem; height: auto; min-height: 0;"
                onclick={() => { testSimPath = '/dashboard/settings'; }}
              >
                /dashboard/settings
              </button>
            </div>
          </div>
          <p class="text-xs text-muted" style="margin-bottom: 0.85rem; line-height: 1.5;">
            Type any relative path (e.g. <code>/api/login</code>) or full URL (e.g. <code>https://my-app.klouds.online/api/login?ref=1</code>) to preview how your redirect and rewrite rules will execute in real-time.
          </p>
          <div style="display: flex; gap: 0.5rem; align-items: center;">
            <input
              type="text"
              class="form-input font-mono text-xs"
              placeholder="/api/login?ref=test or https://example.com/api/users"
              bind:value={testSimPath}
              style="width: 100%; height: 38px;"
            />
            {#if testSimPath.trim()}
              <button
                type="button"
                class="btn btn-secondary"
                style="padding: 0 12px; height: 38px; font-size: 0.75rem; white-space: nowrap; display: inline-flex; align-items: center; gap: 4px;"
                onclick={() => testLiveRequest(testSimPath)}
                disabled={liveTestLoading}
                title="Send a live test request to verify the route response"
              >
                {#if liveTestLoading}
                  <Loader2 size={13} class="animate-spin" /> Testing...
                {:else}
                  <Rocket size={13} /> Test Live
                {/if}
              </button>
            {/if}
          </div>

          {#if testSimPath.trim()}
            {@const simResult = simulateRuleMatch(testSimPath.trim(), serviceRoutes)}
            <div style="margin-top: 0.85rem; padding: 1rem; border-radius: var(--radius-md); font-size: 0.8125rem; background: var(--color-canvas); border: 1px solid var(--color-border); display: flex; flex-direction: column; gap: 0.65rem;">
              <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 0.5rem;">
                <div style="display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap;">
                  <span style="font-weight: 700; color: var(--color-ink);">Simulation Result:</span>
                  {#if simResult.matched}
                    <span class="badge" style="background: {simResult.action.startsWith('redirect') ? 'rgba(245,158,11,0.18)' : 'rgba(16,185,129,0.18)'}; color: {simResult.action.startsWith('redirect') ? '#b45309' : '#047857'}; font-size: 0.75rem; font-weight: 700; padding: 3px 8px;">
                      {simResult.actionLabel}
                    </span>
                    <span class="font-mono text-xs" style="color: var(--color-accent); font-weight: 600; word-break: break-all;">
                      {simResult.destination}
                    </span>
                  {:else}
                    <span class="badge" style="background: rgba(148,163,184,0.18); color: #64748b; font-size: 0.75rem; font-weight: 600;">
                      No Rule Match
                    </span>
                    <span class="text-xs text-muted">
                      Serves physical static file or SPA fallback
                    </span>
                  {/if}
                </div>
              </div>

              <div class="text-xs text-muted" style="font-size: 0.75rem; border-top: 1px dashed var(--color-border); padding-top: 0.5rem; display: flex; align-items: center; gap: 4px;">
                <Sparkles size={13} /> {simResult.explanation}
              </div>

              {#if liveTestResult}
                <div style="background: var(--color-surface); border: 1px solid {liveTestResult.ok ? 'rgba(16,185,129,0.3)' : 'rgba(239,68,68,0.3)'}; border-radius: var(--radius-sm); padding: 0.6rem 0.85rem; font-size: 0.75rem; display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 0.5rem;">
                  <div style="display: flex; align-items: center; gap: 0.5rem;">
                    <span class="badge" style="background: {liveTestResult.ok ? 'rgba(16,185,129,0.18)' : 'rgba(239,68,68,0.18)'}; color: {liveTestResult.ok ? '#047857' : '#b91c1c'}; font-weight: 700;">
                      HTTP {liveTestResult.status} {liveTestResult.statusText}
                    </span>
                    <span class="text-muted">Target: <code class="font-mono text-xs">{liveTestResult.finalUrl}</code></span>
                  </div>
                  <span class="text-muted font-mono text-xs">Latency: {liveTestResult.timeMs}ms</span>
                </div>
              {/if}
            </div>
          {/if}
        </div>
      </div>
    </div>

  {:else if tab === 'scale'}
    <div class="card">
      <div class="card-header">
        <h3 style="margin:0;">Scale & Resource Limits</h3>
      </div>
      <div style="display:flex; align-items:center; justify-content:space-between; padding:1rem 0; border-bottom:1px solid var(--color-border);">
        <div>
          <div style="font-weight:600;">Scale to Zero (Sablier)</div>
          <div class="text-sm text-muted">Automatically suspend this container when inactive to save RAM and CPU.</div>
        </div>
        <span class="badge badge-running">Enabled</span>
      </div>
      <div style="padding-top:1rem; display:flex; justify-content:space-between; align-items:center;">
        <div>
          <div style="font-weight:600;">Replicas</div>
          <div class="text-sm text-muted">Number of container instances running behind Traefik.</div>
        </div>
        <span class="font-mono text-sm" style="font-weight:600;">1 instance</span>
      </div>
    </div>

  {:else if tab === 'settings'}
    <div style="display: flex; flex-direction: column; gap: 1.5rem;">
      <!-- General & Build Configuration Form -->
      <form onsubmit={saveServiceSettings} class="card" style="padding: 1.5rem; background: var(--color-surface); border: 1px solid var(--color-border);">
        <div class="card-header" style="margin-bottom: 1.25rem; display: flex; justify-content: space-between; align-items: center;">
          <div>
            <h3 style="margin: 0; font-size: 1.05rem;">Build & Deployment Settings</h3>
            <p class="text-xs text-muted" style="margin: 2px 0 0 0;">Configure build pipeline, start commands, branches, and auto-deploy behavior.</p>
          </div>
          <div style="display: flex; gap: 0.5rem; align-items: center;">
            <button 
              type="button" 
              class="btn btn-secondary" 
              style="display: flex; align-items: center; gap: 6px; font-size: 0.8125rem;" 
              onclick={importBlueprintSettings}
              disabled={blueprintImportLoading}
              title="Sync build and start commands from repository klouds.yaml / render.yaml"
            >
              {#if blueprintImportLoading}
                <Loader2 size={13} class="animate-spin" />
              {:else}
                <Sparkles size={13} style="color:var(--color-accent);" />
              {/if}
              Sync from Blueprint
            </button>
            <button type="submit" class="btn btn-primary" style="display: flex; align-items: center; gap: 6px; font-size: 0.8125rem;" disabled={settingsSaving}>
              {#if settingsSaving}<Loader2 size={14} class="animate-spin" /> Saving...{:else}<Save size={14} /> Save Settings{/if}
            </button>
          </div>
        </div>

        {#if settingsSaved}
          <div style="background:#d1fae5; border:1px solid #6ee7b7; color:#065f46; border-radius:var(--radius-md); padding:0.65rem 1rem; font-size:0.875rem; margin-bottom:1.25rem;">
            Service configuration updated successfully. Click "Trigger Deployment" to apply changes.
          </div>
        {/if}

        {#if settingsError}
          <div style="background:#fee2e2; border:1px solid #fca5a5; color:#991b1b; border-radius:var(--radius-md); padding:0.65rem 1rem; font-size:0.875rem; margin-bottom:1.25rem;">
            {settingsError}
          </div>
        {/if}

        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(240px, 1fr)); gap: 1.25rem; margin-bottom: 1.25rem;">
          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-name">Service Name</label>
            <input 
              id="settings-name" 
              type="text" 
              class="form-input" 
              bind:value={settingsName} 
              oninput={() => settingsDirty = true} 
              required 
            />
          </div>

          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-preset">Runtime / Framework Preset</label>
            <select 
              id="settings-preset" 
              class="form-select" 
              bind:value={settingsPreset} 
              onchange={() => settingsDirty = true}
            >
              <option value="node">Node.js (Next.js / Express / Nest / Remix / Astro)</option>
              <option value="python">Python (FastAPI / Flask / Django / Celery)</option>
              <option value="go">Go (Fiber / Gin / Chi / Echo)</option>
              <option value="rust">Rust (Actix / Axum / Rocket / Cargo)</option>
              <option value="java">Java (Spring Boot / Maven / Gradle)</option>
              <option value="php">PHP (Laravel / Symfony / Apache)</option>
              <option value="ruby">Ruby on Rails / Puma</option>
              <option value="elixir">Elixir (Phoenix / Plug)</option>
              <option value="deno">Deno (Fresh / Hono / Oak)</option>
              <option value="bun">Bun (Elysia / Hono / Next.js)</option>
              <option value="dotnet">.NET / C# (ASP.NET Core)</option>
              <option value="scala">Scala (sbt / Play / Akka)</option>
              <option value="kotlin">Kotlin (Ktor / Spring Boot)</option>
              <option value="swift">Swift (Vapor / Hummingbird)</option>
              <option value="haskell">Haskell (Servant / Yesod)</option>
              <option value="clojure">Clojure (Ring / Leiningen)</option>
              <option value="crystal">Crystal (Kemal / Lucky)</option>
              <option value="zig">Zig (HTTP / Native)</option>
              <option value="dart">Dart (Shelf / Server)</option>
              <option value="static">Static Site (SPA / React / Vite / HTML)</option>
              <option value="dockerfile">Custom Dockerfile</option>
              <option value="worker">Background Worker Daemon</option>
              <option value="cron">Scheduled Cron Job</option>
            </select>
          </div>

          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-runtime-version">Runtime Version</label>
            <input 
              id="settings-runtime-version" 
              type="text" 
              class="form-input font-mono text-sm" 
              placeholder="auto (e.g. 20, 3.12, 1.23)" 
              bind:value={settingsRuntimeVersion} 
              oninput={() => settingsDirty = true} 
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">Set <code>auto</code> to auto-detect from project files or specify an explicit version tag.</p>
          </div>
        </div>

        <!-- Security & Resource Limits Configuration Box in Settings -->
        <div style="background: var(--color-canvas); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border); margin-bottom: 1.25rem;">
          <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.75rem;">
            <div style="display: flex; align-items: center; gap: 0.5rem;">
              <ShieldCheck size={18} style="color: var(--color-accent);" />
              <span style="font-size: 0.875rem; font-weight: 700;">Sandbox Resource Limits & Security</span>
            </div>
            <span class="badge" style="background: var(--color-success-subtle); color: var(--color-success); font-size: 0.7rem; display: flex; align-items: center; gap: 4px;">
              <ShieldCheck size={12} /> Non-Root Sandbox Active
            </span>
          </div>

          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem;">
            <div class="form-group" style="margin: 0;">
              <label for="settings-mem-limit" class="form-label" style="font-size: 0.8125rem;">Memory Limit</label>
              <select id="settings-mem-limit" class="form-select font-mono text-xs" bind:value={settingsMemoryLimit} onchange={() => settingsDirty = true}>
                <option value="256m">256 MB (Micro)</option>
                <option value="512m">512 MB (Standard)</option>
                <option value="1g">1 GB (Medium)</option>
                <option value="2g">2 GB (High Performance)</option>
                <option value="4g">4 GB (Extra Large)</option>
                <option value="8g">8 GB (Maximum)</option>
              </select>
            </div>

            <div class="form-group" style="margin: 0;">
              <label for="settings-cpu-limit" class="form-label" style="font-size: 0.8125rem;">CPU Limit</label>
              <select id="settings-cpu-limit" class="form-select font-mono text-xs" bind:value={settingsCPULimit} onchange={() => settingsDirty = true}>
                <option value="0.5">0.5 Cores (Eco)</option>
                <option value="1.0">1.0 Core (Standard)</option>
                <option value="2.0">2.0 Cores (High Throughput)</option>
                <option value="4.0">4.0 Cores (Maximum)</option>
              </select>
            </div>
          </div>
        </div>

        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1.25rem; margin-bottom: 1.25rem;">
          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-build">Build Command</label>
            <input 
              id="settings-build" 
              type="text" 
              class="form-input font-mono text-sm" 
              placeholder="e.g. npm install && npm run build" 
              bind:value={settingsBuildCmd} 
              oninput={() => settingsDirty = true} 
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">Command executed inside the build sandbox to compile assets or install dependencies.</p>
          </div>

          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-start">Start / Run Command</label>
            <input 
              id="settings-start" 
              type="text" 
              class="form-input font-mono text-sm" 
              placeholder="e.g. npm start or node server.js" 
              bind:value={settingsStartCmd} 
              oninput={() => settingsDirty = true} 
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">Command executed when starting the runtime production container.</p>
          </div>
        </div>

        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 1.25rem; margin-bottom: 1.25rem;">
          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-root">Root Directory</label>
            <input 
              id="settings-root" 
              type="text" 
              class="form-input font-mono text-sm" 
              placeholder="." 
              bind:value={settingsRootDir} 
              oninput={() => settingsDirty = true} 
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">Subdirectory containing code for monorepos (defaults to repository root <code>.</code>).</p>
          </div>

          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-branch">Git Branch</label>
            <input 
              id="settings-branch" 
              type="text" 
              class="form-input font-mono text-sm" 
              placeholder="main" 
              bind:value={settingsBranch} 
              oninput={() => settingsDirty = true} 
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">Branch tracked for deployments.</p>
          </div>

          <div class="form-group" style="margin: 0;">
            <label class="form-label" for="settings-port">Internal Container Port</label>
            <input 
              id="settings-port" 
              type="number" 
              class="form-input font-mono text-sm" 
              placeholder="80" 
              bind:value={settingsPort} 
              oninput={() => settingsDirty = true} 
            />
            <p class="text-xs text-muted" style="margin-top: 4px;">Port your application listens on inside the container.</p>
          </div>
        </div>

        {#if settingsRepoUrl}
          <div class="form-group" style="margin-bottom: 1.25rem;">
            <label class="form-label" for="settings-repo">Git Repository URL</label>
            <input 
              id="settings-repo" 
              type="text" 
              class="form-input font-mono text-sm" 
              bind:value={settingsRepoUrl} 
              oninput={() => settingsDirty = true} 
            />
          </div>
        {/if}

        <div style="display: flex; align-items: center; justify-content: space-between; padding: 1rem; background: rgba(0,0,0,0.02); border: 1px solid var(--color-border); border-radius: var(--radius-md);">
          <div>
            <div style="font-weight: 600; font-size: 0.875rem;">Auto-Deploy on Git Push</div>
            <div class="text-xs text-muted">Automatically build and deploy whenever new commits are pushed to the tracked branch.</div>
          </div>
          <label style="display: inline-flex; align-items: center; cursor: pointer;">
            <input 
              type="checkbox" 
              bind:checked={settingsAutoDeploy} 
              onchange={() => settingsDirty = true}
              style="width: 18px; height: 18px; accent-color: var(--color-accent);"
            />
          </label>
        </div>
      </form>

      <!-- Container Operations Card -->
      <div class="card" style="padding: 1.5rem; background: var(--color-surface); border: 1px solid var(--color-border);">
        <div class="card-header" style="margin-bottom: 1rem;">
          <h3 style="margin: 0; font-size: 1.05rem;">Service Operations & Lifecycle</h3>
        </div>

        <div style="display: flex; gap: 1rem; flex-wrap: wrap;">
          <button 
            type="button" 
            class="btn btn-secondary" 
            style="display: flex; align-items: center; gap: 6px; font-size: 0.8125rem;"
            onclick={restartService}
            disabled={actionLoading}
          >
            <RefreshCw size={14} class={actionLoading ? 'animate-spin' : ''} />
            Restart Container
          </button>

          <button 
            type="button" 
            class="btn btn-primary" 
            style="display: flex; align-items: center; gap: 6px; font-size: 0.8125rem;"
            onclick={triggerDeploy}
            disabled={actionLoading}
          >
            <Rocket size={14} />
            Trigger Rebuild & Deploy
          </button>
        </div>
      </div>

      <!-- Danger Zone Card -->
      <div class="card" style="padding: 1.5rem; background: var(--color-surface); border: 1px solid #fca5a5;">
        <div class="card-header" style="border-bottom-color: #fee2e2; margin-bottom: 1rem;">
          <h3 style="color: var(--color-danger); margin: 0; font-size: 1.05rem;">Danger Zone</h3>
        </div>
        <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 1rem;">
          <div>
            <div style="font-weight: 600; color: var(--color-ink);">Delete this Service</div>
            <div class="text-sm text-muted">Permanently delete this service, its configuration, SSL certificates, and all deployment history.</div>
          </div>
          <button class="btn btn-danger" onclick={deleteService} disabled={actionLoading}>
            <Trash2 size={16} /> Delete Service
          </button>
        </div>
      </div>
    </div>
  {/if}
{/if}
