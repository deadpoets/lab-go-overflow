#!/bin/sh
# Usage: run.sh <goroot> [GOOS]  Copies the probe into <goroot>/src/runtime, runs it, removes it.
set -e
R="$1/src/runtime"; D="$(dirname "$0")"
cp "$D"/excg0probe_amd64.s "$D"/*_test.go "$R/"
trap 'rm -f "$R"/excg0probe_amd64.s "$R"/excg0probe*_test.go "$R"/export_excg0probe_test.go' EXIT
if [ -n "$2" ]; then
  GOOS=$2 GOARCH=amd64 "$1/bin/go" test -c -o "$D/runtime_$2.test" runtime
else
  "$1/bin/go" test runtime -run '^TestExcG0Probe$' -count=1 -v 2>&1 | grep -E 'Ms$|stack|^(ok|FAIL|---)'
fi
