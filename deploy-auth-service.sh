#!/bin/bash
set -euo pipefail

# Deploy auth-service to VPS
# Usage: deploy-auth-service.sh [branch]
# Branch: main → prod (auth.abuamar.online, port 8080)
#         development → dev (dev.auth.abuamar.online, port 8082)

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

# Determine environment
if [ "$BRANCH" = "development" ]; then
    ENV="dev"
    COMPOSE_FILES="-f docker-compose.yml -f docker-compose.dev.yml"
    HEALTH_PORT=8082
    ENV_FILE=".env.dev"
else
    ENV="prod"
    COMPOSE_FILES="-f docker-compose.yml -f docker-compose.prod.yml"
    HEALTH_PORT=8080
    ENV_FILE=".env"
fi

# Check .env exists
if [ ! -f "$ENV_FILE" ]; then
    echo "ERROR: $ENV_FILE not found at $DEPLOY_DIR/$ENV_FILE"
    exit 1
fi

# Build and deploy
echo "Building ($ENV)..."
docker compose --project-name "auth-service-$ENV" --env-file "$ENV_FILE" $COMPOSE_FILES build

echo "Starting services ($ENV)..."
if [ "$ENV" = "dev" ]; then
    docker compose --project-name "auth-service-dev" --env-file "$ENV_FILE" $COMPOSE_FILES up -d --remove-orphans app-dev db-dev
else
    docker compose --project-name "auth-service" --env-file "$ENV_FILE" $COMPOSE_FILES up -d --remove-orphans app db
fi

# Health check
echo "Waiting for health check on port $HEALTH_PORT..."
for i in {1..30}; do
    HTTP_CODE=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$HEALTH_PORT/api/health" 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
        echo "✅ auth-service ($ENV) is healthy"
        notify_deploy_success "auth-service" "$BRANCH"
        exit 0
    fi
    sleep 1
done

echo "❌ Health check failed (last HTTP: $HTTP_CODE)"
notify_deploy_fail "auth-service" "$BRANCH"
exit 1
