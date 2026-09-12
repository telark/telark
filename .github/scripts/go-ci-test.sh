#!/usr/bin/env bash
set -euo pipefail

# -coverpkg=./... so tests in internal/tests/* count against the code they
# exercise (per-package coverage reports 0 for this layout).
go test -race -coverpkg=./... -coverprofile=coverage.out ./...
total=$(go tool cover -func=coverage.out | awk '/^total:/{print $3}' | tr -d '%')
echo "Total coverage: ${total}%"
awk -v t="$total" -v m="$COVERAGE_THRESHOLD" \
  'BEGIN { if (t+0 < m+0) { printf "coverage %.1f%% is below threshold %s%%\n", t, m; exit 1 } }'
