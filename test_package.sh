#!/bin/bash
set -euo pipefail

echo "🚀 Running PKG Tests..."
export APP_ENV=test

# Run tests with coverage
go test -v -short -coverprofile=gofr.cov -coverpkg=./pkg/gofr ./pkg/gofr
go test -v -coverprofile=subpkgs.cov -coverpkg=./pkg/gofr ./pkg/gofr/...

# Combine coverage
echo "mode: atomic" > pkg_coverage.cov
grep -h -v "mode:" gofr.cov subpkgs.cov | grep -v '/mock_' >> pkg_coverage.cov

# Output for CI
if [ -n "${CI:-}" ]; then
  echo "📊 PKG Coverage:"
  go tool cover -func pkg_coverage.cov
else
  go tool cover -html=pkg_coverage.cov -o pkg_coverage.html
fi