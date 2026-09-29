#!/bin/sh
# Usage: run.sh <goroot>   Copies the probe into <goroot>/src/runtime, runs it, removes it.
set -e
R="$1/src/runtime"
cp "$(dirname "$0")"/*_test.go "$R/"
trap 'rm -f "$R/export_gsignalprobe_test.go" "$R/gsignalprobe_test.go"' EXIT
"$1/bin/go" test runtime -run '^TestGsignalProbe$' -count=1 -v 2>&1 | grep -E 'Ms,|^(ok|FAIL|---)'
