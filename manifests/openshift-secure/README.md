# OpenShift secure (greenfield)

Three namespaces, no `pcap-frontend`, no migration delete patches.

| Namespace | Role | Workloads |
|-----------|------|-----------|
| `spcg-landing` | restricted | `spcg-frontend` + Route `spcg` |
| `spcg-control` | restricted | `spcg-ui-portal`, `spcg-neo4j` + Route `spcg-api` |
| `pcap-capture` | privileged | `spcg-backend-engine`, ephemeral sensors |

## Prerequisites (cluster admin, once)

1. **OAuthClient** `spcg-ui` with redirect (after first apply, use real host):

   `https://$(oc get route spcg-api -n spcg-control -o jsonpath='{.spec.host}')/api/v1/auth/openshift/callback`

2. **Secret** in control namespace:

   ```bash
   oc create secret generic spcg-oauth-client -n spcg-control \
     --from-literal=client-secret='<matches OAuthClient>'
   ```

3. Rotate default secrets in `control/secrets.yaml` before production (Neo4j + graph key).

## Apply

```bash
./scripts/openshift-secure-apply.sh
```

Or:

```bash
oc apply -k manifests/openshift-secure
# then set Route URLs (script does this):
oc set env deployment/spcg-frontend -n spcg-landing \
  SPCG_PUBLIC_API_BASE=https://<spcg-api-host> SPCG_DISABLE_API_PROXY=true
oc set env deployment/spcg-ui-portal -n spcg-control CORS_ORIGIN=https://<spcg-host>
```

## Images

| Component | Tag |
|-----------|-----|
| frontend | `quay.io/moby/spcg-frontend:small-20260616` |
| portal | `quay.io/moby/spcg-ui-portal:small-20260616` |
| engine | `quay.io/moby/spcg-backend-engine:small-20260614` |

See [docs/DEPLOYMENT.md](../../docs/DEPLOYMENT.md) and [docs/openshift-security.md](../../docs/openshift-security.md).
