<script lang="ts">
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { onMount } from 'svelte';
  import {
    ArrowLeft,
    Rocket,
    Globe,
    Server,
    Clock,
    Layers,
    Code,
    Cpu,
    Plus,
    X,
    Save,
    Loader2,
    Check,
    GitBranch,
    FolderGit2,
    Lock,
    Unlock,
    ExternalLink,
    Terminal,
    Settings,
    Search,
    FileCode,
    Sparkles,
    Database,
    KeyRound,
    AlertTriangle,
    Wand2
  } from 'lucide-svelte';
  import FrameworkIcon from '$lib/components/icons/FrameworkIcon.svelte';

  const { slug } = $derived($page.params);
  let project = $state<any>(null);
  let loading = $state(true);

  // Source Type: 'git_public' | 'git_provider' | 'image'
  let sourceType = $state<'git_public' | 'git_provider' | 'image'>('git_public');

  // Public Git source fields
  let gitRepoUrl = $state('');
  let gitBranch = $state('main');
  let rootDirectory = $state('.');

  // render.yaml / blueprint auto-detect state
  let parsingYaml = $state(false);
  let detectedBlueprint = $state<any>(null);
  let detectedServices = $state<any[]>([]);
  let detectedDatabases = $state<any[]>([]);
  let selectedBlueprintIndex = $state(0);
  let deployingBlueprint = $state(false);
  let yamlParsedInfo = $state<string | null>(null);

  // Required Environment Variables Prompt Modal State
  let showEnvPromptModal = $state(false);
  let pendingAction = $state<'blueprint' | 'single' | null>(null);

  function generateRandomSecret(length = 32) {
    const chars = 'abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-';
    let res = '';
    const arr = new Uint8Array(length);
    window.crypto.getRandomValues(arr);
    for (let i = 0; i < length; i++) {
      res += chars[arr[i] % chars.length];
    }
    return res;
  }

  function getBlueprintUnfilledEnvVars() {
    const list: Array<{ svcName: string; svcIdx: number; key: string; value: string; isSecret: boolean; isRequired: boolean }> = [];
    detectedServices.forEach((svc, sIdx) => {
      const reqs = svc.required_env_vars || [];
      Object.entries(svc.env_vars || {}).forEach(([k, v]) => {
        const strVal = String(v || '').trim();
        const isReq = reqs.includes(k) || strVal === '' || strVal.toLowerCase().startsWith('your_') || strVal.toLowerCase().startsWith('replace_') || strVal.toLowerCase() === 'changeme';
        const isSecret = k.includes('SECRET') || k.includes('KEY') || k.includes('PASS') || k.includes('TOKEN') || k.includes('AUTH');
        if (isReq || isSecret) {
          list.push({ svcName: svc.name, svcIdx: sIdx, key: k, value: strVal, isSecret, isRequired: isReq });
        }
      });
    });
    return list;
  }

  function updateDetectedEnv(sIdx: number, key: string, val: string) {
    if (detectedServices[sIdx]) {
      if (!detectedServices[sIdx].env_vars) detectedServices[sIdx].env_vars = {};
      detectedServices[sIdx].env_vars[key] = val;
      if (selectedBlueprintIndex === sIdx) {
        envVars = Object.entries(detectedServices[sIdx].env_vars).map(([k, v]) => ({ key: k, value: String(v) }));
      }
    }
  }

  function autoFillAllSecrets() {
    detectedServices.forEach((svc, sIdx) => {
      Object.keys(svc.env_vars || {}).forEach(k => {
        const strVal = String(svc.env_vars[k] || '').trim();
        const isSecret = k.includes('SECRET') || k.includes('KEY') || k.includes('PASS') || k.includes('TOKEN') || k.includes('AUTH');
        if (isSecret && (strVal === '' || strVal.toLowerCase().startsWith('your_') || strVal.toLowerCase().startsWith('replace_') || strVal.toLowerCase() === 'changeme')) {
          svc.env_vars[k] = generateRandomSecret(32);
        }
      });
    });
    envVars = envVars.map(e => {
      const isSecret = e.key.includes('SECRET') || e.key.includes('KEY') || e.key.includes('PASS') || e.key.includes('TOKEN') || e.key.includes('AUTH');
      if (isSecret && (!e.value || e.value.toLowerCase().startsWith('your_') || e.value.toLowerCase().startsWith('replace_') || e.value.toLowerCase() === 'changeme')) {
        return { ...e, value: generateRandomSecret(32) };
      }
      return e;
    });
  }

  // Git Provider integration state
  let gitIntegrations = $state<any[]>([]);
  let providerRepos = $state<any[]>([]);
  let selectedProvider = $state<'github' | 'bitbucket' | 'gitlab'>('github');
  let repoSearchQuery = $state('');
  let showConnectModal = $state(false);
  let providerToken = $state('');
  let providerUsername = $state('');
  let connecting = $state(false);

  // Active category filter
  let activeCategory = $state<'all' | 'web' | 'static' | 'worker'>('all');

  // Selected service preset
  let selectedPreset = $state<any>(null);

  // Form fields
  let name = $state('');
  let svcSlug = $state('');
  let slugEdited = false;
  let kind = $state('web');
  let imageRef = $state('node:20-alpine');
  let internalPort = $state(3000);
  let buildCommand = $state('npm install && npm run build');
  let startCommand = $state('npm start');
  let cronSchedule = $state('0 * * * *');
  let envVars = $state<Array<{ key: string; value: string }>>([
    { key: 'NODE_ENV', value: 'production' },
    { key: 'PORT', value: '3000' }
  ]);
  let autoDeploy = $state(true);

  let submitting = $state(false);
  let error = $state<string | null>(null);

  type ServicePreset = {
    id: string;
    title: string;
    description: string;
    category: 'web' | 'static' | 'worker';
    kind: 'web' | 'worker' | 'cron' | 'static';
    image: string;
    port: number;
    badge: string;
    iconColor: string;
    iconText: string;
    iconSvg?: string;
    defaultBuild?: string;
    defaultStart?: string;
  };

  const presets: ServicePreset[] = [
    // Web / Dynamic Runtimes
    {
      id: 'node',
      title: 'Node.js (Next.js / Express / Nest)',
      description: 'Fullstack JavaScript/TypeScript apps with Node 20 & npm/pnpm/yarn',
      category: 'web',
      kind: 'web',
      image: 'node:20-alpine',
      port: 3000,
      badge: 'JavaScript/TS',
      iconColor: '#22c55e',
      iconText: 'Node',
      iconSvg: '/icons/nodejs.svg',
      defaultBuild: 'npm install && npm run build',
      defaultStart: 'npm start'
    },
    {
      id: 'python',
      title: 'Python (FastAPI / Flask / Django)',
      description: 'WSGI / ASGI applications with requirements.txt or Pipfile',
      category: 'web',
      kind: 'web',
      image: 'python:3.11-slim',
      port: 5000,
      badge: 'Python 3.11',
      iconColor: '#3b82f6',
      iconText: 'Py',
      iconSvg: '/icons/python.svg',
      defaultBuild: 'pip install -r requirements.txt',
      defaultStart: 'gunicorn app:app --bind 0.0.0.0:5000 --workers 2'
    },
    {
      id: 'go',
      title: 'Go (Fiber / Gin / Chi / Echo)',
      description: 'Ultra-fast compiled Go binary web services',
      category: 'web',
      kind: 'web',
      image: 'golang:1.22-alpine',
      port: 8080,
      badge: 'Go 1.22',
      iconColor: '#06b6d4',
      iconText: 'Go',
      iconSvg: '/icons/golang.svg',
      defaultBuild: 'go build -o server .',
      defaultStart: './server'
    },
    {
      id: 'rust',
      title: 'Rust (Actix / Axum / Rocket)',
      description: 'High-performance Rust web services built with Cargo',
      category: 'web',
      kind: 'web',
      image: 'rust:1.77-alpine',
      port: 8080,
      badge: 'Rust Cargo',
      iconColor: '#f97316',
      iconText: 'Rust',
      iconSvg: '/icons/rust.svg',
      defaultBuild: 'cargo build --release',
      defaultStart: './target/release/app'
    },
    {
      id: 'java',
      title: 'Java (Spring Boot / Quarkus)',
      description: 'JVM application built with Maven or Gradle wrapper',
      category: 'web',
      kind: 'web',
      image: 'eclipse-temurin:21-jdk-alpine',
      port: 8080,
      badge: 'Java 21',
      iconColor: '#ef4444',
      iconText: 'Java',
      iconSvg: '/icons/java.svg',
      defaultBuild: './mvnw clean package -DskipTests',
      defaultStart: 'java -jar target/*.jar'
    },
    {
      id: 'php',
      title: 'PHP (Laravel / Symfony)',
      description: 'PHP 8.3 Apache runtime with composer package management',
      category: 'web',
      kind: 'web',
      image: 'php:8.3-apache',
      port: 80,
      badge: 'PHP 8.3',
      iconColor: '#8b5cf6',
      iconText: 'PHP',
      iconSvg: '/icons/php.svg',
      defaultBuild: 'composer install --no-dev --optimize-autoloader',
      defaultStart: 'apache2-foreground'
    },
    {
      id: 'ruby',
      title: 'Ruby on Rails / Sinatra',
      description: 'Rails web application with bundler and Puma server',
      category: 'web',
      kind: 'web',
      image: 'ruby:3.3-alpine',
      port: 3000,
      badge: 'Ruby 3.3',
      iconColor: '#e11d48',
      iconText: 'Ruby',
      iconSvg: '/icons/ruby.svg',
      defaultBuild: 'bundle install && rails assets:precompile',
      defaultStart: 'bundle exec puma -C config/puma.rb'
    },
    {
      id: 'dockerfile',
      title: 'Custom Dockerfile',
      description: 'Use the Dockerfile located in your repository directory',
      category: 'web',
      kind: 'web',
      image: 'custom',
      port: 80,
      badge: 'Docker',
      iconColor: '#0284c7',
      iconText: 'Docker',
      iconSvg: '/icons/docker.svg',
      defaultBuild: 'docker build -t app .',
      defaultStart: 'docker run app'
    },

    // Static Sites
    {
      id: 'static-spa',
      title: 'Static SPA (React / Vite / Vue / Svelte)',
      description: 'Single page applications compiled to static HTML/CSS/JS',
      category: 'static',
      kind: 'static',
      image: 'nginx:alpine',
      port: 80,
      badge: 'Static / SPA',
      iconColor: '#0ea5e9',
      iconText: 'SPA',
      iconSvg: '/icons/react.svg',
      defaultBuild: 'npm install && npm run build',
      defaultStart: 'nginx -g "daemon off;"'
    },
    {
      id: 'nginx',
      title: 'Nginx Static Server',
      description: 'High-performance static file serving and reverse proxying',
      category: 'static',
      kind: 'static',
      image: 'nginx:alpine',
      port: 80,
      badge: 'Web Server',
      iconColor: '#10b981',
      iconText: 'Nginx',
      iconSvg: '/icons/nginx.svg',
      defaultBuild: '',
      defaultStart: 'nginx -g "daemon off;"'
    },

    // Background Workers & Cron
    {
      id: 'worker',
      title: 'Background Worker',
      description: 'Continuous queue consumer, event listener, or background task',
      category: 'worker',
      kind: 'worker',
      image: 'node:20-alpine',
      port: 0,
      badge: 'Worker',
      iconColor: '#6366f1',
      iconText: 'Worker',
      defaultBuild: 'npm install',
      defaultStart: 'npm run worker'
    },
    {
      id: 'cron-job',
      title: 'Scheduled Cron Job',
      description: 'Periodic task executed on a recurring cron schedule',
      category: 'worker',
      kind: 'cron',
      image: 'alpine:latest',
      port: 0,
      badge: 'Cron',
      iconColor: '#d97706',
      iconText: 'Cron',
      defaultStart: 'echo "Running scheduled job"'
    }
  ];

  const filteredPresets = $derived(
    activeCategory === 'all' 
      ? presets 
      : presets.filter(p => p.category === activeCategory)
  );

  let oauthEnabledMap = $state<Record<string, boolean>>({});
  let currentProviderInfo = $state<any>(null);

  async function loadIntegrations() {
    try {
      const res = await fetch('/api/v1/integrations/git', { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        gitIntegrations = data.integrations ?? [];
        oauthEnabledMap = data.oauthEnabled ?? {};
      }
    } catch {}
  }

  async function loadProviderRepos(provider: string) {
    try {
      const res = await fetch(`/api/v1/integrations/git/${provider}/repos`, { credentials: 'include' });
      if (res.ok) {
        const data = await res.json();
        providerRepos = data.repos ?? [];
        currentProviderInfo = {
          connected: data.connected,
          username: data.username,
          avatar_url: data.avatar_url
        };
      }
    } catch {}
  }

  async function handleLinkGitProvider(e: Event) {
    e.preventDefault();
    if (!providerToken) return;
    connecting = true;
    try {
      const res = await fetch('/api/v1/integrations/git', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          provider: selectedProvider,
          username: providerUsername,
          token: providerToken
        })
      });
      if (res.ok) {
        showConnectModal = false;
        providerToken = '';
        providerUsername = '';
        await loadIntegrations();
        await loadProviderRepos(selectedProvider);
      } else {
        const d = await res.json().catch(() => ({}));
        alert(d.error || 'Failed to connect Git provider');
      }
    } catch (e: any) {
      alert('Error connecting: ' + e.message);
    } finally {
      connecting = false;
    }
  }

  async function disconnectProvider(provider: string) {
    if (!confirm(`Are you sure you want to disconnect ${provider}?`)) return;
    try {
      const res = await fetch(`/api/v1/integrations/git/${provider}`, {
        method: 'DELETE',
        credentials: 'include'
      });
      if (res.ok) {
        providerRepos = [];
        currentProviderInfo = null;
        await loadIntegrations();
        await loadProviderRepos(selectedProvider);
      }
    } catch {}
  }

  function authorizeGitOAuth(provider: string) {
    const returnTo = window.location.pathname;
    window.location.href = `/api/v1/integrations/git/${provider}/authorize?return_to=${encodeURIComponent(returnTo)}`;
  }

  onMount(async () => {
    try {
      const res = await fetch(`/api/v1/projects/${slug}`, { credentials: 'include' });
      if (res.ok) {
        project = await res.json();
      } else {
        goto('/workspaces');
      }
      await loadIntegrations();
      await loadProviderRepos('github');
      choosePreset(presets[0]);
    } catch (e) {
      goto('/workspaces');
    } finally {
      loading = false;
    }
  });

  let autoDetectDebounce: any = null;

  function handleRepoUrlChange(url: string) {
    gitRepoUrl = url.trim();
    if (!gitRepoUrl) return;

    try {
      const clean = gitRepoUrl.replace(/\.git$/, '').replace(/\/$/, '');
      const parts = clean.split('/');
      const repoBase = parts[parts.length - 1];
      if (repoBase && !slugEdited) {
        name = repoBase;
        svcSlug = repoBase.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)+/g, '');
      }
    } catch {}

    clearTimeout(autoDetectDebounce);
    autoDetectDebounce = setTimeout(() => {
      autoDetectRenderYaml();
    }, 600);
  }

  async function autoDetectRenderYaml() {
    if (!gitRepoUrl.trim()) return;
    parsingYaml = true;
    detectedBlueprint = null;
    detectedServices = [];
    detectedDatabases = [];
    yamlParsedInfo = null;
    try {
      const res = await fetch('/api/v1/services/parse-render-yaml', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          repoUrl: gitRepoUrl
        })
      });
      if (res.ok) {
        const data = await res.json();
        if (data.services && data.services.length > 0) {
          detectedServices = data.services;
          detectedBlueprint = data.services[0];
          detectedDatabases = data.databases || [];
        }
      }
    } catch {} finally {
      parsingYaml = false;
    }
  }

  function applyDetectedService(svc: any, idx: number) {
    selectedBlueprintIndex = idx;
    detectedBlueprint = svc;
    name = svc.name || name;
    svcSlug = svc.slug || svcSlug;
    kind = svc.kind || kind;
    rootDirectory = svc.root_dir || svc.rootDir || '.';

    const matchingPreset = presets.find(p => p.id === svc.preset) || 
                          presets.find(p => p.kind === svc.kind) || 
                          presets[0];
    choosePreset(matchingPreset);

    if (svc.build_command) buildCommand = svc.build_command;
    if (svc.start_command) startCommand = svc.start_command;
    if (svc.internal_port) internalPort = svc.internal_port;
    if (svc.cron_schedule) cronSchedule = svc.cron_schedule;
    if (svc.env_vars && Object.keys(svc.env_vars).length > 0) {
      envVars = Object.entries(svc.env_vars).map(([k, v]) => ({ key: k, value: String(v) }));
    }
    yamlParsedInfo = `✓ Configured "${svc.name}" (${svc.kind.toUpperCase()} • ${svc.env || svc.preset} in ${rootDirectory === '.' ? 'root' : '/' + rootDirectory} on port :${internalPort})`;
  }

  function requestDeployBlueprint() {
    const unfilled = getBlueprintUnfilledEnvVars().filter(x => x.isRequired && (!x.value || x.value.toLowerCase().startsWith('your_') || x.value.toLowerCase().startsWith('replace_') || x.value.toLowerCase() === 'changeme'));
    if (unfilled.length > 0) {
      pendingAction = 'blueprint';
      showEnvPromptModal = true;
      return;
    }
    deployEntireBlueprint();
  }

  async function deployEntireBlueprint() {
    if (detectedServices.length === 0 || !project || deployingBlueprint) return;
    deployingBlueprint = true;
    showEnvPromptModal = false;
    try {
      const projId = project.id || project.ID || slug;
      const res = await fetch(`/api/v1/projects/${encodeURIComponent(projId)}/blueprint/deploy`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          repoUrl: gitRepoUrl,
          branch: gitBranch,
          services: detectedServices,
          databases: detectedDatabases
        })
      });
      if (res.ok) {
        goto(`/projects/${slug}`);
      } else {
        const d = await res.json().catch(() => ({}));
        alert(d.error || 'Failed to deploy blueprint services');
      }
    } catch (e: any) {
      alert('Error deploying blueprint: ' + e.message);
    } finally {
      deployingBlueprint = false;
    }
  }

  function selectProviderRepo(repo: any) {
    gitRepoUrl = repo.url;
    gitBranch = repo.default_branch || 'main';
    name = repo.name;
    svcSlug = repo.name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)+/g, '');
    
    if (repo.language === 'Python') {
      choosePreset(presets.find(p => p.id === 'python') || presets[0]);
    } else if (repo.language === 'Go') {
      choosePreset(presets.find(p => p.id === 'go') || presets[0]);
    } else if (repo.language === 'TypeScript' || repo.language === 'JavaScript') {
      choosePreset(presets.find(p => p.id === 'node') || presets[0]);
    }

    autoDetectRenderYaml();
  }

  function choosePreset(p: ServicePreset) {
    selectedPreset = p;
    kind = p.kind;
    imageRef = p.image;
    internalPort = p.port || 80;
    buildCommand = p.defaultBuild || '';
    startCommand = p.defaultStart || '';
  }

  $effect(() => {
    if (!slugEdited && name) {
      svcSlug = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)+/g, '');
    }
  });

  function addEnv() {
    envVars = [...envVars, { key: '', value: '' }];
  }

  function removeEnv(index: number) {
    envVars = envVars.filter((_, i) => i !== index);
  }

  function handleSubmit(e: Event) {
    e.preventDefault();
    const unfilled = envVars.filter(x => x.key.trim() && (!x.value.trim() || x.value.toLowerCase().startsWith('your_') || x.value.toLowerCase().startsWith('replace_') || x.value.toLowerCase() === 'changeme'));
    if (unfilled.length > 0) {
      pendingAction = 'single';
      showEnvPromptModal = true;
      return;
    }
    executeSubmitSingle();
  }

  async function executeSubmitSingle() {
    if (!project) return;
    submitting = true;
    error = null;
    showEnvPromptModal = false;

    try {
      const envMap: Record<string, string> = {};
      for (const item of envVars) {
        if (item.key.trim()) {
          envMap[item.key.trim()] = item.value;
        }
      }

      const resourcePayload = {
        gitRepoUrl: (sourceType === 'git_public' || sourceType === 'git_provider') ? gitRepoUrl : '',
        gitBranch: gitBranch,
        rootDirectory: rootDirectory,
        sourceType: sourceType,
        presetId: selectedPreset?.id ?? 'custom',
        image: imageRef,
        buildCommand: buildCommand,
        startCommand: startCommand,
        cronSchedule: kind === 'cron' ? cronSchedule : '',
        env: envMap
      };

      const res = await fetch('/api/v1/services', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          projectId: project.id,
          name: name,
          slug: svcSlug,
          kind: kind,
          internalPort: kind === 'cron' ? 0 : internalPort,
          resourceJson: JSON.stringify(resourcePayload)
        })
      });

      if (!res.ok) {
        const d = await res.json();
        error = d.error || d.detail || 'Failed to create service';
        return;
      }

      const created = await res.json();
      const svcId = created.id || created.ID;

      // Trigger initial deployment automatically
      try {
        await fetch(`/api/v1/services/${svcId}/deploy`, { method: 'POST', credentials: 'include' });
      } catch {}

      goto(`/services/${svcId}/overview`);
    } catch (e: any) {
      error = e.message || 'An unexpected error occurred';
    } finally {
      submitting = false;
    }
  }
