
#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PROJECT_NAME="xazavier-smoke"

cd "$ROOT"

compose() {
  docker compose -p "$PROJECT_NAME" "$@"
}

if ! command -v docker >/dev/null 2>&1; then
  echo "ERROR: Docker is required for smoke tests" >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "ERROR: Docker Compose is unavailable" >&2
  exit 1
fi

if [ ! -f .env ]; then
  echo "ERROR: .env is missing. Configure JWT_SECRET first." >&2
  exit 1
fi

cleanup() {
  echo "Cleaning up smoke-test containers..."
  compose down || true
}
trap cleanup EXIT

echo "Starting isolated Xazavier smoke-test environment..."
compose up -d --build

echo "Waiting for API health check..."
healthy=false

for i in $(seq 1 40); do
  if curl -fsS http://localhost:8080/healthz >/dev/null 2>&1; then
    healthy=true
    break
  fi
  sleep 2
done

if [ "$healthy" != true ]; then
  echo "ERROR: API did not become healthy within 80 seconds." >&2
  compose logs api
  exit 1
fi

echo "Checking health endpoint..."
curl -fsS http://localhost:8080/healthz
echo

echo "Checking readiness endpoint..."
curl -fsS http://localhost:8080/readyz
echo

echo "Smoke tests passed."
