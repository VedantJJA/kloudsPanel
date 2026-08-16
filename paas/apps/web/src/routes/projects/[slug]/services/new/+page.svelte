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
    Wand2,
    Sliders,
    Info,
    ShieldCheck,
    ChevronRight,
    ChevronLeft,
    Package
  } from 'lucide-svelte';
  import FrameworkIcon from '$lib/components/icons/FrameworkIcon.svelte';

  const slug = $derived($page.params.slug);
  let project = $state<any>(null);

  // Studio Active Tab / Step: 'source' | 'stack' | 'config' | 'environment'
  let activeTab = $state<'source' | 'stack' | 'config' | 'environment'>('source');

  // Source Type: 'git_public' | 'git_provider' | 'image'
  let sourceType = $state<'git_public' | 'git_provider' | 'image'>('git_public');

  // Public Git source fields
  let gitRepoUrl = $state('');
  let gitBranch = $state('main');
  let rootDirectory = $state('.');

  // render.yaml / blueprint auto-detect state
  let parsingYaml = $state(false);
  let detectedBlueprint = $state<any>(null);
  let detectedBlueprintSource = $state<string>('klouds.yaml');
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
        const isSecret = k.includes('SECRET') || k.includes('KEY') || k.includes('PASS') || k.includes('TOKEN') || k.includes('AUTH');
        const isMissingOrPlaceholder = strVal === '' || strVal.toLowerCase().startsWith('your_') || strVal.toLowerCase().startsWith('replace_') || strVal.toLowerCase() === 'changeme';
        const isExplicitlyRequired = reqs.includes(k);

        if (isExplicitlyRequired || isMissingOrPlaceholder) {
          list.push({ svcName: svc.name, svcIdx: sIdx, key: k, value: strVal, isSecret, isRequired: true });
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

  // Form fields (initialized to Node preset by default)
  let selectedPreset = $state<any>(null);
  let name = $state('');
  let svcSlug = $state('');
  let slugEdited = false;
  let kind = $state('web');
  let imageRef = $state('node:22-alpine');
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

  // Runtime Version & Security Resource limits
  let runtimeVersion = $state('auto');
  let memoryLimit = $state('512m');
  let cpuLimit = $state('1.0');
  let pidsLimit = $state(256);

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
    versions: Array<{ value: string; label: string; default?: boolean }>;
  };

  const presets: ServicePreset[] = [
    // --- Web / Dynamic Runtimes ---------------------------------------------
    {
      id: 'node',
      title: 'Node.js (Next.js / Express / Nest / Remix / Astro)',
      description: 'Fullstack JavaScript/TypeScript apps with Node.js & npm/pnpm/yarn/bun',
      category: 'web',
      kind: 'web',
      image: 'node:22-alpine',
      port: 3000,
      badge: 'JavaScript/TS',
      iconColor: '#22c55e',
      iconText: 'Node',
      defaultBuild: 'npm install && npm run build',
      defaultStart: 'npm start',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / .node-version)', default: true },
        { value: '22', label: 'Node.js 22 (Current Active)' },
        { value: '20', label: 'Node.js 20 (LTS)' },
        { value: '18', label: 'Node.js 18 (LTS)' },
        { value: '16', label: 'Node.js 16 (Legacy)' }
      ]
    },
    {
      id: 'python',
      title: 'Python (FastAPI / Flask / Django / Celery)',
      description: 'ASGI / WSGI applications with requirements.txt, poetry, pipenv or uv',
      category: 'web',
      kind: 'web',
      image: 'python:3.12-slim',
      port: 5000,
      badge: 'Python',
      iconColor: '#3b82f6',
      iconText: 'Py',
      defaultBuild: 'pip install -r requirements.txt',
      defaultStart: 'gunicorn app:app --bind 0.0.0.0:5000 --workers 2',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / .python-version)', default: true },
        { value: '3.12', label: 'Python 3.12 (Recommended)' },
        { value: '3.11', label: 'Python 3.11' },
        { value: '3.10', label: 'Python 3.10' },
        { value: '3.9', label: 'Python 3.9' }
      ]
    },
    {
      id: 'go',
      title: 'Go (Fiber / Gin / Chi / Echo / Standard HTTP)',
      description: 'Ultra-fast compiled Go binary web services with zero runtime dependencies',
      category: 'web',
      kind: 'web',
      image: 'golang:1.23-alpine',
      port: 8080,
      badge: 'Go',
      iconColor: '#06b6d4',
      iconText: 'Go',
      defaultBuild: 'go build -o server .',
      defaultStart: './server',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / go.mod)', default: true },
        { value: '1.23', label: 'Go 1.23 (Latest)' },
        { value: '1.22', label: 'Go 1.22' },
        { value: '1.21', label: 'Go 1.21' }
      ]
    },
    {
      id: 'rust',
      title: 'Rust (Actix / Axum / Rocket / Warp)',
      description: 'High-performance memory-safe Rust services compiled via Cargo release mode',
      category: 'web',
      kind: 'web',
      image: 'rust:1.84-alpine',
      port: 8080,
      badge: 'Rust Cargo',
      iconColor: '#f97316',
      iconText: 'Rust',
      defaultBuild: 'cargo build --release',
      defaultStart: './app/server',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / rust-toolchain.toml)', default: true },
        { value: '1.84', label: 'Rust 1.84 (Latest)' },
        { value: '1.82', label: 'Rust 1.82' },
        { value: '1.80', label: 'Rust 1.80' },
        { value: '1.78', label: 'Rust 1.78' }
      ]
    },
    {
      id: 'java',
      title: 'Java (Spring Boot / Quarkus / Micronaut)',
      description: 'JVM application built with Maven or Gradle wrapper with multi-stage build',
      category: 'web',
      kind: 'web',
      image: 'eclipse-temurin:21-jdk-alpine',
      port: 8080,
      badge: 'Java Temurin',
      iconColor: '#ef4444',
      iconText: 'Java',
      defaultBuild: './mvnw clean package -DskipTests',
      defaultStart: 'java -jar target/*.jar',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / .java-version)', default: true },
        { value: '21', label: 'Java 21 (LTS / Recommended)' },
        { value: '17', label: 'Java 17 (LTS)' },
        { value: '11', label: 'Java 11 (LTS)' }
      ]
    },
    {
      id: 'php',
      title: 'PHP (Laravel / Symfony / WordPress)',
      description: 'PHP Apache + Composer runtime with opcache and MySQL/Postgres drivers',
      category: 'web',
      kind: 'web',
      image: 'php:8.3-apache',
      port: 80,
      badge: 'PHP Apache',
      iconColor: '#8b5cf6',
      iconText: 'PHP',
      defaultBuild: 'composer install --no-dev --optimize-autoloader',
      defaultStart: 'apache2-foreground',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / composer.json)', default: true },
        { value: '8.3', label: 'PHP 8.3 (Latest)' },
        { value: '8.2', label: 'PHP 8.2' },
        { value: '8.1', label: 'PHP 8.1' }
      ]
    },
    {
      id: 'ruby',
      title: 'Ruby (Rails / Sinatra / Puma)',
      description: 'Rails web application with Bundler and Puma application server',
      category: 'web',
      kind: 'web',
      image: 'ruby:3.3-alpine',
      port: 3000,
      badge: 'Ruby Rails',
      iconColor: '#e11d48',
      iconText: 'Ruby',
      defaultBuild: 'bundle install && rails assets:precompile',
      defaultStart: 'bundle exec puma -C config/puma.rb',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / .ruby-version)', default: true },
        { value: '3.3', label: 'Ruby 3.3 (Latest)' },
        { value: '3.2', label: 'Ruby 3.2' }
      ]
    },
    {
      id: 'elixir',
      title: 'Elixir (Phoenix / Plug)',
      description: 'Fault-tolerant concurrent services on the BEAM VM with Mix package manager',
      category: 'web',
      kind: 'web',
      image: 'elixir:1.18-alpine',
      port: 4000,
      badge: 'Elixir Phoenix',
      iconColor: '#4e2a8e',
      iconText: 'Elixir',
      defaultBuild: 'mix deps.get --only prod && mix compile',
      defaultStart: 'mix phx.server',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / .elixir-version)', default: true },
        { value: '1.18', label: 'Elixir 1.18 (Latest / OTP 27)' },
        { value: '1.17', label: 'Elixir 1.17 (OTP 26)' },
        { value: '1.16', label: 'Elixir 1.16' }
      ]
    },
    {
      id: 'deno',
      title: 'Deno (Fresh / Hono / Oak)',
      description: 'Secure TypeScript, JavaScript and WebAssembly runtime with zero setup',
      category: 'web',
      kind: 'web',
      image: 'denoland/deno:alpine',
      port: 8000,
      badge: 'Deno Runtime',
      iconColor: '#000000',
      iconText: 'Deno',
      defaultBuild: 'deno cache main.ts',
      defaultStart: 'deno run --allow-net --allow-env --allow-read main.ts',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / deno.json)', default: true },
        { value: 'latest', label: 'Deno (Latest)' },
        { value: '2.0', label: 'Deno 2.0' },
        { value: '1.45', label: 'Deno 1.45' }
      ]
    },
    {
      id: 'bun',
      title: 'Bun (Elysia / Hono / Next.js)',
      description: 'All-in-one JavaScript toolkit & fast runtime with native TypeScript support',
      category: 'web',
      kind: 'web',
      image: 'oven/bun:alpine',
      port: 3000,
      badge: 'Bun All-In-One',
      iconColor: '#fbf0df',
      iconText: 'Bun',
      defaultBuild: 'bun install --frozen-lockfile',
      defaultStart: 'bun run start',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / bunfig.toml)', default: true },
        { value: 'latest', label: 'Bun (Latest)' },
        { value: '1.1', label: 'Bun 1.1' }
      ]
    },
    {
      id: 'dotnet',
      title: '.NET / C# (ASP.NET Core / Web API)',
      description: 'High-performance cross-platform enterprise web applications and APIs',
      category: 'web',
      kind: 'web',
      image: 'mcr.microsoft.com/dotnet/sdk:9.0-alpine',
      port: 5000,
      badge: '.NET Core',
      iconColor: '#512bd4',
      iconText: '.NET',
      defaultBuild: 'dotnet restore && dotnet publish -c Release -o /app/publish',
      defaultStart: 'dotnet /app/publish/*.dll',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / global.json)', default: true },
        { value: '9.0', label: '.NET 9.0 (Latest)' },
        { value: '8.0', label: '.NET 8.0 (LTS / Recommended)' }
      ]
    },
    {
      id: 'scala',
      title: 'Scala (Play / Akka / Http4s / sbt)',
      description: 'Functional and object-oriented JVM services compiled with sbt',
      category: 'web',
      kind: 'web',
      image: 'eclipse-temurin:21-jdk-alpine',
      port: 9000,
      badge: 'Scala sbt',
      iconColor: '#dc2626',
      iconText: 'Scala',
      defaultBuild: 'sbt stage',
      defaultStart: './target/universal/stage/bin/*',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / build.sbt)', default: true },
        { value: '21', label: 'Java 21 Runtime' },
        { value: '17', label: 'Java 17 Runtime' }
      ]
    },
    {
      id: 'kotlin',
      title: 'Kotlin (Ktor / Spring Boot / Micronaut)',
      description: 'Concise modern JVM applications built with Gradle or Maven wrapper',
      category: 'web',
      kind: 'web',
      image: 'eclipse-temurin:21-jdk-alpine',
      port: 8080,
      badge: 'Kotlin JVM',
      iconColor: '#7f52ff',
      iconText: 'Kotlin',
      defaultBuild: './gradlew build -x test',
      defaultStart: 'java -jar build/libs/*.jar',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / build.gradle.kts)', default: true },
        { value: '21', label: 'Java 21 Runtime' },
        { value: '17', label: 'Java 17 Runtime' }
      ]
    },
    {
      id: 'swift',
      title: 'Swift (Vapor / Hummingbird / Server)',
      description: 'Fast, type-safe compiled backend services with Swift Package Manager',
      category: 'web',
      kind: 'web',
      image: 'swift:6.0-jammy',
      port: 8080,
      badge: 'Swift Vapor',
      iconColor: '#f05138',
      iconText: 'Swift',
      defaultBuild: 'swift build -c release',
      defaultStart: './.build/release/App serve --env production --hostname 0.0.0.0 --port 8080',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / Package.swift)', default: true },
        { value: '6.0', label: 'Swift 6.0 (Latest)' },
        { value: '5.10', label: 'Swift 5.10' }
      ]
    },
    {
      id: 'haskell',
      title: 'Haskell (Yesod / Servant / Scotty / Cabal)',
      description: 'Purely functional statically typed web applications compiled with GHC & Cabal',
      category: 'web',
      kind: 'web',
      image: 'haskell:9.10-slim',
      port: 8080,
      badge: 'Haskell GHC',
      iconColor: '#5e5086',
      iconText: 'Haskell',
      defaultBuild: 'cabal update && cabal build --enable-optimization',
      defaultStart: './dist-newstyle/build/server',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / cabal.project)', default: true },
        { value: '9.10', label: 'GHC 9.10 (Latest)' },
        { value: '9.8', label: 'GHC 9.8' }
      ]
    },
    {
      id: 'clojure',
      title: 'Clojure (Ring / Compojure / Leiningen)',
      description: 'Lisp dialect on the JVM for data-driven backend services and APIs',
      category: 'web',
      kind: 'web',
      image: 'clojure:temurin-21-lein-alpine',
      port: 3000,
      badge: 'Clojure Lein',
      iconColor: '#5881d8',
      iconText: 'Clojure',
      defaultBuild: 'lein uberjar',
      defaultStart: 'java -jar target/*-standalone.jar',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / project.clj)', default: true }
      ]
    },
    {
      id: 'crystal',
      title: 'Crystal (Lucky / Kemal / Amber)',
      description: 'Ruby-inspired syntax with C-like compiled performance and memory safety',
      category: 'web',
      kind: 'web',
      image: 'crystallang/crystal:latest',
      port: 3000,
      badge: 'Crystal Shards',
      iconColor: '#000000',
      iconText: 'Crystal',
      defaultBuild: 'shards install && crystal build src/*.cr --release -o app',
      defaultStart: './app',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / shard.yml)', default: true },
        { value: 'latest', label: 'Crystal (Latest)' }
      ]
    },
    {
      id: 'zig',
      title: 'Zig (HTTP / Native Backend)',
      description: 'General-purpose programming language and toolchain for robust software',
      category: 'web',
      kind: 'web',
      image: 'alpine:3.21',
      port: 8080,
      badge: 'Zig Native',
      iconColor: '#f7a41d',
      iconText: 'Zig',
      defaultBuild: 'zig build -Doptimize=ReleaseFast',
      defaultStart: './zig-out/bin/server',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / build.zig.zon)', default: true },
        { value: 'latest', label: 'Zig (Latest)' }
      ]
    },
    {
      id: 'dart',
      title: 'Dart (Shelf / dart_frog / Server)',
      description: 'Client-optimized language for fast apps on any platform with AOT compilation',
      category: 'web',
      kind: 'web',
      image: 'dart:stable',
      port: 8080,
      badge: 'Dart Shelf',
      iconColor: '#0175c2',
      iconText: 'Dart',
      defaultBuild: 'dart pub get && dart compile exe bin/server.dart -o server',
      defaultStart: './server',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Latest / pubspec.yaml)', default: true },
        { value: 'stable', label: 'Dart (Stable)' }
      ]
    },
    {
      id: 'dockerfile',
      title: 'Custom Dockerfile',
      description: 'Use the Dockerfile located in your repository directory (Hardened & Scanned)',
      category: 'web',
      kind: 'web',
      image: 'custom',
      port: 80,
      badge: 'Docker',
      iconColor: '#0284c7',
      iconText: 'Docker',
      defaultBuild: 'docker build -t app .',
      defaultStart: 'docker run app',
      versions: [
        { value: 'custom', label: 'Custom Dockerfile', default: true }
      ]
    },

    // --- Static Sites -------------------------------------------------------
    {
      id: 'static-spa',
      title: 'Static SPA (React / Vite / Vue / SvelteKit / Astro / HTML)',
      description: 'Single page applications compiled to static assets served by high-speed Nginx',
      category: 'static',
      kind: 'static',
      image: 'nginx:alpine',
      port: 80,
      badge: 'Static / SPA',
      iconColor: '#0ea5e9',
      iconText: 'SPA',
      defaultBuild: 'npm install && npm run build',
      defaultStart: 'nginx -g "daemon off;"',
      versions: [
        { value: 'auto', label: 'Static Asset Ingress (Nginx)', default: true }
      ]
    },
    {
      id: 'nginx',
      title: 'Nginx Static Web Server',
      description: 'High-performance static file serving and reverse proxying',
      category: 'static',
      kind: 'static',
      image: 'nginx:alpine',
      port: 80,
      badge: 'Web Server',
      iconColor: '#10b981',
      iconText: 'Nginx',
      defaultBuild: '',
      defaultStart: 'nginx -g "daemon off;"',
      versions: [
        { value: 'auto', label: 'Nginx Alpine', default: true }
      ]
    },

    // --- Background Workers & Cron ------------------------------------------
    {
      id: 'worker',
      title: 'Background Worker',
      description: 'Continuous queue consumer, event listener, Celery, BullMQ, or worker task',
      category: 'worker',
      kind: 'worker',
      image: 'node:22-alpine',
      port: 0,
      badge: 'Worker',
      iconColor: '#6366f1',
      iconText: 'Worker',
      defaultBuild: 'npm install',
      defaultStart: 'npm run worker',
      versions: [
        { value: 'auto', label: 'Auto-Detect Worker Runtime', default: true },
        { value: 'node', label: 'Node.js Worker' },
        { value: 'python', label: 'Python Celery / RQ Worker' },
        { value: 'go', label: 'Go Worker Daemon' }
      ]
    },
    {
      id: 'cron-job',
      title: 'Scheduled Cron Job',
      description: 'Periodic task executed on an automated recurring cron schedule',
      category: 'worker',
      kind: 'cron',
      image: 'alpine:3.21',
      port: 0,
      badge: 'Cron',
      iconColor: '#d97706',
      iconText: 'Cron',
      defaultStart: 'echo "Running scheduled job"',
      versions: [
        { value: 'auto', label: 'Auto-Detect Runtime', default: true },
        { value: 'node', label: 'Node.js Script' },
        { value: 'python', label: 'Python Script' }
      ]
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

  function getPresetVersions(presetId: string) {
    const p = presets.find(x => x.id === (presetId || '').toLowerCase()) || 
              presets.find(x => x.kind === (presetId || '').toLowerCase()) || 
              presets[0];
    return p?.versions || [
      { value: 'auto', label: 'Auto-Detect (Latest)' }
    ];
  }

  function getDatabaseVersions(engine: string) {
    const eng = (engine || 'postgres').toLowerCase();
    if (eng === 'postgres' || eng === 'postgresql') {
      return [
        { value: 'auto', label: 'PostgreSQL (Latest / 17)' },
        { value: '17', label: 'PostgreSQL 17' },
        { value: '16', label: 'PostgreSQL 16' },
        { value: '15', label: 'PostgreSQL 15' },
        { value: '14', label: 'PostgreSQL 14' }
      ];
    } else if (eng === 'mysql') {
      return [
        { value: 'auto', label: 'MySQL (Latest / 8.4 LTS)' },
        { value: '8.4', label: 'MySQL 8.4 LTS' },
        { value: '8.0', label: 'MySQL 8.0' }
      ];
    } else if (eng === 'redis') {
      return [
        { value: 'auto', label: 'Redis (Latest / 7.4)' },
        { value: '7.4', label: 'Redis 7.4' },
        { value: '7.2', label: 'Redis 7.2' },
        { value: '7.0', label: 'Redis 7.0' }
      ];
    } else if (eng === 'mongodb' || eng === 'mongo') {
      return [
        { value: 'auto', label: 'MongoDB (Latest / 8.0)' },
        { value: '8.0', label: 'MongoDB 8.0' },
        { value: '7.0', label: 'MongoDB 7.0' },
        { value: '6.0', label: 'MongoDB 6.0' }
      ];
    } else if (eng === 'clickhouse') {
      return [
        { value: 'auto', label: 'ClickHouse (Latest / 24.8)' },
        { value: '24.8', label: 'ClickHouse 24.8' },
        { value: '24.3', label: 'ClickHouse 24.3' }
      ];
    }
    return [
      { value: 'auto', label: 'Auto-Detect (Latest)' }
    ];
  }

  function updateImageRefFromVersion(presetId: string, ver: string) {
    runtimeVersion = ver || 'auto';
    const pId = (presetId || selectedPreset?.id || 'node').toLowerCase();

    if (pId === 'node' || pId === 'nodejs') {
      if (ver === 'auto' || ver === '22' || !ver) imageRef = 'node:22-alpine';
      else if (ver === '20') imageRef = 'node:20-alpine';
      else if (ver === '18') imageRef = 'node:18-alpine';
      else if (ver === '16') imageRef = 'node:16-alpine';
      else imageRef = `node:${ver}-alpine`;
    } else if (pId === 'python') {
      if (ver === 'auto' || ver === '3.12' || !ver) imageRef = 'python:3.12-slim';
      else if (ver === '3.11') imageRef = 'python:3.11-slim';
      else if (ver === '3.10') imageRef = 'python:3.10-slim';
      else if (ver === '3.9') imageRef = 'python:3.9-slim';
      else imageRef = `python:${ver}-slim`;
    } else if (pId === 'go' || pId === 'golang') {
      if (ver === 'auto' || ver === '1.23' || !ver) imageRef = 'golang:1.23-alpine';
      else if (ver === '1.22') imageRef = 'golang:1.22-alpine';
      else if (ver === '1.21') imageRef = 'golang:1.21-alpine';
      else imageRef = `golang:${ver}-alpine`;
    } else if (pId === 'rust') {
      if (ver === 'auto' || ver === '1.84' || !ver) imageRef = 'rust:1.84-alpine';
      else if (ver === '1.82') imageRef = 'rust:1.82-alpine';
      else if (ver === '1.80') imageRef = 'rust:1.80-alpine';
      else imageRef = `rust:${ver}-alpine`;
    } else if (pId === 'java') {
      if (ver === 'auto' || ver === '21' || !ver) imageRef = 'eclipse-temurin:21-jdk-alpine';
      else if (ver === '17') imageRef = 'eclipse-temurin:17-jdk-alpine';
      else if (ver === '11') imageRef = 'eclipse-temurin:11-jdk-alpine';
      else imageRef = `eclipse-temurin:${ver}-jdk-alpine`;
    } else if (pId === 'php') {
      if (ver === 'auto' || ver === '8.3' || !ver) imageRef = 'php:8.3-apache';
      else if (ver === '8.2') imageRef = 'php:8.2-apache';
      else if (ver === '8.1') imageRef = 'php:8.1-apache';
      else imageRef = `php:${ver}-apache`;
    } else if (pId === 'ruby') {
      if (ver === 'auto' || ver === '3.3' || !ver) imageRef = 'ruby:3.3-alpine';
      else if (ver === '3.2') imageRef = 'ruby:3.2-alpine';
      else imageRef = `ruby:${ver}-alpine`;
    } else if (pId === 'elixir' || pId === 'phoenix') {
      if (ver === 'auto' || ver === '1.18' || !ver) imageRef = 'elixir:1.18-alpine';
      else if (ver === '1.17') imageRef = 'elixir:1.17-alpine';
      else if (ver === '1.16') imageRef = 'elixir:1.16-alpine';
      else imageRef = `elixir:${ver}-alpine`;
    } else if (pId === 'dotnet' || pId === 'csharp' || pId === 'aspnet') {
      if (ver === 'auto' || ver === '9.0' || !ver) imageRef = 'mcr.microsoft.com/dotnet/sdk:9.0-alpine';
      else if (ver === '8.0') imageRef = 'mcr.microsoft.com/dotnet/sdk:8.0-alpine';
      else imageRef = `mcr.microsoft.com/dotnet/sdk:${ver}-alpine`;
    }

    if (detectedServices && detectedServices[selectedBlueprintIndex]) {
      detectedServices[selectedBlueprintIndex].runtime_version = ver;
    }
  }

  onMount(() => {
    choosePreset(presets[0]);
    const projSlug = $page.params.slug || slug || '';
    if (projSlug) {
      fetch(`/api/v1/projects/${encodeURIComponent(projSlug)}`, { credentials: 'include' })
        .then(res => res.ok ? res.json() : { id: projSlug, slug: projSlug, name: projSlug })
        .then(data => { project = data; })
        .catch(() => { project = { id: projSlug, slug: projSlug, name: projSlug }; });
    } else {
      project = { id: 'default', slug: 'default', name: 'Default Project' };
    }

    loadIntegrations().catch(() => {});
    loadProviderRepos('github').catch(() => {});
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
          detectedServices = data.services.map((s: any) => ({
            ...s,
            runtime_version: s.runtime_version || s.runtimeVersion || 'auto'
          }));
          detectedBlueprint = detectedServices[0];
          detectedDatabases = (data.databases || []).map((db: any) => ({
            ...db,
            version: db.version || 'auto'
          }));
          detectedBlueprintSource = data.blueprintType || 'auto-detected';
          applyDetectedService(detectedServices[0], 0);
          activeTab = 'stack';
        }
      }
    } catch {} finally {
      parsingYaml = false;
    }
  }

  function applyDetectedService(svc: any, idx: number) {
    selectedBlueprintIndex = idx;
    detectedBlueprint = svc;
    let svcName = svc.name || name;
    let sSlug = svc.slug || svcSlug;
    if (svc.kind === 'static' || svc.preset === 'static-spa' || svc.preset === 'nginx') {
      const cleaned = cleanStaticName(svcName);
      if (cleaned) {
        svcName = cleaned;
        sSlug = cleaned.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)+/g, '');
      }
    }
    name = svcName;
    svcSlug = sSlug;
    kind = svc.kind || kind;
    rootDirectory = svc.root_dir || svc.rootDir || '.';

    const matchingPreset = presets.find(p => p.id === svc.preset) || 
                          presets.find(p => p.kind === svc.kind) || 
                          presets[0];
    choosePreset(matchingPreset);

    const ver = svc.runtime_version || svc.runtimeVersion || 'auto';
    updateImageRefFromVersion(matchingPreset.id, ver);

    if (svc.build_command) buildCommand = svc.build_command;
    if (svc.buildCommand) buildCommand = svc.buildCommand;
    if (svc.start_command) startCommand = svc.start_command;
    if (svc.startCommand) startCommand = svc.startCommand;
    if (svc.internal_port) internalPort = svc.internal_port;
    if (svc.internalPort) internalPort = svc.internalPort;
    if (svc.cron_schedule) cronSchedule = svc.cron_schedule;
    if (svc.cronSchedule) cronSchedule = svc.cronSchedule;
    if (svc.env_vars && Object.keys(svc.env_vars).length > 0) {
      envVars = Object.entries(svc.env_vars).map(([k, v]) => ({ key: k, value: String(v) }));
    } else if (svc.env && Object.keys(svc.env).length > 0) {
      envVars = Object.entries(svc.env).map(([k, v]) => ({ key: k, value: String(v) }));
    }
    yamlParsedInfo = `Configured "${name}" (${svc.kind.toUpperCase()} | ${svc.env || svc.preset} in ${rootDirectory === '.' ? 'root' : '/' + rootDirectory} on port :${internalPort})`;
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

  function cleanStaticName(raw: string): string {
    return raw
      .replace(/-backend$/i, '')
      .replace(/_backend$/i, '')
      .replace(/-api$/i, '')
      .replace(/_api$/i, '')
      .replace(/-server$/i, '')
      .replace(/_server$/i, '')
      .replace(/-frontend$/i, '')
      .replace(/_frontend$/i, '')
      .replace(/-client$/i, '')
      .replace(/_client$/i, '')
      .replace(/-ui$/i, '')
      .replace(/_ui$/i, '')
      .replace(/-web$/i, '')
      .replace(/_web$/i, '');
  }

  function choosePreset(p: ServicePreset) {
    selectedPreset = p;
    kind = p.kind;
    internalPort = p.port || 80;
    buildCommand = p.defaultBuild || '';
    startCommand = p.defaultStart || '';
    const defVer = p.versions?.find(v => v.default)?.value || 'auto';
    updateImageRefFromVersion(p.id, defVer);

    if (p.kind === 'static' || p.id === 'static-spa' || p.id === 'nginx') {
      if (!slugEdited && name) {
        const cleaned = cleanStaticName(name);
        if (cleaned) {
          name = cleaned;
          svcSlug = cleaned.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)+/g, '');
        }
      }
    }
  }

  $effect(() => {
    if (!slugEdited && name) {
      svcSlug = name.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)+/g, '');
    }
  });

  $effect(() => {
    if (detectedServices && detectedServices[selectedBlueprintIndex]) {
      const s = detectedServices[selectedBlueprintIndex];
      s.name = name;
      s.slug = svcSlug;
      s.internal_port = internalPort;
      s.build_command = buildCommand;
      s.start_command = startCommand;
      s.runtime_version = runtimeVersion;
      if (selectedPreset?.id) s.preset = selectedPreset.id;
      const envMap: Record<string, string> = {};
      for (const item of envVars) {
        if (item.key.trim()) envMap[item.key.trim()] = item.value;
      }
      s.env_vars = envMap;
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
        runtimeVersion: runtimeVersion === 'auto' ? '' : runtimeVersion,
        mem_limit: memoryLimit,
        cpu_limit: cpuLimit,
        pids_limit: pidsLimit,
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
  <title>Deploy Service Studio - kloudsPanel</title>
</svelte:head>

<!-- Studio Header -->
<div class="page-header" style="margin-bottom: 1.5rem;">
  <div style="display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: 1rem;">
    <div style="display: flex; align-items: center; gap: 1rem;">
      <button 
        class="btn btn-secondary" 
        onclick={() => goto(`/projects/${slug}`)} 
        style="padding: 0; width: 38px; height: 38px; min-height: 38px; border-radius: var(--radius-md); display: flex; align-items: center; justify-content: center; flex-shrink: 0;"
        aria-label="Back to Project"
      >
        <ArrowLeft size={18} />
      </button>
      <div>
        <div class="page-breadcrumbs" style="margin-bottom: 0.2rem;">
          <a href="/workspaces">Workspaces</a>
          <span>/</span>
          <a href="/projects/{slug}">{project?.name || slug}</a>
          <span>/</span>
          <span>Deploy Studio</span>
        </div>
        <h1 class="page-title" style="display: flex; align-items: center; gap: 0.5rem; margin: 0; font-size: 1.35rem;">
          <Rocket size={22} style="color: var(--color-accent);" /> Deploy Service Studio
        </h1>
      </div>
    </div>

    <!-- Blueprint Quick Status Badge -->
    {#if detectedServices.length > 0}
      <div style="display: flex; align-items: center; gap: 0.5rem; background: var(--color-surface-subtle); border: 1px solid var(--color-accent); padding: 5px 12px; border-radius: var(--radius-md);">
        <Sparkles size={16} style="color: var(--color-accent);" />
        <span style="font-size: 0.8125rem; font-weight: 600; color: var(--color-ink);">
          {detectedBlueprintSource}: {detectedServices.length} Services ({detectedDatabases.length} DBs)
        </span>
        <button 
          type="button"
          class="badge" 
          style="background: var(--color-accent); color: var(--color-accent-contrast); border: none; cursor: pointer; font-size: 0.7rem; padding: 2px 8px;"
          onclick={() => activeTab = 'stack'}
        >
          View Stack
        </button>
      </div>
    {/if}
  </div>
</div>

<!-- Studio Workflow Tabs Navigation -->
<div style="display: flex; align-items: center; gap: 0.5rem; background: var(--color-surface); padding: 6px; border-radius: var(--radius-lg); border: 1px solid var(--color-border); margin-bottom: 1.5rem; overflow-x: auto;">
  <button 
    type="button"
    class="btn btn-secondary"
    style="
      flex: 1; 
      min-width: 140px; 
      font-size: 0.8125rem; 
      font-weight: {activeTab === 'source' ? '700' : '500'}; 
      background: {activeTab === 'source' ? 'var(--color-surface-subtle)' : 'transparent'}; 
      border-color: {activeTab === 'source' ? 'var(--color-accent)' : 'transparent'};
      color: {activeTab === 'source' ? 'var(--color-ink)' : 'var(--color-ink-muted)'};
      display: flex; 
      align-items: center; 
      justify-content: center; 
      gap: 6px;
    "
    onclick={() => activeTab = 'source'}
  >
    <FolderGit2 size={15} style="color: {activeTab === 'source' ? 'var(--color-accent)' : 'inherit'};" />
    <span>1. Source & Repo</span>
  </button>

  {#if detectedServices.length > 0}
    <button 
      type="button"
      class="btn btn-secondary"
      style="
        flex: 1; 
        min-width: 160px; 
        font-size: 0.8125rem; 
        font-weight: {activeTab === 'stack' ? '700' : '500'}; 
        background: {activeTab === 'stack' ? 'var(--color-surface-subtle)' : 'transparent'}; 
        border-color: {activeTab === 'stack' ? 'var(--color-accent)' : 'transparent'};
        color: {activeTab === 'stack' ? 'var(--color-ink)' : 'var(--color-ink-muted)'};
        display: flex; 
        align-items: center; 
        justify-content: center; 
        gap: 6px;
      "
      onclick={() => activeTab = 'stack'}
    >
      <Sparkles size={15} style="color: var(--color-accent);" />
      <span>2. Blueprint Stack ({detectedServices.length})</span>
    </button>
  {/if}

  <button 
    type="button"
    class="btn btn-secondary"
    style="
      flex: 1; 
      min-width: 150px; 
      font-size: 0.8125rem; 
      font-weight: {activeTab === 'config' ? '700' : '500'}; 
      background: {activeTab === 'config' ? 'var(--color-surface-subtle)' : 'transparent'}; 
      border-color: {activeTab === 'config' ? 'var(--color-accent)' : 'transparent'};
      color: {activeTab === 'config' ? 'var(--color-ink)' : 'var(--color-ink-muted)'};
      display: flex; 
      align-items: center; 
      justify-content: center; 
      gap: 6px;
    "
    onclick={() => activeTab = 'config'}
  >
    <Code size={15} style="color: {activeTab === 'config' ? 'var(--color-accent)' : 'inherit'};" />
    <span>{detectedServices.length > 0 ? '3' : '2'}. Build & Runtime</span>
  </button>

  <button 
    type="button"
    class="btn btn-secondary"
    style="
      flex: 1; 
      min-width: 150px; 
      font-size: 0.8125rem; 
      font-weight: {activeTab === 'environment' ? '700' : '500'}; 
      background: {activeTab === 'environment' ? 'var(--color-surface-subtle)' : 'transparent'}; 
      border-color: {activeTab === 'environment' ? 'var(--color-accent)' : 'transparent'};
      color: {activeTab === 'environment' ? 'var(--color-ink)' : 'var(--color-ink-muted)'};
      display: flex; 
      align-items: center; 
      justify-content: center; 
      gap: 6px;
    "
    onclick={() => activeTab = 'environment'}
  >
    <Sliders size={15} style="color: {activeTab === 'environment' ? 'var(--color-accent)' : 'inherit'};" />
    <span>{detectedServices.length > 0 ? '4' : '3'}. Env & Resources</span>
  </button>
</div>

{#if error}
  <div style="background: var(--color-danger-subtle); border: 1px solid var(--color-danger); color: var(--color-danger); border-radius: var(--radius-md); padding: 0.75rem 1rem; margin-bottom: 1.5rem; display: flex; align-items: center; gap: 0.5rem; font-size: 0.875rem;">
    <AlertTriangle size={16} /> {error}
  </div>
{/if}

<form onsubmit={handleSubmit}>
  <!-- ========================================================================= -->
  <!-- TAB 1: SOURCE & CODEBASE                                                  -->
  <!-- ========================================================================= -->
  {#if activeTab === 'source'}
    <div class="card" style="padding: 1.5rem; margin-bottom: 1.5rem; border-radius: var(--radius-lg);">
      <h3 style="margin: 0 0 1rem 0; font-size: 1.05rem; display: flex; align-items: center; gap: 0.5rem;">
        <FolderGit2 size={18} style="color: var(--color-accent);" /> Select Codebase Source
      </h3>

      <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 260px), 1fr)); gap: 1rem; margin-bottom: 1.5rem;">
        <button 
          type="button" 
          class="card"
          style="
            cursor: pointer; 
            text-align: left; 
            padding: 1.1rem; 
            border: 2px solid {sourceType === 'git_public' ? 'var(--color-accent)' : 'var(--color-border)'}; 
            background: {sourceType === 'git_public' ? 'var(--color-surface-subtle)' : 'var(--color-surface)'};
            border-radius: var(--radius-md);
          "
          onclick={() => sourceType = 'git_public'}
        >
          <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.4rem;">
            <div style="display: flex; align-items: center; gap: 0.5rem; font-weight: 700;">
              <Unlock size={18} style="color: var(--color-accent);" /> Public Git Repo
            </div>
            <span class="badge badge-running" style="font-size: 0.65rem;">Instant</span>
          </div>
          <p class="text-xs text-muted" style="margin: 0;">Clone any public GitHub, Bitbucket, or GitLab URL with automated build detection.</p>
        </button>

        <button 
          type="button" 
          class="card"
          style="
            cursor: pointer; 
            text-align: left; 
            padding: 1.1rem; 
            border: 2px solid {sourceType === 'git_provider' ? 'var(--color-accent)' : 'var(--color-border)'}; 
            background: {sourceType === 'git_provider' ? 'var(--color-surface-subtle)' : 'var(--color-surface)'};
            border-radius: var(--radius-md);
          "
          onclick={() => { sourceType = 'git_provider'; loadProviderRepos(selectedProvider); }}
        >
          <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.4rem;">
            <div style="display: flex; align-items: center; gap: 0.5rem; font-weight: 700;">
              <FolderGit2 size={18} style="color: var(--color-info);" /> Linked Accounts
            </div>
            <span class="badge" style="background:var(--color-info-subtle); color:var(--color-info); font-size: 0.65rem;">GitHub / GitLab</span>
          </div>
          <p class="text-xs text-muted" style="margin: 0;">Browse and select repositories directly from your connected Git accounts.</p>
        </button>

        <button 
          type="button" 
          class="card"
          style="
            cursor: pointer; 
            text-align: left; 
            padding: 1.1rem; 
            border: 2px solid {sourceType === 'image' ? 'var(--color-accent)' : 'var(--color-border)'}; 
            background: {sourceType === 'image' ? 'var(--color-surface-subtle)' : 'var(--color-surface)'};
            border-radius: var(--radius-md);
          "
          onclick={() => sourceType = 'image'}
        >
          <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.4rem;">
            <div style="display: flex; align-items: center; gap: 0.5rem; font-weight: 700;">
              <Server size={18} style="color: var(--color-ink-secondary);" /> Container Image
            </div>
            <span class="badge" style="background:var(--color-surface-subtle); color:var(--color-ink-secondary); font-size: 0.65rem;">Registry</span>
          </div>
          <p class="text-xs text-muted" style="margin: 0;">Deploy pre-built container image directly from Docker Hub or GitHub Container Registry.</p>
        </button>
      </div>

      {#if sourceType === 'git_public'}
        <div style="background: var(--color-surface); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border);">
          <div class="form-group">
            <label for="git-repo-input" class="form-label">Public Git Repository URL</label>
            <div style="position: relative;">
              <input 
                id="git-repo-input" 
                type="url" 
                class="form-input font-mono" 
                placeholder="https://github.com/organization/repository" 
                value={gitRepoUrl} 
                oninput={(e: any) => handleRepoUrlChange(e.target.value)} 
                required 
              />
              {#if parsingYaml}
                <div style="position: absolute; right: 0.75rem; top: 50%; transform: translateY(-50%); display: flex; align-items: center; gap: 0.35rem; font-size: 0.75rem; color: var(--color-accent);">
                  <Loader2 size={13} class="animate-spin" /> Auto-detecting stack...
                </div>
              {/if}
            </div>
            <p class="text-xs text-muted" style="margin-top:0.35rem;">
              Supports any public Git repository (GitHub, Bitbucket, GitLab, Gitea).
            </p>
          </div>

          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem;">
            <div class="form-group" style="margin:0;">
              <label for="git-branch-input" class="form-label">Branch</label>
              <input 
                id="git-branch-input" 
                type="text" 
                class="form-input font-mono" 
                placeholder="main" 
                bind:value={gitBranch} 
                required 
              />
            </div>
            <div class="form-group" style="margin:0;">
              <label for="root-dir-input" class="form-label">Root Directory</label>
              <input 
                id="root-dir-input" 
                type="text" 
                class="form-input font-mono" 
                placeholder="." 
                bind:value={rootDirectory} 
              />
            </div>
          </div>
        </div>

      {:else if sourceType === 'git_provider'}
        <div style="background: var(--color-surface); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border);">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; flex-wrap: wrap; gap: 0.5rem;">
            <div style="display: flex; gap: 0.5rem;">
              <button 
                type="button" 
                class="btn btn-secondary" 
                style="padding: 4px 12px; font-size: 0.8125rem; font-weight: {selectedProvider === 'github' ? '700' : '500'}; background: {selectedProvider === 'github' ? 'var(--color-surface-subtle)' : 'transparent'};"
                onclick={() => { selectedProvider = 'github'; loadProviderRepos('github'); }}
              >
                GitHub
              </button>
              <button 
                type="button" 
                class="btn btn-secondary" 
                style="padding: 4px 12px; font-size: 0.8125rem; font-weight: {selectedProvider === 'bitbucket' ? '700' : '500'}; background: {selectedProvider === 'bitbucket' ? 'var(--color-surface-subtle)' : 'transparent'};"
                onclick={() => { selectedProvider = 'bitbucket'; loadProviderRepos('bitbucket'); }}
              >
                Bitbucket
              </button>
              <button 
                type="button" 
                class="btn btn-secondary" 
                style="padding: 4px 12px; font-size: 0.8125rem; font-weight: {selectedProvider === 'gitlab' ? '700' : '500'}; background: {selectedProvider === 'gitlab' ? 'var(--color-surface-subtle)' : 'transparent'};"
                onclick={() => { selectedProvider = 'gitlab'; loadProviderRepos('gitlab'); }}
              >
                GitLab
              </button>
            </div>

            {#if currentProviderInfo?.connected}
              <div style="display: flex; align-items: center; gap: 0.5rem; background: var(--color-surface-subtle); padding: 3px 10px; border-radius: var(--radius-md); border: 1px solid var(--color-border);">
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
            {:else}
              <button 
                type="button" 
                class="btn btn-primary" 
                style="font-size: 0.8125rem; padding: 4px 14px; display: flex; align-items: center; gap: 6px;"
                onclick={() => authorizeGitOAuth(selectedProvider)}
              >
                <FolderGit2 size={14} /> Connect with {selectedProvider.charAt(0).toUpperCase() + selectedProvider.slice(1)}
              </button>
            {/if}
          </div>

          {#if providerRepos.length === 0}
            <div style="text-align: center; padding: 1.5rem 0;">
              <p class="text-sm text-muted" style="margin-bottom: 0.75rem;">No linked {selectedProvider} repositories found.</p>
              <button 
                type="button" 
                class="btn btn-primary" 
                style="font-size: 0.8125rem; display: inline-flex; align-items: center; gap: 6px;" 
                onclick={() => authorizeGitOAuth(selectedProvider)}
              >
                <FolderGit2 size={14} /> Connect {selectedProvider.charAt(0).toUpperCase() + selectedProvider.slice(1)}
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

            <div style="display: flex; flex-direction: column; gap: 0.5rem; max-height: 240px; overflow-y: auto;">
              {#each providerRepos.filter(r => r.name.toLowerCase().includes(repoSearchQuery.toLowerCase())) as repo}
                <button 
                  type="button"
                  class="card"
                  style="
                    display: flex; 
                    align-items: center; 
                    justify-content: space-between; 
                    padding: 8px 12px; 
                    border: 1px solid {gitRepoUrl === repo.url ? 'var(--color-accent)' : 'var(--color-border)'}; 
                    background: {gitRepoUrl === repo.url ? 'var(--color-surface-subtle)' : 'var(--color-surface)'};
                    border-radius: var(--radius-md);
                    cursor: pointer;
                    text-align: left;
                    width: 100%;
                  "
                  onclick={() => selectProviderRepo(repo)}
                >
                  <div style="min-width: 0;">
                    <div style="font-weight: 700; font-size: 0.875rem; color: var(--color-ink);">{repo.full_name || repo.name}</div>
                    <div class="text-xs text-muted">Branch: {repo.default_branch || 'main'} | Language: {repo.language || 'Multi-language'}</div>
                  </div>
                  <span class="badge" style="background: {gitRepoUrl === repo.url ? 'var(--color-accent)' : 'var(--color-border)'}; color: {gitRepoUrl === repo.url ? 'var(--color-accent-contrast)' : 'var(--color-ink)'}; font-size: 0.72rem;">
                    {gitRepoUrl === repo.url ? 'Selected' : 'Select'}
                  </span>
                </button>
              {/each}
            </div>
          {/if}
        </div>

      {:else}
        <div style="background: var(--color-surface); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border);">
          <div class="form-group" style="margin: 0;">
            <label for="image-source-input" class="form-label">Docker Registry Image Tag</label>
            <input 
              id="image-source-input" 
              type="text" 
              class="form-input font-mono" 
              placeholder="e.g. redis:7.4-alpine, postgres:17-alpine, nginx:alpine" 
              bind:value={imageRef} 
              required 
            />
            <p class="text-xs text-muted" style="margin-top:0.35rem;">
              Full container image identifier from Docker Hub, GHCR, or a public registry.
            </p>
          </div>
        </div>
      {/if}

      <div style="display: flex; justify-content: flex-end; margin-top: 1.5rem;">
        <button 
          type="button" 
          class="btn btn-primary"
          onclick={() => {
            if (detectedServices.length > 0) activeTab = 'stack';
            else activeTab = 'config';
          }}
        >
          <span>Next: Configure Service</span>
          <ChevronRight size={16} />
        </button>
      </div>
    </div>
  {/if}

  <!-- ========================================================================= -->
  <!-- TAB 2: BLUEPRINT MULTI-SERVICE STACK                                      -->
  <!-- ========================================================================= -->
  {#if activeTab === 'stack' && detectedServices.length > 0}
    <div class="card" style="padding: 1.5rem; margin-bottom: 1.5rem; border-radius: var(--radius-lg); border: 1.5px solid var(--color-accent);">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 1.25rem; flex-wrap: wrap; gap: 1rem;">
        <div>
          <div style="display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.25rem;">
            <Sparkles size={20} style="color: var(--color-accent);" />
            <h3 style="margin: 0; font-size: 1.15rem; font-weight: 700;">
              Discovered Full Stack Blueprint ({detectedBlueprintSource})
            </h3>
          </div>
          <p class="text-xs text-muted" style="margin: 0;">
            Review and choose individual runtime versions for each service before launching the entire stack.
          </p>
        </div>

        <div style="display: flex; align-items: center; gap: 0.5rem;">
          <button 
            type="button" 
            class="btn btn-secondary"
            style="font-size: 0.8125rem;"
            onclick={() => { pendingAction = 'blueprint'; showEnvPromptModal = true; }}
          >
            <Sliders size={14} /> Configure Env Vars
          </button>
          <button 
            type="button" 
            class="btn btn-primary" 
            style="font-size: 0.8125rem; padding: 7px 16px; display: flex; align-items: center; gap: 6px;"
            onclick={requestDeployBlueprint}
            disabled={deployingBlueprint}
          >
            {#if deployingBlueprint}
              <Loader2 size={14} class="animate-spin" /> Deploying Stack...
            {:else}
              <Rocket size={14} /> Deploy All {detectedServices.length + detectedDatabases.length} Stack Services
            {/if}
          </button>
        </div>
      </div>

      <!-- Services & Databases List with Individual Version Dropdowns -->
      <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 300px), 1fr)); gap: 0.75rem; margin-bottom: 1.25rem;">
        {#each detectedServices as s, idx}
          <div
            class="card"
            style="
              padding: 12px 14px; 
              background: {selectedBlueprintIndex === idx ? 'var(--color-surface-subtle)' : 'var(--color-surface)'};
              border: 1.5px solid {selectedBlueprintIndex === idx ? 'var(--color-accent)' : 'var(--color-border)'};
              border-radius: var(--radius-md);
              display: flex;
              flex-direction: column;
              gap: 8px;
            "
          >
            <div style="display: flex; align-items: center; justify-content: space-between; gap: 8px;">
              <div style="display: flex; align-items: center; gap: 6px;">
                <FrameworkIcon name={s.preset || s.env || s.kind} size={18} />
                <span style="font-weight: 700; font-size: 0.875rem; color: var(--color-ink);">{s.name}</span>
              </div>
              <button
                type="button"
                class="badge"
                style="background: {selectedBlueprintIndex === idx ? 'var(--color-accent)' : 'var(--color-border)'}; color: {selectedBlueprintIndex === idx ? 'var(--color-accent-contrast)' : 'var(--color-ink)'}; font-size: 0.68rem; border: none; cursor: pointer;"
                onclick={() => { applyDetectedService(s, idx); activeTab = 'config'; }}
              >
                {selectedBlueprintIndex === idx ? 'Editing' : 'Customize'}
              </button>
            </div>

            <div class="text-xs text-muted">
              {s.kind.toUpperCase()} {s.root_dir ? `| /${s.root_dir}` : ''} | Port :{s.internal_port || 8080}
            </div>

            <!-- Individual Runtime Version Selector -->
            <div style="display: flex; align-items: center; justify-content: space-between; gap: 6px; padding-top: 6px; border-top: 1px solid var(--color-border);">
              <span class="text-xs text-muted" style="font-size: 0.72rem; font-weight: 600;">Runtime:</span>
              <select
                class="form-select font-mono text-xs"
                style="padding: 2px 6px; height: 26px; font-size: 0.75rem; width: auto; max-width: 180px;"
                bind:value={s.runtime_version}
                onchange={(e: any) => {
                  if (selectedBlueprintIndex === idx) {
                    updateImageRefFromVersion(s.preset || s.env || s.kind, e.target.value);
                  }
                }}
              >
                {#each getPresetVersions(s.preset || s.env || s.kind) as v}
                  <option value={v.value}>{v.label}</option>
                {/each}
              </select>
            </div>
          </div>
        {/each}

        {#each detectedDatabases as db}
          <div 
            class="card"
            style="
              padding: 12px 14px; 
              border-radius: var(--radius-md); 
              background: var(--color-surface); 
              border: 1px solid var(--color-border);
              display: flex;
              flex-direction: column;
              gap: 8px;
            "
          >
            <div style="display: flex; align-items: center; gap: 0.5rem;">
              <Database size={18} style="color: var(--color-info); flex-shrink: 0;" />
              <span style="font-weight: 700; font-size: 0.875rem; color: var(--color-ink);">{db.name}</span>
            </div>
            <div class="text-xs text-muted">
              Managed {db.engine || 'postgres'} database
            </div>
            <!-- Individual Database Version Selector -->
            <div style="display: flex; align-items: center; justify-content: space-between; gap: 6px; padding-top: 6px; border-top: 1px solid var(--color-border);">
              <span class="text-xs text-muted" style="font-size: 0.72rem; font-weight: 600;">Engine Ver:</span>
              <select
                class="form-select font-mono text-xs"
                style="padding: 2px 6px; height: 26px; font-size: 0.75rem; width: auto; max-width: 180px;"
                bind:value={db.version}
              >
                {#each getDatabaseVersions(db.engine) as v}
                  <option value={v.value}>{v.label}</option>
                {/each}
              </select>
            </div>
          </div>
        {/each}
      </div>

      <div style="display: flex; justify-content: space-between; align-items: center; border-top: 1px solid var(--color-border); padding-top: 1rem;">
        <button type="button" class="btn btn-secondary" onclick={() => activeTab = 'source'}>
          <ChevronLeft size={16} /> Back to Source
        </button>
        <button type="button" class="btn btn-primary" onclick={() => activeTab = 'config'}>
          <span>Customize Active Service</span> <ChevronRight size={16} />
        </button>
      </div>
    </div>
  {/if}

  <!-- ========================================================================= -->
  <!-- TAB 3: BUILD & RUNTIME CONFIGURATION                                      -->
  <!-- ========================================================================= -->
  {#if activeTab === 'config'}
    <div class="card" style="padding: 1.5rem; margin-bottom: 1.5rem; border-radius: var(--radius-lg);">
      <h3 style="margin: 0 0 1rem 0; font-size: 1.05rem; display: flex; align-items: center; gap: 0.5rem;">
        <Code size={18} style="color: var(--color-accent);" /> Service & Runtime Specification
      </h3>

      <!-- Basic Identification -->
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; margin-bottom: 1.5rem;">
        <div class="form-group" style="margin:0;">
          <label for="svc-name-input" class="form-label">Service Name</label>
          <input 
            id="svc-name-input" 
            type="text" 
            class="form-input" 
            placeholder="my-web-app" 
            bind:value={name} 
            required 
          />
        </div>
        <div class="form-group" style="margin:0;">
          <label for="svc-slug-input" class="form-label">Internal Hostname / Slug</label>
          <input 
            id="svc-slug-input" 
            type="text" 
            class="form-input font-mono" 
            placeholder="my-web-app" 
            bind:value={svcSlug} 
            oninput={() => slugEdited = true} 
            required 
          />
        </div>
      </div>

      <!-- Framework / Preset Selector -->
      <div style="margin-bottom: 1.5rem;">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.75rem; flex-wrap: wrap; gap: 0.5rem;">
          <span class="form-label" style="margin: 0; font-weight: 600;">Framework & Runtime Stack</span>
          <div style="display: flex; gap: 0.35rem;">
            <button 
              type="button" 
              class="btn btn-secondary" 
              style="padding: 2px 10px; font-size: 0.75rem; background: {activeCategory === 'all' ? 'var(--color-surface-subtle)' : 'transparent'};" 
              onclick={() => activeCategory = 'all'}
            >
              All
            </button>
            <button 
              type="button" 
              class="btn btn-secondary" 
              style="padding: 2px 10px; font-size: 0.75rem; background: {activeCategory === 'web' ? 'var(--color-surface-subtle)' : 'transparent'};" 
              onclick={() => activeCategory = 'web'}
            >
              Web / APIs
            </button>
            <button 
              type="button" 
              class="btn btn-secondary" 
              style="padding: 2px 10px; font-size: 0.75rem; background: {activeCategory === 'static' ? 'var(--color-surface-subtle)' : 'transparent'};" 
              onclick={() => activeCategory = 'static'}
            >
              Static SPA
            </button>
            <button 
              type="button" 
              class="btn btn-secondary" 
              style="padding: 2px 10px; font-size: 0.75rem; background: {activeCategory === 'worker' ? 'var(--color-surface-subtle)' : 'transparent'};" 
              onclick={() => activeCategory = 'worker'}
            >
              Workers
            </button>
          </div>
        </div>

        <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 200px), 1fr)); gap: 0.6rem; max-height: 220px; overflow-y: auto; padding: 4px;">
          {#each filteredPresets as p}
            <button
              type="button"
              class="card"
              style="
                padding: 10px 12px; 
                text-align: left; 
                cursor: pointer; 
                border: 2px solid {selectedPreset?.id === p.id ? 'var(--color-accent)' : 'var(--color-border)'}; 
                background: {selectedPreset?.id === p.id ? 'var(--color-surface-subtle)' : 'var(--color-surface)'};
                border-radius: var(--radius-md);
                display: flex;
                flex-direction: column;
                gap: 4px;
              "
              onclick={() => choosePreset(p)}
            >
              <div style="display: flex; align-items: center; justify-content: space-between;">
                <FrameworkIcon name={p.id} size={20} />
                <span class="badge" style="font-size: 0.65rem;">{p.badge}</span>
              </div>
              <div style="font-weight: 700; font-size: 0.8125rem; color: var(--color-ink); margin-top: 2px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
                {p.title}
              </div>
            </button>
          {/each}
        </div>
      </div>

      <!-- Runtime Version & Base Image Specification -->
      <div style="background: var(--color-canvas); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border); margin-bottom: 1.5rem;">
        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; margin-bottom: 1rem;">
          <div class="form-group" style="margin: 0;">
            <label for="runtime-version-select" class="form-label">Runtime Version</label>
            <select 
              id="runtime-version-select" 
              class="form-select font-mono text-sm" 
              bind:value={runtimeVersion}
              onchange={(e: any) => updateImageRefFromVersion(selectedPreset?.id, e.target.value)}
            >
              {#each selectedPreset?.versions || getPresetVersions(selectedPreset?.id) as v}
                <option value={v.value}>{v.label}</option>
              {/each}
            </select>
          </div>

          <div class="form-group" style="margin: 0;">
            <label for="image-ref-input" class="form-label">Runtime Base Image (Resolved)</label>
            <input 
              id="image-ref-input" 
              type="text" 
              class="form-input font-mono text-sm" 
              bind:value={imageRef} 
              required 
            />
          </div>
        </div>

        <div style="display: grid; grid-template-columns: 1fr 2fr; gap: 1rem;">
          <div class="form-group" style="margin: 0;">
            <label for="port-input" class="form-label">Internal Port</label>
            <input 
              id="port-input" 
              type="number" 
              class="form-input font-mono text-sm" 
              bind:value={internalPort} 
              required={kind === 'web' || kind === 'static'} 
            />
          </div>
          <div class="form-group" style="margin: 0;">
            <label for="build-cmd-input" class="form-label">Build Command</label>
            <input 
              id="build-cmd-input" 
              type="text" 
              class="form-input font-mono text-sm" 
              bind:value={buildCommand} 
            />
          </div>
        </div>

        <div class="form-group" style="margin: 1rem 0 0 0;">
          <label for="start-cmd-input" class="form-label">Start / Run Command</label>
          <input 
            id="start-cmd-input" 
            type="text" 
            class="form-input font-mono text-sm" 
            bind:value={startCommand} 
          />
        </div>
      </div>

      <div style="display: flex; justify-content: space-between; align-items: center;">
        <button type="button" class="btn btn-secondary" onclick={() => activeTab = detectedServices.length > 0 ? 'stack' : 'source'}>
          <ChevronLeft size={16} /> Previous
        </button>
        <button type="button" class="btn btn-primary" onclick={() => activeTab = 'environment'}>
          <span>Next: Environment & Limits</span> <ChevronRight size={16} />
        </button>
      </div>
    </div>
  {/if}

  <!-- ========================================================================= -->
  <!-- TAB 4: ENVIRONMENT VARIABLES & CONTAINER LIMITS                           -->
  <!-- ========================================================================= -->
  {#if activeTab === 'environment'}
    <div class="card" style="padding: 1.5rem; margin-bottom: 1.5rem; border-radius: var(--radius-lg);">
      <h3 style="margin: 0 0 1rem 0; font-size: 1.05rem; display: flex; align-items: center; gap: 0.5rem;">
        <Sliders size={18} style="color: var(--color-accent);" /> Environment & Resource Limits
      </h3>

      <!-- Container Limits & Non-Root Sandbox -->
      <div style="background: var(--color-canvas); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border); margin-bottom: 1.5rem;">
        <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.75rem;">
          <div style="display: flex; align-items: center; gap: 0.5rem;">
            <ShieldCheck size={18} style="color: var(--color-accent);" />
            <span style="font-size: 0.875rem; font-weight: 700;">Security Hardening & Limits</span>
          </div>
          <span class="badge" style="background: var(--color-success-subtle); color: var(--color-success); font-size: 0.7rem; display: flex; align-items: center; gap: 4px;">
            <ShieldCheck size={12} /> Non-Root Sandbox Active
          </span>
        </div>

        <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; margin-bottom: 0.75rem;">
          <div class="form-group" style="margin: 0;">
            <label for="mem-limit-select" class="form-label" style="font-size: 0.8125rem;">Memory Limit</label>
            <select id="mem-limit-select" class="form-select font-mono text-xs" bind:value={memoryLimit}>
              <option value="256m">256 MB (Micro)</option>
              <option value="512m">512 MB (Default Standard)</option>
              <option value="1g">1 GB (Medium)</option>
              <option value="2g">2 GB (High Performance)</option>
              <option value="4g">4 GB (Extra Large)</option>
              <option value="8g">8 GB (Maximum)</option>
            </select>
          </div>

          <div class="form-group" style="margin: 0;">
            <label for="cpu-limit-select" class="form-label" style="font-size: 0.8125rem;">CPU Limit</label>
            <select id="cpu-limit-select" class="form-select font-mono text-xs" bind:value={cpuLimit}>
              <option value="0.5">0.5 Cores (Eco)</option>
              <option value="1.0">1.0 Core (Default Standard)</option>
              <option value="2.0">2.0 Cores (High Throughput)</option>
              <option value="4.0">4.0 Cores (Maximum)</option>
            </select>
          </div>
        </div>

        <div style="display: flex; gap: 0.5rem; flex-wrap: wrap; margin-top: 0.5rem;">
          <span class="badge" style="background: var(--color-surface); border: 1px solid var(--color-border); font-size: 0.7rem; display: flex; align-items: center; gap: 4px;">
            <Lock size={11} /> Process Limit: {pidsLimit} PIDs
          </span>
          <span class="badge" style="background: var(--color-surface); border: 1px solid var(--color-border); font-size: 0.7rem; display: flex; align-items: center; gap: 4px;">
            <Sliders size={11} /> User 1001 Non-Root
          </span>
        </div>
      </div>

      <!-- Environment Variables Table -->
      <div style="margin-bottom: 1.5rem;">
        <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.75rem;">
          <div style="font-weight: 700; font-size: 0.9375rem;">Environment Variables</div>
          <div style="display: flex; gap: 0.5rem;">
            <button 
              type="button" 
              class="btn btn-secondary" 
              style="padding: 2px 10px; min-height: 28px; font-size: 0.75rem;" 
              onclick={autoFillAllSecrets}
            >
              <Wand2 size={12} /> Generate Secrets
            </button>
            <button 
              type="button" 
              class="btn btn-secondary" 
              style="padding: 2px 10px; min-height: 28px; font-size: 0.75rem;" 
              onclick={addEnv}
            >
              <Plus size={12} /> Add Variable
            </button>
          </div>
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
                  style="flex: 2; border-color: {!env.value && env.key ? 'var(--color-accent)' : 'var(--color-border)'};" 
                />
                <button
                  type="button"
                  class="btn btn-secondary"
                  style="padding: 2px 8px; min-height: 28px; font-size: 0.72rem; display: flex; align-items: center; gap: 4px;"
                  title="Generate secret"
                  onclick={() => env.value = generateRandomSecret(32)}
                >
                  <Wand2 size={12} /> Secret
                </button>
                <button 
                  type="button" 
                  class="btn btn-secondary" 
                  style="padding: 4px; color: var(--color-danger); min-height: 28px;" 
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

      <div style="display: flex; justify-content: space-between; align-items: center; border-top: 1px solid var(--color-border); padding-top: 1rem; flex-wrap: wrap; gap: 0.75rem;">
        <button type="button" class="btn btn-secondary" onclick={() => activeTab = 'config'}>
          <ChevronLeft size={16} /> Back to Build Config
        </button>

        <div style="display: flex; align-items: center; gap: 0.75rem; flex-wrap: wrap;">
          {#if detectedServices.length > 1}
            <button 
              type="button" 
              class="btn btn-secondary" 
              disabled={submitting || !name || !svcSlug}
              onclick={handleSubmit}
            >
              Deploy Only {name || 'Active Service'}
            </button>
            <button 
              type="button" 
              class="btn btn-primary" 
              disabled={deployingBlueprint}
              onclick={requestDeployBlueprint}
              style="padding: 0.625rem 1.75rem; display: flex; align-items: center; gap: 6px;"
            >
              {#if deployingBlueprint}
                <Loader2 size={16} class="animate-spin" /> Deploying Full Stack...
              {:else}
                <Rocket size={16} /> Deploy All {detectedServices.length + detectedDatabases.length} Stack Services
              {/if}
            </button>
          {:else}
            <button 
              type="submit" 
              class="btn btn-primary" 
              disabled={submitting || !name || !svcSlug}
              style="padding: 0.625rem 1.75rem;"
            >
              {#if submitting}
                <Loader2 size={16} class="animate-spin" /> Provisioning & Deploying...
              {:else}
                <Rocket size={16} /> Deploy {name || 'Service'}
              {/if}
            </button>
          {/if}
        </div>
      </div>
    </div>
  {/if}
</form>

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
        <div style="background: var(--color-surface-subtle); border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: 0.75rem 1rem; margin-bottom: 1.25rem; display: flex; align-items: flex-start; gap: 0.75rem;">
          <AlertTriangle size={18} style="color: var(--color-accent); flex-shrink: 0; margin-top: 2px;" />
          <div class="text-xs" style="line-height: 1.5;">
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
            <Wand2 size={13} /> Auto-Generate All Secrets
          </button>
        </div>

        {#if pendingAction === 'blueprint'}
          <div style="display: flex; flex-direction: column; gap: 1rem;">
            {#each detectedServices as svc, sIdx}
              {#if svc.env_vars && Object.keys(svc.env_vars).length > 0}
                <div style="border: 1px solid var(--color-border); border-radius: var(--radius-md); padding: 0.85rem; background: var(--color-surface-subtle);">
                  <div style="font-weight: 700; font-size: 0.8125rem; margin-bottom: 0.6rem; color: var(--color-ink); display: flex; align-items: center; gap: 6px;">
                    <span class="badge" style="background: var(--color-accent); color: var(--color-accent-contrast); font-size: 0.7rem;">{svc.name}</span>
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
                        />
                        <button
                          type="button"
                          class="btn btn-secondary"
                          style="padding: 4px 8px; min-height: 28px; font-size: 0.7rem;"
                          title="Generate random secret"
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
                />
                <button
                  type="button"
                  class="btn btn-secondary"
                  style="padding: 4px 8px; min-height: 28px; font-size: 0.7rem;"
                  title="Generate secret"
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
