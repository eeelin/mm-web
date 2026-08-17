#!/usr/bin/env bash
set -euo pipefail

if command -v go >/dev/null 2>&1; then
  GO_BIN="$(command -v go)"
elif [[ -x "$HOME/.local/go/bin/go" ]]; then
  GO_BIN="$HOME/.local/go/bin/go"
else
  echo "Go is required to run the ModemManager API." >&2
  exit 1
fi

GO="$GO_BIN" npm run dev:api &
API_PID=$!
trap 'kill "$API_PID" 2>/dev/null || true' EXIT INT TERM

npm run dev:web
