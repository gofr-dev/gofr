#!/bin/bash
set -euo pipefail

echo "🚀 Running Examples Tests..."
export APP_ENV=test

# CI detection - only start services if not in CI
if [ -z "${CI:-}" ]; then
  docker-compose -f test-services.yml up -d
  trap "docker-compose -f test-services.yml down" EXIT
  sleep 10
fi

# Run tests with coverage
go test ./examples/... -v -short -coverprofile=examples.cov -coverpkg=./examples/...

# Process coverage
grep -vE '(/client/|grpc-.+-client/main\.go|_client\.go|_gofr\.go|_grpc\.pb\.go|\.pb\.go|\.proto|health_.*\.go)' examples.cov > examples_filtered.cov

# Output for CI
if [ -n "${CI:-}" ]; then
  echo "📊 Examples Coverage:"
  go tool cover -func examples_filtered.cov
else
  go tool cover -html=examples_filtered.cov -o examples_coverage.html
fi