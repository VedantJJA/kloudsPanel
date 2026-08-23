package http

import (
	"fmt"
	"strings"
)

// --- Multi-Language Dockerfile Generator ------------------------------------

// nodePnpmSupportedArchSetup ensures pnpm resolves optional native
// dependencies (rollup, esbuild, lightningcss, swc, etc.) for whatever
// architecture the build is actually running on. Must run BEFORE
// pnpm install. Creates pnpm-workspace.yaml if missing so this works
// for both single-package repos and monorepos.
func nodePnpmSupportedArchSetup() string {
	return `if [ ! -f pnpm-workspace.yaml ]; then printf 'packages:\n  - "."\n' > pnpm-workspace.yaml; fi; ` +
		`sed -i '/linux-arm64.*"-"/d' pnpm-workspace.yaml 2>/dev/null || true; ` +
		`sed -i '/linux-x64.*"-"/d' pnpm-workspace.yaml 2>/dev/null || true; ` +
		`grep -q 'supportedArchitectures:' pnpm-workspace.yaml || { printf '\nsupportedArchitectures:\n  os:\n    - current\n  cpu:\n    - current\n  libc:\n    - current\n' >> pnpm-workspace.yaml; rm -f pnpm-lock.yaml; }`
}

// detectNodePackageManager returns the install and build commands based on lockfile presence.
// order: bun > pnpm > yarn > npm (fallback)
func detectNodeInstallCommand(buildCmd string) string {
	setup := nodePnpmSupportedArchSetup()
	installBlock := fmt.Sprintf(`if [ -f bun.lockb ]; then bun install --frozen-lockfile 2>/dev/null || bun install; \
elif [ -f pnpm-lock.yaml ]; then corepack enable && (%s) && pnpm install --no-frozen-lockfile && (pnpm rebuild 2>/dev/null || true); \
elif [ -f yarn.lock ]; then corepack enable && (yarn install --frozen-lockfile 2>/dev/null || yarn install); \
elif [ -f package-lock.json ]; then npm ci 2>/dev/null || npm install; \
elif [ -f package.json ]; then npm install; fi`, setup)

	if buildCmd != "" {
		if strings.Contains(buildCmd, "install") || strings.Contains(buildCmd, "ci") || strings.Contains(buildCmd, "add") {
			return buildCmd
		}
		return fmt.Sprintf(`%s && \
(npm config set audit false 2>/dev/null || true; npm config set fund false 2>/dev/null || true; npm config set progress false 2>/dev/null || true) && \
%s`, installBlock, buildCmd)
	}

	return fmt.Sprintf(`%s && \
if grep -q '"build":' package.json 2>/dev/null; then \
  if [ -f bun.lockb ]; then bun run build; \
  elif [ -f pnpm-lock.yaml ]; then pnpm run build; \
  elif [ -f yarn.lock ]; then yarn build; \
  else npm run build; fi; \
fi`, installBlock)
}

// detectPythonInstallCommand returns the install command based on dependency file presence.
func detectPythonInstallCommand(buildCmd string) string {
	if buildCmd != "" {
		return buildCmd
	}
	return `if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; \
elif [ -f Pipfile ]; then pip install pipenv && pipenv install --system --deploy; \
elif [ -f pyproject.toml ]; then \
  if [ -f poetry.lock ]; then pip install poetry && poetry config virtualenvs.create false && poetry install --no-dev --no-interaction; \
  elif [ -f pdm.lock ]; then pip install pdm && pdm install --prod --no-self; \
  elif [ -f uv.lock ]; then pip install uv && uv sync --no-dev; \
  else pip install .; fi; \
fi`
}