</script>

<svelte:head>
  <title>Deploy a Service - kloudsPanel</title>
</svelte:head>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={48} /></div>
    <p>Loading deployment studio…</p>
  </div>
{:else}
  <!-- Header -->
  <div class="page-header" style="margin-bottom: 2rem;">
    <div style="display: flex; align-items: center; gap: 1rem;">
      <button 
        class="btn btn-secondary" 
        onclick={() => goto(`/projects/${slug}`)} 
        style="padding: 0; width: 40px; height: 40px; min-height: 40px; border-radius: var(--radius-md); display: flex; align-items: center; justify-content: center; flex-shrink: 0;"
        aria-label="Back to Project"
      >
        <ArrowLeft size={18} />
      </button>
      <div>
        <h1 class="page-title">Deploy a Service</h1>
        <p class="page-subtitle">Clone public repositories, link Git accounts, or deploy container images with automatic render.yaml detection.</p>
      </div>
    </div>
  </div>

  <!-- Source Type Selection Tabs -->
  <div class="card" style="margin-bottom: 2rem; padding: 1.5rem; background: var(--color-surface); border: 1px solid var(--color-border);">
    <div style="margin-bottom: 1.25rem;">
      <div style="font-size: 1rem; font-weight: 700; margin-bottom: 0.25rem;">1. Choose Repository / Deployment Source</div>
      <p class="text-xs text-muted" style="margin:0;">Select how you want kloudsPanel to fetch or configure your application source code.</p>
    </div>

    <!-- Source Type Radios / Tabs -->
    <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 0.75rem; margin-bottom: 1.5rem;">
      <button 
        type="button" 
        class="card"
        style="
          cursor: pointer; 
          text-align: left; 
          padding: 1rem; 
          border: 2px solid {sourceType === 'git_public' ? 'var(--color-accent)' : 'var(--color-border)'}; 
          background: {sourceType === 'git_public' ? 'rgba(0,166,166,0.05)' : 'var(--color-surface)'};
        "
        onclick={() => sourceType = 'git_public'}
      >
        <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.4rem;">
          <div style="display: flex; align-items: center; gap: 0.5rem; font-weight: 600;">
            <Unlock size={18} style="color: var(--color-accent);" /> Public Git Repo
          </div>
          <span class="badge badge-running" style="font-size: 0.65rem;">Instant</span>
        </div>
        <p class="text-xs text-muted" style="margin: 0;">Clone any public GitHub, Bitbucket, or GitLab URL.</p>
      </button>

      <button 
        type="button" 
        class="card"
        style="
          cursor: pointer; 
          text-align: left; 
          padding: 1rem; 
          border: 2px solid {sourceType === 'git_provider' ? 'var(--color-accent)' : 'var(--color-border)'}; 
          background: {sourceType === 'git_provider' ? 'rgba(0,166,166,0.05)' : 'var(--color-surface)'};
        "
        onclick={() => sourceType = 'git_provider'}
      >
        <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.4rem;">
          <div style="display: flex; align-items: center; gap: 0.5rem; font-weight: 600;">
            <FolderGit2 size={18} style="color: #3b82f6;" /> Linked Git Accounts
          </div>
          <span class="badge" style="background:#e0f2fe; color:#0369a1; font-size: 0.65rem;">GitHub / Bitbucket</span>
        </div>
        <p class="text-xs text-muted" style="margin: 0;">Browse and select repositories from linked accounts.</p>
      </button>

      <button 
        type="button" 
        class="card"
        style="
          cursor: pointer; 
          text-align: left; 
          padding: 1rem; 
          border: 2px solid {sourceType === 'image' ? 'var(--color-accent)' : 'var(--color-border)'}; 
          background: {sourceType === 'image' ? 'rgba(0,166,166,0.05)' : 'var(--color-surface)'};
        "
        onclick={() => sourceType = 'image'}
      >
        <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.4rem;">
          <div style="display: flex; align-items: center; gap: 0.5rem; font-weight: 600;">
            <Server size={18} style="color: #8b5cf6;" /> Container Image
          </div>
          <span class="badge" style="background:#f3e8ff; color:#6b21a8; font-size: 0.65rem;">Registry</span>
        </div>
        <p class="text-xs text-muted" style="margin: 0;">Deploy container image directly from Docker Hub or GHCR.</p>
      </button>
    </div>

    <!-- Conditional Source Inputs -->
    {#if sourceType === 'git_public'}
      <div style="background: rgba(0,0,0,0.02); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border);">
        <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(180px, 1fr)); gap: 1rem; align-items: flex-end;">
          <div class="form-group" style="margin:0; min-width: 220px; flex: 2;">
            <label for="public-repo-url" class="form-label">Public Repository URL</label>
            <input 
              id="public-repo-url" 
              type="url" 
              class="form-input font-mono" 
              placeholder="https://github.com/username/repository" 
              bind:value={gitRepoUrl} 
              oninput={(e: any) => handleRepoUrlChange(e.target.value)}
              required 
            />
          </div>

          <div class="form-group" style="margin:0;">
            <label for="public-repo-branch" class="form-label">Branch</label>
            <input 
              id="public-repo-branch" 
              type="text" 
              class="form-input font-mono" 
              placeholder="main" 
              bind:value={gitBranch} 
              required 
            />
          </div>

          <div class="form-group" style="margin:0;">
            <label for="public-repo-root" class="form-label">Root Dir</label>
            <input 
              id="public-repo-root" 
              type="text" 
              class="form-input font-mono" 
              placeholder="." 
              bind:value={rootDirectory} 
              required 
            />
          </div>
        </div>

        <div style="display:flex; justify-content:space-between; align-items:center; margin-top:0.75rem; flex-wrap:wrap; gap:0.5rem;">
          <p class="text-xs text-muted" style="margin: 0;">
            Paste any public Git repository URL (e.g. <code>https://github.com/username/repository</code>).
          </p>
          <button 
            type="button" 
            class="btn btn-secondary" 
            style="font-size:0.75rem; padding:4px 10px; color:var(--color-accent);"
            onclick={() => autoDetectRenderYaml()}
            disabled={parsingYaml || !gitRepoUrl}
          >
            {#if parsingYaml}<Loader2 size={12} class="animate-spin" /> Checking Repo...{:else}<Sparkles size={12} /> Auto-Detect klouds.yaml / Blueprint in Repo{/if}
          </button>
        </div>

        {#if detectedServices.length > 0}
          {@const unfilledEnvList = getBlueprintUnfilledEnvVars()}
          <div style="background: rgba(16,185,129,0.06); border: 1.5px solid #10b981; border-radius: var(--radius-md); padding: 1rem 1.25rem; margin-top: 1rem;">
            <div style="display: flex; justify-content: space-between; align-items: flex-start; gap: 1rem; flex-wrap: wrap; margin-bottom: 0.85rem;">
              <div style="display: flex; align-items: center; gap: 0.75rem;">
                <Sparkles size={22} style="color: #059669; flex-shrink: 0;" />
                <div>
                  <div style="font-weight: 700; color: #065f46; font-size: 0.9375rem;">
                    klouds.yaml / Blueprint detected ({detectedServices.length} Service{detectedServices.length > 1 ? 's' : ''}{detectedDatabases.length > 0 ? `, ${detectedDatabases.length} Database` : ''})
                  </div>
                  <div class="text-xs" style="color: #047857; margin-top: 2px;">
                    This repository defines a multi-service stack. Review required environment variables below, or deploy all services together.
                  </div>
                </div>
              </div>

              {#if detectedServices.length > 1 || detectedDatabases.length > 0}
                <button 
                  type="button" 
                  class="btn btn-primary" 
                  style="font-size: 0.8125rem; padding: 7px 16px; background: #059669; border-color: #059669; display: flex; align-items: center; gap: 6px;"
                  onclick={requestDeployBlueprint}
                  disabled={deployingBlueprint}
                >
                  {#if deployingBlueprint}
                    <Loader2 size={14} class="animate-spin" /> Deploying All Services...
                  {:else}
                    <Rocket size={14} /> Deploy All {detectedServices.length + detectedDatabases.length} Stack Services
                  {/if}
                </button>
              {/if}
            </div>

            <!-- Required Environment Variables Setup Prompt Card -->
            {#if unfilledEnvList.length > 0}
              <div style="background: rgba(245,158,11,0.09); border: 1px solid rgba(245,158,11,0.4); border-radius: var(--radius-md); padding: 0.85rem 1rem; margin-bottom: 0.85rem;">
                <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.6rem; flex-wrap: wrap; gap: 0.5rem;">
                  <div style="display: flex; align-items: center; gap: 6px; font-weight: 700; font-size: 0.8125rem; color: #92400e;">
                    <AlertTriangle size={15} style="color: #d97706;" />
                    Setup Required Environment Variables ({unfilledEnvList.length})
                  </div>
                  <button
                    type="button"
                    class="btn btn-secondary"
                    style="font-size: 0.72rem; padding: 3px 8px; background: var(--color-surface); display: flex; align-items: center; gap: 4px; color: #92400e; border-color: rgba(245,158,11,0.4);"
                    onclick={autoFillAllSecrets}
                  >
                    <Wand2 size={12} /> Auto-Generate Secrets
                  </button>
                </div>

                <div style="display: flex; flex-direction: column; gap: 0.45rem;">
                  {#each unfilledEnvList as item}
                    <div style="display: grid; grid-template-columns: 140px 180px 1fr auto; gap: 0.5rem; align-items: center; background: var(--color-surface); padding: 4px 8px; border-radius: var(--radius-sm); border: 1px solid rgba(0,0,0,0.06);">
                      <span class="badge" style="background: rgba(0,166,166,0.12); color: var(--color-accent); font-size: 0.68rem; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
                        {item.svcName}
                      </span>
                      <span class="font-mono text-xs" style="font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;" title={item.key}>
                        {item.key}
                      </span>
                      <input
                        type="text"
                        class="form-input font-mono text-xs"
                        placeholder={item.isSecret ? "Enter or generate secret..." : "Enter value..."}
                        value={item.value}
                        oninput={(e: any) => updateDetectedEnv(item.svcIdx, item.key, e.target.value)}
                        style="padding: 3px 8px; height: 26px; border-color: {!item.value ? '#f59e0b' : 'var(--color-border)'};"
                      />
                      {#if item.isSecret}
                        <button
                          type="button"
                          class="btn btn-secondary"
                          style="padding: 2px 6px; min-height: 24px; font-size: 0.68rem;"
                          title="Generate random secret"
                          onclick={() => updateDetectedEnv(item.svcIdx, item.key, generateRandomSecret(32))}
                        >
                          <Wand2 size={11} />
                        </button>
                      {/if}
                    </div>
                  {/each}
                </div>
              </div>
            {/if}

            <!-- Discovered Items Grid -->
            <div style="display: flex; flex-direction: column; gap: 0.5rem; border-top: 1px solid rgba(16,185,129,0.2); padding-top: 0.75rem;">
              <div class="text-xs" style="font-weight: 700; color: #065f46; text-transform: uppercase; letter-spacing: 0.04em;">
                Declared Blueprint Services & Databases:
              </div>
              <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 0.5rem;">
                {#each detectedServices as s, idx}
                  <button
                    type="button"
                    class="btn btn-secondary"
                    style="
                      text-align: left; 
                      display: flex; 
                      align-items: center; 
                      justify-content: space-between; 
                      padding: 8px 12px; 
                      background: {selectedBlueprintIndex === idx ? 'rgba(5,150,105,0.15)' : 'var(--color-surface)'};
                      border-color: {selectedBlueprintIndex === idx ? '#059669' : 'var(--color-border)'};
                    "
                    onclick={() => applyDetectedService(s, idx)}
                  >
                    <div>
                      <div style="font-weight: 700; font-size: 0.8125rem; color: var(--color-ink);">{s.name}</div>
                      <div class="text-xs text-muted">
                        {s.kind} • {s.env || s.preset || 'custom'} {s.root_dir ? `• /${s.root_dir}` : ''} • :{s.internal_port}
                      </div>
                    </div>
                    <span class="badge" style="background: rgba(16,185,129,0.2); color: #065f46; font-size: 0.7rem;">
                      {selectedBlueprintIndex === idx ? 'Active' : 'Customize'}
                    </span>
                  </button>
                {/each}

                {#each detectedDatabases as db}
                  <div style="display: flex; align-items: center; gap: 0.5rem; padding: 8px 12px; border-radius: var(--radius-md); background: var(--color-surface); border: 1px solid var(--color-border);">
                    <Database size={16} style="color: #0369a1;" />
                    <div>
                      <div style="font-weight: 700; font-size: 0.8125rem; color: var(--color-ink);">{db.name}</div>
                      <div class="text-xs text-muted">Managed {db.engine || 'postgres'} database</div>
                    </div>
                  </div>
                {/each}
              </div>
            </div>
          </div>
        {/if}

        {#if yamlParsedInfo}
          <div style="background:#d1fae5; border:1px solid #6ee7b7; color:#065f46; border-radius:var(--radius-md); padding:0.6rem 0.85rem; font-size:0.8125rem; margin-top:0.75rem; display: flex; align-items: center; gap: 0.5rem;">
            <Check size={16} /> {yamlParsedInfo}
          </div>
        {/if}
      </div>

    {:else if sourceType === 'git_provider'}
      <div style="background: rgba(0,0,0,0.02); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border);">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; flex-wrap: wrap; gap: 0.5rem;">
          <div style="display: flex; gap: 0.5rem;">
            <button 
              type="button" 
              class="btn btn-secondary" 
              style="padding: 4px 12px; font-size: 0.8125rem; font-weight: {selectedProvider === 'github' ? '700' : '500'}; background: {selectedProvider === 'github' ? 'var(--color-surface)' : 'transparent'};"
              onclick={() => { selectedProvider = 'github'; loadProviderRepos('github'); }}
            >
              GitHub
            </button>
            <button 
              type="button" 
              class="btn btn-secondary" 
              style="padding: 4px 12px; font-size: 0.8125rem; font-weight: {selectedProvider === 'bitbucket' ? '700' : '500'}; background: {selectedProvider === 'bitbucket' ? 'var(--color-surface)' : 'transparent'};"
              onclick={() => { selectedProvider = 'bitbucket'; loadProviderRepos('bitbucket'); }}
            >
              Bitbucket
            </button>
            <button 
              type="button" 
              class="btn btn-secondary" 
              style="padding: 4px 12px; font-size: 0.8125rem; font-weight: {selectedProvider === 'gitlab' ? '700' : '500'}; background: {selectedProvider === 'gitlab' ? 'var(--color-surface)' : 'transparent'};"
              onclick={() => { selectedProvider = 'gitlab'; loadProviderRepos('gitlab'); }}
            >
              GitLab
            </button>
          </div>

          <div style="display: flex; align-items: center; gap: 0.5rem;">
            {#if currentProviderInfo?.connected}
              <div style="display: flex; align-items: center; gap: 0.5rem; background: var(--color-surface); padding: 3px 10px; border-radius: var(--radius-md); border: 1px solid var(--color-border);">
                {#if currentProviderInfo.avatar_url}
                  <img src={currentProviderInfo.avatar_url} alt="" style="width: 20px; height: 20px; border-radius: 50%;" />
                {/if}
                <span style="font-size: 0.8125rem; font-weight: 600;">
                  @{currentProviderInfo.username || 'connected'}
                </span>
                <button 
                  type="button" 
                  class="btn btn-secondary" 
                  style="font-size: 0.75rem; padding: 2px 6px; min-height: 24px; color: var(--color-danger); border: none;"
                  onclick={() => disconnectProvider(selectedProvider)}
                  title="Disconnect account"
                >
                  <X size={13} />
                </button>
              </div>
            {:else if oauthEnabledMap[selectedProvider]}
              <button 
                type="button" 
                class="btn btn-primary" 
                style="font-size: 0.8125rem; padding: 4px 14px; background: {selectedProvider === 'github' ? '#24292f' : selectedProvider === 'gitlab' ? '#fc6d26' : '#0052cc'}; border-color: transparent; display: flex; align-items: center; gap: 6px;"
                onclick={() => authorizeGitOAuth(selectedProvider)}
              >
                <FolderGit2 size={14} /> Authorize with {selectedProvider.charAt(0).toUpperCase() + selectedProvider.slice(1)}
              </button>
            {:else}
              <button 
                type="button" 
                class="btn btn-primary" 
                style="font-size: 0.8125rem; padding: 4px 12px;"
                onclick={() => showConnectModal = true}
              >
                <Plus size={14} /> Link Account
              </button>
            {/if}
          </div>
        </div>

        {#if providerRepos.length === 0}
          <div style="text-align: center; padding: 1.5rem 0;">
            <p class="text-sm text-muted" style="margin-bottom: 0.75rem;">No linked {selectedProvider} repositories found.</p>
            {#if oauthEnabledMap[selectedProvider]}
              <button 
                type="button" 
                class="btn btn-primary" 
                style="font-size: 0.8125rem; background: {selectedProvider === 'github' ? '#24292f' : selectedProvider === 'gitlab' ? '#fc6d26' : '#0052cc'}; border-color: transparent; display: inline-flex; align-items: center; gap: 6px;" 
                onclick={() => authorizeGitOAuth(selectedProvider)}
              >
                <FolderGit2 size={14} /> Connect {selectedProvider.charAt(0).toUpperCase() + selectedProvider.slice(1)} (1-Click OAuth)
              </button>
            {:else}
              <button type="button" class="btn btn-secondary" style="font-size: 0.8125rem;" onclick={() => showConnectModal = true}>
                Connect {selectedProvider} Account
              </button>
            {/if}
          </div>
        {:else}
          <div class="form-group" style="margin-bottom: 0.75rem;">
            <div style="position: relative;">
              <input 
                type="text" 
                class="form-input" 
                placeholder="Search your repositories..." 
                bind:value={repoSearchQuery} 
                style="padding-left: 2rem;" 
              />
              <Search size={14} style="position: absolute; left: 0.75rem; top: 50%; transform: translateY(-50%); color: var(--color-ink-muted);" />
            </div>
          </div>

          <div style="display: flex; flex-direction: column; gap: 0.5rem; max-height: 220px; overflow-y: auto;">
            {#each providerRepos.filter(r => r.name.toLowerCase().includes(repoSearchQuery.toLowerCase())) as repo}
              <div 
                style="
                  display: flex; 
                  align-items: center; 
                  justify-content: space-between; 
                  padding: 0.6rem 0.85rem; 
                  border-radius: var(--radius-md); 
                  border: 1px solid {gitRepoUrl === repo.url ? 'var(--color-accent)' : 'var(--color-border)'}; 
                  background: {gitRepoUrl === repo.url ? 'rgba(0,166,166,0.08)' : 'var(--color-surface)'};
                "
              >
                <div>
                  <div style="font-weight: 600; font-size: 0.875rem;">{repo.full_name || repo.name}</div>
                  <div class="text-xs text-muted" style="display: flex; gap: 0.75rem; margin-top: 2px;">
                    <span>Branch: <span class="font-mono">{repo.default_branch || 'main'}</span></span>
                    {#if repo.language}<span>{repo.language}</span>{/if}
                  </div>
                </div>
                <button 
                  type="button" 
                  class="btn {gitRepoUrl === repo.url ? 'btn-primary' : 'btn-secondary'}" 
                  style="font-size: 0.75rem; padding: 4px 10px;"
                  onclick={() => selectProviderRepo(repo)}
                >
                  {gitRepoUrl === repo.url ? 'Selected' : 'Select'}
                </button>
              </div>
            {/each}
          </div>
        {/if}

        {#if detectedServices.length > 0}
          <div style="background: rgba(16,185,129,0.06); border: 1.5px solid #10b981; border-radius: var(--radius-md); padding: 1rem 1.25rem; margin-top: 1rem;">
            <div style="display: flex; justify-content: space-between; align-items: flex-start; gap: 1rem; flex-wrap: wrap; margin-bottom: 0.85rem;">
              <div style="display: flex; align-items: center; gap: 0.75rem;">
                <Sparkles size={22} style="color: #059669; flex-shrink: 0;" />
                <div>
                  <div style="font-weight: 700; color: #065f46; font-size: 0.9375rem;">
                    render.yaml / Blueprint detected ({detectedServices.length} Service{detectedServices.length > 1 ? 's' : ''}{detectedDatabases.length > 0 ? `, ${detectedDatabases.length} Database` : ''})
                  </div>
                  <div class="text-xs" style="color: #047857; margin-top: 2px;">
                    This repository defines a multi-service stack. Deploy all services together or customize individually.
                  </div>
                </div>
              </div>

              {#if detectedServices.length > 1 || detectedDatabases.length > 0}
                <button 
                  type="button" 
                  class="btn btn-primary" 
                  style="font-size: 0.8125rem; padding: 7px 16px; background: #059669; border-color: #059669; display: flex; align-items: center; gap: 6px;"
                  onclick={deployEntireBlueprint}
                  disabled={deployingBlueprint}
                >
                  {#if deployingBlueprint}
                    <Loader2 size={14} class="animate-spin" /> Deploying All Services...
                  {:else}
                    <Rocket size={14} /> Deploy All {detectedServices.length + detectedDatabases.length} Stack Services
                  {/if}
                </button>
              {/if}
            </div>

            <!-- Discovered Items Grid -->
            <div style="display: flex; flex-direction: column; gap: 0.5rem; border-top: 1px solid rgba(16,185,129,0.2); padding-top: 0.75rem;">
              <div class="text-xs" style="font-weight: 700; color: #065f46; text-transform: uppercase; letter-spacing: 0.04em;">
                Declared Blueprint Services & Databases:
              </div>
              <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 0.5rem;">
                {#each detectedServices as s, idx}
                  <button
                    type="button"
                    class="btn btn-secondary"
                    style="
                      text-align: left; 
                      display: flex; 
                      align-items: center; 
                      justify-content: space-between; 
                      padding: 8px 12px; 
                      background: {selectedBlueprintIndex === idx ? 'rgba(5,150,105,0.15)' : 'var(--color-surface)'};
                      border-color: {selectedBlueprintIndex === idx ? '#059669' : 'var(--color-border)'};
                    "
                    onclick={() => applyDetectedService(s, idx)}
                  >
                    <div>
                      <div style="font-weight: 700; font-size: 0.8125rem; color: var(--color-ink);">{s.name}</div>
                      <div class="text-xs text-muted">
                        {s.kind} • {s.env || s.preset || 'custom'} {s.root_dir ? `• /${s.root_dir}` : ''} • :{s.internal_port}
                      </div>
                    </div>
                    <span class="badge" style="background: rgba(16,185,129,0.2); color: #065f46; font-size: 0.7rem;">
                      {selectedBlueprintIndex === idx ? 'Active' : 'Customize'}
                    </span>
                  </button>
                {/each}

                {#each detectedDatabases as db}
                  <div style="display: flex; align-items: center; gap: 0.5rem; padding: 8px 12px; border-radius: var(--radius-md); background: var(--color-surface); border: 1px solid var(--color-border);">
                    <Database size={16} style="color: #0369a1;" />
                    <div>
                      <div style="font-weight: 700; font-size: 0.8125rem; color: var(--color-ink);">{db.name}</div>
                      <div class="text-xs text-muted">Managed {db.engine || 'postgres'} database</div>
                    </div>
                  </div>
                {/each}
              </div>
            </div>
          </div>
        {/if}

        {#if yamlParsedInfo}
          <div style="background:#d1fae5; border:1px solid #6ee7b7; color:#065f46; border-radius:var(--radius-md); padding:0.6rem 0.85rem; font-size:0.8125rem; margin-top:0.75rem; display: flex; align-items: center; gap: 0.5rem;">
            <Check size={16} /> {yamlParsedInfo}
          </div>
        {/if}
      </div>

    {:else if sourceType === 'image'}
      <div style="background: rgba(0,0,0,0.02); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border);">
        <div class="form-group" style="margin-bottom: 0.85rem;">
          <label for="direct-image-ref" class="form-label">Docker Container Image Tag</label>
          <input 
            id="direct-image-ref" 
            type="text" 
            class="form-input font-mono" 
            placeholder="e.g. nginx:alpine, redis:7.2-alpine, mongo:7.0, or ghcr.io/org/image:tag" 
            bind:value={imageRef} 
            required 
          />
          <p class="text-xs text-muted" style="margin-top: 0.4rem;">
            Pulled directly from Docker Hub, GHCR, or your specified registry without building source code.
          </p>
        </div>

        <div>
          <div class="text-xs text-muted" style="margin-bottom: 0.4rem; font-weight: 600;">Popular Docker Images:</div>
          <div style="display: flex; flex-wrap: wrap; gap: 0.4rem;">
            {#each [
              { name: 'Nginx', img: 'nginx:alpine', port: 80, kind: 'web' },
              { name: 'Redis', img: 'redis:7.2-alpine', port: 6379, kind: 'web' },
              { name: 'PostgreSQL', img: 'postgres:16-alpine', port: 5432, kind: 'web' },
              { name: 'MySQL', img: 'mysql:8.0', port: 3306, kind: 'web' },
              { name: 'MongoDB', img: 'mongo:7.0', port: 27017, kind: 'web' },
              { name: 'ClickHouse', img: 'clickhouse/clickhouse-server:24.3-alpine', port: 8123, kind: 'web' },
              { name: 'Metabase', img: 'metabase/metabase:latest', port: 3000, kind: 'web' },
              { name: 'RabbitMQ', img: 'rabbitmq:3-management', port: 15672, kind: 'web' }
            ] as pop}
              <button
                type="button"
                class="btn btn-secondary"
                style="padding: 3px 8px; font-size: 0.75rem;"
                onclick={() => {
                  imageRef = pop.img;
                  if (!name) name = pop.name.toLowerCase();
                  internalPort = pop.port;
                  kind = pop.kind as any;
                }}
              >
                {pop.name} ({pop.img})
              </button>
            {/each}
          </div>
        </div>
      </div>
    {/if}
  </div>

  <!-- Runtime / Framework Presets -->
  <div class="card" style="margin-bottom: 2rem; padding: 1.5rem; background: var(--color-surface); border: 1px solid var(--color-border);">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.25rem; flex-wrap: wrap; gap: 1rem;">
      <div>
        <div style="font-size: 1rem; font-weight: 700; margin-bottom: 0.25rem;">2. Select Runtime / Framework Preset</div>
        <p class="text-xs text-muted" style="margin:0;">Choose the execution environment for building and running your service.</p>
      </div>

      <!-- Filter Buttons -->
      <div style="display: flex; gap: 0.35rem; background: rgba(0,0,0,0.03); padding: 3px; border-radius: var(--radius-md);">
        <button 
          type="button" 
          class="btn" 
          style="padding: 4px 10px; font-size: 0.75rem; font-weight: {activeCategory === 'all' ? '700' : '500'}; background: {activeCategory === 'all' ? 'var(--color-surface)' : 'transparent'}; box-shadow: {activeCategory === 'all' ? 'var(--shadow-sm)' : 'none'};"
          onclick={() => activeCategory = 'all'}
        >
          All ({presets.length})
        </button>
        <button 
          type="button" 
          class="btn" 
          style="padding: 4px 10px; font-size: 0.75rem; font-weight: {activeCategory === 'web' ? '700' : '500'}; background: {activeCategory === 'web' ? 'var(--color-surface)' : 'transparent'}; box-shadow: {activeCategory === 'web' ? 'var(--shadow-sm)' : 'none'};"
          onclick={() => activeCategory = 'web'}
        >
          Web Apps & APIs
        </button>
        <button 
          type="button" 
          class="btn" 
          style="padding: 4px 10px; font-size: 0.75rem; font-weight: {activeCategory === 'static' ? '700' : '500'}; background: {activeCategory === 'static' ? 'var(--color-surface)' : 'transparent'}; box-shadow: {activeCategory === 'static' ? 'var(--shadow-sm)' : 'none'};"
          onclick={() => activeCategory = 'static'}
        >
          Static Sites
        </button>
        <button 
          type="button" 
          class="btn" 
          style="padding: 4px 10px; font-size: 0.75rem; font-weight: {activeCategory === 'worker' ? '700' : '500'}; background: {activeCategory === 'worker' ? 'var(--color-surface)' : 'transparent'}; box-shadow: {activeCategory === 'worker' ? 'var(--shadow-sm)' : 'none'};"
          onclick={() => activeCategory = 'worker'}
        >
          Background & Cron
        </button>
      </div>
    </div>

    <!-- Presets Grid -->
    <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1rem;">
      {#each filteredPresets as preset}
        <button
          type="button"
          class="card"
          style="
            cursor: pointer; 
            text-align: left; 
            padding: 1.15rem; 
            border: 2px solid {selectedPreset?.id === preset.id ? 'var(--color-accent)' : 'var(--color-border)'}; 
            background: {selectedPreset?.id === preset.id ? 'rgba(0,166,166,0.06)' : 'var(--color-surface)'};
            border-radius: var(--radius-lg);
            display: flex; 
            flex-direction: column; 
            justify-content: space-between;
            transition: all 0.15s ease;
          "
          onclick={() => choosePreset(preset)}
        >
          <div>
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.75rem;">
              <div style="display: flex; align-items: center; gap: 0.6rem;">
                <div style="width: 34px; height: 34px; border-radius: var(--radius-sm); background: rgba(0,0,0,0.03); border: 1px solid var(--color-border); display: flex; align-items: center; justify-content: center; padding: 4px;">
                  <FrameworkIcon name={preset.id} size={20} />
                </div>
                <span class="badge" style="background: rgba(0,0,0,0.04); font-size: 0.7rem; font-weight: 600;">{preset.badge}</span>
              </div>
              {#if selectedPreset?.id === preset.id}
                <span class="badge badge-running" style="padding: 2px 8px; font-size: 0.7rem;"><Check size={11} /> Selected</span>
              {/if}
            </div>
            <div style="font-weight: 700; font-size: 0.9375rem; color: var(--color-ink); margin-bottom: 0.35rem;">{preset.title}</div>
            <p class="text-xs text-muted" style="margin: 0; line-height: 1.45;">{preset.description}</p>
          </div>
        </button>
      {/each}
    </div>
  </div>

  <!-- Form Configuration -->
  <form onsubmit={handleSubmit}>
    <div class="card" style="margin-bottom: 2rem; padding: 1.5rem; background: var(--color-surface); border: 1px solid var(--color-border);">
      <div style="margin-bottom: 1.25rem;">
        <div style="font-size: 1rem; font-weight: 700; margin-bottom: 0.25rem;">3. Service Configuration & Commands</div>
        <p class="text-xs text-muted" style="margin:0;">Fine-tune build commands, runtime execution commands, and environment variables.</p>
      </div>

      {#if error}
        <div style="background:#fee2e2; border:1px solid #fca5a5; color:#991b1b; border-radius:var(--radius-md); padding:0.75rem 1rem; font-size:0.875rem; margin-bottom:1.5rem">
          {error}
        </div>
      {/if}

      <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 1.25rem; margin-bottom: 1.25rem;">
        <div class="form-group" style="margin:0;">
          <label for="svc-name-input" class="form-label">Service Name</label>
          <input id="svc-name-input" type="text" class="form-input" placeholder="e.g. my-api-service" bind:value={name} required />
        </div>

        <div class="form-group" style="margin:0;">
          <label for="svc-slug-input" class="form-label">URL Slug</label>
          <div style="display: flex; align-items: center;">
            <input 
              id="svc-slug-input" 
              type="text" 
              class="form-input font-mono" 
              placeholder="my-api-service" 
              bind:value={svcSlug} 
              oninput={() => slugEdited = true} 
              required 
            />
          </div>
          <p class="text-xs text-muted" style="margin-top:0.25rem;">
            Preview URL: <strong>https://{svcSlug || 'app'}.{typeof window !== 'undefined' ? window.location.hostname : 'yourdomain.com'}</strong>
          </p>
        </div>
      </div>

      <!-- Build and Start Commands -->
      <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(260px, 1fr)); gap: 1.25rem; margin-bottom: 1.25rem;">
        <div class="form-group" style="margin:0;">
          <label for="build-cmd-input" class="form-label">Build Command</label>
          <input 
            id="build-cmd-input" 
            type="text" 
            class="form-input font-mono text-sm" 
            placeholder="pip install -r requirements.txt" 
            bind:value={buildCommand} 
          />
        </div>

        <div class="form-group" style="margin:0;">
          <label for="start-cmd-input" class="form-label">Start / Run Command</label>
          <input 
            id="start-cmd-input" 
            type="text" 
            class="form-input font-mono text-sm" 
            placeholder="gunicorn app:app --bind 0.0.0.0:5000" 
            bind:value={startCommand} 
          />
        </div>
      </div>

      <!-- Port and Kind -->
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1.25rem; margin-bottom: 1.5rem;">
        <div class="form-group" style="margin:0;">
          <label for="port-input" class="form-label">Internal Port</label>
          <input 
            id="port-input" 
            type="number" 
            class="form-input font-mono" 
            placeholder="5000" 
            bind:value={internalPort} 
            required={kind === 'web' || kind === 'static'} 
          />
          <p class="text-xs text-muted" style="margin-top:0.25rem;">
            Port your app listens on inside the container (e.g. 5000 for Flask, 3000 for Node/Express, 8080 for Go).
          </p>
        </div>

        <div class="form-group" style="margin:0;">
          <label for="image-ref-input" class="form-label">Runtime Base Image</label>
          <input 
            id="image-ref-input" 
            type="text" 
            class="form-input font-mono" 
            placeholder="python:3.11-slim" 
            bind:value={imageRef} 
            required 
          />
        </div>
      </div>

      <!-- Environment Variables -->
      <div style="border-top: 1px solid var(--color-border); padding-top: 1.25rem; margin-bottom: 1.5rem;">
        <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.5rem;">
          <div style="font-weight: 600; font-size: 0.9375rem;">Environment Variables</div>
          <button type="button" class="btn btn-secondary" style="padding: 2px 10px; min-height: 28px; font-size: 0.75rem;" onclick={addEnv}>
            <Plus size={12} /> Add Variable
          </button>
        </div>

        {#if envVars.length === 0}
          <p class="text-xs text-muted" style="margin: 0.5rem 0;">No environment variables configured. Click "+ Add Variable" to add one.</p>
        {:else}
          <div style="display: flex; flex-direction: column; gap: 0.5rem;">
            {#each envVars as env, i}
              <div style="display: flex; gap: 0.5rem; align-items: center;">
                <input 
                  type="text" 
                  class="form-input font-mono text-xs" 
                  placeholder="VARIABLE_NAME" 
                  bind:value={env.key} 
                  style="flex: 1;" 
                />
                <span class="text-muted">=</span>
                <input 
                  type="text" 
                  class="form-input font-mono text-xs" 
                  placeholder="value" 
                  bind:value={env.value} 
                  style="flex: 2; border-color: {!env.value && env.key ? '#f59e0b' : 'var(--color-border)'};" 
                />
                <button
                  type="button"
                  class="btn btn-secondary"
                  style="padding: 2px 8px; min-height: 28px; font-size: 0.72rem; display: flex; align-items: center; gap: 4px;"
                  title="Generate a random secure 32-character secret"
                  onclick={() => env.value = generateRandomSecret(32)}
                >
                  <Wand2 size={12} /> Secret
                </button>
                <button 
                  type="button" 
                  class="btn btn-secondary" 
                  style="padding: 4px; color: var(--color-error); min-height: 28px;" 
                  onclick={() => removeEnv(i)}
                  aria-label="Remove variable"
                >
                  <X size={14} />
                </button>
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <!-- Submit button -->
      <div style="display: flex; justify-content: flex-end; gap: 0.75rem;">
        <button 
          type="button" 
          class="btn btn-secondary" 
          onclick={() => goto(`/projects/${slug}`)} 
          disabled={submitting}
        >
          Cancel
        </button>
        <button 
          type="submit" 
          class="btn btn-primary" 
          disabled={submitting || !name || !svcSlug}
          style="padding: 0.625rem 1.5rem;"
        >
          {#if submitting}
            <Loader2 size={16} class="animate-spin" /> Provisioning & Deploying...
          {:else}
            <Rocket size={16} /> Deploy {name || 'Service'}
          {/if}
        </button>
      </div>
    </div>
  </form>
{/if}

<!-- Modal: Link Git Provider -->
{#if showConnectModal}
  <div style="position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 999; padding: 1rem;">
    <div class="card" style="width: 100%; max-width: 480px; box-shadow: var(--shadow-lg); background: var(--color-surface);">
      <div class="card-header" style="display: flex; justify-content: space-between; align-items: center;">
        <h3 style="margin:0; text-transform: capitalize;">Link {selectedProvider} Account</h3>
        <button class="btn btn-secondary" style="padding: 4px; min-height: 28px;" onclick={() => showConnectModal = false} aria-label="Close">
          <X size={16} />
        </button>
      </div>

      <div style="padding: 1rem 0 0 0;">
        {#if oauthEnabledMap[selectedProvider]}
          <div style="margin-bottom: 1.25rem;">
            <button 
              type="button" 
              class="btn btn-primary" 
              style="width: 100%; padding: 0.65rem; background: {selectedProvider === 'github' ? '#24292f' : selectedProvider === 'gitlab' ? '#fc6d26' : '#0052cc'}; border-color: transparent; display: flex; align-items: center; justify-content: center; gap: 8px; font-weight: 600;" 
              onclick={() => authorizeGitOAuth(selectedProvider)}
            >
              <FolderGit2 size={16} /> Authorize Directly with {selectedProvider.charAt(0).toUpperCase() + selectedProvider.slice(1)} (1-Click)
            </button>
            <div style="text-align: center; margin: 1.15rem 0 0.85rem 0; position: relative;">
              <span style="background: var(--color-surface); padding: 0 8px; color: var(--color-ink-muted); font-size: 0.75rem; position: relative; z-index: 1;">OR USE PERSONAL ACCESS TOKEN / PASSWORD</span>
              <div style="position: absolute; left: 0; top: 50%; width: 100%; border-top: 1px solid var(--color-border); z-index: 0;"></div>
            </div>
          </div>
        {/if}

        <form onsubmit={handleLinkGitProvider}>
          <div class="form-group">
            <label for="modal-git-user" class="form-label">Username / Organization (optional for GitHub/GitLab)</label>
            <input 
              id="modal-git-user" 
              type="text" 
              class="form-input" 
              placeholder="e.g. your-username" 
              bind:value={providerUsername} 
            />
          </div>

          <div class="form-group">
            <label for="modal-git-token" class="form-label">
              {selectedProvider === 'bitbucket' ? 'Bitbucket App Password' : 'Personal Access Token (PAT)'}
            </label>
            <input 
              id="modal-git-token" 
              type="password" 
              class="form-input font-mono" 
              placeholder={selectedProvider === 'github' ? 'ghp_...' : selectedProvider === 'gitlab' ? 'glpat-...' : 'App password'} 
              bind:value={providerToken} 
              required 
            />
          </div>

          <div style="display: flex; justify-content: flex-end; gap: 0.5rem; margin-top: 1.5rem;">
            <button type="button" class="btn btn-secondary" onclick={() => showConnectModal = false}>
              Cancel
            </button>
            <button type="submit" class="btn btn-primary" disabled={connecting || !providerToken}>
              {#if connecting}<Loader2 size={14} class="animate-spin" /> Connecting...{:else}Save & Connect {selectedProvider}{/if}
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
{/if}

<!-- Modal: Required Environment Variables Setup Prompt -->
{#if showEnvPromptModal}
  <div style="position: fixed; inset: 0; background: rgba(0,0,0,0.6); backdrop-filter: blur(2px); display: flex; align-items: center; justify-content: center; z-index: 1000; padding: 1rem;">
    <div class="card" style="width: 100%; max-width: 640px; max-height: 85vh; display: flex; flex-direction: column; box-shadow: var(--shadow-lg); background: var(--color-surface); border: 2px solid var(--color-accent); border-radius: var(--radius-lg);">
      <div class="card-header" style="display: flex; justify-content: space-between; align-items: center; border-bottom: 1px solid var(--color-border); padding: 1rem 1.25rem;">
        <div style="display: flex; align-items: center; gap: 0.5rem;">
          <KeyRound size={20} style="color: var(--color-accent);" />
          <h3 style="margin:0; font-size: 1.0625rem;">Setup Environment Variables</h3>
        </div>
        <button class="btn btn-secondary" style="padding: 4px; min-height: 28px;" onclick={() => showEnvPromptModal = false} aria-label="Close">
          <X size={16} />
        </button>
      </div>

      <div style="padding: 1.25rem; overflow-y: auto; flex: 1;">
        <div style="background: rgba(245,158,11,0.08); border: 1px solid #f59e0b; border-radius: var(--radius-md); padding: 0.75rem 1rem; margin-bottom: 1.25rem; display: flex; align-items: flex-start; gap: 0.75rem;">
          <AlertTriangle size={18} style="color: #d97706; flex-shrink: 0; margin-top: 2px;" />
          <div class="text-xs" style="color: #92400e; line-height: 1.5;">
            <strong>Setup Required:</strong> Some environment variables are empty or use placeholders. Please fill in your values or click <strong>Auto-Generate Secrets</strong> before starting the deployment.
          </div>
        </div>

        <div style="display: flex; justify-content: flex-end; margin-bottom: 0.75rem;">
          <button
            type="button"
            class="btn btn-secondary"
            style="font-size: 0.75rem; padding: 4px 10px; display: flex; align-items: center; gap: 5px;"
            onclick={autoFillAllSecrets}
          >
            <Wand2 size={13} /> Auto-Generate All Secrets (JWT, Keys, Tokens)
          </button>
        </div>

        {#if pendingAction === 'blueprint'}
          <div style="display: flex; flex-direction: column; gap: 1rem;">
            {#each detectedServices as svc, sIdx}
              {#if svc.env_vars && Object.keys(svc.env_vars).length > 0}
                <div style="border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: 0.85rem; background: rgba(0,0,0,0.015);">
                  <div style="font-weight: 700; font-size: 0.8125rem; margin-bottom: 0.6rem; color: var(--color-ink); display: flex; align-items: center; gap: 6px;">
                    <span class="badge" style="background: rgba(0,166,166,0.15); color: var(--color-accent); font-size: 0.7rem;">{svc.name}</span>
                    <span class="text-muted text-xs">Environment Variables</span>
                  </div>
                  <div style="display: flex; flex-direction: column; gap: 0.5rem;">
                    {#each Object.entries(svc.env_vars) as [key, val]}
                      <div style="display: grid; grid-template-columns: 160px 1fr auto; gap: 0.5rem; align-items: center;">
                        <div class="font-mono text-xs" style="font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;" title={key}>
                          {key}
                        </div>
                        <input
                          type="text"
                          class="form-input font-mono text-xs"
                          placeholder="Enter value..."
                          value={val}
                          oninput={(e: any) => updateDetectedEnv(sIdx, key, e.target.value)}
                          style="border-color: {!val ? '#f59e0b' : 'var(--color-border)'};"
                        />
                        <button
                          type="button"
                          class="btn btn-secondary"
                          style="padding: 4px 8px; min-height: 28px; font-size: 0.7rem;"
                          title="Generate random 32-char secret"
                          onclick={() => updateDetectedEnv(sIdx, key, generateRandomSecret(32))}
                        >
                          <Wand2 size={12} />
                        </button>
                      </div>
                    {/each}
                  </div>
                </div>
              {/if}
            {/each}
          </div>
        {:else}
          <div style="display: flex; flex-direction: column; gap: 0.5rem;">
            {#each envVars as env, i}
              <div style="display: grid; grid-template-columns: 160px 1fr auto; gap: 0.5rem; align-items: center;">
                <div class="font-mono text-xs" style="font-weight: 600; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;" title={env.key}>
                  {env.key || `Variable #${i+1}`}
                </div>
                <input
                  type="text"
                  class="form-input font-mono text-xs"
                  placeholder="Enter value..."
                  bind:value={env.value}
                  style="border-color: {!env.value && env.key ? '#f59e0b' : 'var(--color-border)'};"
                />
                <button
                  type="button"
                  class="btn btn-secondary"
                  style="padding: 4px 8px; min-height: 28px; font-size: 0.7rem;"
                  title="Generate random 32-char secret"
                  onclick={() => env.value = generateRandomSecret(32)}
                >
                  <Wand2 size={12} />
                </button>
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <div style="display: flex; justify-content: space-between; align-items: center; border-top: 1px solid var(--color-border); padding: 1rem 1.25rem;">
        <button type="button" class="btn btn-secondary" onclick={() => showEnvPromptModal = false}>
          Cancel
        </button>
        <button
          type="button"
          class="btn btn-primary"
          style="padding: 0.6rem 1.5rem;"
          onclick={() => {
            if (pendingAction === 'blueprint') {
              showEnvPromptModal = false;
              deployEntireBlueprint();
            } else {
              executeSubmitSingle();
            }
          }}
        >
          <Rocket size={16} /> Confirm & Deploy Now
        </button>
      </div>
    </div>
  </div>
{/if}
