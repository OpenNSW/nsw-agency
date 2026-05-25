#!/usr/bin/env bash
# Run Agency backends and/or frontends with per-agency config.
#
# Usage:
#   ./start-dev.sh [--clean-run] [--env-file=PATH] <agency> [target]
#
#   <agency>  One of: npqs, fcau, ird, cda, all
#             'all' fans out and starts every agency in parallel.
#   [target]  One of: all (default), backend, frontend
#
# Flags:
#   --clean-run       Wipe agency database(s) before starting.
#                     SQLite: deletes {agency}_applications.db files.
#                     Postgres: drops and recreates the database.
#   --env-file=PATH   Load additional env vars (non-clobbering) before
#   --env-file PATH   per-agency defaults. Useful for sharing a root .env.
#
# Each agency maps to its own:
#   - backend HTTP port and SQLite DB file
#   - frontend dev server port
#   - frontend branding config (public/configs/<agency>.branding.json)
#   - IdP client id
#
# Env-var precedence (highest to lowest):
#   parent shell env > --env-file > backend/.env (for backend vars) > script defaults
# i.e. PORT=9000 ./start-dev.sh npqs honours the override; .env can fill in
# anything the parent didn't set; the per-agency defaults below are the floor.
#
# Examples:
#   ./start-dev.sh npqs              # NPQS backend + frontend
#   ./start-dev.sh fcau backend      # FCAU backend only
#   ./start-dev.sh ird frontend      # IRD frontend only
#   ./start-dev.sh all               # every backend + frontend, in parallel
#   ./start-dev.sh all backend       # every backend, no frontends
#   ./start-dev.sh all --clean-run   # wipe all agency DBs, then start
#
# Ctrl-C terminates every child process (each runs in its own process group).

set -euo pipefail
# Enable job control so each backgrounded subshell becomes its own process
# group leader — that lets us kill `go run`'s grandchild binary on cleanup.
set -m

# Single source of truth for per-agency config: "BE_PORT|FE_PORT|IDP_CLIENT_ID|NSW_CLIENT_ID|APP_NAME".
# Adding an agency means one line here — nothing else.
# (Scalar vars rather than `declare -A` so this works on stock macOS bash 3.2.)
CONFIG_npqs="8081|5174|OGA_PORTAL_APP_NPQS|NPQS_TO_NSW|National Plant Quarantine Service (NPQS)"
CONFIG_fcau="8082|5175|OGA_PORTAL_APP_FCAU|FCAU_TO_NSW|Food Control Administration Unit (FCAU)"
CONFIG_ird="8083|5176|OGA_PORTAL_APP_IRD|IRD_TO_NSW|Inland Revenue Department (IRD)"
CONFIG_cda="8084|5177|OGA_PORTAL_APP_CDA|CDA_TO_NSW|Coconut Development Authority (CDA)"

# Agencies (every CONFIG_* ), alphabetised for predictable launch order in 'all' mode
#  Derived from the config above so adding an agency only requires editing the CONFIG_* block.
ALL_AGENCIES=()
while IFS= read -r _v; do
  _agency="${_v#CONFIG_}"
  [[ "$_agency" == "default" ]] && continue
  ALL_AGENCIES+=("$_agency")
done < <(compgen -A variable CONFIG_ | sort)
unset _v _agency

usage() {
  cat <<EOF >&2
Usage: $0 [--clean-run] [--env-file=PATH] <agency> [target]

  <agency>  One of: ${ALL_AGENCIES[*]}, all
  [target]  One of: all (default), backend, frontend

Flags:
  --clean-run       Wipe agency DB(s) before starting
  --env-file=PATH   Load a root-level env file (non-clobbering);
  --env-file PATH   both forms are supported

Examples:
  $0 npqs                       # NPQS backend + frontend
  $0 fcau backend               # FCAU backend only
  $0 all                        # every agency, backends + frontends
  $0 all frontend               # every agency, frontends only
  $0 all --clean-run            # wipe all agency DBs, then start
EOF
  exit 1
}

CLEAN_RUN=false
ENV_FILE=""
POSITIONAL=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --clean-run)
      CLEAN_RUN=true
      ;;
    --env-file=*)
      ENV_FILE="${1#*=}"
      ;;
    --env-file)
      shift
      if [[ $# -eq 0 ]] || [[ "$1" == --* ]]; then
        echo "[start-dev] Error: --env-file requires a path value." >&2
        usage
      fi
      ENV_FILE="$1"
      ;;
    *)
      POSITIONAL+=("$1")
      ;;
  esac
  shift
done

AGENCY="${POSITIONAL[0]:-}"
TARGET="${POSITIONAL[1]:-all}"

[[ -z "$AGENCY" ]] && usage

case "$TARGET" in
  all|backend|frontend) ;;
  *)
    echo "Unknown target '$TARGET'." >&2
    usage
    ;;
