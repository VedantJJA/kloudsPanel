<script lang="ts">
  interface Props {
    name?: string;
    size?: number;
    class?: string;
  }

  let { name = 'node', size = 20, class: className = '' }: Props = $props();

  const iconMap: Record<string, string> = {
    // Node / JS / TS
    node: '/icons/nodejs.svg',
    nodejs: '/icons/nodejs.svg',
    javascript: '/icons/nodejs.svg',
    typescript: '/icons/nodejs.svg',
    express: '/icons/nodejs.svg',
    nextjs: '/icons/nodejs.svg',
    astro: '/icons/nodejs.svg',
    remix: '/icons/nodejs.svg',
    nest: '/icons/nodejs.svg',
    
    // Python
    python: '/icons/python.svg',
    django: '/icons/python.svg',
    flask: '/icons/python.svg',
    fastapi: '/icons/python.svg',
    celery: '/icons/python.svg',

    // Go
    go: '/icons/golang.svg',
    golang: '/icons/golang.svg',
    fiber: '/icons/golang.svg',
    gin: '/icons/golang.svg',

    // Rust
    rust: '/icons/rust.svg',
    cargo: '/icons/rust.svg',
    actix: '/icons/rust.svg',
    axum: '/icons/rust.svg',

    // Java / Kotlin / Scala / Clojure
    java: '/icons/java.svg',
    spring: '/icons/java.svg',
    quarkus: '/icons/java.svg',
    kotlin: '/icons/kotlin.svg',
    scala: '/icons/scala.svg',
    clojure: '/icons/clojure.svg',

    // PHP / Ruby
    php: '/icons/php.svg',
    laravel: '/icons/php.svg',
    symfony: '/icons/php.svg',
    ruby: '/icons/ruby.svg',
    rails: '/icons/ruby.svg',

    // Elixir / Erlang
    elixir: '/icons/elixir.svg',
    phoenix: '/icons/elixir.svg',

    // Modern JS / TS Engines
    deno: '/icons/deno.svg',
    bun: '/icons/bun.svg',

    // .NET / C#
    dotnet: '/icons/dotnet.svg',
    'dot-net': '/icons/dotnet.svg',
    csharp: '/icons/dotnet.svg',

    // Native & Systems
    swift: '/icons/swift.svg',
    vapor: '/icons/swift.svg',
    haskell: '/icons/haskell.svg',
    crystal: '/icons/crystal.svg',
    zig: '/icons/zig.svg',
    dart: '/icons/dart.svg',
    flutter: '/icons/dart.svg',

    // Frontend & Web Ingress
    react: '/icons/react.svg',
    static: '/icons/react.svg',
    'static-spa': '/icons/react.svg',
    vue: '/icons/react.svg',
    svelte: '/icons/react.svg',
    nginx: '/icons/nginx.svg',
    docker: '/icons/docker.svg',
    dockerfile: '/icons/docker.svg',

    // Databases
    postgres: '/icons/postgres.svg',
    postgresql: '/icons/postgres.svg',
    mysql: '/icons/mysql.svg',
    redis: '/icons/redis.svg',
    mongo: '/icons/mongodb.svg',
    mongodb: '/icons/mongodb.svg',
    clickhouse: '/icons/clickhouse.svg',

    // Git Providers
    github: '/icons/github.svg',
    gitlab: '/icons/gitlab.svg',
    bitbucket: '/icons/bitbucket.svg',

    // Fallbacks
    worker: '/icons/nodejs.svg',
    cron: '/icons/nodejs.svg',
    'cron-job': '/icons/nodejs.svg'
  };

  const cleanName = $derived((name || '').toLowerCase().trim());
  const iconSrc = $derived(iconMap[cleanName] || '/icons/nodejs.svg');
</script>

<img
  src={iconSrc}
  alt="{cleanName} logo"
  width={size}
  height={size}
  class={className}
  style="width: {size}px; height: {size}px; object-fit: contain; display: inline-block; vertical-align: middle; flex-shrink: 0;"
  loading="lazy"
/>
