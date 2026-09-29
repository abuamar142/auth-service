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

# Serialize deploys across users.
#
# CI connects as `ubuntu` while a manual run is often `sudo`, so the lock path
# must not include the user name — otherwise the two run at once, both recreate
# the same container, and the loser dies with "container name is already in
# use". The file is world-writable so either user can take it; the flock is what
# serializes, not the file's permissions.
LOCK="/tmp/auth-service-${BRANCH}.deploy.lock"
umask 000
exec 9>"$LOCK"
flock 9 || { echo "ERROR: could not acquire lock ($LOCK)"; exit 1; }

cd "$DEPLOY_DIR"

# Pull latest code.
#
# Git runs as `ubuntu`, never as root: CI connects as `ubuntu` and the repo has
# to stay writable by it. A root-owned .git breaks the next CI deploy with
# "cannot open '.git/FETCH_HEAD': Permission denied".
git_ubuntu() {
    if [ "$(id -un)" = "ubuntu" ]; then
        git "$@"
    else
        sudo -u ubuntu git "$@"
    fi
}

echo "Pulling $BRANCH..."
git_ubuntu fetch origin "$BRANCH"
git_ubuntu reset --hard "origin/$BRANCH"

# Determine environment.
#
# Project names follow the convention used across this VPS: "auth-service"
# for prod and "auth-service-dev" for dev. The two environments are kept
# apart by the project name alone — compose attaches every container and
# volume to it — while the container names (auth-service-app / -db and their
# -dev counterparts) and the named volumes (auth-service-pgdata, explicit, not
# project-prefixed) stay fixed, so changing a project name cannot orphan data.
if [ "$BRANCH" = "development" ]; then
    ENV="dev"
    PROJECT="auth-service-dev"
    COMPOSE_FILES="-f docker-compose.yml -f docker-compose.dev.yml"
    HEALTH_PORT=8082
    ENV_FILE=".env.dev"
else
    ENV="prod"
    PROJECT="auth-service"
    COMPOSE_FILES="-f docker-compose.yml -f docker-compose.prod.yml"
    HEALTH_PORT=8080
    ENV_FILE=".env"
fi

# Check .env exists
if [ ! -f "$ENV_FILE" ]; then
    echo "ERROR: $ENV_FILE not found at $DEPLOY_DIR/$ENV_FILE"
    exit 1
fi

# Build and deploy.
#
# The same project name is used for both steps. They used to differ — the image
# was built under "auth-service-$ENV" (auth-service-prod) while the containers
# ran under "auth-service" — and compose tags a built image per project, so
# `up` never saw the image it had just built. Every deploy reported success
# while the container kept running the previous binary.
#
# --force-recreate for the same reason one step later: compose does not
# recreate a container when only the image contents changed under an unchanged
# tag.

echo "Building ($ENV)..."
docker compose --project-name "$PROJECT" --env-file "$ENV_FILE" $COMPOSE_FILES build

echo "Starting services ($ENV)..."
if [ "$ENV" = "dev" ]; then
    docker compose --project-name "$PROJECT" --env-file "$ENV_FILE" $COMPOSE_FILES up -d --remove-orphans --force-recreate app-dev db-dev
else
    docker compose --project-name "$PROJECT" --env-file "$ENV_FILE" $COMPOSE_FILES up -d --remove-orphans --force-recreate app db
fi

# Health check
echo "Waiting for health check on port $HEALTH_PORT..."
for i in {1..30}; do
    HTTP_CODE=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$HEALTH_PORT/api/health" 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
        echo "✅ auth-service ($ENV) is healthy"
        # The library exports notify_deploy_success/notify_deploy_fail; the
        # old notify_success/notify_fail names do not exist, so the deploy
        # notification had been failing silently (and, under `set -e`, would
        # have aborted the script after a successful deploy).
        notify_deploy_success "auth-service-$ENV" "$BRANCH" "$HTTP_CODE"
        exit 0
    fi
    sleep 1
done

echo "❌ Health check failed (last HTTP: $HTTP_CODE)"
notify_deploy_fail "auth-service-$ENV" "$BRANCH" "health check failed (HTTP $HTTP_CODE)"
exit 1
