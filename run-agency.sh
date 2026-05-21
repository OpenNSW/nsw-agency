#!/usr/bin/env bash
# Run NSW Agency backends and/or frontends with per-agency config.
#
# Usage:
#   ./run-agency.sh <agency> [target]
#
#   <agency>  One of: npqs, fcau, ird, cda, default, all
#             'all' fans out and starts every agency in parallel.
#   [target]  One of: all (default), backend, frontend
#
# Each agency maps to its own:
#   - backend HTTP port and SQLite DB file
#   - frontend dev server port
#   - frontend branding config (public/configs/<agency>.branding.json)
#   - IdP client id
#
# Env-var precedence (highest to lowest):
#   parent shell env > backend/.env (for backend vars) > script defaults
# i.e. NSW_AGENCY_PORT=9000 ./run-agency.sh npqs honours the override; .env can fill in
# anything the parent didn't set; the per-agency defaults below are the floor.
#
# Examples:
#   ./run-agency.sh npqs              # NPQS backend + frontend
#   ./run-agency.sh fcau backend      # FCAU backend only
#   ./run-agency.sh ird frontend      # IRD frontend only
#   ./run-agency.sh all               # every backend + frontend, in parallel
#   ./run-agency.sh all backend       # every backend, no frontends
#
# Ctrl-C terminates every child process (each runs in its own process group).

set -euo pipefail
# Enable job control so each backgrounded subshell becomes its own process
# group leader — that lets us kill `go run`'s grandchild binary on cleanup.
set -m

# Single source of truth for per-agency config: "BE_PORT|FE_PORT|IDP_CLIENT_ID|NSW_CLIENT_ID".
# Adding an agency means one line here — nothing else.
# (Scalar vars rather than `declare -A` so this works on stock macOS bash 3.2.)
NSW_AGENCY_CONFIG_npqs="8081|5174|NSW_AGENCY_PORTAL_APP_NPQS|NPQS_TO_NSW"
NSW_AGENCY_CONFIG_fcau="8082|5175|NSW_AGENCY_PORTAL_APP_FCAU|FCAU_TO_NSW"
NSW_AGENCY_CONFIG_ird="8083|5176|NSW_AGENCY_PORTAL_APP_IRD|IRD_TO_NSW"
NSW_AGENCY_CONFIG_cda="8084|5177|NSW_AGENCY_PORTAL_APP_CDA|CDA_TO_NSW"
NSW_AGENCY_CONFIG_default="8081|5174|NSW_AGENCY_TO_NSW|NSW_AGENCY_TO_NSW"

# Real agencies (every NSW_AGENCY_CONFIG_* except 'default'), alphabetised for
# predictable launch order in 'all' mode. Derived from the config above so
# adding an agency only requires editing the NSW_AGENCY_CONFIG_* block.
ALL_AGENCIES=()
while IFS= read -r _v; do
  _agency="${_v#NSW_AGENCY_CONFIG_}"
  [[ "$_agency" == "default" ]] && continue
  ALL_AGENCIES+=("$_agency")
done < <(compgen -A variable NSW_AGENCY_CONFIG_ | sort)
unset _v _agency

usage() {
  cat <<EOF >&2
Usage: $0 <agency> [target]

  <agency>  One of: ${ALL_AGENCIES[*]}, default, all
  [target]  One of: all (default), backend, frontend

Examples:
  $0 npqs                  # NPQS backend + frontend
  $0 fcau backend          # FCAU backend only
  $0 all                   # every agency, backends + frontends
  $0 all frontend          # every agency, frontends only
EOF
  exit 1
}

AGENCY="${1:-}"
TARGET="${2:-all}"

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
    echo "[run-agency] Stopping ${#PIDS[@]} process(es)..."
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
  local varname="NSW_AGENCY_CONFIG_$1"
  local config="${!varname:-}"
  if [[ -z "$config" ]]; then
    echo "Unknown agency '$1'. Expected: ${ALL_AGENCIES[*]}, default, all." >&2
    return 1
  fi
  IFS='|' read -r BE_PORT FE_PORT IDP_CLIENT_ID NSW_CLIENT_ID <<<"$config"
}

# Source a .env file without clobbering vars already set in the environment.
# This preserves parent-shell overrides (e.g. NSW_AGENCY_PORT=9000 ./run-agency.sh npqs).
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

start_backend() {
  local agency=$1
  resolve_agency "$agency"
  echo "[run-agency] Starting $agency backend  -> http://localhost:$BE_PORT (db: ${agency}_applications.db)"
  (
    cd "$BACKEND_DIR"
    # Apply per-agency values BEFORE sourcing .env so they aren't clobbered by
    # the generic .env defaults (which typically have NSW_AGENCY_PORT=8081 etc.).
    # ${VAR:-…} preserves a parent-shell override.
    # Final precedence: parent env > script per-agency > .env > Go-side fallback.
    export NSW_AGENCY_PORT="${NSW_AGENCY_PORT:-$BE_PORT}"
    export NSW_AGENCY_DB_DRIVER="${NSW_AGENCY_DB_DRIVER:-sqlite}"
    export NSW_AGENCY_DB_PATH="${NSW_AGENCY_DB_PATH:-./${agency}_applications.db}"
    export NSW_AGENCY_NSW_CLIENT_ID="${NSW_AGENCY_NSW_CLIENT_ID:-$NSW_CLIENT_ID}"
    # The Go server does not autoload .env — source it (non-clobber) so
    # NSW_AGENCY_NSW_* vars (API base URL, OAuth client secret, token URL) reach
    # the process without overriding anything already set above.
    if [[ -f .env ]]; then
      source_env_nonclobber .env
    else
      echo "[run-agency] WARNING: backend/.env not found — backend will fail if NSW_AGENCY_NSW_* vars are unset." >&2
    fi
    exec go run ./cmd/server
  ) &
  PIDS+=("$!")
}

start_frontend() {
  local agency=$1
  resolve_agency "$agency"
  echo "[run-agency] Starting $agency frontend -> http://localhost:$FE_PORT (branding: $agency, idp: $IDP_CLIENT_ID)"
  (
    cd "$FRONTEND_DIR"
    # Vite autoloads frontend/.env but only reads VITE_PORT from process env.
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

# Resolve the agency list to launch.
if [[ "$AGENCY" == "all" ]]; then
  AGENCIES=("${ALL_AGENCIES[@]}")
else
  # Validate it's a known agency without polluting globals (subshell).
  ( resolve_agency "$AGENCY" > /dev/null ) || usage
  AGENCIES=("$AGENCY")
fi

for o in "${AGENCIES[@]}"; do
  [[ "$TARGET" == "all" || "$TARGET" == "backend"  ]] && start_backend  "$o"
  [[ "$TARGET" == "all" || "$TARGET" == "frontend" ]] && start_frontend "$o"
done

echo "[run-agency] ${#PIDS[@]} process(es) running. Logs from all processes will interleave below. Press Ctrl-C to stop."
wait
