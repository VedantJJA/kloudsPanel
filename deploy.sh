#!/usr/bin/env bash
# ==============================================================================
#  deploy.sh - Smart Continuous Auto-Deployer for kloudsPanel
#
#  Usage:
#    ./deploy.sh          -> Auto-detects changes, deploys, and starts watcher loop
#    ./deploy.sh --once   -> Builds & deploys platform containers once, then exits
#    ./deploy.sh --local  -> Deploys local workspace code without git fetch/reset
#    ./deploy.sh --force  -> Forces a clean rebuild with --no-cache
# ==============================================================================

set -uo pipefail

BOLD='\033[1m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

COMPOSE_FILE="$SCRIPT_DIR/paas/deploy/compose/compose.platform.yaml"
ENV_FILE="$SCRIPT_DIR/paas/deploy/compose/.env"
POLL_INTERVAL=5
FORCE_DEPLOY=false
ONCE_MODE=false
LOCAL_MODE=false

for arg in "$@"; do
    case "$arg" in
        --force|-f)
            FORCE_DEPLOY=true
            ;;
        --once|-1)
            ONCE_MODE=true
            ;;
        --local|-l|--skip-git)
            LOCAL_MODE=true
            ;;
        --poll=*)
            POLL_INTERVAL="${arg#*=}"
            ;;
    esac
done

# Auto-detect current tracking branch (default: main)
CURRENT_BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "main")"
if [ "$CURRENT_BRANCH" = "HEAD" ] || [ -z "$CURRENT_BRANCH" ]; then
    CURRENT_BRANCH="main"
fi
BRANCH="${BRANCH:-$CURRENT_BRANCH}"

# Trap SIGINT (Ctrl+C) and SIGTERM for graceful exit
cleanup() {
    echo ""
    echo -e "${YELLOW}==> Auto-deploy watcher stopped. Containers will remain running.${NC}"
    exit 0
}
trap cleanup SIGINT SIGTERM

ensure_prerequisites() {
    # 1. Ensure Docker is running
    if ! docker info >/dev/null 2>&1; then
        echo -e "${RED}Error: Docker daemon is not accessible or not running. Please start Docker.${NC}"
        exit 1
    fi

    # 2. Ensure platform-control Docker network
    docker network create platform-control >/dev/null 2>&1 || true

    # 3. Ensure dynamic Traefik configs directory exists
    mkdir -p "$SCRIPT_DIR/paas/deploy/traefik/dynamic"

    # 4. Ensure .env exists with cryptographic master keys
    if [ ! -f "$ENV_FILE" ]; then
        echo -e "${CYAN}==> Generating .env from .env.example with secure keys...${NC}"
        cp "$SCRIPT_DIR/paas/deploy/compose/.env.example" "$ENV_FILE" 2>/dev/null || true
        
        # Generate 256-bit random master key (64 hex characters)
        GEN_KEY="$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | xxd -p -c 32 2>/dev/null || date +%s%N | sha256sum | head -c 64)"
        if [ -n "$GEN_KEY" ] && [ -f "$ENV_FILE" ]; then
            sed -i "s|MASTER_KEY_HEX=.*|MASTER_KEY_HEX=${GEN_KEY}|g" "$ENV_FILE" 2>/dev/null || true
        fi
    fi

    # 5. Ensure Traefik ACME storage volume has strict 600 permissions
    docker volume create klouds-traefik-acme >/dev/null 2>&1 || true
    docker run --rm -v klouds-traefik-acme:/acme alpine sh -c "touch /acme/acme.json && chmod 600 /acme/acme.json" >/dev/null 2>&1 || true
}

