#!/bin/bash
set -euo pipefail

# Deploy auth-service to VPS
# Usage: deploy-auth-service.sh [branch]

BRANCH="${1:-main}"
DEPLOY_DIR="/opt/auth-service"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/../lib/notify.sh"

echo "=== Deploying auth-service ($BRANCH) ==="

cd "$DEPLOY_DIR"

# Pull latest code
echo "Pulling $BRANCH..."
git fetch origin "$BRANCH"
git reset --hard "origin/$BRANCH"

# Copy .env from local if not exists
if [ ! -f .env ]; then
    echo "ERROR: .env not found at $DEPLOY_DIR/.env"
    exit 1
fi

# Build and deploy
echo "Building..."
docker compose --env-file .env -f docker-compose.yml -f docker-compose.prod.yml build

echo "Starting services..."
docker compose --env-file .env -f docker-compose.yml -f docker-compose.prod.yml up -d --remove-orphans

# Health check
echo "Waiting for health check..."
for i in {1..30}; do
    HTTP_CODE=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8080/api/health 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
        echo "✅ auth-service is healthy"
        notify_success "auth-service" "auth-service deployed successfully on $BRANCH"
        exit 0
    fi
    sleep 1
done

echo "❌ Health check failed (last HTTP: $HTTP_CODE)"
notify_fail "auth-service" "Health check failed after deploy (HTTP $HTTP_CODE)"
exit 1
