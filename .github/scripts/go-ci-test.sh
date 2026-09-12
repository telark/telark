#!/usr/bin/env bash
set -euo pipefail

# Coverage is charged per package, not per test: -coverpkg=./... rebuilds and
# instruments every package in the module. Callers that enforce no floor were
# paying that for a number nothing reads — measured at 26s vs 4s (rest) and 66s
# vs 9s (kcore). Race detection stays either way; it is nearly free here.
if awk -v m="${COVERAGE_THRESHOLD:-0}" 'BEGIN { exit (m+0 > 0) ? 1 : 0 }'; then
  go test -race ./...
  exit 0
fi

# -coverpkg=./... so tests in internal/tests/* count against the code they
# exercise (per-package coverage reports 0 for this layout).
go test -race -coverpkg=./... -coverprofile=coverage.out ./...
total=$(go tool cover -func=coverage.out | awk '/^total:/{print $3}' | tr -d '%')
echo "Total coverage: ${total}%"
awk -v t="$total" -v m="$COVERAGE_THRESHOLD" \
  'BEGIN { if (t+0 < m+0) { printf "coverage %.1f%% is below threshold %s%%\n", t, m; exit 1 } }'
