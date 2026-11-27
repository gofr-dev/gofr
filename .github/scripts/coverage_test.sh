#!/usr/bin/env bash
# Self-test for coverage.sh. CI runs it before the real gate (parse_coverage in
# .github/workflows/go.yml), so a change that leaves a guard passing everything
# fails here instead of going quietly inert.
#
# Every case asserts the exit code AND a fragment of the message, so a script
# that fails for the wrong reason does not count as a pass.
#
# Usage: bash .github/scripts/coverage_test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
script="$here/coverage.sh"

work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# Synthetic coverage profiles, written into the temporary directory so no .cov
# files are committed. pkg_dip.cov is pkg.cov plus three uncovered blocks, so
# its merged figure is lower. sub_a.cov repeats a block (the merge must keep the
# highest count) and covers a mock_ file (the merge must drop it). sub_empty.cov
# has the mode line and no blocks.
fixtures="$work/fixtures"
mkdir -p "$fixtures"

cat > "$fixtures/example.cov" <<'EOF'
mode: atomic
gofr.dev/examples/http-server/main.go:10.13,14.2 3 1
gofr.dev/examples/http-server/main.go:16.40,18.2 1 0
EOF

cat > "$fixtures/pkg.cov" <<'EOF'
mode: atomic
gofr.dev/pkg/gofr/logging/logger.go:20.30,24.16 3 7
gofr.dev/pkg/gofr/logging/logger.go:24.16,26.3 1 2
gofr.dev/pkg/gofr/logging/logger.go:28.2,28.10 1 7
gofr.dev/pkg/gofr/gofr.go:40.20,45.2 4 1
EOF

cat > "$fixtures/pkg_dip.cov" <<'EOF'
mode: atomic
gofr.dev/pkg/gofr/logging/logger.go:20.30,24.16 3 7
gofr.dev/pkg/gofr/logging/logger.go:24.16,26.3 1 2
gofr.dev/pkg/gofr/logging/logger.go:28.2,28.10 1 7
gofr.dev/pkg/gofr/logging/logger.go:22.30,26.16 3 0
gofr.dev/pkg/gofr/logging/logger.go:26.16,28.3 1 0
gofr.dev/pkg/gofr/logging/logger.go:28.5,28.12 1 0
gofr.dev/pkg/gofr/gofr.go:40.20,45.2 4 1
EOF

cat > "$fixtures/sub_a.cov" <<'EOF'
mode: atomic
gofr.dev/pkg/gofr/datasource/mongo/mongo.go:12.50,15.2 2 4
gofr.dev/pkg/gofr/datasource/mongo/mongo.go:17.40,19.2 1 0
gofr.dev/pkg/gofr/datasource/mongo/mongo.go:12.50,15.2 2 1
gofr.dev/pkg/gofr/datasource/mongo/mock_interfaces.go:5.10,9.2 3 9
EOF

cat > "$fixtures/sub_b.cov" <<'EOF'
mode: atomic
gofr.dev/pkg/gofr/datasource/redis/redis.go:8.30,10.2 1 3
EOF

printf 'mode: atomic\n' > "$fixtures/sub_empty.cov"

passed=0
failed=0

