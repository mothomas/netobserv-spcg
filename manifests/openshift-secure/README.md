# OpenShift secure (greenfield)

Three namespaces, no `pcap-frontend`, no migration delete patches.

| Namespace | Role | Workloads |
|-----------|------|-----------|
| `spcg-landing` | restricted | `spcg-frontend` + Route `spcg` |
| `spcg-control` | restricted | `spcg-ui-portal`, `spcg-neo4j` + Route `spcg-api` |
| `pcap-capture` | privileged | `spcg-backend-engine`, ephemeral sensors |

## Prerequisites

- **Cluster-admin once** (or RBAC: create `oauthclients`, secrets in `spcg-control`) for OAuth bootstrap — same class of permission as Argo CD Operator SSO.
- Rotate default Neo4j/graph secrets in `control/secrets.yaml` before production.

OAuth **client secret is not typed by the admin**: `openshift-secure-apply.sh` runs `openshift-oauth-bootstrap.sh`, which generates a secret, registers `OAuthClient` `spcg-ui`, and creates `spcg-oauth-client` in `spcg-control` (redirect URI taken from Route `spcg-api`).

Manual bootstrap only if needed:

```bash
oc apply -k manifests/openshift-secure
SPCG_OAUTH_LAYOUT=secure ./scripts/openshift-oauth-bootstrap.sh
```

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
