#!/usr/bin/env bash
# ==============================================================================
#  paas/deploy.sh - Delegates to root deploy.sh
# ==============================================================================
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [ -f "$SCRIPT_DIR/../deploy.sh" ]; then
    exec "$SCRIPT_DIR/../deploy.sh" "$@"
elif [ -f "$SCRIPT_DIR/deploy.sh" ]; then
    exec "$SCRIPT_DIR/deploy.sh" "$@"
fi
