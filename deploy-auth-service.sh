#!/bin/bash
set -euo pipefail

# Deploy auth-service to VPS
# Usage: deploy-auth-service.sh [branch]
# Branch: main        → prod (auth.abuamar.online, 127.0.0.1:8080)
#         development → dev  (auth-dev.abuamar.online, 127.0.0.1:8082)
#
# /opt/auth-service-dev is not a repo — only compose + .env; its build context
# points at /opt/auth-service. Prod and dev therefore SHARE one source dir, so
# two deploys at once would overwrite each other's checkout. One lock covers
# both. Same shape as deploy-portfolio-service.sh.

BRANCH="${1:-main}"
DEPLOY_DIR="/opt/auth-service"
DEV_DIR="/opt/auth-service-dev"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/../lib/notify.sh"

if [ "$BRANCH" = "development" ]; then
    ENV="dev"
    PROJECT="auth-service-dev"
    WORK_DIR="$DEV_DIR"
    HEALTH_PORT=8082
    APP_CONTAINER="auth-service-app-dev"
    UP_SERVICES=(app-dev db-dev)
else
    ENV="prod"
    PROJECT="auth-service"
    WORK_DIR="$DEPLOY_DIR"
    HEALTH_PORT=8080
    APP_CONTAINER="auth-service-app"
    UP_SERVICES=(app db)
fi

LOG_DIR="/var/log/deploy"
LOG_FILE="$LOG_DIR/auth-service-$(date +%Y%m%d-%H%M%S).log"
mkdir -p "$LOG_DIR"
exec > >(tee -a "$LOG_FILE") 2>&1

# One lock for both envs, not one each: they share $DEPLOY_DIR, and a
# simultaneous prod+dev deploy would reset the checkout under the other's
# build. Per-env locks let that race through.
LOCK_FILE="/tmp/auth-service-deploy-$(id -un).deploy.lock"
exec 9>"$LOCK_FILE"
flock 9 || { echo "ERROR: could not acquire lock ($LOCK_FILE)"; exit 1; }

echo "=== Deploying auth-service ($BRANCH → $ENV) ==="
echo "    log: $LOG_FILE"

# fetch/reset without a fallback: swallowing a failed auth or a bad ref makes
# the deploy "succeed" on stale code. reset --hard is idempotent, so a retry
# after a race is safe.
echo "Pulling $BRANCH..."
git -C "$DEPLOY_DIR" fetch origin "$BRANCH"
git -C "$DEPLOY_DIR" reset --hard "origin/$BRANCH"
HEAD_SHA="$(git -C "$DEPLOY_DIR" rev-parse HEAD)"
WANT_SHA="$(git -C "$DEPLOY_DIR" rev-parse "origin/$BRANCH")"
if [ "$HEAD_SHA" != "$WANT_SHA" ]; then
    echo "ERROR: HEAD ($HEAD_SHA) != origin/$BRANCH ($WANT_SHA)"
    exit 1
fi
echo "    source at $HEAD_SHA"

if [ ! -f "$WORK_DIR/.env" ]; then
    echo "ERROR: $WORK_DIR/.env not found"
    exit 1
fi

echo "Building ($ENV)..."
cd "$WORK_DIR"
docker compose --project-name "$PROJECT" build

echo "Starting services ($ENV)..."
# --force-recreate: compose does not recreate a container when only the image
# contents change under the same tag, so a deploy can report success while the
# old image keeps running.
docker compose --project-name "$PROJECT" up -d --remove-orphans --force-recreate "${UP_SERVICES[@]}"

echo "Waiting for health check on port $HEALTH_PORT..."
for i in {1..30}; do
    HTTP_CODE=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:$HEALTH_PORT/api/health" 2>/dev/null || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
        break
    fi
    sleep 1
done
if [ "${HTTP_CODE:-000}" != "200" ]; then
    echo "❌ Health check failed (last HTTP: $HTTP_CODE)"
    notify_deploy_fail "auth-service" "$BRANCH" "Health check failed (HTTP $HTTP_CODE)"
    exit 1
fi

# A 200 is not proof the new code is running. Check the container is actually
# on the image this build produced.
IMG="$(docker inspect "$APP_CONTAINER" --format '{{.Config.Image}}')"
RUNNING_IMG="$(docker inspect "$APP_CONTAINER" --format '{{.Image}}')"
WANTED_IMG="$(docker image inspect "$IMG" --format '{{.Id}}')"
if [ "$RUNNING_IMG" != "$WANTED_IMG" ]; then
    echo "❌ IMAGE BASI: $APP_CONTAINER running=$RUNNING_IMG image=$IMG wanted=$WANTED_IMG"
    notify_deploy_fail "auth-service" "$BRANCH" "Image basi: container tidak memakai image hasil build"
    exit 1
fi

echo "✅ auth-service ($ENV) is healthy, image sinkron"
notify_deploy_success "auth-service" "$BRANCH" "$HTTP_CODE"
exit 0
