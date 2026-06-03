#!/usr/bin/env bash
set -euo pipefail
if [[ -z "${SPCG_API:-}" ]]; then
  echo "Set SPCG_API to your UI URL (e.g. export SPCG_API=http://<node-ip>:30080)" >&2
  exit 1
fi
API="${SPCG_API}"
KUBECONFIG="${KUBECONFIG:-$(dirname "$0")/../kubeconfig}"
KC_B64=$(python3 -c "import base64; print(base64.b64encode(open('$KUBECONFIG','rb').read()).decode())")

SID=$(curl -sS -X POST "$API/api/v1/auth/login" -H "Content-Type: application/json" \
  -d "{\"mode\":\"kubeconfig\",\"kubeconfig\":\"$KC_B64\"}" | python3 -c "import sys,json; print(json.load(sys.stdin)['session_id'])")
echo "auth_session=$SID"

echo "=== limits ==="
curl -sS "$API/api/v1/capture/limits" -H "X-SPCG-Session: $SID" | python3 -m json.tool

BODY='{"namespaces":["demo-traffic"],"selections":[{"type":"owner","namespace":"demo-traffic","owner_kind":"Deployment","owner_name":"ping-worker"}],"s3":{"enabled":false}}'
curl -sS -N -X POST "$API/api/v1/capture/stream" \
  -H "Content-Type: application/json" -H "X-SPCG-Session: $SID" \
  -d "$BODY" --max-time 12 > /tmp/small-cap.out || true

CAP=$(python3 -c "import re; t=open('/tmp/small-cap.out').read(); m=re.search(r'session_id\":\"([^\"]+)\"',t); print(m.group(1) if m else '')")
CHUNKS=$(grep -c 'event: chunk' /tmp/small-cap.out || true)
echo "capture=$CAP chunks=$CHUNKS"

if [[ -z "$CAP" ]]; then
  echo "FAIL: no capture session"
  head -5 /tmp/small-cap.out
  exit 1
fi

echo "=== ai context ==="
curl -sS -X POST "$API/api/v1/ai/context" -H "Content-Type: application/json" -H "X-SPCG-Session: $SID" \
  -d "{\"session_id\":\"$CAP\",\"max_lines\":50}" | python3 -c "
import sys,json
c=json.load(sys.stdin)
print('events', c.get('event_count'), 's3_export', c.get('s3_export'))
t=c.get('topology') or {}
print('topology nodes', len(t.get('nodes',[])), 'edges', len(t.get('edges',[])))
"

echo "=== sigma graph ==="
curl -sS -X POST "$API/api/v1/graph/topology" -H "Content-Type: application/json" -H "X-SPCG-Session: $SID" \
  -d "{\"capture_session_id\":\"$CAP\"}" | python3 -c "
import sys,json
g=json.load(sys.stdin)
print('sigma nodes', len(g.get('nodes',[])), 'edges', len(g.get('edges',[])))
"

echo "=== RAM merge download ==="
curl -sS -o /tmp/merged.pcapng -w "HTTP %{http_code} bytes %{size_download}\n" \
  "$API/api/v1/capture/merge/$CAP" -H "X-SPCG-Session: $SID"

echo "=== teardown ==="
curl -sS -o /dev/null -w "HTTP %{http_code}\n" -X POST "$API/api/v1/capture/teardown/$CAP" -H "X-SPCG-Session: $SID"
echo "PASS small tier smoke test"
