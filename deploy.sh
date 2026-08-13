#!/usr/bin/env bash
# deploy.sh — Pull latest code, rebuild images, and restart the kloudsPanel stack.
# Usage:  ./deploy.sh
# Run from any directory; the script locates itself automatically.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="$SCRIPT_DIR/paas/deploy/compose/compose.platform.yaml"
ENV_FILE="$SCRIPT_DIR/paas/deploy/compose/.env"

echo "==> Pulling latest changes from GitHub..."
git -C "$SCRIPT_DIR" pull origin main

echo "==> Rebuilding images and restarting containers..."
docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE" up --build -d --remove-orphans

echo "==> Waiting for containers to become healthy..."
sleep 5

echo "==> Running containers:"
docker compose -f "$COMPOSE_FILE" ps

echo ""
echo "Deploy complete! Your panel should be available at https://klouds.online"
