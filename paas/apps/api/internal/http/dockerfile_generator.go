package http

import (
	"fmt"
	"strings"
)

// ─── Multi-Language Dockerfile Generator ─────────────────────────────────────

func generateDockerfileForPreset(preset, buildCmd, startCmd string, port int, proxyDirective string) string {
	switch strings.ToLower(preset) {
	case "python":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = fmt.Sprintf("if [ -f main.py ]; then (uvicorn main:app --host 0.0.0.0 --port %d || gunicorn main:app --bind 0.0.0.0:%d --workers 2 || python main.py); elif [ -f app.py ]; then (gunicorn app:app --bind 0.0.0.0:%d --workers 2 || uvicorn app:app --host 0.0.0.0 --port %d || flask run --host=0.0.0.0 --port=%d || python app.py); else (python -m uvicorn app:app --host 0.0.0.0 --port %d || python -m flask run --host=0.0.0.0 --port=%d || python server.py || python main.py || python app.py); fi", port, port, port, port, port, port, port)
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
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; elif [ -f Pipfile ]; then pip install pipenv && pipenv install --system --deploy; elif [ -f pyproject.toml ]; then pip install .; fi"
		}
		return fmt.Sprintf(`FROM python:3.11-slim
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends gcc libpq-dev curl && rm -rf /var/lib/apt/lists/*
COPY . /app
RUN %s
ENV PORT=%d HOST=0.0.0.0 FLASK_RUN_HOST=0.0.0.0 FLASK_RUN_PORT=%d UVICORN_HOST=0.0.0.0 UVICORN_PORT=%d FASTAPI_HOST=0.0.0.0 FASTAPI_PORT=%d GUNICORN_CMD_ARGS="--bind=0.0.0.0:%d" PYTHONUNBUFFERED=1
EXPOSE %d
CMD ["sh", "-c", "%s"]
`, bCmd, port, port, port, port, port, port, port, sCmd)

	case "node", "nodejs":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "npm start || node index.js || node server.js || node app.js || node dist/index.js || node dist/server.js"
		}
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "if [ -f package.json ]; then (npm ci || npm install); fi && if grep -q '\"build\":' package.json 2>/dev/null; then npm run build; fi"
		} else if strings.Contains(bCmd, "npm ci") {
			bCmd = strings.ReplaceAll(bCmd, "npm ci", "(npm ci || npm install)")
		}
		return fmt.Sprintf(`FROM node:20-alpine
WORKDIR /app
COPY . /app
RUN %s
ENV PORT=%d HOST=0.0.0.0 NODE_ENV=production
EXPOSE %d
CMD ["sh", "-c", "%s"]
`, bCmd, port, port, sCmd)

	case "go", "golang":
		bCmd := buildCmd
		if bCmd == "" {
			bCmd = "CGO_ENABLED=0 go build -o server ."
		}
		return fmt.Sprintf(`FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN %s
FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/server /app/server
EXPOSE %d
CMD ["/app/server"]
`, bCmd, port)

	case "java":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "java -jar target/*.jar || java -jar build/libs/*.jar"
		}
		return fmt.Sprintf(`FROM maven:3.9-eclipse-temurin-21-alpine AS builder
WORKDIR /app
COPY . .
RUN if [ -f pom.xml ]; then mvn clean package -DskipTests; fi
FROM eclipse-temurin:21-jdk-alpine
WORKDIR /app
COPY --from=builder /app /app
EXPOSE %d
CMD ["sh", "-c", "%s"]
`, port, sCmd)

	case "php":
		return fmt.Sprintf(`FROM php:8.3-apache
COPY . /var/www/html/
RUN a2enmod rewrite
EXPOSE 80
CMD ["apache2-foreground"]
`)

	case "ruby":
		sCmd := startCmd
		if sCmd == "" {
			sCmd = "bundle exec rackup -p 3000 -o 0.0.0.0 || ruby app.rb"
		}
		return fmt.Sprintf(`FROM ruby:3.3-alpine
WORKDIR /app
RUN apk add --no-cache build-base
COPY . /app
RUN if [ -f Gemfile ]; then bundle install; fi
EXPOSE %d
CMD ["sh", "-c", "%s"]
`, port, sCmd)

	case "rust":
		return fmt.Sprintf(`FROM rust:1.77-alpine AS builder
WORKDIR /app
RUN apk add --no-cache musl-dev
COPY . .
RUN cargo build --release
FROM alpine:3.21
WORKDIR /app
COPY --from=builder /app/target/release/* /app/
EXPOSE %d
CMD ["sh", "-c", "./$(ls -p | grep -v / | head -n 1)"]
`, port)

	case "static", "static-spa", "nginx":
		bCmd := buildCmd
		if bCmd != "" {
			if strings.Contains(bCmd, "npm ci") {
				bCmd = strings.ReplaceAll(bCmd, "npm ci", "(npm ci || npm install)")
			}
			return fmt.Sprintf(`FROM node:20-alpine AS builder
WORKDIR /app
RUN apk add --no-cache bash curl
RUN corepack enable 2>/dev/null || npm install -g pnpm@latest yarn@latest 2>/dev/null || true
ARG VITE_API_URL
ENV VITE_API_URL=$VITE_API_URL
ARG API_URL
ENV API_URL=$API_URL
ARG REACT_APP_API_URL
ENV REACT_APP_API_URL=$REACT_APP_API_URL
ARG NEXT_PUBLIC_API_URL
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
COPY . ./
RUN %s
RUN mkdir -p /dist && \
    if [ -d artifacts/*/dist/public ]; then cp -a artifacts/*/dist/public/. /dist/; \
    elif [ -d dist/public ]; then cp -a dist/public/. /dist/; \
    elif [ -d dist ]; then cp -a dist/. /dist/; \
    elif [ -d build ]; then cp -a build/. /dist/; \
    elif [ -d public ]; then cp -a public/. /dist/; \
    elif [ -d out ]; then cp -a out/. /dist/; \
    else cp -a . /dist/; fi

FROM nginx:alpine
COPY nginx.default.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`, bCmd)
		}
		return fmt.Sprintf(`FROM nginx:alpine
COPY nginx.default.conf /etc/nginx/conf.d/default.conf
COPY . /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`)

	default:
		sCmd := startCmd
		if sCmd == "" {
			sCmd = fmt.Sprintf("python app.py || python main.py || python server.py || node server.js || node index.js || ./server || nginx -g 'daemon off;'")
		}
		return fmt.Sprintf(`FROM python:3.11-slim
WORKDIR /app
COPY . /app
RUN if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; elif [ -f pyproject.toml ]; then pip install --no-cache-dir .; fi
EXPOSE %d
CMD ["sh", "-c", "%s"]
`, port, sCmd)
	}
}
