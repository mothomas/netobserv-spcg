#!/usr/bin/env bash
# Verify OpenShift login wiring without spawning a restricted-violating curl pod.
set -euo pipefail
NS="${NS:-pcap-frontend}"

echo "=== Deployments (image + auth env) ==="
for d in spcg-ui-portal spcg-frontend; do
  echo "--- $d ---"
  oc get deploy "$d" -n "$NS" -o jsonpath='  image: {.spec.template.spec.containers[0].image}{"\n"}'
  oc set env deployment/"$d" -n "$NS" --list 2>/dev/null | grep -E '^SPCG_AUTH|^OAUTH_' || echo "  (no SPCG_AUTH / OAUTH env)"
done

echo ""
echo "=== Portal /api/v1/auth/config (in-pod) ==="
if oc exec -n "$NS" deployment/spcg-ui-portal -- wget -qO- http://127.0.0.1:8080/api/v1/auth/config 2>/dev/null; then
  echo ""
else
  echo "  wget failed; try: oc port-forward -n $NS deployment/spcg-ui-portal 18080:8080"
  echo "  then: curl -s http://127.0.0.1:18080/api/v1/auth/config | jq ."
fi

echo ""
echo "=== Route spcg ==="
oc get route spcg -n "$NS" -o jsonpath='  https://{.spec.host}{"\n"}' 2>/dev/null || echo "  Route spcg not found — run: oc apply -k manifests/overlays/openshift-small"
