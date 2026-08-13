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
    Database
  } from 'lucide-svelte';

  const { slug } = $derived($page.params);
  let project = $state<any>(null);
  let loading = $state(true);

  // Source Type: 'git_public' | 'git_provider' | 'render_yaml' | 'image'
  let sourceType = $state<'git_public' | 'git_provider' | 'render_yaml' | 'image'>('git_public');

  // Public Git source fields
  let gitRepoUrl = $state('https://github.com/vedantjja/vtopc');
  let gitBranch = $state('main');
  let rootDirectory = $state('.');

  // render.yaml parser state
  let renderYamlContent = $state(`services:
  - type: web
    name: vtopc
    env: python
    buildCommand: pip install -r requirements.txt
    startCommand: gunicorn app:app --bind 0.0.0.0:5000 --workers 2
    envVars:
      - key: PYTHONUNBUFFERED
        value: "1"
      - key: PORT
        value: "5000"
    autoDeploy: true`);
  let parsingYaml = $state(false);
  let yamlParsedInfo = $state<string | null>(null);
  let detectedDatabases = $state<any[]>([]);

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
  let name = $state('vtopc');
  let svcSlug = $state('vtopc');
  let slugEdited = false;
  let kind = $state('web');
  let imageRef = $state('python:3.11-slim');
  let internalPort = $state(5000);
  let buildCommand = $state('pip install -r requirements.txt');
  let startCommand = $state('gunicorn app:app --bind 0.0.0.0:5000 --workers 2');
  let cronSchedule = $state('0 * * * *');
  let envVars = $state<Array<{ key: string; value: string }>>([
    { key: 'PYTHONUNBUFFERED', value: '1' },
    { key: 'PORT', value: '5000' }
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
    defaultBuild?: string;
    defaultStart?: string;
  };

  const presets: ServicePreset[] = [
    // Web / Dynamic Runtimes
    {
      id: 'python',
      title: 'Python (Flask / FastAPI / Django)',
      description: 'WSGI / ASGI apps with requirements.txt or Pipfile',
      category: 'web',
      kind: 'web',
      image: 'python:3.11-slim',
      port: 5000,
      badge: 'Dynamic Web',
      defaultBuild: 'pip install -r requirements.txt',
      defaultStart: 'gunicorn app:app --bind 0.0.0.0:5000 --workers 2'
    },
    {
      id: 'node',
      title: 'Node.js (Express / Nest / Next / Remix)',
      description: 'Fullstack or API server powered by Node.js 20 & npm/pnpm/yarn',
      category: 'web',
      kind: 'web',
      image: 'node:20-alpine',
      port: 3000,
      badge: 'Dynamic Web',
      defaultBuild: 'npm install && npm run build',
      defaultStart: 'npm start'
    },
    {
      id: 'go',
      title: 'Go (Gin / Fiber / Echo / Chi)',
      description: 'Ultra-fast compiled Go binary web services',
      category: 'web',
      kind: 'web',
      image: 'golang:1.22-alpine',
      port: 8080,
      badge: 'Dynamic Web',
      defaultBuild: 'go build -o server .',
      defaultStart: './server'
    },
    {
      id: 'java',
      title: 'Java / Spring Boot / Quarkus',
      description: 'JVM application with Maven or Gradle wrapper',
      category: 'web',
      kind: 'web',
      image: 'eclipse-temurin:21-jdk-alpine',
      port: 8080,
      badge: 'Dynamic Web',
      defaultBuild: './mvnw clean package -DskipTests',
      defaultStart: 'java -jar target/*.jar'
    },
    {
      id: 'rust',
      title: 'Rust (Actix / Axum / Rocket)',
      description: 'High-performance Rust web services built with Cargo',
      category: 'web',
      kind: 'web',
      image: 'rust:1.77-alpine',
      port: 8080,
      badge: 'Dynamic Web',
      defaultBuild: 'cargo build --release',
      defaultStart: './target/release/app'
    },
    {
      id: 'php',
      title: 'PHP (Laravel / Symfony / WordPress)',
      description: 'PHP 8.3 Apache runtime with composer package management',
      category: 'web',
      kind: 'web',
      image: 'php:8.3-apache',
      port: 80,
      badge: 'Dynamic Web',
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
      badge: 'Dynamic Web',
      defaultBuild: 'bundle install && rails assets:precompile',
      defaultStart: 'bundle exec puma -C config/puma.rb'
    },
    {
      id: 'dockerfile',
      title: 'Repo Dockerfile (Custom)',
      description: 'Use the Dockerfile located at the root of the repository',
      category: 'web',
      kind: 'web',
      image: 'custom',
      port: 80,
      badge: 'Custom Container',
      defaultBuild: 'docker build -t app .',
      defaultStart: 'docker run app'
    },

    // Static Sites
    {
      id: 'static-spa',
      title: 'Static SPA / Jamstack',
      description: 'Vite, React, Vue, SvelteKit (static adapter), HTML/CSS/JS',
      category: 'static',
      kind: 'static',
      image: 'nginx:alpine',
      port: 80,
      badge: 'Static Site',
      defaultBuild: 'npm install && npm run build',
      defaultStart: 'nginx -g "daemon off;"'
    },
    {
      id: 'nginx',
      title: 'Nginx Web Server',
      description: 'High performance static file serving and reverse proxying',
      category: 'static',
      kind: 'static',
      image: 'nginx:alpine',
      port: 80,
      badge: 'Static Site',
      defaultBuild: '',
      defaultStart: 'nginx -g "daemon off;"'
    },

    // Background Workers & Cron
    {
      id: 'worker',
      title: 'Background Worker',
      description: 'Continuous queue consumer, event listener, or polling script',
      category: 'worker',
      kind: 'worker',
      image: 'node:20-alpine',
      port: 0,
      badge: 'Background Worker',
      defaultBuild: 'npm install',
      defaultStart: 'npm run worker'
    },
    {
      id: 'cron-job',
      title: 'Scheduled Cron Job',
      description: 'Periodic background task executed on a recurring cron schedule',
      category: 'worker',
      kind: 'cron',
      image: 'alpine:latest',
      port: 0,
      badge: 'Scheduled Task',
      defaultStart: 'echo "Running scheduled job"'
    }
  ];

  const filteredPresets = $derived(
    activeCategory === 'all' 
      ? presets 
      : presets.filter(p => p.category === activeCategory)
  );

  async function loadIntegrations() {
    try {
      const res = await fetch('/api/v1/integrations/git', { credentials: 'include' });
      if (res.ok) {
        gitIntegrations = (await res.json()).integrations ?? [];
      }
    } catch {}
  }

  async function loadProviderRepos(provider: string) {
    try {
      const res = await fetch(`/api/v1/integrations/git/${provider}/repos`, { credentials: 'include' });
      if (res.ok) {
        providerRepos = (await res.json()).repos ?? [];
      }
    } catch {}
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
  }

  async function parseRenderYaml(customContent?: string) {
    parsingYaml = true;
    yamlParsedInfo = null;
    error = null;
    try {
      const res = await fetch('/api/v1/services/parse-render-yaml', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          content: customContent || renderYamlContent,
          repoUrl: gitRepoUrl
        })
      });
      if (!res.ok) {
        const d = await res.json();
        error = d.error || 'Failed to parse render.yaml';
        return;
      }
      const data = await res.json();
      if (data.services && data.services.length > 0) {
        const svc = data.services[0];
        name = svc.name || name;
        svcSlug = svc.slug || svcSlug;
        kind = svc.kind || kind;
        
        // Find matching preset
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
        detectedDatabases = data.databases || [];
        yamlParsedInfo = `✓ Applied render.yaml for "${svc.name}" (${svc.kind.toUpperCase()} • ${svc.env || svc.preset} on port :${internalPort})`;
      }
    } catch (e: any) {
      error = 'Error parsing render.yaml: ' + e.message;
    } finally {
      parsingYaml = false;
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

  async function handleLinkGitProvider(e: Event) {
    e.preventDefault();
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
        await loadIntegrations();
        await loadProviderRepos(selectedProvider);
      }
    } finally {
      connecting = false;
    }
  }

  async function handleSubmit(e: Event) {
    e.preventDefault();
    if (!project) return;
    submitting = true;
    error = null;

    try {
      const envMap: Record<string, string> = {};
      for (const item of envVars) {
        if (item.key.trim()) {
          envMap[item.key.trim()] = item.value;
        }
      }

      const resourcePayload = {
        gitRepoUrl: (sourceType === 'git_public' || sourceType === 'git_provider' || sourceType === 'render_yaml') ? gitRepoUrl : '',
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
  <title>Deploy a Service — kloudsPanel</title>
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
        <p class="page-subtitle">Clone public repositories, link Git accounts, import render.yaml blueprints, or run container images.</p>
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
    <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 0.75rem; margin-bottom: 1.5rem;">
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
          border: 2px solid {sourceType === 'render_yaml' ? 'var(--color-accent)' : 'var(--color-border)'}; 
          background: {sourceType === 'render_yaml' ? 'rgba(0,166,166,0.05)' : 'var(--color-surface)'};
        "
        onclick={() => sourceType = 'render_yaml'}
      >
        <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.4rem;">
          <div style="display: flex; align-items: center; gap: 0.5rem; font-weight: 600;">
            <FileCode size={18} style="color: #ec4899;" /> render.yaml Blueprint
          </div>
          <span class="badge" style="background:#fdf2f8; color:#be185d; font-size: 0.65rem;">Parser</span>
        </div>
        <p class="text-xs text-muted" style="margin: 0;">Paste or parse Render Blueprint specification directly.</p>
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
        <div style="display: grid; grid-template-columns: 1fr 180px 120px; gap: 1rem; align-items: flex-end;">
          <div class="form-group" style="margin:0;">
            <label for="public-repo-url" class="form-label">Public Repository URL</label>
            <input 
              id="public-repo-url" 
              type="url" 
              class="form-input font-mono" 
              placeholder="https://github.com/vedantjja/vtopc" 
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
            💡 Try pasting any public repo URL (e.g. <code>https://github.com/vedantjja/vtopc</code>).
          </p>
          <button 
            type="button" 
            class="btn btn-secondary" 
            style="font-size:0.75rem; padding:4px 10px; color:var(--color-accent);"
            onclick={() => parseRenderYaml()}
            disabled={parsingYaml || !gitRepoUrl}
          >
            {#if parsingYaml}<Loader2 size={12} class="animate-spin" /> Auto-Detecting...{:else}<Sparkles size={12} /> Auto-Detect render.yaml in Repo{/if}
          </button>
        </div>

        {#if yamlParsedInfo}
          <div style="background:#d1fae5; border:1px solid #6ee7b7; color:#065f46; border-radius:var(--radius-md); padding:0.6rem 0.85rem; font-size:0.8125rem; margin-top:0.75rem;">
            {yamlParsedInfo}
          </div>
        {/if}
      </div>

    {:else if sourceType === 'render_yaml'}
      <div style="background: rgba(0,0,0,0.02); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border);">
        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:0.75rem;">
          <div>
            <div style="font-weight:600; font-size:0.9375rem;">Render Blueprint Specification (render.yaml)</div>
            <p class="text-xs text-muted" style="margin:0;">Paste your render.yaml file or blueprint definition below.</p>
          </div>
          <button 
            type="button" 
            class="btn btn-primary" 
            style="font-size:0.8125rem; padding:6px 14px;"
            onclick={() => parseRenderYaml()}
            disabled={parsingYaml || !renderYamlContent.trim()}
          >
            {#if parsingYaml}<Loader2 size={14} class="animate-spin" /> Parsing...{:else}<Sparkles size={14} /> Parse & Apply Blueprint{/if}
          </button>
        </div>

        <textarea 
          class="form-input font-mono text-xs" 
          rows={10} 
          style="width:100%; resize:vertical; background:var(--color-surface); line-height:1.5;"
          bind:value={renderYamlContent}
          placeholder={`services:\n  - type: web\n    name: my-app\n    env: python\n    buildCommand: pip install -r requirements.txt\n    startCommand: gunicorn app:app --bind 0.0.0.0:5000\n    envVars:\n      - key: PORT\n        value: 5000`}
        ></textarea>

        {#if yamlParsedInfo}
          <div style="background:#d1fae5; border:1px solid #6ee7b7; color:#065f46; border-radius:var(--radius-md); padding:0.6rem 0.85rem; font-size:0.8125rem; margin-top:0.75rem;">
            {yamlParsedInfo}
          </div>
        {/if}

        {#if detectedDatabases.length > 0}
          <div style="background:#eff6ff; border:1px solid #bfdbfe; color:#1e40af; border-radius:var(--radius-md); padding:0.6rem 0.85rem; font-size:0.8125rem; margin-top:0.5rem; display:flex; align-items:center; gap:0.5rem;">
            <Database size={16} /> Blueprint includes {detectedDatabases.length} database definition(s): {detectedDatabases.map(d => d.name).join(', ')}. You can provision them under Databases.
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

          <button 
            type="button" 
            class="btn btn-primary" 
            style="font-size: 0.8125rem; padding: 4px 12px;"
            onclick={() => showConnectModal = true}
          >
            <Plus size={14} /> Link Account
          </button>
        </div>

        {#if providerRepos.length === 0}
          <div style="text-align: center; padding: 1.5rem 0;">
            <p class="text-sm text-muted" style="margin-bottom: 0.75rem;">No linked {selectedProvider} repositories found.</p>
            <button type="button" class="btn btn-secondary" style="font-size: 0.8125rem;" onclick={() => showConnectModal = true}>
              Connect {selectedProvider} Account
            </button>
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
      </div>

    {:else if sourceType === 'image'}
      <div style="background: rgba(0,0,0,0.02); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border);">
        <div class="form-group" style="margin:0;">
          <label for="direct-image-ref" class="form-label">Docker Container Image Tag</label>
          <input 
            id="direct-image-ref" 
            type="text" 
            class="form-input font-mono" 
            placeholder="e.g. nginx:alpine, redis:7-alpine, or ghcr.io/org/image:tag" 
            bind:value={imageRef} 
            required 
          />
          <p class="text-xs text-muted" style="margin-top: 0.4rem;">
            Pulled directly from Docker Hub or specified registry without building source code.
          </p>
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
    <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 0.85rem;">
      {#each filteredPresets as preset}
        <button
          type="button"
          class="card"
          style="
            cursor: pointer; 
            text-align: left; 
            padding: 1rem; 
            border: 2px solid {selectedPreset?.id === preset.id ? 'var(--color-accent)' : 'var(--color-border)'}; 
            background: {selectedPreset?.id === preset.id ? 'rgba(0,166,166,0.06)' : 'var(--color-surface)'};
            display: flex; 
            flex-direction: column; 
            justify-content: space-between;
          "
          onclick={() => choosePreset(preset)}
        >
          <div>
            <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.4rem;">
              <span class="badge" style="background: rgba(0,0,0,0.05); font-size: 0.65rem;">{preset.badge}</span>
              {#if selectedPreset?.id === preset.id}
                <span class="badge badge-running" style="padding: 2px 6px;"><Check size={10} /> Selected</span>
              {/if}
            </div>
            <div style="font-weight: 700; font-size: 0.9375rem; margin-bottom: 0.25rem;">{preset.title}</div>
            <p class="text-xs text-muted" style="margin: 0; line-height: 1.4;">{preset.description}</p>
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

      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1.25rem; margin-bottom: 1.25rem;">
        <div class="form-group" style="margin:0;">
          <label for="svc-name-input" class="form-label">Service Name</label>
          <input id="svc-name-input" type="text" class="form-input" placeholder="e.g. vtopc" bind:value={name} required />
        </div>

        <div class="form-group" style="margin:0;">
          <label for="svc-slug-input" class="form-label">URL Slug</label>
          <div style="display: flex; align-items: center;">
            <input 
              id="svc-slug-input" 
              type="text" 
              class="form-input font-mono" 
              placeholder="vtopc" 
              bind:value={svcSlug} 
              oninput={() => slugEdited = true} 
              required 
            />
          </div>
          <p class="text-xs text-muted" style="margin-top:0.25rem;">
            Preview URL: <strong>https://{svcSlug || 'app'}.klouds.online</strong>
          </p>
        </div>
      </div>

      <!-- Build and Start Commands -->
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1.25rem; margin-bottom: 1.25rem;">
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
                  style="flex: 2;" 
                />
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

      <form onsubmit={handleLinkGitProvider}>
        <div class="form-group">
          <label for="modal-git-user" class="form-label">Username / Organization</label>
          <input 
            id="modal-git-user" 
            type="text" 
            class="form-input" 
            placeholder="e.g. vedantjja" 
            bind:value={providerUsername} 
            required 
          />
        </div>

        <div class="form-group">
          <label for="modal-git-token" class="form-label">Personal Access Token / App Password</label>
          <input 
            id="modal-git-token" 
            type="password" 
            class="form-input font-mono" 
            placeholder="ghp_... or Bitbucket app password" 
            bind:value={providerToken} 
            required 
          />
        </div>

        <div style="display: flex; justify-content: flex-end; gap: 0.5rem; margin-top: 1.5rem;">
          <button type="button" class="btn btn-secondary" onclick={() => showConnectModal = false}>
            Cancel
          </button>
          <button type="submit" class="btn btn-primary" disabled={connecting || !providerUsername || !providerToken}>
            {#if connecting}<Loader2 size={14} class="animate-spin" /> Connecting...{:else}Connect {selectedProvider}{/if}
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}
