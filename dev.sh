#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
API_DIR="$ROOT_DIR/api"
DASHBOARD_DIR="$ROOT_DIR/dashboard"

cleanup() {
  echo ""
  echo "Stopping..."
  [[ -n "${API_PID:-}" ]] && kill "$API_PID" 2>/dev/null || true
  [[ -n "${DASHBOARD_PID:-}" ]] && kill "$DASHBOARD_PID" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

if ! docker info >/dev/null 2>&1; then
  echo "Docker isn't running. Start Docker Desktop and try again." >&2
  exit 1
fi

if ! command -v pnpm >/dev/null 2>&1; then
  echo "pnpm isn't installed. Install it (e.g. 'npm install -g pnpm') and try again." >&2
  exit 1
fi

if [[ ! -f "$API_DIR/.env" ]]; then
  cat > "$API_DIR/.env" <<EOF
DB_HOST=localhost
DB_USER=vitract
DB_PASSWORD=vitract
DB_NAME=vitract
DB_PORT=5433
EOF
  echo "Created $API_DIR/.env with local dev defaults"
fi

COMPOSE_PROJECT="vitract-assessment"

echo "Starting Postgres..."
(cd "$API_DIR" && docker compose -p "$COMPOSE_PROJECT" up -d)

echo "Waiting for Postgres to accept connections..."
until (cd "$API_DIR" && docker compose -p "$COMPOSE_PROJECT" exec -T db pg_isready -U vitract) >/dev/null 2>&1; do
  sleep 1
done

echo "Running migrations..."
(cd "$API_DIR" && go run cmd/migrate/main.go)

echo "Registering lab result files..."
(cd "$API_DIR" && go run cmd/ingest/main.go)

if [[ ! -d "$DASHBOARD_DIR/node_modules" ]]; then
  echo "Installing dashboard dependencies..."
  (cd "$DASHBOARD_DIR" && pnpm install)
fi

echo "Starting API on :8080..."
(cd "$API_DIR" && go run main.go) &
API_PID=$!

echo "Starting dashboard on :5173..."
(cd "$DASHBOARD_DIR" && pnpm run dev) &
DASHBOARD_PID=$!

echo ""
echo "API:       http://localhost:8080"
echo "Dashboard: http://localhost:5173"
echo "Press Ctrl+C to stop both."

wait