# artifacts <name> <example|-> <pkg|-> [submodule fixtures...]
# Builds an artifacts directory laid out the way actions/download-artifact
# lays out the three coverage artifacts. "-" leaves that artifact out.
artifacts() {
  local dir="$work/$1" example="$2" pkg="$3"
  shift 3
  mkdir -p "$dir"
  if [ "$example" != "-" ]; then
    mkdir -p "$dir/Example-Test-Report"
    cp "$fixtures/$example" "$dir/Example-Test-Report/profile.cov"
  fi
  if [ "$pkg" != "-" ]; then
    mkdir -p "$dir/PKG-Coverage-Report"
    cp "$fixtures/$pkg" "$dir/PKG-Coverage-Report/profile.cov"
  fi
  if [ $# -gt 0 ]; then
    mkdir -p "$dir/submodule-coverage-reports"
    local f
    for f in "$@"; do
      cp "$fixtures/$f" "$dir/submodule-coverage-reports/$f"
    done
  fi
  printf '%s' "$dir"
}

# expect <name> <want-rc> <want-fragment> -- <command...>
# Runs the command with GITHUB_STEP_SUMMARY pointing at a scratch file, and
# checks the exit code and that the fragment appears in stdout+stderr+summary.
expect() {
  local name="$1" want_rc="$2" want_text="$3"
  shift 4
  local out="$work/out.$passed.$failed" summary="$work/summary.$passed.$failed" rc=0
  : > "$summary"
  GITHUB_STEP_SUMMARY="$summary" "$@" > "$out" 2>&1 || rc=$?
  cat "$summary" >> "$out"
  if [ "$rc" -ne "$want_rc" ]; then
    echo "FAIL $name: exit $rc, want $want_rc"
    sed 's/^/    /' "$out"
    failed=$((failed + 1))
    return 0
  fi
  if ! grep -qF -- "$want_text" "$out"; then
    echo "FAIL $name: output lacks \"$want_text\""
    sed 's/^/    /' "$out"
    failed=$((failed + 1))
    return 0
  fi
  echo "ok   $name"
  passed=$((passed + 1))
}

# --- merge -------------------------------------------------------------------

clean="$(artifacts clean example.cov pkg.cov sub_a.cov sub_b.cov)"
expect "merge: clean profiles pass" 0 "merged 4 profile(s)" -- \
  bash "$script" merge "$clean" "$work/clean.cov"

# The merged profile keeps one mode line, drops mocks, keeps everything else.
# Identical blocks (sub_a lists one twice, as a multi-package -coverpkg run
# does) are not an overlap and stay in the profile for `go tool cover`.
if [ -f "$work/clean.cov" ] &&
  [ "$(grep -c '^mode:' "$work/clean.cov")" -eq 1 ] &&
  ! grep -q '/mock_' "$work/clean.cov" &&
  [ "$(grep -vc '^mode:' "$work/clean.cov")" -eq 10 ]; then
  echo "ok   merge: output has one mode line, no mocks, 10 blocks"
  passed=$((passed + 1))
else
  echo "FAIL merge: merged profile content is wrong"
  [ -f "$work/clean.cov" ] && sed 's/^/    /' "$work/clean.cov"
  failed=$((failed + 1))
fi

dip="$(artifacts dip example.cov pkg_dip.cov sub_a.cov sub_b.cov)"
expect "merge: dip-shaped overlap fails" 1 "gofr.dev/pkg/gofr/logging/logger.go" -- \
  bash "$script" merge "$dip" "$work/dip.cov"
expect "merge: dip overlap counts blocks and files" 1 "6 overlapping block(s) in 1 file(s)" -- \
  bash "$script" merge "$dip" "$work/dip.cov"

empty_sub="$(artifacts empty_sub example.cov pkg.cov sub_a.cov sub_empty.cov)"
expect "merge: empty submodule profile fails" 1 "sub_empty.cov has no coverage blocks" -- \
  bash "$script" merge "$empty_sub" "$work/empty_sub.cov"

no_pkg="$(artifacts no_pkg example.cov - sub_a.cov)"
expect "merge: missing PKG profile fails" 1 "PKG-Coverage-Report/profile.cov is missing" -- \
  bash "$script" merge "$no_pkg" "$work/no_pkg.cov"

no_example="$(artifacts no_example - pkg.cov sub_a.cov)"
expect "merge: missing Example profile fails" 1 "Example-Test-Report/profile.cov is missing" -- \
  bash "$script" merge "$no_example" "$work/no_example.cov"

no_sub="$(artifacts no_sub example.cov pkg.cov)"
expect "merge: no submodule profiles fails" 1 "no submodule coverage profiles" -- \
  bash "$script" merge "$no_sub" "$work/no_sub.cov"

# --- floor -------------------------------------------------------------------

expect "floor: 93.9 is below 94" 1 "93.9% is below the 94% floor" -- \
  bash "$script" floor 93.9 94
expect "floor: 94.0 meets 94" 0 "94.0% meets the 94% floor" -- \
  bash "$script" floor 94.0 94
expect "floor: a non-number figure fails" 1 "not a coverage figure" -- \
  bash "$script" floor "" 94

# --- no drop -----------------------------------------------------------------

expect "nodrop: 95.4 vs 95.5 fails" 1 "95.4% is below development's 95.5%" -- \
  bash "$script" nodrop 95.4 95.5 development
expect "nodrop: 95.5 vs 95.5 passes" 0 "95.5% is not below development's 95.5%" -- \
  bash "$script" nodrop 95.5 95.5 development
expect "nodrop: 95.6 vs 95.5 passes" 0 "95.6% is not below development's 95.5%" -- \
  bash "$script" nodrop 95.6 95.5 development
expect "nodrop: no baseline passes with a warning" 0 "no-drop check NOT run: no baseline on development" -- \
  bash "$script" nodrop 95.5 "" development
expect "nodrop: a malformed baseline fails" 1 "not a coverage figure" -- \
  bash "$script" nodrop 95.5 "95.5%x" development

echo
echo "coverage_test.sh: $passed passed, $failed failed"
[ "$failed" -eq 0 ] && [ "$passed" -gt 0 ]
