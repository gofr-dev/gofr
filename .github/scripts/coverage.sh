#!/usr/bin/env bash
# Coverage merge and gates for the Workflow-Pipeline (.github/workflows/go.yml).
#
#   coverage.sh merge  <artifacts-dir> <out-profile>
#   coverage.sh total  <profile>
#   coverage.sh floor  <figure> <floor>
#   coverage.sh nodrop <figure> <baseline|""> <base-branch>
#
# parse_coverage and upload_coverage both call `merge`, so the figure gated on
# a PR and the figure published to qlty come from one merge.
#
# Why the overlap check exists (#4155): on 2026-09-25 the reported figure fell
# from 95% to 92.3% with no test removed. A cached `go test` result replayed a
# coverage profile written against an older logging/logger.go, so the merged
# profile held two sets of blocks for one file at shifted positions. Covered
# statements stayed level and the statement count grew by about 600. Two
# DIFFERENT blocks over overlapping positions in one file can only come from
# profiles of two versions of that file, so the merge refuses them. Identical
# blocks are normal (a multi-package -coverpkg run emits each block once per
# test binary) and are not an overlap.
#
# Messages use GitHub workflow commands (::error::, ::warning::). When
# GITHUB_STEP_SUMMARY is set, results are also appended to the job summary.
set -euo pipefail

usage() {
  sed -n '2,7p' "$0" >&2
  exit 2
}

summary() {
  if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    printf '%s\n' "$*" >> "$GITHUB_STEP_SUMMARY"
  fi
}

# blocks <profile>: number of coverage block lines (everything but the mode line).
blocks() {
  awk '!/^mode:/ && NF { n++ } END { print n + 0 }' "$1"
}

# require_profile <path>: fail unless the profile exists and has a block.
require_profile() {
  if [ ! -f "$1" ]; then
    echo "::error::$1 is missing: that test job's coverage would silently drop out of the figure"
    exit 1
  fi
  if [ "$(blocks "$1")" -eq 0 ]; then
    echo "::error::$1 has no coverage blocks: that test job's coverage would silently drop out of the figure"
    exit 1
  fi
}

# overlaps <profile>: print "<count> <file>" for every file holding two
# different blocks over overlapping positions, most blocks first. Prints
# nothing when the profile is clean.
overlaps() {
  # Identical block keys are de-duplicated first. Positions become
  # line*1e6+column so a numeric sort orders them; a block is [start, end).
  awk '!/^mode:/ && NF { print $1 }' "$1" |
    sort -u |
    awk '{
      n = split($1, parts, ":"); pos = parts[n]
      file = substr($1, 1, length($1) - length(pos) - 1)
      split(pos, se, ","); split(se[1], s, "."); split(se[2], e, ".")
      printf "%s\t%d\t%d\n", file, s[1] * 1000000 + s[2], e[1] * 1000000 + e[2]
    }' |
    sort -t "$(printf '\t')" -k1,1 -k2,2n |
    awk -F '\t' '
      function flush(   i, c) {
        c = 0
        for (i = 1; i <= nb; i++) if (bad[i]) c++
        if (c > 0) print c, prev
        delete bad
        nb = 0
      }
      $1 != prev { flush(); prev = $1 }
      {
        nb++; st[nb] = $2; en[nb] = $3
        # Starts are sorted, so an earlier block overlaps this one exactly when
        # it ends after this one starts.
        for (i = 1; i < nb; i++) if (en[i] > $2) { bad[i] = 1; bad[nb] = 1 }
      }
      END { flush() }' |
    sort -k1,1nr -k2,2
}

