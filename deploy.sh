#!/usr/bin/env bash
# deploy.sh — Instant deployment & Continuous Git Push Auto-Deployer for kloudsPanel
#
# Usage:
#   ./deploy.sh          -> Deploys immediately, then keeps watching and auto-deploying every git push
#   ./deploy.sh --once   -> Runs a one-time deployment and exits immediately

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/paas/deploy/compose/compose.platform.yaml"
ENV_FILE="$SCRIPT_DIR/paas/deploy/compose/.env"
BRANCH="main"
POLL_INTERVAL=5

# Trap SIGINT (Ctrl+C) and SIGTERM for graceful exit
cleanup() {
    echo ""
    echo "==> Auto-deploy watcher stopped. Containers will continue running in the background."
    exit 0
}
trap cleanup SIGINT SIGTERM

run_deployment() {
    local reason="${1:-Manual trigger}"
    echo ""
    echo "================================================================="
    echo " [$(date '+%Y-%m-%d %H:%M:%S')] Starting Deployment: $reason"
    echo "================================================================="

    if [ ! -f "$ENV_FILE" ]; then
        echo "==> Creating .env from .env.example..."
        cp "$SCRIPT_DIR/paas/deploy/compose/.env.example" "$ENV_FILE"
    fi

    mkdir -p "$SCRIPT_DIR/paas/deploy/traefik/dynamic"

    echo "==> Pulling latest changes from GitHub ($BRANCH)..."
    git -C "$SCRIPT_DIR" pull origin "$BRANCH"

    CURRENT_COMMIT="$(git -C "$SCRIPT_DIR" rev-parse --short HEAD)"
    COMMIT_MSG="$(git -C "$SCRIPT_DIR" log -1 --pretty=%B | head -n 1)"
    echo "==> Active Commit: [$CURRENT_COMMIT] $COMMIT_MSG"

    echo "==> Rebuilding platform images and updating containers..."
    docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up --build -d --remove-orphans

    echo "==> Running containers:"
    docker compose -f "$COMPOSE_FILE" ps

    echo ""
    echo "✓ Deployment complete! kloudsPanel is live at https://klouds.online"
    echo "================================================================="
}

# Run initial deployment
run_deployment "Initial startup"

# If --once flag is passed, exit after single deployment
if [[ "${1:-}" == "--once" || "${1:-}" == "-1" ]]; then
    echo "==> One-time deployment finished. Exiting."
    exit 0
fi

echo ""
echo "================================================================="
echo " 👀 Auto-Deploy Watcher is ACTIVE!"
echo "    Polling GitHub ($BRANCH) every ${POLL_INTERVAL}s for new git pushes."
echo "    As long as this terminal is open, any git push will auto-deploy."
echo "    Press [Ctrl+C] anytime to stop the auto-deployer."
echo "================================================================="
echo ""

while true; do
    # Fetch remote branch references silently
    git -C "$SCRIPT_DIR" fetch origin "$BRANCH" --quiet 2>/dev/null || true

    LOCAL_HASH="$(git -C "$SCRIPT_DIR" rev-parse HEAD 2>/dev/null || echo "")"
    REMOTE_HASH="$(git -C "$SCRIPT_DIR" rev-parse "origin/$BRANCH" 2>/dev/null || echo "")"

    if [[ -n "$LOCAL_HASH" && -n "$REMOTE_HASH" && "$LOCAL_HASH" != "$REMOTE_HASH" ]]; then
        NEW_COMMIT_MSG="$(git -C "$SCRIPT_DIR" log "origin/$BRANCH" -1 --pretty=%B | head -n 1)"
        echo ""
        echo "⚡ New git push detected on origin/$BRANCH!"
        echo "   Incoming Commit: [${REMOTE_HASH:0:7}] $NEW_COMMIT_MSG"
        run_deployment "New git push (${REMOTE_HASH:0:7})"
        echo ""
        echo "👀 Resuming watch on origin/$BRANCH (every ${POLL_INTERVAL}s)... Press [Ctrl+C] to stop."
    fi

    sleep "$POLL_INTERVAL"
done
