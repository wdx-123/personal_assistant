#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

EXCLUDE_GLOBS=(
  "--glob=!scripts/cleanup_legacy_error_tracking.ps1"
  "--glob=!scripts/check_no_legacy_error_tracking.sh"
  "--glob=!.git/**"
)

DENY_PATTERNS=(
  "pkg/observability/errors"
  "\\bObservabilityErrorEvent\\b"
  "\\bObservabilityErrorRepository\\b"
  "observability_error_events"
)

failed=0

for pattern in "${DENY_PATTERNS[@]}"; do
  if rg -n --hidden "${EXCLUDE_GLOBS[@]}" -e "$pattern" .; then
    echo "found forbidden legacy error tracking pattern: $pattern"
    failed=1
  fi
done

if [[ "$failed" -ne 0 ]]; then
  echo "legacy error tracking guard check failed"
  exit 1
fi

echo "legacy error tracking guard check passed"

