#!/usr/bin/env bash
# Split complete top-level tests deterministically. Subtests run with their parent.
set -euo pipefail

if [[ $# -lt 2 || ! $1 =~ ^[0-9]$ || ! $2 =~ ^[1-9]$ || $1 -ge $2 ]]; then
  echo 'Usage: bash tools/ci/trimmed-shard.sh INDEX COUNT [--list | GO_TEST_FLAGS...]' >&2
  exit 2
fi
index=$1
count=$2
shift 2

listing=$(go test ./brep/trimmed -list '^(Test|Example|Fuzz)')
selected=$(printf '%s\n' "$listing" | LC_ALL=C sort | awk -v index_="$index" -v count="$count" '
  /^(Test|Example|Fuzz)[A-Za-z0-9_]*$/ && n++ % count == index_ { print }
')
if [[ -z $selected ]]; then
  echo 'No trimmed-solid tests assigned to shard' >&2
  exit 1
fi
if [[ $# -eq 1 && $1 == --list ]]; then
  printf '%s\n' "$selected"
  exit 0
fi
for flag in "$@"; do
  case $flag in
    -run|--run|-run=*|--run=*|-skip|--skip|-skip=*|--skip=*|-args|--args|-fuzz|--fuzz|-fuzz=*|--fuzz=*)
      echo 'Flags that change test selection are not allowed in a shard' >&2
      exit 2 ;;
  esac
done
pattern=$(printf '%s\n' "$selected" | paste -sd '|')
exec go test ./brep/trimmed -run "^($pattern)$" "$@"
