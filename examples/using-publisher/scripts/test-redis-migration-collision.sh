#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

COMPOSE_FILE="$ROOT_DIR/docker-compose.yml"
PROJECT_NAME="using-publisher"

function compose() {
  docker compose -p "$PROJECT_NAME" -f "$COMPOSE_FILE" "$@"
}

function cleanup() {
  compose down -v --remove-orphans >/dev/null 2>&1 || true
}

trap cleanup EXIT

echo "Starting Redis via docker compose..."
compose up -d

echo "Waiting for Redis healthcheck..."
for _ in $(seq 1 60); do
  if compose exec -T redis redis-cli ping >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

compose exec -T redis redis-cli ping >/dev/null

echo "Flushing DB 0, DB 1, and DB 15..."
compose exec -T redis redis-cli -n 0 flushdb >/dev/null
compose exec -T redis redis-cli -n 1 flushdb >/dev/null
compose exec -T redis redis-cli -n 15 flushdb >/dev/null

export GOFR_TELEMETRY=false
export PUBSUB_BACKEND=REDIS
export REDIS_HOST=localhost
export REDIS_PORT=6379
export REDIS_PUBSUB_MODE=streams
export REDIS_STREAMS_CONSUMER_GROUP=gofr-example
export MIGRATE_ONLY=true

echo
echo "Case 1: REDIS_PUBSUB_DB unset (defaults to DB 15, expected to PASS)..."
export REDIS_DB=0
unset REDIS_PUBSUB_DB || true

OUTPUT_UNSET_DB="$(cd "$ROOT_DIR" && go run . 2>&1)"
echo "$OUTPUT_UNSET_DB"

if [[ $? -ne 0 ]]; then
  echo "ERROR: expected success when REDIS_PUBSUB_DB is unset (defaults to DB 15), but command failed"
  exit 1
fi

echo "OK: migrations succeeded when REDIS_PUBSUB_DB is unset (defaults to safe DB 15)"

echo
echo "Case 2: Same DB explicitly set (expected to FAIL with WRONGTYPE)..."
export REDIS_DB=0
export REDIS_PUBSUB_DB=0

set +e
OUTPUT_SAME_DB="$(cd "$ROOT_DIR" && go run . 2>&1)"
EXIT_SAME_DB=$?
set -e

echo "$OUTPUT_SAME_DB"

if [[ $EXIT_SAME_DB -eq 0 ]]; then
  echo "ERROR: expected failure when Redis PubSub shares Redis DB, but command succeeded"
  exit 1
fi

echo "$OUTPUT_SAME_DB" | grep -q "WRONGTYPE Operation against a key holding the wrong kind of value"
echo "OK: saw WRONGTYPE for same DB"

echo
echo "Case 3: Split DB (expected to PASS)..."
export REDIS_DB=0
export REDIS_PUBSUB_DB=1

OUTPUT_SPLIT_DB="$(cd "$ROOT_DIR" && go run . 2>&1)"
echo "$OUTPUT_SPLIT_DB"

if [[ $? -ne 0 ]]; then
  echo "ERROR: expected success when PubSub uses a different Redis DB, but command failed"
  exit 1
fi

echo "OK: migrations succeeded when PubSub uses a different Redis DB"

