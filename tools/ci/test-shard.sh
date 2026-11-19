#!/usr/bin/env bash
# Split top-level tests deterministically. Subtests run with their parent.
set -euo pipefail

if [[ $# -lt 3 || ! $2 =~ ^(0|[1-9][0-9]{0,2})$ || ! $3 =~ ^[1-9][0-9]{0,2}$ || $2 -ge $3 ]]; then
  echo 'Usage: bash tools/ci/test-shard.sh PACKAGE INDEX COUNT [--regular | --long] [--list | GO_TEST_FLAGS...]' >&2
  exit 2
fi

package=$1
index=$2
count=$3
shift 3

mode=complete
case ${1-} in
  --regular) mode=regular; shift ;;
  --long) mode=long; shift ;;
esac
if [[ $mode != complete && $package != ./brep/trimmed ]]; then
  echo 'Regular and long partitions are defined only for ./brep/trimmed' >&2
  exit 2
fi

list_args=("$package" -list '^(Test|Example|Fuzz)')
for flag in "$@"; do
  case $flag in
    -race|--race|-race=*|--race=*) list_args+=("$flag") ;;
    -run|--run|-run=*|--run=*|-skip|--skip|-skip=*|--skip=*|-args|--args|-fuzz|--fuzz|-fuzz=*|--fuzz=*)
      echo 'Flags that change test selection are not allowed in a shard' >&2
      exit 2 ;;
  esac
done
listing=$(go test "${list_args[@]}")
long_names='^$'
if [[ $mode != complete ]]; then
  long_tests=(
    TestAlgebraicShellTripleRadialMaterialComponents
    TestCertifiedSelectedNestedDiskAnnulus
    TestNestedQuarticTubeMaterial
  )
  for test in "${long_tests[@]}"; do
    if ! grep -Fxq "$test" <<< "$listing"; then
      echo "Missing long test in test-shard.sh: $test (the long tests are behind the research build tag; set GOFLAGS=-tags=research)" >&2
      exit 1
    fi
  done
  long_names=$(printf '%s\n' "${long_tests[@]}" | paste -sd '|' -)
fi

selected=$(printf '%s\n' "$listing" | LC_ALL=C sort | awk -v index_="$index" -v count_="$count" -v mode_="$mode" -v long_="$long_names" '
  /^(Test|Example|Fuzz)[^[:space:]]*$/ {
    is_long = ($0 ~ ("^(" long_ ")$"))
    if ((mode_ == "regular" && is_long) || (mode_ == "long" && !is_long)) next
    if (n++ % count_ == index_) print
  }
')
if [[ -z $selected ]]; then
  echo "No $package tests assigned to shard $index/$count" >&2
  exit 1
fi
if [[ $# -eq 1 && $1 == --list ]]; then
  printf '%s\n' "$selected"
  exit 0
fi
pattern=$(printf '%s\n' "$selected" | paste -sd '|' -)
exec go test "$package" -run "^($pattern)$" "$@"
