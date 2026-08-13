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
    Check
  } from 'lucide-svelte';

  const { slug } = $derived($page.params);
  let project = $state<any>(null);
  let loading = $state(true);

  // Active category filter
  let activeCategory = $state<'all' | 'web' | 'static' | 'worker'>('all');

  // Selected service preset
  let selectedPreset = $state<any>(null);

  // Form fields
  let name = $state('');
  let svcSlug = $state('');
  let slugEdited = false;
  let kind = $state('web');
  let imageRef = $state('');
  let internalPort = $state(80);
  let buildCommand = $state('');
  let startCommand = $state('');
  let cronSchedule = $state('0 * * * *');
  let envVars = $state<Array<{ key: string; value: string }>>([]);
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
      id: 'node',
      title: 'Node.js',
      description: 'Express, NestJS, Fastify, Next.js, Remix, and Node servers',
      category: 'web',
      kind: 'web',
      image: 'node:20-alpine',
      port: 3000,
      badge: 'Dynamic Web',
      defaultBuild: 'npm install && npm run build',
      defaultStart: 'npm start'
    },
    {
      id: 'python',
      title: 'Python',
      description: 'FastAPI, Flask, Django, Uvicorn, and Python web APIs',
      category: 'web',
      kind: 'web',
      image: 'python:3.11-slim',
      port: 8000,
      badge: 'Dynamic Web',
      defaultBuild: 'pip install -r requirements.txt',
      defaultStart: 'uvicorn main:app --host 0.0.0.0 --port 8000'
    },
    {
      id: 'go',
      title: 'Go (Golang)',
      description: 'Gin, Fiber, Echo, or standard library microservices',
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
      description: 'Spring Boot, Quarkus, Micronaut, Maven, and Gradle applications',
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
      description: 'Deploy any public or private container image from Docker Hub, GHCR, or ECR',
      category: 'web',
      kind: 'web',
      image: 'nginx:alpine',
      port: 80,
      badge: 'Container'
    },

    // Static Sites
    {
      id: 'static-html',
      title: 'Static HTML / CSS / JS',
      description: 'Pure static web pages served through an ultra-fast Nginx container',
      category: 'static',
      kind: 'static',
      image: 'nginx:alpine',
      port: 80,
      badge: 'Static Site'
    },
    {
      id: 'static-spa',
      title: 'React / Vite / Vue / Svelte',
      description: 'Single-page applications with client-side routing & build step',
      category: 'static',
      kind: 'static',
      image: 'nginx:alpine',
      port: 80,
      badge: 'Static SPA',
      defaultBuild: 'npm install && npm run build'
    },
    {
      id: 'static-jamstack',
      title: 'Astro / Jamstack / Next Export',
      description: 'Static generated sites and documentation frameworks',
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

  onMount(async () => {
    try {
      const res = await fetch(`/api/v1/projects/${slug}`, { credentials: 'include' });
      if (res.ok) {
        project = await res.json();
      } else {
        goto('/workspaces');
      }
    } catch (e) {
      goto('/workspaces');
    } finally {
      loading = false;
    }
  });

  function choosePreset(p: ServicePreset) {
    selectedPreset = p;
    kind = p.kind;
    imageRef = p.image;
    internalPort = p.port || 80;
    buildCommand = p.defaultBuild || '';
    startCommand = p.defaultStart || '';
    if (!name) {
      name = `my-${p.id}-app`;
      svcSlug = name;
    }
    // Scroll to form smoothly
    setTimeout(() => {
      document.getElementById('service-config-form')?.scrollIntoView({ behavior: 'smooth' });
    }, 50);
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

  async function handleDeploy(e: Event) {
    e.preventDefault();
    if (!name || !svcSlug || !imageRef) return;
    
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

      // Trigger initial deployment automatically
      await fetch(`/api/v1/services/${svcId}/deploy`, { method: 'POST', credentials: 'include' });

      // Navigate to the newly created service overview
      goto(`/services/${svcId}/overview`);
    } catch (e: any) {
      error = e.message;
    } finally {
      submitting = false;
    }
  }
</script>

<svelte:head>
  <title>Deploy New Service — kloudsPanel</title>
</svelte:head>

{#if loading}
  <div class="empty-state">
    <div class="animate-spin text-muted" style="margin-bottom:1rem"><Loader2 size={48} /></div>
    <p>Loading catalog…</p>
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
        <p class="page-subtitle">Choose a runtime template, static hosting, background worker, or custom container.</p>
      </div>
    </div>
  </div>

  <!-- Category Filter Pills -->
  <div style="display: flex; gap: 0.5rem; margin-bottom: 1.5rem; overflow-x: auto; padding-bottom: 0.25rem;">
    <button 
      class="btn" 
      class:btn-primary={activeCategory === 'all'} 
      class:btn-secondary={activeCategory !== 'all'}
      style="padding: 6px 16px; font-size: 0.875rem; border-radius: 999px; min-height: 36px;"
      onclick={() => activeCategory = 'all'}
    >
      All Services ({presets.length})
    </button>
    <button 
      class="btn" 
      class:btn-primary={activeCategory === 'web'} 
      class:btn-secondary={activeCategory !== 'web'}
      style="padding: 6px 16px; font-size: 0.875rem; border-radius: 999px; min-height: 36px;"
      onclick={() => activeCategory = 'web'}
    >
      Web & Dynamic APIs
    </button>
    <button 
      class="btn" 
      class:btn-primary={activeCategory === 'static'} 
      class:btn-secondary={activeCategory !== 'static'}
      style="padding: 6px 16px; font-size: 0.875rem; border-radius: 999px; min-height: 36px;"
      onclick={() => activeCategory = 'static'}
    >
      Static Sites & SPAs
    </button>
    <button 
      class="btn" 
      class:btn-primary={activeCategory === 'worker'} 
      class:btn-secondary={activeCategory !== 'worker'}
      style="padding: 6px 16px; font-size: 0.875rem; border-radius: 999px; min-height: 36px;"
      onclick={() => activeCategory = 'worker'}
    >
      Workers & Scheduled Tasks
    </button>
  </div>

  <!-- Catalog Grid of Cards -->
  <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(280px, 1fr)); gap: 1rem; margin-bottom: 2.5rem;">
    {#each filteredPresets as preset}
      {@const isSelected = selectedPreset?.id === preset.id}
      <button 
        type="button" 
        class="card"
        style="
          cursor: pointer; 
          text-align: left; 
          padding: 1.25rem; 
          border: 2px solid {isSelected ? 'var(--color-accent)' : 'var(--color-border)'}; 
          background: {isSelected ? 'rgba(0,166,166,0.04)' : 'var(--color-surface)'};
          display: flex;
          flex-direction: column;
          justify-content: space-between;
          transition: all var(--transition-fast);
        "
        onclick={() => choosePreset(preset)}
      >
        <div>
          <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.75rem;">
            <span class="badge" style="background:#f1f5f9; color:#334155; font-size: 0.7rem;">
              {preset.badge}
            </span>
            {#if isSelected}
              <span style="display: inline-flex; align-items: center; justify-content: center; width: 20px; height: 20px; border-radius: 50%; background: var(--color-accent); color: #fff;">
                <Check size={12} strokeWidth={3} />
              </span>
            {/if}
          </div>
          <h3 style="margin: 0 0 0.4rem 0; font-size: 1.125rem; font-weight: 700; color: var(--color-ink);">
            {preset.title}
          </h3>
          <p class="text-xs text-muted" style="margin: 0; line-height: 1.5;">
            {preset.description}
          </p>
        </div>

        <div style="margin-top: 1.25rem; padding-top: 0.75rem; border-top: 1px solid var(--color-border); display: flex; justify-content: space-between; align-items: center;">
          <span class="font-mono text-xs text-muted truncate" style="max-width: 170px;">
            {preset.image}
          </span>
          {#if preset.port}
            <span class="text-xs font-mono" style="color: var(--color-accent); font-weight: 600;">:{preset.port}</span>
          {:else}
            <span class="text-xs text-muted">No port</span>
          {/if}
        </div>
      </button>
    {/each}
  </div>

  <!-- Configuration Form (Visible once a card is selected or for custom setup) -->
  {#if selectedPreset}
    <div id="service-config-form" class="card" style="max-width: 720px; border: 1px solid var(--color-accent); box-shadow: 0 4px 20px rgba(0,166,166,0.1); margin-bottom: 3rem;">
      <div class="card-header" style="display: flex; align-items: center; justify-content: space-between;">
        <div>
          <h2 style="margin: 0; font-size: 1.25rem;">Configure {selectedPreset.title}</h2>
          <p class="text-xs text-muted" style="margin-top: 0.25rem;">Fine-tune deployment options, port mappings, and runtime variables.</p>
        </div>
        <span class="badge" style="background: rgba(0,166,166,0.1); color: var(--color-accent); font-weight: 600;">
          {selectedPreset.badge}
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
              placeholder="e.g. backend-api, frontend-web" 
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
              placeholder="backend-api" 
              bind:value={svcSlug} 
              oninput={() => slugEdited = true}
              required 
              pattern="[a-z0-9-]+"
            />
          </div>
        </div>

        <!-- Docker Image Reference & Port -->
        <div style="display: grid; grid-template-columns: 1fr 140px; gap: 1rem; margin-bottom: 1.25rem;">
          <div class="form-group">
            <label for="svc-img-ref" class="form-label">Docker Image Reference</label>
            <input 
              id="svc-img-ref" 
              type="text" 
              class="form-input font-mono" 
              placeholder="e.g. nginx:alpine, ghcr.io/org/repo:latest" 
              bind:value={imageRef} 
              required 
            />
            <p class="text-xs text-muted" style="margin-top: 0.25rem;">You can specify any custom Docker Hub or GHCR image tag.</p>
          </div>

          {#if kind === 'web' || kind === 'static'}
            <div class="form-group">
              <label for="svc-port-inp" class="form-label">Internal Port</label>
              <input 
                id="svc-port-inp" 
                type="number" 
                class="form-input font-mono" 
                placeholder="80" 
                bind:value={internalPort} 
                required 
                min="1" 
                max="65535" 
              />
            </div>
          {/if}
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
            <p class="text-xs text-muted" style="margin-top: 0.25rem;">Standard 5-field cron expression: (minute hour day month weekday).</p>
          </div>
        {/if}

        <!-- Build & Start Command (Optional) -->
        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; margin-bottom: 1.25rem;">
          <div class="form-group">
            <label for="svc-build-cmd" class="form-label">Build Command (Optional)</label>
            <input 
              id="svc-build-cmd" 
              type="text" 
              class="form-input font-mono text-xs" 
              placeholder="e.g. npm run build" 
              bind:value={buildCommand} 
            />
          </div>

          <div class="form-group">
            <label for="svc-start-cmd" class="form-label">Start Command / Entrypoint (Optional)</label>
            <input 
              id="svc-start-cmd" 
              type="text" 
              class="form-input font-mono text-xs" 
              placeholder="e.g. npm start" 
              bind:value={startCommand} 
            />
          </div>
        </div>

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
                  <input type="text" class="form-input font-mono text-xs" placeholder="KEY (e.g. DATABASE_URL)" bind:value={env.key} style="flex:1;" />
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
            <div style="font-size: 0.875rem; font-weight: 600;">Auto Deploy on Updates</div>
            <div class="text-xs text-muted">Automatically redeploy when new revisions or config changes occur.</div>
          </div>
          <input type="checkbox" bind:checked={autoDeploy} style="width: 18px; height: 18px; accent-color: var(--color-accent);" />
        </div>

        {#if error}
          <div class="alert alert-error" style="margin-bottom: 1.25rem; background: #fee2e2; border: 1px solid #fca5a5; color: #991b1b; padding: 0.75rem 1rem; border-radius: var(--radius-md); font-size: 0.875rem;">
            {error}
          </div>
        {/if}

        <div style="display: flex; justify-content: flex-end; gap: 1rem; padding-top: 1rem; border-top: 1px solid var(--color-border);">
          <button type="button" class="btn btn-secondary" onclick={() => selectedPreset = null} disabled={submitting}>
            Cancel
          </button>
          <button type="submit" class="btn btn-primary" disabled={submitting || !name || !imageRef}>
            {#if submitting}
              <Loader2 size={16} class="animate-spin" style="margin-right: 0.5rem;" />
              Deploying Service...
            {:else}
              <Rocket size={16} style="margin-right: 0.5rem;" />
              Deploy {selectedPreset.title}
            {/if}
          </button>
        </div>
      </form>
    </div>
  {/if}
{/if}
