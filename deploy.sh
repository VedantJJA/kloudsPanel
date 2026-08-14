#!/usr/bin/env bash
# ==============================================================================
#  deploy.sh - Smart Continuous Auto-Deployer for kloudsPanel
#
#  Usage:
#    ./deploy.sh          -> Checks for changes, deploys if needed, and starts watcher
#    ./deploy.sh --once   -> Checks for changes, deploys if needed, and exits
#    ./deploy.sh --force  -> Forces a clean rebuild even if no git changes exist
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

for arg in "$@"; do
    case "$arg" in
        --force|-f)
            FORCE_DEPLOY=true
            ;;
        --once|-1)
            ONCE_MODE=true
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

are_containers_running() {
    if [ ! -f "$COMPOSE_FILE" ]; then
        return 1
    fi
    local count
    count="$(docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" ps -q 2>/dev/null | wc -l || echo "0")"
    [ "$count" -gt 0 ]
}

run_deployment() {
    local reason="${1:-Manual trigger}"
    echo ""
    echo -e "${CYAN}${BOLD}=================================================================${NC}"
    echo -e "${CYAN}${BOLD} [$(date '+%Y-%m-%d %H:%M:%S')] Deploying: $reason${NC}"
    echo -e "${CYAN}${BOLD}=================================================================${NC}"

    if [ ! -f "$ENV_FILE" ]; then
        echo "==> Creating .env from .env.example..."
        cp "$SCRIPT_DIR/paas/deploy/compose/.env.example" "$ENV_FILE"
    fi

    mkdir -p "$SCRIPT_DIR/paas/deploy/traefik/dynamic"

    # Ensure Traefik ACME storage volume has strict 600 permissions
    docker volume create klouds-traefik-acme >/dev/null 2>&1 || true
    docker run --rm -v klouds-traefik-acme:/acme alpine sh -c "touch /acme/acme.json && chmod 600 /acme/acme.json" >/dev/null 2>&1 || true

    # Pull changes safely
    echo "==> Pulling latest changes from origin/$BRANCH..."
    git fetch origin "$BRANCH" --quiet 2>/dev/null || true
    git reset --hard "origin/$BRANCH" 2>/dev/null || git pull --rebase origin "$BRANCH" 2>/dev/null || true

    CURRENT_COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")"
    COMMIT_MSG="$(git log -1 --pretty=%B 2>/dev/null | head -n 1 || echo "")"
    echo -e "${GREEN}==> Active Commit: [${CURRENT_COMMIT}] ${COMMIT_MSG}${NC}"

    echo "==> Rebuilding platform images & updating containers..."
    docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up --build -d --remove-orphans

    echo ""
    ROOT_HOST=$(grep -E '^ROOT_DOMAIN=' "$ENV_FILE" 2>/dev/null | cut -d '=' -f2 | tr -d '"' | tr -d "'" | tr -d ' ')
    [ -z "$ROOT_HOST" ] && ROOT_HOST="yourdomain.com"
    echo -e "${GREEN}${BOLD}Deployment complete! kloudsPanel is live at https://${ROOT_HOST}${NC}"
    echo -e "${CYAN}=================================================================${NC}"
}

# 1. Check if update is needed on startup
echo -e "${CYAN}==> Checking repository status on branch ${BOLD}$BRANCH${NC}..."
git fetch origin "$BRANCH" --quiet 2>/dev/null || true

LOCAL_HASH="$(git rev-parse HEAD 2>/dev/null || echo "")"
REMOTE_HASH="$(git rev-parse "origin/$BRANCH" 2>/dev/null || git rev-parse FETCH_HEAD 2>/dev/null || echo "")"

if [ "$FORCE_DEPLOY" = true ]; then
    run_deployment "Forced deployment (--force)"
elif ! are_containers_running; then
    run_deployment "Containers are not running"
elif [ -n "$LOCAL_HASH" ] && [ -n "$REMOTE_HASH" ] && [ "$LOCAL_HASH" != "$REMOTE_HASH" ]; then
    run_deployment "New remote commits found on startup (${REMOTE_HASH:0:7})"
else
    COMMIT_MSG="$(git log -1 --pretty=%B 2>/dev/null | head -n 1 || echo "")"
    echo -e "${GREEN}✓ Everything is up to date at commit [${LOCAL_HASH:0:7}] ${COMMIT_MSG}${NC}"
    echo -e "${GREEN}✓ All kloudsPanel containers are running.${NC}"
fi

if [ "$ONCE_MODE" = true ]; then
    echo "==> One-time check finished. Exiting."
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

LAST_SEEN_HASH="$LOCAL_HASH"

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
