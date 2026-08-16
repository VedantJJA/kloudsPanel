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
    Info
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
    // ─── Web / Dynamic Runtimes ─────────────────────────────────────────────
    {
      id: 'node',
      title: 'Node.js (Next.js / Express / Nest / Remix / Astro)',
      description: 'Fullstack JavaScript/TypeScript apps with Node.js & npm/pnpm/yarn/bun',
      category: 'web',
      kind: 'web',
      image: 'node:20-alpine',
      port: 3000,
      badge: 'JavaScript/TS',
      iconColor: '#22c55e',
      iconText: 'Node',
      defaultBuild: 'npm install && npm run build',
      defaultStart: 'npm start',
      versions: [
        { value: 'auto', label: 'Auto-Detect (.node-version / package.json)', default: true },
        { value: '22', label: 'Node.js 22 (Current)' },
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
        { value: 'auto', label: 'Auto-Detect (.python-version / runtime.txt)', default: true },
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
        { value: 'auto', label: 'Auto-Detect (go.mod)', default: true },
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
      image: 'rust:1.82-alpine',
      port: 8080,
      badge: 'Rust Cargo',
      iconColor: '#f97316',
      iconText: 'Rust',
      defaultBuild: 'cargo build --release',
      defaultStart: './app/server',
      versions: [
        { value: 'auto', label: 'Auto-Detect (rust-toolchain.toml)', default: true },
        { value: '1.82', label: 'Rust 1.82 (Latest)' },
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
        { value: 'auto', label: 'Auto-Detect (.java-version / pom.xml)', default: true },
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
        { value: 'auto', label: 'Auto-Detect (composer.json)', default: true },
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
        { value: 'auto', label: 'Auto-Detect (.ruby-version / Gemfile)', default: true },
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
      image: 'elixir:1.17-alpine',
      port: 4000,
      badge: 'Elixir Phoenix',
      iconColor: '#4e2a8e',
      iconText: 'Elixir',
      defaultBuild: 'mix deps.get --only prod && mix compile',
      defaultStart: 'mix phx.server',
      versions: [
        { value: 'auto', label: 'Auto-Detect (.elixir-version / mix.exs)', default: true },
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
        { value: 'auto', label: 'Auto-Detect (deno.json)', default: true },
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
        { value: 'auto', label: 'Auto-Detect (bunfig.toml / package.json)', default: true },
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
      image: 'mcr.microsoft.com/dotnet/sdk:8.0-alpine',
      port: 5000,
      badge: '.NET Core',
      iconColor: '#512bd4',
      iconText: '.NET',
      defaultBuild: 'dotnet restore && dotnet publish -c Release -o /app/publish',
      defaultStart: 'dotnet /app/publish/*.dll',
      versions: [
        { value: 'auto', label: 'Auto-Detect (global.json / .csproj)', default: true },
        { value: '8.0', label: '.NET 8.0 (LTS / Recommended)' },
        { value: '9.0', label: '.NET 9.0' }
      ]
    },
    {
      id: 'scala',
      title: 'Scala (sbt / Play / Akka / Http4s)',
      description: 'Functional and object-oriented JVM services compiled with sbt',
      category: 'web',
      kind: 'web',
      image: 'sbtscala/scala-sbt:eclipse-temurin-jammy-21.0.6_7_1.10.11_3.6.4',
      port: 9000,
      badge: 'Scala sbt',
      iconColor: '#dc322f',
      iconText: 'Scala',
      defaultBuild: 'sbt stage',
      defaultStart: './target/universal/stage/bin/*',
      versions: [
        { value: 'auto', label: 'Auto-Detect (build.sbt)', default: true },
        { value: '21', label: 'Java 21 (Temurin)' },
        { value: '17', label: 'Java 17 (Temurin)' }
      ]
    },
    {
      id: 'kotlin',
      title: 'Kotlin (Ktor / Spring Boot / Micronaut)',
      description: 'Modern concise server-side Kotlin applications built with Gradle',
      category: 'web',
      kind: 'web',
      image: 'eclipse-temurin:21-jdk-alpine',
      port: 8080,
      badge: 'Kotlin Gradle',
      iconColor: '#7f52ff',
      iconText: 'Kotlin',
      defaultBuild: './gradlew build -x test',
      defaultStart: 'java -jar build/libs/*.jar',
      versions: [
        { value: 'auto', label: 'Auto-Detect (build.gradle.kts)', default: true },
        { value: '21', label: 'Kotlin (Java 21 LTS)' },
        { value: '17', label: 'Kotlin (Java 17 LTS)' }
      ]
    },
    {
      id: 'swift',
      title: 'Swift (Vapor / Hummingbird)',
      description: 'Server-side Swift web framework with async/await and non-blocking I/O',
      category: 'web',
      kind: 'web',
      image: 'swift:5.10-jammy',
      port: 8080,
      badge: 'Swift Vapor',
      iconColor: '#f05138',
      iconText: 'Swift',
      defaultBuild: 'swift build -c release',
      defaultStart: '/app/Run serve --env production --hostname 0.0.0.0 --port 8080',
      versions: [
        { value: 'auto', label: 'Auto-Detect (Package.swift)', default: true },
        { value: '5.10', label: 'Swift 5.10 (Jammy)' },
        { value: '5.9', label: 'Swift 5.9' }
      ]
    },
    {
      id: 'haskell',
      title: 'Haskell (Servant / Yesod / Scotty)',
      description: 'Purely functional web APIs built with Stack or Cabal',
      category: 'web',
      kind: 'web',
      image: 'haskell:9.8-slim',
      port: 8080,
      badge: 'Haskell GHC',
      iconColor: '#5d4f85',
      iconText: 'Haskell',
      defaultBuild: 'stack build --copy-bins',
      defaultStart: '/app/server',
      versions: [
        { value: 'auto', label: 'Auto-Detect (stack.yaml / cabal)', default: true },
        { value: '9.8', label: 'GHC 9.8' },
        { value: '9.6', label: 'GHC 9.6' }
      ]
    },
    {
      id: 'clojure',
      title: 'Clojure (Ring / Compojure / Kit)',
      description: 'Dynamic functional Lisp dialect targeting the Java Virtual Machine',
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
        { value: 'auto', label: 'Auto-Detect (project.clj / deps.edn)', default: true },
        { value: '21', label: 'Temurin 21 Lein' }
      ]
    },
    {
      id: 'crystal',
      title: 'Crystal (Kemal / Lucky / Spider-Gazelle)',
      description: 'C-like speed with Ruby-like syntax and static type checking',
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
        { value: 'auto', label: 'Auto-Detect (shard.yml)', default: true },
        { value: 'latest', label: 'Crystal (Latest)' }
      ]
    },
    {
      id: 'zig',
      title: 'Zig (HTTP / Native Backend)',
      description: 'General-purpose programming language and toolchain for robust software',
      category: 'web',
      kind: 'web',
      image: 'archlinux:latest',
      port: 8080,
      badge: 'Zig Native',
      iconColor: '#f7a41d',
      iconText: 'Zig',
      defaultBuild: 'zig build -Doptimize=ReleaseFast',
      defaultStart: './zig-out/bin/server',
      versions: [
        { value: 'auto', label: 'Auto-Detect (build.zig.zon)', default: true },
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
        { value: 'auto', label: 'Auto-Detect (pubspec.yaml)', default: true },
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

    // ─── Static Sites ───────────────────────────────────────────────────────
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

    // ─── Background Workers & Cron ──────────────────────────────────────────
    {
      id: 'worker',
      title: 'Background Worker',
      description: 'Continuous queue consumer, event listener, Celery, BullMQ, or worker task',
      category: 'worker',
      kind: 'worker',
      image: 'node:20-alpine',
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
      image: 'alpine:latest',
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

  onMount(async () => {
    try {
      const projSlug = slug || '';
      const res = await fetch(`/api/v1/projects/${encodeURIComponent(projSlug)}`, { credentials: 'include' });
      if (res.ok) {
        project = await res.json();
      } else {
        project = { id: projSlug, slug: projSlug, name: projSlug };
      }
      try {
        await loadIntegrations();
      } catch {}
      try {
        await loadProviderRepos('github');
      } catch {}
      choosePreset(presets[0]);
    } catch (e) {
      project = { id: slug, slug: slug, name: slug };
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
          detectedBlueprintSource = data.blueprintType || 'auto-detected';
          applyDetectedService(data.services[0], 0);
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
    yamlParsedInfo = `✓ Configured "${name}" (${svc.kind.toUpperCase()} • ${svc.env || svc.preset} in ${rootDirectory === '.' ? 'root' : '/' + rootDirectory} on port :${internalPort})`;
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
    imageRef = p.image;
    internalPort = p.port || 80;
    buildCommand = p.defaultBuild || '';
    startCommand = p.defaultStart || '';
    runtimeVersion = p.versions?.find(v => v.default)?.value || 'auto';

    // For static sites, automatically clean -backend / -api / -server / -frontend suffixes so the site has its clean primary name
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
          <div style="background: {detectedBlueprintSource === 'auto-detected' ? 'rgba(37,99,235,0.06)' : 'rgba(16,185,129,0.06)'}; border: 1.5px solid {detectedBlueprintSource === 'auto-detected' ? 'var(--color-accent)' : '#10b981'}; border-radius: var(--radius-md); padding: 1rem 1.25rem; margin-top: 1rem; overflow: hidden; width: 100%; box-sizing: border-box;">
            <div style="display: flex; justify-content: space-between; align-items: flex-start; gap: 1rem; flex-wrap: wrap; margin-bottom: 0.85rem;">
              <div style="display: flex; align-items: center; gap: 0.75rem; min-width: 0;">
                <Sparkles size={22} style="color: {detectedBlueprintSource === 'auto-detected' ? 'var(--color-accent)' : '#059669'}; flex-shrink: 0;" />
                <div style="min-width: 0;">
                  <div style="font-weight: 700; color: {detectedBlueprintSource === 'auto-detected' ? 'var(--color-ink)' : '#065f46'}; font-size: 0.9375rem; overflow: hidden; text-overflow: ellipsis;">
                    {#if detectedBlueprintSource === 'auto-detected'}
                      Smart Framework & Runtime Detected ({detectedServices.length} Component{detectedServices.length > 1 ? 's' : ''})
                    {:else if detectedBlueprintSource === 'both'}
                      klouds.yaml detected (render.yaml also found) ({detectedServices.length} Service{detectedServices.length > 1 ? 's' : ''})
                    {:else if detectedBlueprintSource === 'klouds.yaml'}
                      klouds.yaml blueprint detected ({detectedServices.length} Service{detectedServices.length > 1 ? 's' : ''})
                    {:else if detectedBlueprintSource === 'render.yaml'}
                      render.yaml blueprint detected ({detectedServices.length} Service{detectedServices.length > 1 ? 's' : ''})
                    {:else}
                      Blueprint detected ({detectedServices.length} Service{detectedServices.length > 1 ? 's' : ''})
                    {/if}
                  </div>
                  <div class="text-xs" style="color: {detectedBlueprintSource === 'auto-detected' ? 'var(--color-ink-muted)' : '#047857'}; margin-top: 2px;">
                    {#if detectedBlueprintSource === 'auto-detected'}
                      Analyzed repository structure and auto-configured runtime, build, and start parameters.
                    {:else}
                      This repository defines a multi-service stack. Review required environment variables below, or deploy all services together.
                    {/if}
                  </div>
                </div>
              </div>

              <div style="display: flex; gap: 0.5rem; align-items: center; flex-wrap: wrap;">
                <button
                  type="button"
                  class="btn btn-secondary"
                  style="font-size: 0.8125rem; padding: 7px 12px; display: flex; align-items: center; gap: 5px; color: #065f46; border-color: #10b981; background: var(--color-surface);"
                  onclick={() => { pendingAction = 'blueprint'; showEnvPromptModal = true; }}
                >
                  <Sliders size={14} /> Configure Env Vars
                </button>
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
              <div style="display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 240px), 1fr)); gap: 0.5rem; width: 100%;">
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
                      min-width: 0;
                      overflow: hidden;
                    "
                    onclick={() => applyDetectedService(s, idx)}
                  >
                    <div style="min-width: 0; overflow: hidden; margin-right: 8px;">
                      <div style="font-weight: 700; font-size: 0.8125rem; color: var(--color-ink); overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">{s.name}</div>
                      <div class="text-xs text-muted" style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
                        {s.kind} • {s.env || s.preset || 'custom'} {s.root_dir ? `• /${s.root_dir}` : ''} • :{s.internal_port}
                      </div>
                    </div>
                    <span class="badge" style="background: rgba(16,185,129,0.2); color: #065f46; font-size: 0.7rem; flex-shrink: 0;">
                      {selectedBlueprintIndex === idx ? 'Active' : 'Customize'}
                    </span>
                  </button>
                {/each}

                {#each detectedDatabases as db}
                  <div style="display: flex; align-items: center; gap: 0.5rem; padding: 8px 12px; border-radius: var(--radius-md); background: var(--color-surface); border: 1px solid var(--color-border); min-width: 0; overflow: hidden;">
                    <Database size={16} style="color: #0369a1; flex-shrink: 0;" />
                    <div style="min-width: 0; overflow: hidden;">
                      <div style="font-weight: 700; font-size: 0.8125rem; color: var(--color-ink); overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">{db.name}</div>
                      <div class="text-xs text-muted" style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">Managed {db.engine || 'postgres'} database</div>
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
            {:else}
              <button 
                type="button" 
                class="btn btn-primary" 
                style="font-size: 0.8125rem; padding: 4px 14px; background: {selectedProvider === 'github' ? '#24292f' : selectedProvider === 'gitlab' ? '#fc6d26' : '#0052cc'}; border-color: transparent; display: flex; align-items: center; gap: 6px;"
                onclick={() => authorizeGitOAuth(selectedProvider)}
              >
                <FolderGit2 size={14} /> Connect with {selectedProvider.charAt(0).toUpperCase() + selectedProvider.slice(1)}
              </button>
            {/if}
          </div>
        </div>

        {#if providerRepos.length === 0}
          <div style="text-align: center; padding: 1.5rem 0;">
            <p class="text-sm text-muted" style="margin-bottom: 0.75rem;">No linked {selectedProvider} repositories found.</p>
            <button 
              type="button" 
              class="btn btn-primary" 
              style="font-size: 0.8125rem; background: {selectedProvider === 'github' ? '#24292f' : selectedProvider === 'gitlab' ? '#fc6d26' : '#0052cc'}; border-color: transparent; display: inline-flex; align-items: center; gap: 6px;" 
              onclick={() => authorizeGitOAuth(selectedProvider)}
            >
              <FolderGit2 size={14} /> Connect {selectedProvider.charAt(0).toUpperCase() + selectedProvider.slice(1)} (1-Click Auth)
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

        {#if detectedServices.length > 0}
          <div style="background: {detectedBlueprintSource === 'auto-detected' ? 'rgba(37,99,235,0.06)' : 'rgba(16,185,129,0.06)'}; border: 1.5px solid {detectedBlueprintSource === 'auto-detected' ? 'var(--color-accent)' : '#10b981'}; border-radius: var(--radius-md); padding: 1rem 1.25rem; margin-top: 1rem;">
            <div style="display: flex; justify-content: space-between; align-items: flex-start; gap: 1rem; flex-wrap: wrap; margin-bottom: 0.85rem;">
              <div style="display: flex; align-items: center; gap: 0.75rem;">
                <Sparkles size={22} style="color: {detectedBlueprintSource === 'auto-detected' ? 'var(--color-accent)' : '#059669'}; flex-shrink: 0;" />
                <div>
                  <div style="font-weight: 700; color: {detectedBlueprintSource === 'auto-detected' ? 'var(--color-ink)' : '#065f46'}; font-size: 0.9375rem;">
                    {#if detectedBlueprintSource === 'auto-detected'}
                      Smart Framework & Runtime Detected ({detectedServices.length} Component{detectedServices.length > 1 ? 's' : ''})
                    {:else}
                      render.yaml / Blueprint detected ({detectedServices.length} Service{detectedServices.length > 1 ? 's' : ''}{detectedDatabases.length > 0 ? `, ${detectedDatabases.length} Database` : ''})
                    {/if}
                  </div>
                  <div class="text-xs" style="color: {detectedBlueprintSource === 'auto-detected' ? 'var(--color-ink-muted)' : '#047857'}; margin-top: 2px;">
                    {#if detectedBlueprintSource === 'auto-detected'}
                      Analyzed repository structure and auto-configured runtime, build, and start parameters.
                    {:else}
                      This repository defines a multi-service stack. Deploy all services together or customize individually.
                    {/if}
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
                  {#if preset.iconSvg}
                    <img src={preset.iconSvg} alt={preset.title} width="22" height="22" style="width: 22px; height: 22px; object-fit: contain; display: block;" />
                  {:else}
                    <FrameworkIcon name={preset.id} size={20} />
                  {/if}
                </div>
                <span class="badge" style="background: rgba(0,0,0,0.04); font-size: 0.7rem; font-weight: 600;">{preset.badge}</span>
              </div>
              {#if selectedPreset?.id === preset.id}
                <span class="badge badge-running" style="padding: 2px 8px; font-size: 0.7rem;"><Check size={11} /> Selected</span>
              {/if}
            </div>
            <div style="display: flex; align-items: center; gap: 6px; font-weight: 700; font-size: 0.9375rem; color: var(--color-ink); margin-top: 0.25rem;">
              <span 
                title={preset.description} 
                style="display: inline-flex; align-items: center; justify-content: center; width: 18px; height: 18px; border-radius: 50%; background: var(--color-canvas); border: 1px solid var(--color-border); color: var(--color-ink-muted); cursor: help; flex-shrink: 0;"
              >
                <Info size={12} />
              </span>
              <span>{preset.title}</span>
            </div>
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

      <!-- Port and Base Image -->
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1.25rem; margin-bottom: 1.25rem;">
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
            placeholder="node:20-alpine" 
            bind:value={imageRef} 
            required 
          />
        </div>
      </div>

      <!-- Dynamic Runtime Version Selection -->
      {#if selectedPreset?.versions && selectedPreset.versions.length > 1}
        <div style="background: var(--color-canvas); padding: 1rem 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border); margin-bottom: 1.25rem;">
          <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.5rem;">
            <div>
              <label for="runtime-version-select" class="form-label" style="margin: 0; font-size: 0.875rem;">
                {selectedPreset.title} Runtime Version
              </label>
              <p class="text-xs text-muted" style="margin: 2px 0 0 0;">
                Choose an explicit version or let kloudsPanel auto-detect from repository manifest files (.node-version, go.mod, etc.).
              </p>
            </div>
          </div>
          <select id="runtime-version-select" class="form-select font-mono text-sm" bind:value={runtimeVersion}>
            {#each selectedPreset.versions as v}
              <option value={v.value}>{v.label}</option>
            {/each}
          </select>
        </div>
      {/if}

      <!-- Container Resource Limits & Security Hardening Box -->
      <div style="background: var(--color-canvas); padding: 1.25rem; border-radius: var(--radius-md); border: 1px solid var(--color-border); margin-bottom: 1.5rem;">
        <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 0.75rem;">
          <div style="display: flex; align-items: center; gap: 0.5rem;">
            <ShieldCheck size={18} style="color: var(--color-accent);" />
            <span style="font-size: 0.875rem; font-weight: 700;">Resource Limits & Security Hardening</span>
          </div>
          <span class="badge" style="background: rgba(16,185,129,0.15); color: #065f46; font-size: 0.7rem;">
            🛡️ Non-Root Sandbox Active
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
          <span class="badge" style="background: var(--color-surface); border: 1px solid var(--color-border); font-size: 0.7rem;">
            🔒 Process Limit: {pidsLimit} PIDs (Fork-Bomb Protected)
          </span>
          <span class="badge" style="background: var(--color-surface); border: 1px solid var(--color-border); font-size: 0.7rem;">
            👤 User 1001 (Non-Root Execution)
          </span>
          <span class="badge" style="background: var(--color-surface); border: 1px solid var(--color-border); font-size: 0.7rem;">
            🛡️ Linux Capabilities Dropped
          </span>
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