esac

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"

PIDS=()

cleanup() {
  # Avoid recursion if the trap fires more than once.
  trap - EXIT INT TERM
  if (( ${#PIDS[@]} > 0 )); then
    echo
    echo "[start-dev] Stopping ${#PIDS[@]} process(es)..."
    for pid in "${PIDS[@]}"; do
      if kill -0 "$pid" 2>/dev/null; then
        # Negative PID -> signal the whole process group (set -m makes each
        # background subshell its own pgroup leader with pgid == pid).
        kill -TERM "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
      fi
    done
    wait 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

# Sets BE_PORT, FE_PORT, IDP_CLIENT_ID, NSW_CLIENT_ID for the given agency.
resolve_agency() {
  local varname="CONFIG_$1"
  local config="${!varname:-}"
  if [[ -z "$config" ]]; then
    echo "Unknown agency '$1'. Expected: ${ALL_AGENCIES[*]}, all." >&2
    return 1
  fi
  IFS='|' read -r BE_PORT FE_PORT IDP_CLIENT_ID NSW_CLIENT_ID APP_NAME <<<"$config"
  APP_NAME="${APP_NAME:-$1}"
}

# Source a .env file without clobbering vars already set in the environment.
# This preserves parent-shell overrides (e.g. PORT=9000 ./start.sh npqs).
source_env_nonclobber() {
  local file=$1 line key
  [[ -f "$file" ]] || return 0
  while IFS= read -r line || [[ -n "$line" ]]; do
    case "$line" in ''|\#*) continue ;; esac
    [[ "$line" == *=* ]] || continue
    key="${line%%=*}"
    key="${key#export }"
    [[ "$key" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || continue
    # Already set in env (even to empty string) -> skip.
    [[ -n "${!key+x}" ]] && continue
    set -a
    eval "$line"
    set +a
  done <"$file"
}

# ---------------------------------------------------------------------------
# clean_databases: wipe agency DB(s) before starting.
#   SQLite   -> delete {agency}_applications.db from BACKEND_DIR
#   Postgres -> terminate connections, drop, and recreate the database
# ---------------------------------------------------------------------------
clean_databases() {
  local agencies=("$@")
  local db_driver="${DB_DRIVER:-sqlite}"

  echo "[start-dev] Cleaning agency databases (driver: $db_driver)..."

  if [[ "$db_driver" == "sqlite" ]]; then
    for agency in "${agencies[@]}"; do
      local db_path="$BACKEND_DIR/${agency}_applications.db"
      if [[ -f "$db_path" ]]; then
        echo "[start-dev]   Deleting SQLite DB for $agency: $db_path"
        rm -f "$db_path"
      else
        echo "[start-dev]   SQLite DB for $agency not found (nothing to delete): $db_path"
      fi
    done

  elif [[ "$db_driver" == "postgres" ]]; then
    if ! command -v psql >/dev/null 2>&1; then
      echo "[start-dev] Error: psql required for Postgres DB cleaning but not found in PATH." >&2
      exit 1
    fi
    local db_host="${DB_HOST:-localhost}"
    local db_port="${DB_PORT:-5432}"
    local db_user="${DB_USER:-postgres}"
    local db_password="${DB_PASSWORD:-changeme}"
    local db_name="${DB_NAME:-nsw_agency_db}"
    # Postgres uses a single shared database; warn if only a subset of agencies
    # was selected since this will wipe data for all agencies, not just the chosen ones.
    if [[ "${#agencies[@]}" -lt "${#ALL_AGENCIES[@]}" ]]; then
      echo "[start-dev] Warning: Postgres uses a shared database ($db_name). --clean-run will wipe data for ALL agencies, not just: ${agencies[*]}." >&2
    fi
    echo "[start-dev]   Dropping and recreating Postgres database: $db_name"
    PGPASSWORD="$db_password" psql -h "$db_host" -p "$db_port" -U "$db_user" -d postgres -c \
      "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '$db_name' AND pid <> pg_backend_pid();" \
      >/dev/null
    PGPASSWORD="$db_password" psql -h "$db_host" -p "$db_port" -U "$db_user" -d postgres \
      -c "DROP DATABASE IF EXISTS \"$db_name\";"
    PGPASSWORD="$db_password" psql -h "$db_host" -p "$db_port" -U "$db_user" -d postgres \
      -c "CREATE DATABASE \"$db_name\";"

  else
    echo "[start-dev] Unknown DB_DRIVER '$db_driver'; skipping database clean." >&2
  fi
}

ensure_branding_file() {
  local agency=$1 app_name=$2
  local config_dir="$ROOT_DIR/frontend/public/configs"
  local file="$config_dir/${agency}.branding.json"
  mkdir -p "$config_dir"
  cat >"$file" <<EOF
{
  "branding": {
    "systemName": "NSW",
    "appName": "${app_name}",
    "logoUrl": "",
    "systemLogoUrl": "",
    "favicon": "",
    "portalName": "",
    "description": "",
    "heroImageUrl": "",
    "partnerLogos": [{"url": "", "alt": ""}]
  }
}
EOF
  echo "[start-dev] Wrote branding file: $file"
}

start_backend() {
  local agency=$1
  resolve_agency "$agency"
  echo "[start-dev] Starting $agency backend  -> http://localhost:$BE_PORT (db: ${agency}_applications.db)"
  (
    cd "$BACKEND_DIR"
    # Apply per-agency values BEFORE sourcing .env so they aren't clobbered by
    # the generic .env defaults (which typically have PORT=8081 etc.).
    # ${VAR:-…} preserves a parent-shell override.
    # Final precedence: parent env > script per-agency > .env > Go-side fallback.
    export PORT="${PORT:-$BE_PORT}"
    export DB_DRIVER="${DB_DRIVER:-sqlite}"
    export DB_PATH="${DB_PATH:-./${agency}_applications.db}"
    export NSW_CLIENT_ID
    # The Go server does not autoload .env — source it (non-clobber) so
    # NSW_* vars (API base URL, OAuth client secret, token URL) reach
    # the process without overriding anything already set above.
    if [[ -f .env ]]; then
      source_env_nonclobber .env
    else
      echo "[start-dev] WARNING: backend/.env not found — backend will fail if NSW_* vars are unset." >&2
    fi
    exec go run ./cmd/server
  ) &
  PIDS+=("$!")
}

start_frontend() {
  local agency=$1
  resolve_agency "$agency"
  echo "[start-dev] Starting $agency frontend -> http://localhost:$FE_PORT (branding: $agency, idp: $IDP_CLIENT_ID)"
  (
    cd "$FRONTEND_DIR"
    # Vite autoloads frontend/.env but only reads VITE_PORT from process env.
    ensure_branding_file "$agency" "$APP_NAME"
    VITE_PORT="${VITE_PORT:-$FE_PORT}" \
    VITE_BRANDING_NAME="${VITE_BRANDING_NAME:-$agency}" \
    VITE_API_BASE_URL="${VITE_API_BASE_URL:-http://localhost:$BE_PORT}" \
    VITE_IDP_BASE_URL="${VITE_IDP_BASE_URL:-https://localhost:8090}" \
    VITE_IDP_CLIENT_ID="${VITE_IDP_CLIENT_ID:-$IDP_CLIENT_ID}" \
    VITE_APP_URL="${VITE_APP_URL:-http://localhost:$FE_PORT}" \
    exec pnpm run dev
  ) &
  PIDS+=("$!")
}

# Load optional root-level env file before per-agency defaults.
if [[ -n "$ENV_FILE" ]]; then
  if [[ ! -f "$ENV_FILE" ]]; then
    echo "[start-dev] Error: --env-file not found: $ENV_FILE" >&2
    exit 1
  fi
  source_env_nonclobber "$ENV_FILE"
fi

# Resolve the agency list to launch.
if [[ "$AGENCY" == "all" ]]; then
  AGENCIES=("${ALL_AGENCIES[@]}")
else
  # Validate it's a known agency without polluting globals (subshell).
  ( resolve_agency "$AGENCY" > /dev/null ) || usage
  AGENCIES=("$AGENCY")
fi

if [[ "$CLEAN_RUN" == "true" ]]; then
  clean_databases "${AGENCIES[@]}"
fi

for o in "${AGENCIES[@]}"; do
  [[ "$TARGET" == "all" || "$TARGET" == "backend"  ]] && start_backend  "$o"
  [[ "$TARGET" == "all" || "$TARGET" == "frontend" ]] && start_frontend "$o"
done

echo "[start-dev] ${#PIDS[@]} process(es) running. Logs from all processes will interleave below. Press Ctrl-C to stop."
wait
