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
    Search
  } from 'lucide-svelte';

  const { slug } = $derived($page.params);
  let project = $state<any>(null);
  let loading = $state(true);

  // Source Type: 'git_public' | 'git_provider' | 'image'
  let sourceType = $state<'git_public' | 'git_provider' | 'image'>('git_public');

  // Public Git source fields
  let gitRepoUrl = $state('https://github.com/vedantjja/vtopc');
  let gitBranch = $state('main');
  let rootDirectory = $state('.');

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
      title: 'Node.js (Express / Nest / Next)',
      description: 'JavaScript & TypeScript web services, SSR, and APIs',
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
      title: 'Go (Golang)',
      description: 'Gin, Fiber, Echo, and compiled Go HTTP backends',
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
      title: 'Java / Spring Boot',
      description: 'Spring Boot, Quarkus, Maven, and Gradle applications',
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
      title: 'Rust',
      description: 'Actix-web, Axum, Rocket, and high-performance services',
      category: 'web',
      kind: 'web',
      image: 'rust:1.77-alpine',
      port: 8080,
      badge: 'Dynamic Web',
      defaultBuild: 'cargo build --release',
      defaultStart: './target/release/server'
    },
    {
      id: 'php',
      title: 'PHP / Laravel',
      description: 'Laravel, Symfony, WordPress, and Apache/PHP applications',
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
      title: 'Ruby on Rails',
      description: 'Ruby on Rails, Sinatra, and Puma web applications',
      category: 'web',
      kind: 'web',
      image: 'ruby:3.3-alpine',
      port: 3000,
      badge: 'Dynamic Web',
      defaultBuild: 'bundle install',
      defaultStart: 'bundle exec rails server -b 0.0.0.0'
    },
    {
      id: 'dockerfile',
      title: 'Repo Dockerfile',
      description: 'Build and run directly using the Dockerfile inside your repository',
      category: 'web',
      kind: 'web',
      image: 'custom',
      port: 80,
      badge: 'Dockerfile'
    },
    {
      id: 'nginx',
      title: 'Nginx Web Server',
      description: 'High-performance HTTP server, static asset delivery, and reverse proxy',
      category: 'web',
      kind: 'web',
      image: 'nginx:alpine',
      port: 80,
      badge: 'Web Server'
    },
    {
      id: 'custom-docker',
      title: 'Custom Docker Image',
      description: 'Deploy any public or private image from Docker Hub, GHCR, or ECR',
      category: 'web',
      kind: 'web',
      image: 'nginx:alpine',
      port: 80,
      badge: 'Container'
    },

    // Static Sites
    {
      id: 'static-spa',
      title: 'React / Vite / Vue / Svelte Static',
      description: 'Single-page applications built to static files (dist/build)',
      category: 'static',
      kind: 'static',
      image: 'nginx:alpine',
      port: 80,
      badge: 'Static SPA',
      defaultBuild: 'npm install && npm run build'
    },
    {
      id: 'static-jamstack',
      title: 'Astro / Jamstack / Hugo',
      description: 'Static site generators and documentation frameworks',
      category: 'static',
      kind: 'static',
      image: 'nginx:alpine',
      port: 80,
      badge: 'Jamstack',
      defaultBuild: 'npm run build'
    },

    // Background Workers & Cron
    {
      id: 'worker-script',
      title: 'Background Worker',
      description: 'Continuous queue consumer, telemetry processor, or message worker',
      category: 'worker',
      kind: 'worker',
      image: 'node:20-alpine',
      port: 0,
      badge: 'Background Runner',
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
      // Default to Python preset for vtopc
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

    // Extract repo name from URL (e.g. https://github.com/vedantjja/vtopc -> vtopc)
    const cleanUrl = gitRepoUrl.replace(/\/+$/, '').replace(/\.git$/, '');
    const parts = cleanUrl.split('/');
    if (parts.length > 0) {
      const candidateName = parts[parts.length - 1];
      if (candidateName && candidateName !== 'github.com' && candidateName !== 'bitbucket.org') {
        name = candidateName;
        svcSlug = candidateName.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)+/g, '');
      }
    }
  }

  function selectProviderRepo(repo: any) {
    gitRepoUrl = repo.url;
    gitBranch = repo.default_branch || 'main';
    name = repo.name;
    svcSlug = repo.name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)+/g, '');
    
    // Auto-detect preset by repo language
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

  async function saveIntegration(e: Event) {
    e.preventDefault();
    connecting = true;
    try {
      const res = await fetch('/api/v1/integrations/git', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          provider: selectedProvider,
          token: providerToken,
          username: providerUsername || 'vedantjja'
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

  async function handleDeploy(e: Event) {
    e.preventDefault();
    if (!name || !svcSlug) return;
    
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
        sourceType,
        gitRepoUrl: sourceType !== 'image' ? gitRepoUrl : undefined,
        gitBranch: sourceType !== 'image' ? gitBranch : undefined,
        rootDirectory: sourceType !== 'image' ? rootDirectory : undefined,
        preset: selectedPreset?.id,
        image: imageRef,
        buildCommand,
        startCommand,
        cronSchedule: kind === 'cron' ? cronSchedule : undefined,
        autoDeploy,
        env: envMap
      };

      const res = await fetch('/api/v1/services', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        credentials: 'include',
        body: JSON.stringify({
          projectId: slug,
          name,
          slug: svcSlug,
          kind,
          internalPort: kind === 'worker' || kind === 'cron' ? null : Number(internalPort) || 80,
          resourceJson: JSON.stringify(resourcePayload)
        })
      });

      if (!res.ok) {
        const d = await res.json().catch(() => ({}));
        throw new Error(d.detail || d.message || 'Failed to create service');
      }

      const svc = await res.json();
      const svcId = svc.id || svc.ID;

      // Trigger initial deployment
      await fetch(`/api/v1/services/${svcId}/deploy`, { method: 'POST', credentials: 'include' });

      // Navigate to the newly created service overview
      goto(`/services/${svcId}/overview`);
    } catch (e: any) {
      error = e.message;
    } finally {
      submitting = false;
    }
  }

  const filteredProviderRepos = $derived(
    repoSearchQuery 
      ? providerRepos.filter(r => r.name.toLowerCase().includes(repoSearchQuery.toLowerCase()) || r.full_name?.toLowerCase().includes(repoSearchQuery.toLowerCase()))
      : providerRepos
  );
