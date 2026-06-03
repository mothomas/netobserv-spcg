#!/usr/bin/env bash
set -euo pipefail
KUBECONFIG="${KUBECONFIG:-$(dirname "$0")/../kubeconfig}"
if [[ -z "${SPCG_API:-}" ]]; then
  echo "Set SPCG_API to your UI URL (e.g. export SPCG_API=http://<node-ip>:30080)" >&2
  exit 1
fi
API="${SPCG_API}"
KC_B64=$(python3 -c "import base64; print(base64.b64encode(open('$KUBECONFIG','rb').read()).decode())")
SID=$(curl -sS -X POST "$API/api/v1/auth/login" -H "Content-Type: application/json" \
  -d "{\"mode\":\"kubeconfig\",\"kubeconfig\":\"$KC_B64\"}" | python3 -c "import sys,json; print(json.load(sys.stdin)['session_id'])")
echo "session=$SID"
BODY='{"namespaces":["demo-traffic"],"selections":[{"type":"owner","namespace":"demo-traffic","owner_kind":"Deployment","owner_name":"ping-worker"},{"type":"owner","namespace":"demo-traffic","owner_kind":"Deployment","owner_name":"ping-icmp"}]}'
curl -sS -N -X POST "$API/api/v1/capture/stream" \
  -H "Content-Type: application/json" -H "X-SPCG-Session: $SID" \
  -d "$BODY" --max-time 90 | tee /tmp/spcg-capture.out
echo
grep -E 'event: (chunk|error|session)' /tmp/spcg-capture.out | head -20
