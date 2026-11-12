#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required for smoke tests" >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "docker compose not available" >&2
  exit 1
fi

if ! docker compose up -d --build >/dev/null 2>&1; then
  echo "docker compose up failed" >&2
  exit 1
fi

cleanup() {
  docker compose down -v >/dev/null 2>&1 || true
}
trap cleanup EXIT

for i in $(seq 1 40); do
  if curl -fsS http://localhost:8080/healthz >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

curl -fsS http://localhost:8080/healthz
curl -fsS http://localhost:8080/readyz