</script>

<svelte:head>
  <title>Deploy New Service — kloudsPanel</title>
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
        <p class="page-subtitle">Clone any public repository (GitHub, Bitbucket, GitLab), link your accounts, or run a container image.</p>
      </div>
    </div>
  </div>

  <!-- Source Type Selection Tabs -->
  <div class="card" style="margin-bottom: 2rem; padding: 1.5rem; background: var(--color-surface); border: 1px solid var(--color-border);">
    <div style="margin-bottom: 1.25rem;">
      <label class="form-label" style="font-size: 1rem; font-weight: 700; margin-bottom: 0.25rem;">1. Choose Repository / Deployment Source</label>
      <p class="text-xs text-muted" style="margin:0;">Select how you want kloudsPanel to fetch your application source code.</p>
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
            <Unlock size={18} style="color: var(--color-accent);" /> Public Git Repository
          </div>
          <span class="badge badge-running" style="font-size: 0.65rem;">Instant</span>
        </div>
        <p class="text-xs text-muted" style="margin: 0;">Clone any public GitHub, Bitbucket, or GitLab URL without login.</p>
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
        <p class="text-xs text-muted" style="margin: 0;">Browse and select repositories from your personal or organization accounts.</p>
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
        <p class="text-xs text-muted" style="margin: 0;">Deploy prebuilt container image directly from Docker Hub or GHCR.</p>
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
        <p class="text-xs text-muted" style="margin: 0.75rem 0 0 0;">
          💡 Try pasting any public repo URL (e.g. <code>https://github.com/vedantjja/vtopc</code>, <code>https://bitbucket.org/user/repo</code>, or GitLab).
        </p>
      </div>

    {:else if sourceType === 'git_provider'}
      <div style="background: rgba(0,0,0,0.02); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border);">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; flex-wrap: wrap; gap: 0.5rem;">
          <div style="display: flex; gap: 0.5rem;">
            <button 
              type="button" 
              class="btn" 
              class:btn-primary={selectedProvider === 'github'} 
              class:btn-secondary={selectedProvider !== 'github'}
              style="padding: 4px 12px; font-size: 0.8125rem;"
              onclick={() => { selectedProvider = 'github'; loadProviderRepos('github'); }}
            >
              GitHub
            </button>
            <button 
              type="button" 
              class="btn" 
              class:btn-primary={selectedProvider === 'bitbucket'} 
              class:btn-secondary={selectedProvider !== 'bitbucket'}
              style="padding: 4px 12px; font-size: 0.8125rem;"
              onclick={() => { selectedProvider = 'bitbucket'; loadProviderRepos('bitbucket'); }}
            >
              Bitbucket
            </button>
            <button 
              type="button" 
              class="btn" 
              class:btn-primary={selectedProvider === 'gitlab'} 
              class:btn-secondary={selectedProvider !== 'gitlab'}
              style="padding: 4px 12px; font-size: 0.8125rem;"
              onclick={() => { selectedProvider = 'gitlab'; loadProviderRepos('gitlab'); }}
            >
              GitLab
            </button>
          </div>

          <button type="button" class="btn btn-secondary" style="font-size: 0.8125rem; padding: 4px 12px;" onclick={() => showConnectModal = true}>
            <Settings size={14} /> Link / Configure {selectedProvider}
          </button>
        </div>

        <!-- Search Bar -->
        <div style="margin-bottom: 1rem;">
          <input 
            type="text" 
            class="form-input" 
            placeholder="Search your repositories..." 
            bind:value={repoSearchQuery} 
          />
        </div>

        <!-- Repos List -->
        <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 0.75rem; max-height: 240px; overflow-y: auto;">
          {#each filteredProviderRepos as repo}
            {@const isSelected = gitRepoUrl === repo.url}
            <button 
              type="button" 
              class="card" 
              style="
                padding: 0.75rem 1rem; 
                text-align: left; 
                cursor: pointer; 
                border: 1px solid {isSelected ? 'var(--color-accent)' : 'var(--color-border)'}; 
                background: {isSelected ? 'rgba(0,166,166,0.08)' : 'var(--color-surface)'};
              "
              onclick={() => selectProviderRepo(repo)}
            >
              <div style="font-weight: 600; font-size: 0.875rem; color: var(--color-ink);">{repo.full_name || repo.name}</div>
              <div style="display: flex; gap: 0.5rem; align-items: center; margin-top: 0.25rem;">
                <span class="badge" style="background:#f1f5f9; color:#475569; font-size: 0.65rem;">{repo.language || 'Code'}</span>
                <span class="text-xs text-muted font-mono">{repo.default_branch || 'main'}</span>
              </div>
            </button>
          {/each}
        </div>
      </div>

    {:else if sourceType === 'image'}
      <div style="background: rgba(0,0,0,0.02); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border);">
        <div class="form-group" style="margin:0;">
          <label for="docker-image-tag" class="form-label">Docker Container Image Tag</label>
          <input 
            id="docker-image-tag" 
            type="text" 
            class="form-input font-mono" 
            placeholder="e.g. nginx:alpine, ghcr.io/yourorg/app:latest" 
            bind:value={imageRef} 
            required 
          />
        </div>
      </div>
    {/if}
  </div>

  <!-- Runtime / Preset Selection -->
  <div style="margin-bottom: 2rem;">
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; flex-wrap: wrap; gap: 0.5rem;">
      <div>
        <label class="form-label" style="font-size: 1rem; font-weight: 700; margin-bottom: 0.25rem;">2. Select Runtime / Framework Preset</label>
        <p class="text-xs text-muted" style="margin:0;">Choose the execution environment for building and running your service.</p>
      </div>

      <!-- Filter Pills -->
      <div style="display: flex; gap: 0.4rem; overflow-x: auto;">
        <button 
          type="button"
          class="btn" 
          class:btn-primary={activeCategory === 'all'} 
          class:btn-secondary={activeCategory !== 'all'}
          style="padding: 4px 12px; font-size: 0.8125rem; border-radius: 999px;"
          onclick={() => activeCategory = 'all'}
        >
          All ({presets.length})
        </button>
        <button 
          type="button"
          class="btn" 
          class:btn-primary={activeCategory === 'web'} 
          class:btn-secondary={activeCategory !== 'web'}
          style="padding: 4px 12px; font-size: 0.8125rem; border-radius: 999px;"
          onclick={() => activeCategory = 'web'}
        >
          Web / APIs
        </button>
        <button 
          type="button"
          class="btn" 
          class:btn-primary={activeCategory === 'static'} 
          class:btn-secondary={activeCategory !== 'static'}
          style="padding: 4px 12px; font-size: 0.8125rem; border-radius: 999px;"
          onclick={() => activeCategory = 'static'}
        >
          Static Sites
        </button>
        <button 
          type="button"
          class="btn" 
          class:btn-primary={activeCategory === 'worker'} 
          class:btn-secondary={activeCategory !== 'worker'}
          style="padding: 4px 12px; font-size: 0.8125rem; border-radius: 999px;"
          onclick={() => activeCategory = 'worker'}
        >
          Workers & Cron
        </button>
      </div>
    </div>

    <!-- Catalog Grid of Cards -->
    <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 0.85rem;">
      {#each filteredPresets as preset}
        {@const isSelected = selectedPreset?.id === preset.id}
        <button 
          type="button" 
          class="card"
          style="
            cursor: pointer; 
            text-align: left; 
            padding: 1.125rem; 
            border: 2px solid {isSelected ? 'var(--color-accent)' : 'var(--color-border)'}; 
            background: {isSelected ? 'rgba(0,166,166,0.05)' : 'var(--color-surface)'};
            display: flex;
            flex-direction: column;
            justify-content: space-between;
            transition: all var(--transition-fast);
          "
          onclick={() => choosePreset(preset)}
        >
          <div>
            <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.5rem;">
              <span class="badge" style="background:#f1f5f9; color:#334155; font-size: 0.65rem;">
                {preset.badge}
              </span>
              {#if isSelected}
                <span style="display: inline-flex; align-items: center; justify-content: center; width: 18px; height: 18px; border-radius: 50%; background: var(--color-accent); color: #fff;">
                  <Check size={12} strokeWidth={3} />
                </span>
              {/if}
            </div>
            <h3 style="margin: 0 0 0.25rem 0; font-size: 1rem; font-weight: 700; color: var(--color-ink);">
              {preset.title}
            </h3>
            <p class="text-xs text-muted" style="margin: 0; line-height: 1.4;">
              {preset.description}
            </p>
          </div>

          <div style="margin-top: 1rem; padding-top: 0.5rem; border-top: 1px solid var(--color-border); display: flex; justify-content: space-between; align-items: center;">
            <span class="font-mono text-xs text-muted truncate" style="max-width: 150px;">
              {preset.image}
            </span>
            {#if preset.port}
              <span class="text-xs font-mono" style="color: var(--color-accent); font-weight: 600;">:{preset.port}</span>
            {:else}
              <span class="text-xs text-muted">Daemon</span>
            {/if}
          </div>
        </button>
      {/each}
    </div>
  </div>

  <!-- Fine-Tuning Configuration Form -->
  <div class="card" style="max-width: 760px; border: 1px solid var(--color-accent); box-shadow: 0 4px 24px rgba(0,166,166,0.08); margin-bottom: 3rem;">
    <div class="card-header" style="display: flex; align-items: center; justify-content: space-between;">
      <div>
        <h2 style="margin: 0; font-size: 1.25rem;">3. Service Configuration & Build Settings</h2>
        <p class="text-xs text-muted" style="margin-top: 0.25rem;">Specify build steps, start commands, internal listening ports, and environment variables.</p>
      </div>
      <span class="badge" style="background: rgba(0,166,166,0.1); color: var(--color-accent); font-weight: 600;">
        {selectedPreset?.title || 'Custom'}
      </span>
    </div>

    <form onsubmit={handleDeploy}>
      <!-- Service Name & Slug -->
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; margin-bottom: 1.25rem;">
        <div class="form-group">
          <label for="svc-name" class="form-label">Service Name</label>
          <input 
            id="svc-name" 
            type="text" 
            class="form-input" 
            placeholder="e.g. vtopc, backend-api" 
            bind:value={name} 
            required 
          />
        </div>

        <div class="form-group">
          <label for="svc-slug-inp" class="form-label">URL Slug</label>
          <input 
            id="svc-slug-inp" 
            type="text" 
            class="form-input font-mono" 
            placeholder="vtopc" 
            bind:value={svcSlug} 
            oninput={() => slugEdited = true}
            required 
            pattern="[a-z0-9-]+"
          />
        </div>
      </div>

      <!-- Build Command & Start Command -->
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; margin-bottom: 1.25rem;">
        <div class="form-group">
          <label for="svc-build-cmd" class="form-label">Build Command</label>
          <input 
            id="svc-build-cmd" 
            type="text" 
            class="form-input font-mono text-xs" 
            placeholder="pip install -r requirements.txt" 
            bind:value={buildCommand} 
          />
          <p class="text-xs text-muted" style="margin-top:0.25rem;">e.g. <code>pip install -r requirements.txt</code> or <code>npm run build</code></p>
        </div>

        <div class="form-group">
          <label for="svc-start-cmd" class="form-label">Start / Run Command</label>
          <input 
            id="svc-start-cmd" 
            type="text" 
            class="form-input font-mono text-xs" 
            placeholder="gunicorn app:app --bind 0.0.0.0:5000" 
            bind:value={startCommand} 
          />
          <p class="text-xs text-muted" style="margin-top:0.25rem;">e.g. <code>gunicorn app:app --bind 0.0.0.0:5000</code> or <code>npm start</code></p>
        </div>
      </div>

      <!-- Port & Container Runtime Image -->
      <div style="display: grid; grid-template-columns: 120px 1fr; gap: 1rem; margin-bottom: 1.25rem;">
        <div class="form-group">
          <label for="svc-port-inp" class="form-label">Internal Port</label>
          <input 
            id="svc-port-inp" 
            type="number" 
            class="form-input font-mono" 
            placeholder="5000" 
            bind:value={internalPort} 
            required 
            min="1" 
            max="65535" 
          />
        </div>

        <div class="form-group">
          <label for="svc-img-ref" class="form-label">Base Container Environment</label>
          <input 
            id="svc-img-ref" 
            type="text" 
            class="form-input font-mono" 
            placeholder="python:3.11-slim" 
            bind:value={imageRef} 
            required 
          />
        </div>
      </div>

      <!-- Cron Schedule (if cron) -->
      {#if kind === 'cron'}
        <div class="form-group" style="margin-bottom: 1.25rem;">
          <label for="svc-cron-sched" class="form-label">Cron Schedule Expression</label>
          <input 
            id="svc-cron-sched" 
            type="text" 
            class="form-input font-mono" 
            placeholder="*/5 * * * * (Every 5 minutes)" 
            bind:value={cronSchedule} 
            required 
          />
        </div>
      {/if}

      <!-- Environment Variables -->
      <div style="margin-bottom: 1.5rem;">
        <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.5rem;">
          <label class="form-label" style="margin:0;">Environment Variables</label>
          <button type="button" class="btn btn-secondary" style="padding: 2px 10px; min-height: 28px; font-size: 0.75rem;" onclick={addEnv}>
            <Plus size={12} /> Add Variable
          </button>
        </div>
        {#if envVars.length === 0}
          <p class="text-xs text-muted" style="margin:0;">No environment variables configured.</p>
        {:else}
          <div style="display: flex; flex-direction: column; gap: 0.5rem;">
            {#each envVars as env, i}
              <div style="display: flex; gap: 0.5rem; align-items: center;">
                <input type="text" class="form-input font-mono text-xs" placeholder="KEY (e.g. PYTHONUNBUFFERED)" bind:value={env.key} style="flex:1;" />
                <span class="text-muted">=</span>
                <input type="text" class="form-input font-mono text-xs" placeholder="value" bind:value={env.value} style="flex:2;" />
                <button type="button" class="btn btn-secondary" style="padding: 4px; color: var(--color-error);" onclick={() => removeEnv(i)} aria-label="Remove variable">
                  <X size={14} />
                </button>
              </div>
            {/each}
          </div>
        {/if}
      </div>

      <!-- Auto deploy toggle -->
      <div style="display: flex; align-items: center; justify-content: space-between; padding: 0.75rem 0; border-top: 1px solid var(--color-border); margin-bottom: 1.5rem;">
        <div>
          <div style="font-size: 0.875rem; font-weight: 600;">Auto Deploy on Git Push / Updates</div>
          <div class="text-xs text-muted">Automatically triggers a build whenever new commits are pushed.</div>
        </div>
        <input type="checkbox" bind:checked={autoDeploy} style="width: 18px; height: 18px; accent-color: var(--color-accent);" />
      </div>

      {#if error}
        <div class="alert alert-error" style="margin-bottom: 1.25rem; background: #fee2e2; border: 1px solid #fca5a5; color: #991b1b; padding: 0.75rem 1rem; border-radius: var(--radius-md); font-size: 0.875rem;">
          {error}
        </div>
      {/if}

      <div style="display: flex; justify-content: flex-end; gap: 1rem; padding-top: 1rem; border-top: 1px solid var(--color-border);">
        <button type="button" class="btn btn-secondary" onclick={() => goto(`/projects/${slug}`)} disabled={submitting}>
          Cancel
        </button>
        <button type="submit" class="btn btn-primary" disabled={submitting || !name}>
          {#if submitting}
            <Loader2 size={16} class="animate-spin" style="margin-right: 0.5rem;" />
            Building & Deploying...
          {:else}
            <Rocket size={16} style="margin-right: 0.5rem;" />
            Deploy {name || 'Service'}
          {/if}
        </button>
      </div>
    </form>
  </div>
{/if}

<!-- Connect Git Provider Modal -->
{#if showConnectModal}
  <div 
    style="position:fixed; inset:0; background:rgba(11,31,58,0.5); z-index:100; display:flex; align-items:center; justify-content:center; padding:1rem;"
    onclick={() => showConnectModal = false}
    onkeydown={(e) => e.key === 'Escape' && (showConnectModal = false)}
    role="button"
    tabindex="0"
  >
    <div 
      class="card" 
      style="width:min(500px, 95vw); padding:1.75rem;" 
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
      role="dialog"
      aria-modal="true"
      tabindex="-1"
    >
      <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem; border-bottom:1px solid var(--color-border); padding-bottom:0.75rem;">
        <h3 style="margin:0; text-transform:capitalize;">Connect {selectedProvider} Account</h3>
        <button class="btn btn-secondary" style="padding:4px;" onclick={() => showConnectModal = false}>
          <X size={16} />
        </button>
      </div>

      <form onsubmit={saveIntegration}>
        <div class="form-group" style="margin-bottom:1rem;">
          <label for="git-username-inp" class="form-label">{selectedProvider} Username</label>
          <input 
            id="git-username-inp" 
            type="text" 
            class="form-input" 
            placeholder="e.g. vedantjja" 
            bind:value={providerUsername} 
            required 
          />
        </div>

        <div class="form-group" style="margin-bottom:1.5rem;">
          <label for="git-token-inp" class="form-label">Personal Access Token / App Password</label>
          <input 
            id="git-token-inp" 
            type="password" 
            class="form-input font-mono text-xs" 
            placeholder="ghp_... or Bitbucket App Password" 
            bind:value={providerToken} 
            required 
          />
          <p class="text-xs text-muted" style="margin-top:0.25rem;">Used securely to read repositories and setup webhooks.</p>
        </div>

        <div style="display:flex; justify-content:flex-end; gap:0.75rem;">
          <button type="button" class="btn btn-secondary" onclick={() => showConnectModal = false}>Cancel</button>
          <button type="submit" class="btn btn-primary" disabled={connecting || !providerUsername || !providerToken}>
            {#if connecting}<Loader2 size={14} class="animate-spin" />{:else}<Check size={14} />{/if} Save Integration
          </button>
        </div>
      </form>
    </div>
  </div>
{/if}
