#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
cp -a "$ROOT/." "$TMP/"
local_go="$(GOTOOLCHAIN=local go env GOVERSION | sed 's/^go//')"
python3 - "$TMP/go.mod" "$local_go" <<'PY'
from pathlib import Path
import re, sys
p=Path(sys.argv[1]); v=sys.argv[2]
s=p.read_text()
s=re.sub(r'^go\s+.*$', 'go '+v, s, count=1, flags=re.M)
s=re.sub(r'^toolchain\s+.*\n?', '', s, flags=re.M)
p.write_text(s)
PY
cd "$TMP"
packages="${CHECK_PACKAGES:-./...}"
# shellcheck disable=SC2086
go fmt $packages
# shellcheck disable=SC2086
go vet -tags sqlite_cgo $packages
# shellcheck disable=SC2086
go test -tags sqlite_cgo $packages
if [[ "${RACE:-0}" == "1" ]]; then
  # shellcheck disable=SC2086
  go test -race -tags sqlite_cgo $packages
fi
if [[ "${SKIP_BUILD:-0}" != "1" ]]; then
  build_packages="${BUILD_PACKAGES:-./cmd/...}"
  # shellcheck disable=SC2086
  go build -tags sqlite_cgo -trimpath $build_packages
fi
