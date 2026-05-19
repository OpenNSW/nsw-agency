#!/usr/bin/env bash
# Run OGA backends and/or frontends with per-agency config.
#
# Usage:
#   ./run-oga.sh <oga> [target]
#
#   <oga>     One of: npqs, fcau, ird, cda, default, all
#             'all' fans out and starts every agency in parallel.
#   [target]  One of: all (default), backend, frontend
#
# Each OGA maps to its own:
#   - backend HTTP port and SQLite DB file
#   - frontend dev server port
#   - frontend branding config (public/configs/<oga>.branding.json)
#   - IdP client id
#
# Examples:
#   ./run-oga.sh npqs              # NPQS backend + frontend
#   ./run-oga.sh fcau backend      # FCAU backend only
#   ./run-oga.sh ird frontend      # IRD frontend only
#   ./run-oga.sh all               # every backend + frontend, in parallel
#   ./run-oga.sh all backend       # every backend, no frontends
#
# Ctrl-C terminates every child process (each runs in its own process group).

set -euo pipefail
# Enable job control so each backgrounded subshell becomes its own process
# group leader — that lets us kill `go run`'s grandchild binary on cleanup.
set -m

ALL_OGAS=(npqs fcau ird cda)

usage() {
  cat <<EOF >&2
Usage: $0 <oga> [target]

  <oga>     One of: ${ALL_OGAS[*]}, default, all
  [target]  One of: all (default), backend, frontend

Examples:
  $0 npqs                  # NPQS backend + frontend
  $0 fcau backend          # FCAU backend only
  $0 all                   # every OGA, backends + frontends
  $0 all frontend          # every OGA, frontends only
EOF
  exit 1
}

OGA="${1:-}"
TARGET="${2:-all}"

[[ -z "$OGA" ]] && usage

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
    echo "[run-oga] Stopping ${#PIDS[@]} process(es)..."
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
resolve_oga() {
  case "$1" in
    npqs)    BE_PORT=8081; FE_PORT=5174; IDP_CLIENT_ID="OGA_PORTAL_APP_NPQS"; NSW_CLIENT_ID="NPQS_TO_NSW" ;;
    fcau)    BE_PORT=8082; FE_PORT=5175; IDP_CLIENT_ID="OGA_PORTAL_APP_FCAU"; NSW_CLIENT_ID="FCAU_TO_NSW" ;;
    ird)     BE_PORT=8083; FE_PORT=5176; IDP_CLIENT_ID="OGA_PORTAL_APP_IRD";  NSW_CLIENT_ID="IRD_TO_NSW"  ;;
    cda)     BE_PORT=8084; FE_PORT=5177; IDP_CLIENT_ID="OGA_PORTAL_APP_CDA";  NSW_CLIENT_ID="CDA_TO_NSW"  ;;
    default) BE_PORT=8081; FE_PORT=5174; IDP_CLIENT_ID="OGA_TO_NSW";          NSW_CLIENT_ID="OGA_TO_NSW"  ;;
    *)
      echo "Unknown OGA '$1'. Expected: ${ALL_OGAS[*]}, default, all." >&2
      return 1
      ;;
  esac
}

start_backend() {
  local oga=$1
  resolve_oga "$oga"
  echo "[run-oga] Starting $oga backend  -> http://localhost:$BE_PORT (db: ${oga}_applications.db)"
  (
    cd "$BACKEND_DIR"
    # The Go server does not autoload .env — source it so OGA_NSW_* vars
    # (API base URL, OAuth client secret, token URL) reach the process.
    if [[ -f .env ]]; then
      set -a
      # shellcheck disable=SC1091
      source .env
      set +a
    else
      echo "[run-oga] WARNING: backend/.env not found — backend will fail if OGA_NSW_* vars are unset." >&2
    fi
    OGA_PORT="$BE_PORT" \
    OGA_DB_DRIVER="${OGA_DB_DRIVER:-sqlite}" \
    OGA_DB_PATH="./${oga}_applications.db" \
    OGA_NSW_CLIENT_ID="${OGA_NSW_CLIENT_ID:-$NSW_CLIENT_ID}" \
    exec go run ./cmd/server
  ) &
  PIDS+=("$!")
}

start_frontend() {
  local oga=$1
  resolve_oga "$oga"
  echo "[run-oga] Starting $oga frontend -> http://localhost:$FE_PORT (branding: $oga, idp: $IDP_CLIENT_ID)"
  (
    cd "$FRONTEND_DIR"
    # Vite autoloads frontend/.env but only reads VITE_PORT from process env.
    VITE_PORT="$FE_PORT" \
    VITE_BRANDING_NAME="$oga" \
    VITE_API_BASE_URL="${VITE_API_BASE_URL:-http://localhost:$BE_PORT}" \
    VITE_IDP_BASE_URL="${VITE_IDP_BASE_URL:-https://localhost:8090}" \
    VITE_IDP_CLIENT_ID="${VITE_IDP_CLIENT_ID:-$IDP_CLIENT_ID}" \
    VITE_APP_URL="${VITE_APP_URL:-http://localhost:$FE_PORT}" \
    exec pnpm run dev
  ) &
  PIDS+=("$!")
}

# Resolve the OGA list to launch.
if [[ "$OGA" == "all" ]]; then
  OGAS=("${ALL_OGAS[@]}")
else
  # Validate it's a known OGA without polluting globals (subshell).
  ( resolve_oga "$OGA" >/dev/null ) || usage
  OGAS=("$OGA")
fi

for o in "${OGAS[@]}"; do
  [[ "$TARGET" == "all" || "$TARGET" == "backend"  ]] && start_backend  "$o"
  [[ "$TARGET" == "all" || "$TARGET" == "frontend" ]] && start_frontend "$o"
done

echo "[run-oga] ${#PIDS[@]} process(es) running. Logs from all processes will interleave below. Press Ctrl-C to stop."
wait
