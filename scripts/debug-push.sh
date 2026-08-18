#!/usr/bin/env bash
set -euo pipefail

URL="${MM_WEB_URL:-https://server.ruyi.homes}"
TOKEN="${MM_WEB_DEBUG_PUSH_TOKEN:-}"

if [[ -z "$TOKEN" ]]; then
  echo "Set MM_WEB_DEBUG_PUSH_TOKEN to the same value used by the API." >&2
  exit 1
fi

curl --fail-with-body --silent --show-error \
  -X POST "$URL/api/debug/push" \
  -H "X-Debug-Token: $TOKEN"
echo
