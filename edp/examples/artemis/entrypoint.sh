#!/usr/bin/env bash
#
# Entrypoint for the all-in-one, self-updating Artemis testing container.
#
# Responsibilities (all inside ONE container, foreground supervisor — no cron):
#   1. Bring up a native MySQL server (init data dir on first run).
#   2. Clone the configured repo/branch on first run.
#   3. Build + run Artemis via `./gradlew bootRun` (also builds/serves the client).
#   4. Every PULL_INTERVAL_SECONDS: fetch the branch; on new commits, stop the
#      server, NUKE the database, update the source, and restart (which rebuilds).
#
set -euo pipefail

# --- Configuration (overridable via env / docker/selfupdate/default.env) --------
ARTEMIS_REPO_URL="${ARTEMIS_REPO_URL:-https://github.com/ls1intum/Artemis.git}"
ARTEMIS_BRANCH="${ARTEMIS_BRANCH:-develop}"
# Plain dev profiles: server + client + MySQL on localhost, no localci/buildagent.
ARTEMIS_PROFILES="${ARTEMIS_PROFILES:-artemis,scheduling,core,dev}"
PULL_INTERVAL_SECONDS="${PULL_INTERVAL_SECONDS:-3600}"

WORKDIR="${ARTEMIS_WORKDIR:-/opt/artemis-src}"
DB_NAME="${ARTEMIS_DB_NAME:-Artemis}"
MYSQL_DATADIR="/var/lib/mysql"

# Spring reads SPRING_PROFILES_ACTIVE from the environment.
export SPRING_PROFILES_ACTIVE="$ARTEMIS_PROFILES"

APP_PGID=""
MYSQL_PID=""

log() { echo "[entrypoint] $*"; }

# --- MySQL ---------------------------------------------------------------------
start_mysql() {
    mkdir -p /var/run/mysqld
    chown -R mysql:mysql /var/run/mysqld "$MYSQL_DATADIR"

    if [ ! -d "$MYSQL_DATADIR/mysql" ]; then
        log "Initializing MySQL data directory (insecure: empty root password)"
        mysqld --initialize-insecure --user=mysql --datadir="$MYSQL_DATADIR"
    fi

    log "Starting MySQL"
    mysqld --user=mysql --datadir="$MYSQL_DATADIR" &
    MYSQL_PID=$!

    log "Waiting for MySQL to accept connections"
    for _ in $(seq 1 120); do
        if mysqladmin --silent ping >/dev/null 2>&1; then
            break
        fi
        sleep 1
    done
    mysqladmin --silent ping >/dev/null 2>&1 || { log "MySQL did not come up"; exit 1; }

    # The dev profile connects over TCP (127.0.0.1) as root with an empty password.
    # initialize-insecure only creates root@localhost (socket), so expose root@'%'.
    log "Configuring root access for TCP connections"
    mysql --protocol=socket -uroot <<-SQL
        CREATE USER IF NOT EXISTS 'root'@'%' IDENTIFIED BY '';
        GRANT ALL PRIVILEGES ON *.* TO 'root'@'%' WITH GRANT OPTION;
        CREATE DATABASE IF NOT EXISTS \`${DB_NAME}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
        FLUSH PRIVILEGES;
SQL
}

nuke_db() {
    log "Nuking database '${DB_NAME}'"
    mysql --protocol=socket -uroot <<-SQL
        DROP DATABASE IF EXISTS \`${DB_NAME}\`;
        CREATE DATABASE \`${DB_NAME}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
SQL
}

# --- Source --------------------------------------------------------------------
clone_if_needed() {
    if [ ! -d "$WORKDIR/.git" ]; then
        log "Cloning ${ARTEMIS_REPO_URL} (branch ${ARTEMIS_BRANCH}) into ${WORKDIR}"
        rm -rf "${WORKDIR:?}/"* 2>/dev/null || true
        git clone --branch "$ARTEMIS_BRANCH" "$ARTEMIS_REPO_URL" "$WORKDIR"
    else
        log "Reusing existing checkout in ${WORKDIR}"
    fi
    git config --global --add safe.directory "$WORKDIR"
}

# --- Application ----------------------------------------------------------------
run_app() {
    log "Starting Artemis (profiles: ${SPRING_PROFILES_ACTIVE})"
    setsid bash -c "cd '$WORKDIR' && exec ./gradlew bootRun --console=plain" &
    APP_PGID=$!
}

stop_app() {
    [ -z "$APP_PGID" ] && return
    log "Stopping Artemis (process group ${APP_PGID})"
    kill -TERM -- "-${APP_PGID}" 2>/dev/null || true
    for _ in $(seq 1 90); do
        kill -0 -- "-${APP_PGID}" 2>/dev/null || { APP_PGID=""; return; }
        sleep 1
    done
    log "Artemis did not stop gracefully, forcing"
    kill -KILL -- "-${APP_PGID}" 2>/dev/null || true
    APP_PGID=""
}

# --- Shutdown -------------------------------------------------------------------
shutdown() {
    log "Shutting down"
    stop_app
    [ -n "$MYSQL_PID" ] && mysqladmin --protocol=socket -uroot shutdown 2>/dev/null || true
    exit 0
}
trap shutdown TERM INT

# --- Main -----------------------------------------------------------------------
start_mysql
clone_if_needed
run_app

log "Watcher active: checking ${ARTEMIS_BRANCH} every ${PULL_INTERVAL_SECONDS}s"
while true; do
    sleep "$PULL_INTERVAL_SECONDS" &
    wait $! || true

    cd "$WORKDIR"
    if ! git fetch --quiet origin "$ARTEMIS_BRANCH"; then
        log "git fetch failed, will retry next interval"
        continue
    fi
    local_rev="$(git rev-parse HEAD)"
    remote_rev="$(git rev-parse "origin/${ARTEMIS_BRANCH}")"

    if [ "$local_rev" != "$remote_rev" ]; then
        log "New changes detected (${local_rev:0:7} -> ${remote_rev:0:7}); rebuilding"
        stop_app
        git reset --hard "origin/${ARTEMIS_BRANCH}"
        nuke_db
        run_app
    else
        log "No changes (${local_rev:0:7})"
    fi
done
