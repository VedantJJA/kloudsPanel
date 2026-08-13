.PHONY: all dev build test lint fmt vet clean

GOFLAGS := -trimpath

all: build

## Go targets
go-build:
	go build $(GOFLAGS) -o apps/api/bin/api ./apps/api/cmd/api
	go build $(GOFLAGS) -o apps/agent/bin/agent ./apps/agent/cmd/agent

go-test:
	go test ./apps/api/... ./apps/agent/...

go-lint:
	golangci-lint run ./apps/api/... ./apps/agent/...

go-fmt:
	gofmt -w -s apps/api apps/agent

go-vet:
	go vet ./apps/api/... ./apps/agent/...

## Node targets
node-install:
	cmd /c pnpm install

node-build:
	cmd /c pnpm -r run build

node-check:
	cmd /c pnpm -r run check

node-lint:
	cmd /c pnpm -r run lint

## Combined
build: go-build node-build

test: go-test
	cmd /c pnpm -r run test

lint: go-lint node-lint

fmt: go-fmt

clean:
	del /f /q apps\api\bin\api.exe apps\agent\bin\agent.exe 2>nul || true
	cmd /c pnpm -r exec rimraf dist .svelte-kit build

## Migration helpers
migrate-sqlite:
	go run ./apps/api/cmd/migrate -db sqlite -dir apps/api/migrations/sqlite

migrate-postgres:
	go run ./apps/api/cmd/migrate -db postgres -dir apps/api/migrations/postgres

## Docker helpers
compose-dev:
	docker compose -f deploy/compose/compose.dev.yaml up --build

compose-platform:
	docker compose -f deploy/compose/compose.platform.yaml up -d