run_deployment() {
    local reason="${1:-Manual trigger}"
    echo ""
    echo -e "${CYAN}${BOLD}=================================================================${NC}"
    echo -e "${CYAN}${BOLD} [$(date '+%Y-%m-%d %H:%M:%S')] Deploying: $reason${NC}"
    echo -e "${CYAN}${BOLD}=================================================================${NC}"

    ensure_prerequisites

    if [ "$LOCAL_MODE" = false ]; then
        # Check if there are local uncommitted changes
        local has_local_changes=false
        if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
            has_local_changes=true
        fi

        if [ "$has_local_changes" = false ]; then
            echo "==> Pulling latest changes from origin/$BRANCH..."
            git fetch origin "$BRANCH" --quiet 2>/dev/null || true
            git reset --hard "origin/$BRANCH" 2>/dev/null || git pull --rebase origin "$BRANCH" 2>/dev/null || true
        else
            echo -e "${YELLOW}==> Uncommitted local changes detected. Skipping git hard reset.${NC}"
        fi
    else
        echo -e "${YELLOW}==> Local mode active: building directly from current workspace.${NC}"
    fi

    CURRENT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "local")"
    COMMIT_MSG="$(git log -1 --pretty=%B 2>/dev/null | head -n 1 || echo "local build")"
    echo -e "${GREEN}==> Active Commit: [${CURRENT_COMMIT}] ${COMMIT_MSG}${NC}"

    BUILD_ARGS=()
    if [ "$FORCE_DEPLOY" = true ]; then
        BUILD_ARGS+=("--no-cache")
    fi

    echo "==> Building platform images (API, Web, Agent, Ingress) & updating containers..."
    docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" build "${BUILD_ARGS[@]}"
    docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up -d --remove-orphans

    echo ""
    ROOT_HOST=$(grep -E '^ROOT_DOMAIN=' "$ENV_FILE" 2>/dev/null | cut -d '=' -f2 | tr -d '"' | tr -d "'" | tr -d ' ')
    [ -z "$ROOT_HOST" ] && ROOT_HOST="localhost"
    echo -e "${GREEN}${BOLD}✓ Deployment complete! kloudsPanel is live at https://${ROOT_HOST}${NC}"
    echo -e "${CYAN}=================================================================${NC}"
}

# 1. Initial Deployment
if [ "$LOCAL_MODE" = true ]; then
    run_deployment "Local workspace deployment"
else
    echo -e "${CYAN}==> Checking repository status on branch ${BOLD}$BRANCH${NC}..."
    git fetch origin "$BRANCH" --quiet 2>/dev/null || true

    LOCAL_HASH="$(git rev-parse HEAD 2>/dev/null || echo "")"
    REMOTE_HASH="$(git rev-parse "origin/$BRANCH" 2>/dev/null || git rev-parse FETCH_HEAD 2>/dev/null || echo "")"

    if [ "$LOCAL_HASH" != "$REMOTE_HASH" ] && [ -n "$REMOTE_HASH" ]; then
        run_deployment "Syncing to latest remote commit (${REMOTE_HASH:0:7})"
    else
        run_deployment "Platform startup & container build (${LOCAL_HASH:0:7})"
    fi
fi

if [ "$ONCE_MODE" = true ] || [ "$LOCAL_MODE" = true ]; then
    echo "==> Deployment finished. Exiting."
    exit 0
fi

echo ""
echo -e "${CYAN}${BOLD}=================================================================${NC}"
echo -e "${CYAN}${BOLD} 👀 Auto-Deploy Watcher is ACTIVE on origin/$BRANCH${NC}"
echo -e "    Polling every ${POLL_INTERVAL}s for new git pushes."
echo -e "    Any new git push will be pulled and deployed automatically."
echo -e "    Press [Ctrl+C] to stop watcher anytime."
echo -e "${CYAN}${BOLD}=================================================================${NC}"
echo ""

LAST_SEEN_HASH="$(git rev-parse HEAD 2>/dev/null || echo "")"

while true; do
    # Fetch remote references
    git fetch origin "$BRANCH" --quiet 2>/dev/null || true

    LATEST_REMOTE_HASH="$(git rev-parse "origin/$BRANCH" 2>/dev/null || git rev-parse FETCH_HEAD 2>/dev/null || echo "")"

    if [ -n "$LATEST_REMOTE_HASH" ] && [ "$LATEST_REMOTE_HASH" != "$LAST_SEEN_HASH" ]; then
        NEW_MSG="$(git log "origin/$BRANCH" -1 --pretty=%B 2>/dev/null | head -n 1 || echo "")"
        echo ""
        echo -e "${YELLOW}⚡ New git push detected on origin/$BRANCH!${NC}"
        echo -e "   Commit: [${LATEST_REMOTE_HASH:0:7}] $NEW_MSG"
        run_deployment "New git push (${LATEST_REMOTE_HASH:0:7})"
        LAST_SEEN_HASH="$LATEST_REMOTE_HASH"
        echo ""
        echo -e "${CYAN}👀 Resuming watch on origin/$BRANCH... Press [Ctrl+C] to stop.${NC}"
    fi

    sleep "$POLL_INTERVAL"
done
