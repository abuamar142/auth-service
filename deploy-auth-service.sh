#!/bin/bash
set -euo pipefail

# Deploy auth-service to VPS
# Usage: deploy-auth-service.sh [branch]
# Branch: main        → prod (auth.abuamar.online, 127.0.0.1:8080)
#         development → dev  (auth-dev.abuamar.online, 127.0.0.1:8082)
#
# Two directories, and the distinction matters:
#
#   /opt/auth-service      the git checkout — source for BOTH envs
#   /opt/auth-service-dev  compose file + .env for dev only; not a checkout
#
# Prod and dev therefore share one source tree but have separate compose files,
# so `up` for one env cannot reconcile the other's containers. Same shape as
# deploy-portfolio-service.sh.
#
# The shared source tree is why one lock covers both branches: two deploys at
# once would `git reset --hard` the checkout under each other's build.

BRANCH="${1:-main}"
DEPLOY_DIR="/opt/auth-service"
DEV_DIR="/opt/auth-service-dev"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "$SCRIPT_DIR/../lib/notify.sh"

echo "=== Deploying auth-service ($BRANCH) ==="

# Serialize deploys across users AND branches.
#
# Keyed on the deploy directory, not the branch, because that is the resource
# actually contended: prod and dev share $DEPLOY_DIR, so pushing main and
# development together would otherwise run both at once and the slower one
# would build whatever the faster one left behind.
#
# The lock path must not include the user name. CI connects as `ubuntu` while a
# manual run is often `sudo`, so a per-user lock lets the two race and the
# loser dies with "container name is already in use".
#
# The lock lives in its own NON-STICKY directory. In a sticky directory such as
# /tmp, fs.protected_regular=2 stops root from opening a file owned by another
# user — so a root run could not take a lock left behind by ubuntu and died
# with "Permission denied" before doing anything.
LOCK_DIR="/var/lock/deploy"
sudo mkdir -p "$LOCK_DIR" 2>/dev/null || mkdir -p "$LOCK_DIR"
sudo chmod 0777 "$LOCK_DIR" 2>/dev/null || chmod 0777 "$LOCK_DIR"
LOCK="$LOCK_DIR/$(echo "$DEPLOY_DIR" | tr '/' '_').lock"
# umask is scoped to the lock file: it has to be world-writable so either user
# can open it, but the rest of the script should create files normally.
umask 000
exec 9>"$LOCK"
umask 022
flock 9 || { echo "ERROR: could not acquire lock ($LOCK)"; exit 1; }

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
git_ubuntu -C "$DEPLOY_DIR" fetch origin "$BRANCH"
git_ubuntu -C "$DEPLOY_DIR" reset --hard "origin/$BRANCH"

# Fail loudly if the checkout is not on the commit we asked for. Swallowing
# this makes a deploy "succeed" while shipping stale code.
HEAD_SHA="$(git -C "$DEPLOY_DIR" rev-parse HEAD)"
WANT_SHA="$(git -C "$DEPLOY_DIR" rev-parse "origin/$BRANCH")"
if [ "$HEAD_SHA" != "$WANT_SHA" ]; then
    echo "ERROR: HEAD ($HEAD_SHA) != origin/$BRANCH ($WANT_SHA)"
    exit 1
fi
echo "    source at $HEAD_SHA"

# Determine environment.
#
# Project names follow the convention used across this VPS: "auth-service" for
# prod and "auth-service-dev" for dev. Compose attaches every container and
# volume to the project name, while the container names and the named volumes
# (auth-service-pgdata, explicit, not project-prefixed) stay fixed — so changing
# a project name cannot orphan data.
#
# Prod must pass both compose files: docker-compose.yml alone declares no port
# mapping (it carries the shared service definitions and the abuamar-net join),
# so the container would start unreachable and the health check would fail.
if [ "$BRANCH" = "development" ]; then
    ENV="dev"
    PROJECT="auth-service-dev"
    WORK_DIR="$DEV_DIR"
    COMPOSE_FILES=""
    HEALTH_PORT=8082
    APP_CONTAINER="auth-service-app-dev"
    UP_SERVICES=(app-dev db-dev)
else
    ENV="prod"
    PROJECT="auth-service"
    WORK_DIR="$DEPLOY_DIR"
    COMPOSE_FILES="-f docker-compose.yml -f docker-compose.prod.yml"
    HEALTH_PORT=8080
    APP_CONTAINER="auth-service-app"
    UP_SERVICES=(app db)
fi

# Check .env exists. Each env reads its own file next to its compose file.
if [ ! -f "$WORK_DIR/.env" ]; then
    echo "ERROR: $WORK_DIR/.env not found"
    exit 1
fi

cd "$WORK_DIR"

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
docker compose --project-name "$PROJECT" --env-file .env $COMPOSE_FILES build

echo "Starting services ($ENV)..."
docker compose --project-name "$PROJECT" --env-file .env $COMPOSE_FILES \
    up -d --remove-orphans --force-recreate "${UP_SERVICES[@]}"

# Health check
echo "Waiting for health check on port $HEALTH_PORT..."
HTTP_CODE="000"
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

# A 200 is not proof the new code is running: a container left over from an
# earlier build answers /api/health just as happily. Compare the image the
# container runs against the image this build just produced.
IMG="$(docker inspect "$APP_CONTAINER" --format '{{.Config.Image}}')"
RUNNING_IMG="$(docker inspect "$APP_CONTAINER" --format '{{.Image}}')"
WANTED_IMG="$(docker image inspect "$IMG" --format '{{.Id}}')"
if [ "$RUNNING_IMG" != "$WANTED_IMG" ]; then
    echo "❌ STALE IMAGE: $APP_CONTAINER running=$RUNNING_IMG image=$IMG wanted=$WANTED_IMG"
    notify_deploy_fail "auth-service" "$BRANCH" "Stale image: container is not on the image this build produced"
    exit 1
fi

echo "✅ auth-service ($ENV) is healthy, image in sync"
notify_deploy_success "auth-service" "$BRANCH" "$HTTP_CODE"
exit 0