cmd_merge() {
  [ $# -eq 2 ] || usage
  local dir="$1" out="$2"
  local example="$dir/Example-Test-Report/profile.cov"
  local pkg="$dir/PKG-Coverage-Report/profile.cov"

  require_profile "$example"
  require_profile "$pkg"

  local submodules=() f
  shopt -s nullglob
  for f in "$dir"/submodule-coverage-reports/*.cov; do
    submodules+=("$f")
  done
  shopt -u nullglob
  if [ ${#submodules[@]} -eq 0 ]; then
    echo "::error::no submodule coverage profiles found in $dir/submodule-coverage-reports: the merged figure would silently exclude them"
    exit 1
  fi
  for f in "${submodules[@]}"; do
    require_profile "$f"
  done

  # `/mock_` strips generated gomock files, matching the PKG job, which strips
  # them before it uploads its profile. Without it the two halves of one
  # figure are computed under different rules, and the heavily exercised mocks
  # under the submodules flatter the merged number.
  echo "mode: atomic" > "$out"
  awk '!/^mode:/ && NF && !/\/mock_/' "$example" "$pkg" "${submodules[@]}" >> "$out"

  local found
  found="$(overlaps "$out")"
  if [ -n "$found" ]; then
    local nblocks nfiles
    nblocks="$(printf '%s\n' "$found" | awk '{ s += $1 } END { print s }')"
    nfiles="$(printf '%s\n' "$found" | awk 'END { print NR }')"
    echo "::error::$nblocks overlapping block(s) in $nfiles file(s): the merged profile holds coverage from two versions of the same file, which means a stale profile was merged (see #4155). First 10 files:"
    printf '%s\n' "$found" | awk 'NR <= 10 { printf "  %6d  %s\n", $1, $2 }'
    summary "**Coverage merge failed:** $nblocks overlapping block(s) in $nfiles file(s), so a stale profile was merged."
    exit 1
  fi

  echo "merged $((2 + ${#submodules[@]})) profile(s) (Example, PKG and ${#submodules[@]} submodule(s)) into $out: $(blocks "$out") blocks"
}

cmd_total() {
  [ $# -eq 1 ] || usage
  local report figure
  report="$(go tool cover -func="$1")"
  figure="$(printf '%s\n' "$report" | awk '$1 == "total:" { sub(/%$/, "", $NF); print $NF }')"
  check_figure "$figure" "total"
  printf '%s\n' "$figure"
}

# check_figure <value> <what>: fail unless value looks like 95 or 95.5.
check_figure() {
  if ! [[ "$1" =~ ^[0-9]+(\.[0-9])?$ ]]; then
    echo "::error::$2 \"$1\" is not a coverage figure (expected a number such as 95.5)"
    exit 1
  fi
}

# tenths <figure>: 95.5 -> 955, 94 -> 940. Integer compare, no float rounding.
tenths() {
  local whole="${1%%.*}" frac=0
  [[ "$1" == *.* ]] && frac="${1#*.}"
  echo $((10#$whole * 10 + 10#$frac))
}

cmd_floor() {
  [ $# -eq 2 ] || usage
  check_figure "$1" "coverage"
  check_figure "$2" "floor"
  if [ "$(tenths "$1")" -lt "$(tenths "$2")" ]; then
    echo "::error::coverage $1% is below the $2% floor"
    summary "**Floor: FAILED.** $1% is below the $2% floor."
    exit 1
  fi
  echo "coverage $1% meets the $2% floor"
  summary "**Floor: passed.** $1% meets the $2% floor."
}

cmd_nodrop() {
  [ $# -eq 3 ] || usage
  local figure="$1" baseline="$2" branch="$3"
  check_figure "$figure" "coverage"
  if [ -z "$baseline" ]; then
    echo "::warning::no-drop check NOT run: no baseline on $branch (no unexpired coverage-total artifact from a push run). The floor still applies."
    summary "**No drop: no-drop check NOT run: no baseline on $branch.** The floor still applies."
    return 0
  fi
  check_figure "$baseline" "baseline"
  if [ "$(tenths "$figure")" -lt "$(tenths "$baseline")" ]; then
    echo "::error::coverage $figure% is below $branch's $baseline%: a PR may not lower overall coverage (CONTRIBUTING.md, Test Coverage)"
    summary "**No drop: FAILED.** $figure% is below $branch's $baseline%."
    exit 1
  fi
  echo "coverage $figure% is not below $branch's $baseline%"
  summary "**No drop: passed.** $figure% is not below $branch's $baseline%."
}

[ $# -ge 1 ] || usage
sub="$1"
shift
case "$sub" in
  merge) cmd_merge "$@" ;;
  total) cmd_total "$@" ;;
  floor) cmd_floor "$@" ;;
  nodrop) cmd_nodrop "$@" ;;
  *) usage ;;
esac