// buildArgDirectives generates ARG and ENV directives for build-time environment injection (Next.js, Vite, Nuxt, CRA, etc.)
func buildArgDirectives(envMap ...map[string]string) string {
	var sb strings.Builder
	standardArgs := []string{
		"NEXT_PUBLIC_API_URL", "VITE_API_URL", "REACT_APP_API_URL",
		"PUBLIC_API_URL", "NUXT_PUBLIC_API_URL", "ASTRO_PUBLIC_API_URL",
		"API_URL", "BACKEND_URL", "CLIENT_URL", "FRONTEND_URL",
	}
	seen := make(map[string]bool)
	for _, arg := range standardArgs {
		sb.WriteString(fmt.Sprintf("ARG %s\nENV %s=$%s\n", arg, arg, arg))
		seen[arg] = true
	}
	for _, m := range envMap {
		for k := range m {
			kClean := strings.TrimSpace(k)
			if kClean != "" && !seen[kClean] && !strings.ContainsAny(kClean, " \t\r\n=\"'#") {
				sb.WriteString(fmt.Sprintf("ARG %s\nENV %s=$%s\n", kClean, kClean, kClean))
				seen[kClean] = true
			}
		}
	}
	return sb.String()
}

// generateDockerfileForPreset generates a Dockerfile for the given preset.
// runtimeVersion is an optional user/auto-detected version string (e.g. "20", "3.12").
// If empty, falls back to the default version for that preset.
func generateDockerfileForPreset(preset, buildCmd, startCmd string, port int, runtimeVersion string, envMap ...map[string]string) string {
	p := strings.ToLower(preset)

	// Resolve the Docker image tag dynamically
	resolved := resolveRuntimeVersion(p, "", runtimeVersion)
	imageTag := resolved.FullImage

	// Generate non-root user and healthcheck directives
	nonRoot := NonRootDirective(p)
	healthcheck := HealthcheckDirective(port, p)
	buildArgs := buildArgDirectives(envMap...)

	switch p {

	// --- Python --------------------------------------------------------------
	case "python":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = fmt.Sprintf(
				"if [ -f main.py ]; then (uvicorn main:app --host 0.0.0.0 --port %d 2>/dev/null || gunicorn main:app --bind 0.0.0.0:%d --workers 2 2>/dev/null || python main.py); "+
					"elif [ -f app.py ]; then (gunicorn app:app --bind 0.0.0.0:%d --workers 2 2>/dev/null || uvicorn app:app --host 0.0.0.0 --port %d 2>/dev/null || flask run --host=0.0.0.0 --port=%d 2>/dev/null || python app.py); "+
					"elif [ -f manage.py ]; then (python manage.py collectstatic --noinput 2>/dev/null || true) && (gunicorn config.wsgi:application --bind 0.0.0.0:%d --workers 2 2>/dev/null || python manage.py runserver 0.0.0.0:%d); "+
					"else (python -m uvicorn app:app --host 0.0.0.0 --port %d 2>/dev/null || python -m flask run --host=0.0.0.0 --port=%d 2>/dev/null || python server.py || python main.py || python app.py); fi",
				port, port, port, port, port, port, port, port, port)
		} else {
			if strings.Contains(sCmd, "uvicorn") && !strings.Contains(sCmd, "--host") {
				sCmd = sCmd + " --host 0.0.0.0"
			}
			if strings.Contains(sCmd, "gunicorn") && !strings.Contains(sCmd, "--bind") && !strings.Contains(sCmd, "-b") {
				sCmd = fmt.Sprintf("%s --bind 0.0.0.0:%d", sCmd, port)
			}
			if strings.Contains(sCmd, "flask run") && !strings.Contains(sCmd, "--host") {
				sCmd = sCmd + " --host=0.0.0.0"
			}
		}
		bCmd := detectPythonInstallCommand(buildCmd)
		return fmt.Sprintf(`FROM %s
WORKDIR /app
RUN if command -v apk >/dev/null 2>&1; then \
        apk add --no-cache gcc musl-dev libpq-dev curl git; \
    else \
        apt-get update && apt-get install -y --no-install-recommends gcc libpq-dev curl git && rm -rf /var/lib/apt/lists/*; \
    fi
%sCOPY . /app
RUN %s
ENV PORT=%d HOST=0.0.0.0 FLASK_RUN_HOST=0.0.0.0 FLASK_RUN_PORT=%d UVICORN_HOST=0.0.0.0 UVICORN_PORT=%d FASTAPI_HOST=0.0.0.0 FASTAPI_PORT=%d GUNICORN_CMD_ARGS="--bind=0.0.0.0:%d" PYTHONUNBUFFERED=1
EXPOSE %d%s%s
CMD ["sh", "-c", "%s"]
`, imageTag, buildArgs, bCmd, port, port, port, port, port, port, nonRoot, healthcheck, sCmd)

	// --- Node.js -------------------------------------------------------------
	case "node", "nodejs":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "npm start || npm run start || node index.js || node server.js || node app.js || node dist/index.js || node dist/server.js"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = detectNodeInstallCommand("")
		} else {
			if strings.Contains(bCmd, "npm ci") {
				bCmd = strings.ReplaceAll(bCmd, "npm ci", "{ npm ci || npm install; }")
			}
			if strings.Contains(bCmd, "pnpm") {
				setup := nodePnpmSupportedArchSetup()
				bCmd = fmt.Sprintf("%s; %s", setup, bCmd)
				if strings.Contains(bCmd, "--frozen-lockfile") {
					bCmd = strings.ReplaceAll(bCmd, "--frozen-lockfile", "--no-frozen-lockfile")
				}
			}
			if strings.Contains(bCmd, "npm") {
				bCmd = fmt.Sprintf("npm config set audit false 2>/dev/null || true; npm config set fund false 2>/dev/null || true; npm config set progress false 2>/dev/null || true; %s", bCmd)
			}
			if strings.Contains(bCmd, "--frozen-lockfile") {
				bCmd = strings.ReplaceAll(bCmd, "yarn install --frozen-lockfile", "{ yarn install --frozen-lockfile || yarn install; }")
			}
		}
		return fmt.Sprintf(`FROM %s
WORKDIR /app
RUN corepack enable 2>/dev/null || true
%sCOPY . /app
RUN %s
ENV PORT=%d HOST=0.0.0.0 NODE_ENV=production
EXPOSE %d%s%s
CMD ["sh", "-c", "%s"]
`, imageTag, buildArgs, bCmd, port, port, nonRoot, healthcheck, sCmd)

	// --- Go / Golang ---------------------------------------------------------
	case "go", "golang":
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "CGO_ENABLED=0 go build -o server ."
		}
		outputBinary := "server"
		if strings.Contains(bCmd, "-o ") {
			parts := strings.SplitAfter(bCmd, "-o ")
			if len(parts) > 1 {
				fields := strings.Fields(parts[1])
				if len(fields) > 0 {
					outputBinary = fields[0]
				}
			}
		}
		return fmt.Sprintf(`FROM %s AS builder
WORKDIR /app
RUN if command -v apk >/dev/null 2>&1; then \
        apk add --no-cache git ca-certificates tzdata; \
    else \
        apt-get update && apt-get install -y --no-install-recommends git ca-certificates tzdata && rm -rf /var/lib/apt/lists/*; \
    fi
COPY . .
RUN if [ -f go.mod ]; then go mod download; fi
RUN %s
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /app/%s /app/%s
ENV PORT=%d
EXPOSE %d%s
CMD ["/app/%s"]
`, imageTag, bCmd, outputBinary, outputBinary, port, port, healthcheck, outputBinary)

	// --- Java (Maven + Gradle) -----------------------------------------------
	case "java":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "java -jar target/*.jar || java -jar build/libs/*.jar || java -jar app.jar"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = `if [ -f pom.xml ]; then mvn clean package -DskipTests; \
elif [ -f build.gradle ] || [ -f build.gradle.kts ]; then \
  if [ -f gradlew ]; then chmod +x gradlew && ./gradlew build -x test; \
  else gradle build -x test; fi; \
fi`
		}
		javaVersion := resolved.Version
		builderImage := getJavaBaseImage(javaVersion, "build")
		runtimeImage := getJavaBaseImage(javaVersion, "runtime")
		return fmt.Sprintf(`FROM %s AS builder
WORKDIR /app
RUN if command -v apk >/dev/null 2>&1; then \
        apk add --no-cache bash curl git; \
    else \
        apt-get update && apt-get install -y --no-install-recommends bash curl git && rm -rf /var/lib/apt/lists/*; \
    fi && \
    curl -sL https://dlcdn.apache.org/maven/maven-3/3.9.9/binaries/apache-maven-3.9.9-bin.tar.gz | tar xz -C /opt && \
    ln -s /opt/apache-maven-3.9.9/bin/mvn /usr/local/bin/mvn
COPY . .
RUN %s
FROM %s
WORKDIR /app
COPY --from=builder /app /app
ENV PORT=%d
EXPOSE %d%s%s
CMD ["sh", "-c", "%s"]
`, builderImage, bCmd, runtimeImage, port, port, nonRoot, healthcheck, sCmd)

	// --- PHP (Apache + Composer, Laravel/Symfony) ----------------------------
	case "php":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "apache2-foreground"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = `if [ -f composer.json ]; then curl -sS https://getcomposer.org/installer | php && php composer.phar install --no-dev --optimize-autoloader; fi && \
if [ -f artisan ]; then php artisan config:cache 2>/dev/null; php artisan route:cache 2>/dev/null; php artisan view:cache 2>/dev/null; fi`
		}
		return fmt.Sprintf(`FROM %s
WORKDIR /var/www/html
RUN apt-get update && apt-get install -y --no-install-recommends \
    libpq-dev libzip-dev libpng-dev libjpeg-dev libfreetype6-dev unzip curl git && \
    docker-php-ext-configure gd --with-freetype --with-jpeg && \
    docker-php-ext-install pdo pdo_mysql pdo_pgsql zip gd bcmath opcache && \
    rm -rf /var/lib/apt/lists/*
COPY . /var/www/html/
RUN %s
RUN a2enmod rewrite headers && chown -R www-data:www-data /var/www/html
RUN if [ "%d" != "80" ]; then \
    sed -i "s/80/%d/g" /etc/apache2/sites-available/000-default.conf /etc/apache2/ports.conf; fi
ENV APACHE_PORT=%d PORT=%d
EXPOSE %d%s
CMD ["%s"]
`, imageTag, bCmd, port, port, port, port, port, healthcheck, sCmd)

	// --- Ruby (Bundler, Rails, Sinatra, Rack) --------------------------------
	case "ruby":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = fmt.Sprintf(
				"if [ -f config.ru ]; then bundle exec puma -p %d -b 0.0.0.0 2>/dev/null || bundle exec rackup -p %d -o 0.0.0.0; "+
					"elif [ -f bin/rails ]; then bundle exec rails server -p %d -b 0.0.0.0; "+
					"else ruby app.rb; fi",
				port, port, port)
		}
		return fmt.Sprintf(`FROM %s
WORKDIR /app
RUN if command -v apk >/dev/null 2>&1; then \
        apk add --no-cache build-base libpq-dev tzdata nodejs git; \
    else \
        apt-get update && apt-get install -y --no-install-recommends build-essential libpq-dev tzdata nodejs git && rm -rf /var/lib/apt/lists/*; \
    fi
COPY . /app
RUN if [ -f Gemfile ]; then bundle install --without development test; fi
ENV PORT=%d RAILS_ENV=production RACK_ENV=production
EXPOSE %d%s%s
CMD ["sh", "-c", "%s"]
`, imageTag, port, port, nonRoot, healthcheck, sCmd)

	// --- Rust ----------------------------------------------------------------
	case "rust":
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "cargo build --release"
		}
		return fmt.Sprintf(`FROM %s AS builder
WORKDIR /app
RUN if command -v apk >/dev/null 2>&1; then \
        apk add --no-cache musl-dev pkgconfig openssl-dev build-base git ca-certificates; \
    else \
        apt-get update && apt-get install -y --no-install-recommends pkg-config libssl-dev build-essential git ca-certificates && rm -rf /var/lib/apt/lists/*; \
    fi
COPY . .
RUN %s && \
    binary=$(find target/release -maxdepth 1 -type f -executable ! -name "*.d" ! -name "*.so" | head -1) && \
    if [ -n "$binary" ]; then cp "$binary" /app/server; else echo "No binary found" && exit 1; fi
FROM alpine:3.21
RUN apk add --no-cache libgcc ca-certificates
WORKDIR /app
COPY --from=builder /app/server /app/server
ENV PORT=%d
EXPOSE %d%s
CMD ["/app/server"]
`, imageTag, bCmd, port, port, healthcheck)

	// --- Elixir / Phoenix ----------------------------------------------------
	case "elixir", "phoenix":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "mix phx.server || mix run --no-halt || elixir --sname app -S mix run --no-halt"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "mix deps.get --only prod && mix compile && (mix assets.deploy 2>/dev/null || true) && (mix phx.digest 2>/dev/null || true)"
		}
		return fmt.Sprintf(`FROM %s AS builder
WORKDIR /app
RUN apk add --no-cache build-base git
ENV MIX_ENV=prod
COPY . .
RUN mix local.hex --force && mix local.rebar --force && if [ -f mix.exs ]; then mix deps.get --only prod; fi
RUN %s
FROM %s
WORKDIR /app
RUN apk add --no-cache libstdc++ openssl ncurses-libs
COPY --from=builder /app /app
ENV MIX_ENV=prod PORT=%d
EXPOSE %d%s%s
CMD ["sh", "-c", "%s"]
`, imageTag, bCmd, imageTag, port, port, nonRoot, healthcheck, sCmd)

	// --- Deno ----------------------------------------------------------------
	case "deno":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "deno run --allow-net --allow-env --allow-read main.ts || deno run --allow-net --allow-env --allow-read main.js || deno run --allow-net --allow-env --allow-read mod.ts || deno run --allow-all src/main.ts"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "deno cache main.ts 2>/dev/null || deno cache mod.ts 2>/dev/null || deno cache src/main.ts 2>/dev/null || true"
		}
		return fmt.Sprintf(`FROM %s
WORKDIR /app
COPY . /app
RUN %s
ENV PORT=%d DENO_DIR=/app/.deno
EXPOSE %d%s%s
CMD ["sh", "-c", "%s"]
`, imageTag, bCmd, port, port, nonRoot, healthcheck, sCmd)

	// --- Bun -----------------------------------------------------------------
	case "bun":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "bun run start || bun run index.ts || bun run index.js || bun run src/index.ts || bun run server.ts"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "if [ -f bun.lockb ] || [ -f package.json ]; then bun install --frozen-lockfile 2>/dev/null || bun install; fi && " +
				"if grep -q '\"build\":' package.json 2>/dev/null; then bun run build; fi"
		}
		return fmt.Sprintf(`FROM %s
WORKDIR /app
COPY . /app
RUN %s
ENV PORT=%d NODE_ENV=production
EXPOSE %d%s%s
CMD ["sh", "-c", "%s"]
`, imageTag, bCmd, port, port, nonRoot, healthcheck, sCmd)

	// --- .NET / C# (ASP.NET Core) --------------------------------------------
	case "dotnet", "csharp", "aspnet":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "dotnet $(find /app/publish -name '*.dll' -maxdepth 1 | head -1) --urls http://0.0.0.0:${PORT}"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "dotnet restore && dotnet publish -c Release -o /app/publish"
		}
		dotnetVersion := resolved.Version
		sdkImage := getDotnetBaseImage(dotnetVersion, "build")
		aspImage := getDotnetBaseImage(dotnetVersion, "runtime")
		return fmt.Sprintf(`FROM %s AS builder
WORKDIR /app
COPY . .
RUN %s
FROM %s
WORKDIR /app
COPY --from=builder /app/publish /app/publish
ENV ASPNETCORE_URLS=http://+:%d DOTNET_RUNNING_IN_CONTAINER=true PORT=%d
EXPOSE %d%s%s
CMD ["sh", "-c", "%s"]
`, sdkImage, bCmd, aspImage, port, port, port, nonRoot, healthcheck, sCmd)

	// --- Scala / sbt ---------------------------------------------------------
	case "scala", "sbt":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "java -jar target/universal/stage/lib/*.jar || java -jar target/scala-*/app.jar || ./target/universal/stage/bin/*"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "sbt stage 2>/dev/null || sbt assembly 2>/dev/null || sbt package"
		}
		return fmt.Sprintf(`FROM sbtscala/scala-sbt:eclipse-temurin-jammy-21.0.6_7_1.10.11_3.6.4 AS builder
WORKDIR /app
COPY . .
RUN %s
FROM %s
WORKDIR /app
COPY --from=builder /app/target /app/target
ENV PORT=%d
EXPOSE %d%s%s
CMD ["sh", "-c", "%s"]
`, bCmd, getJavaBaseImage(resolved.Version, "runtime"), port, port, nonRoot, healthcheck, sCmd)

	// --- Kotlin (Ktor / Spring Boot via Gradle) ------------------------------
	case "kotlin", "ktor":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "java -jar build/libs/*-all.jar || java -jar build/libs/*.jar"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "if [ -f gradlew ]; then chmod +x gradlew && ./gradlew build -x test; else gradle build -x test; fi"
		}
		kotlinJavaVer := resolved.Version
		return fmt.Sprintf(`FROM %s AS builder
WORKDIR /app
COPY . .
RUN %s
FROM %s
WORKDIR /app
COPY --from=builder /app/build/libs /app/build/libs
ENV PORT=%d
EXPOSE %d%s%s
CMD ["sh", "-c", "%s"]
`, getJavaBaseImage(kotlinJavaVer, "build"), bCmd, getJavaBaseImage(kotlinJavaVer, "runtime"), port, port, nonRoot, healthcheck, sCmd)

	// --- Swift / Vapor -------------------------------------------------------
	case "swift", "vapor":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = fmt.Sprintf("./Run serve --hostname 0.0.0.0 --port %d 2>/dev/null || ./app --hostname 0.0.0.0 --port %d", port, port)
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "swift build -c release"
		}
		return fmt.Sprintf(`FROM %s AS builder
WORKDIR /app
COPY . .
RUN %s && \
    binary=$(find .build/release -maxdepth 1 -type f -executable | head -1) && \
    cp "$binary" /app/Run
FROM ubuntu:22.04
RUN apt-get update && apt-get install -y --no-install-recommends libcurl4 libxml2 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/Run /app/Run
ENV PORT=%d
EXPOSE %d%s
CMD ["sh", "-c", "%s"]
`, imageTag, bCmd, port, port, healthcheck, sCmd)

	// --- Haskell (Stack / Cabal) ---------------------------------------------
	case "haskell":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "$(find .stack-work/install -type f -executable | head -1) || $(find dist-newstyle -type f -executable | head -1)"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "if [ -f stack.yaml ]; then stack build; else cabal build; fi"
		}
		return fmt.Sprintf(`FROM %s AS builder
WORKDIR /app
COPY . .
RUN %s && \
    binary=$(find . -type f -executable -name "*.exe" -o -type f -executable ! -name "*.so" ! -name "*.a" | grep -v dist-newstyle/tmp | head -1) && \
    cp "$binary" /app/server
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libgmp10 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/server /app/server
ENV PORT=%d
EXPOSE %d%s
CMD ["sh", "-c", "%s"]
`, imageTag, bCmd, port, port, healthcheck, sCmd)

	// --- Clojure (Leiningen / deps.edn) --------------------------------------
	case "clojure":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "java -jar target/*-standalone.jar || java -jar target/app.jar || lein run"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "if [ -f project.clj ]; then lein uberjar; elif [ -f deps.edn ]; then clojure -T:build uber 2>/dev/null || clojure -M -m app.core; fi"
		}
		return fmt.Sprintf(`FROM %s AS builder
WORKDIR /app
COPY . .
RUN %s
FROM eclipse-temurin:21-jre-alpine
WORKDIR /app
COPY --from=builder /app/target /app/target
COPY --from=builder /app /app
ENV PORT=%d
EXPOSE %d%s%s
CMD ["sh", "-c", "%s"]
`, imageTag, bCmd, port, port, nonRoot, healthcheck, sCmd)

	// --- Crystal -------------------------------------------------------------
	case "crystal":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "./app"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "shards install && crystal build src/*.cr --release -o app"
		}
		return fmt.Sprintf(`FROM %s AS builder
WORKDIR /app
COPY . .
RUN %s
FROM ubuntu:22.04
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libevent-dev libpcre3 && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/app /app/app
ENV PORT=%d
EXPOSE %d%s
CMD ["%s"]
`, imageTag, bCmd, port, port, healthcheck, sCmd)

	// --- Zig -----------------------------------------------------------------
	case "zig":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "./zig-out/bin/* || ./app"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "zig build -Doptimize=ReleaseSafe"
		}
		return fmt.Sprintf(`FROM archlinux:latest AS builder
WORKDIR /app
RUN pacman -Sy --noconfirm zig
COPY . .
RUN %s
FROM alpine:3.21
RUN apk add --no-cache libgcc
WORKDIR /app
COPY --from=builder /app/zig-out/bin /app/
ENV PORT=%d
EXPOSE %d%s
CMD ["sh", "-c", "%s"]
`, bCmd, port, port, healthcheck, sCmd)

	// --- Dart (Shelf / dart_frog) --------------------------------------------
	case "dart":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "./server || dart run bin/server.dart || dart run bin/main.dart"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "dart pub get && dart compile exe bin/server.dart -o server 2>/dev/null || dart compile exe bin/main.dart -o server 2>/dev/null || true"
		}
		return fmt.Sprintf(`FROM %s AS builder
WORKDIR /app
COPY . .
RUN %s
FROM alpine:3.21
RUN apk add --no-cache libgcc gcompat
WORKDIR /app
COPY --from=builder /app/server /app/server
ENV PORT=%d
EXPOSE %d%s
CMD ["sh", "-c", "%s"]
`, imageTag, bCmd, port, port, healthcheck, sCmd)

	// --- Static / SPA / Nginx ------------------------------------------------
	case "static", "static-spa", "nginx":
		bCmd := detectNodeInstallCommand(buildCmd)
		if strings.Contains(bCmd, "npm ci") {
			bCmd = strings.ReplaceAll(bCmd, "npm ci", "{ npm ci || npm install; }")
		}
		if strings.Contains(bCmd, "pnpm") && !strings.Contains(bCmd, "pnpm-workspace.yaml") {
			setup := nodePnpmSupportedArchSetup()
			bCmd = fmt.Sprintf("%s; %s", setup, bCmd)
			if strings.Contains(bCmd, "--frozen-lockfile") {
				bCmd = strings.ReplaceAll(bCmd, "--frozen-lockfile", "--no-frozen-lockfile")
			}
		}
		if strings.Contains(bCmd, "--frozen-lockfile") {
			bCmd = strings.ReplaceAll(bCmd, "yarn install --frozen-lockfile", "{ yarn install --frozen-lockfile || yarn install; }")
		}
			return fmt.Sprintf(`FROM node:22-bookworm-slim AS builder
WORKDIR /app
RUN corepack enable 2>/dev/null || true
%sCOPY . ./
RUN %s
RUN mkdir -p /dist && \
    FOUND="" && \
    for candidate in dist/public dist build out public .output/public build/client .svelte-kit/output/client dist/browser dist/*/browser _site artifacts/*/dist/public artifacts/*/dist packages/*/dist apps/*/dist client/dist frontend/dist; do \
        if [ -d "$candidate" ] && [ -f "$candidate/index.html" ]; then \
            FOUND="$candidate"; \
            break; \
        fi; \
    done && \
    if [ -z "$FOUND" ]; then \
        FOUND=$(find . -maxdepth 6 -name "index.html" -not -path "*/node_modules/*" -not -path "*/.git/*" -not -path "*/.pnpm/*" -not -path "*/dist/*" -exec dirname {} \; 2>/dev/null | sort | head -1); \
    fi && \
    if [ -z "$FOUND" ]; then \
        for candidate in dist build out public; do \
            FOUND=$(find . -maxdepth 5 -type d -name "$candidate" -not -path "*/node_modules/*" -not -path "*/.git/*" -not -path "*/.pnpm/*" 2>/dev/null | head -1); \
            if [ -n "$FOUND" ]; then break; fi; \
        done; \
    fi && \
    if [ -n "$FOUND" ] && [ -d "$FOUND" ]; then \
        echo "Found web build output directory: $FOUND"; \
        cp -a "$FOUND"/. /dist/; \
    else \
        echo "No build directory found, copying root"; \
        cp -a . /dist/; \
    fi && \
    if [ ! -f /dist/index.html ]; then \
        FOUND_HTML=$(find . -maxdepth 6 -name "index.html" -not -path "*/node_modules/*" -not -path "*/.git/*" 2>/dev/null | head -1); \
        if [ -n "$FOUND_HTML" ]; then \
            cp "$FOUND_HTML" /dist/index.html; \
        else \
            echo '<!DOCTYPE html><html><head><meta charset="utf-8"><title>App</title></head><body><div id="root"></div><div id="app"></div></body></html>' > /dist/index.html; \
        fi; \
    fi

FROM nginx:alpine
RUN rm -rf /etc/nginx/conf.d/default.conf
RUN printf 'server {\n    listen 80 default_server;\n    listen [::]:80 default_server;\n    server_name _;\n    root /usr/share/nginx/html;\n    index index.html index.htm;\n    include /etc/nginx/mime.types;\n    default_type application/octet-stream;\n    location / {\n        try_files $uri $uri/ /index.html;\n    }\n    location ~* \\.(?:css|js|mjs|map|jpe?g|png|gif|ico|svg|webp|avif|woff2?|ttf|eot|wasm|json)$ {\n        expires 1y;\n        add_header Cache-Control "public, max-age=31536000, immutable";\n        try_files $uri =404;\n    }\n}\n' > /etc/nginx/conf.d/default.conf
COPY --from=builder /dist /usr/share/nginx/html
RUN chmod -R 755 /usr/share/nginx/html 2>/dev/null || true
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`, buildArgs, bCmd)

	// --- Dockerfile (user-provided) ------------------------------------------
	case "dockerfile":
		return ""

	// --- Default fallback ----------------------------------------------------
	default:
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "python app.py || python main.py || python server.py || node server.js || node index.js || ./server || nginx -g 'daemon off;'"
		}
		return fmt.Sprintf(`FROM python:3.12-slim
WORKDIR /app
COPY . /app
RUN if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; \
    elif [ -f pyproject.toml ]; then pip install --no-cache-dir .; \
    elif [ -f package.json ]; then apt-get update && apt-get install -y nodejs npm && npm install; fi
EXPOSE %d
CMD ["sh", "-c", "%s"]
`, port, sCmd)
	}
}
