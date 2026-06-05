# OpenShift secure (greenfield)

Three namespaces, no `pcap-frontend`. OAuth bootstrap is a **Kubernetes Job** (Helm `post-install` hook–ready), not a shell wrapper.

| Namespace | Role | Workloads |
|-----------|------|-----------|
| `spcg-landing` | restricted | `spcg-frontend` + Route `spcg` |
| `spcg-control` | restricted | `spcg-ui-portal`, `spcg-neo4j`, bootstrap Job + Route `spcg-api` |
| `pcap-capture` | privileged | `spcg-backend-engine`, ephemeral sensors |

## Apply (no install script required)

```bash
oc apply -k manifests/openshift-secure
oc wait --for=condition=complete job/spcg-oauth-bootstrap -n spcg-control --timeout=15m
oc logs job/spcg-oauth-bootstrap -n spcg-control
```

The Job (`oauth-bootstrap/`) runs in-cluster and:

1. Waits for Routes `spcg` / `spcg-api`
2. Creates or syncs **OAuthClient `spcg-ui`** + secret **`spcg-oauth-client`** (secret never logged)
3. Syncs optional `spcg-oauth-serving-ca` ConfigMap
4. Sets Route-derived env on frontend and portal (`SPCG_PUBLIC_API_BASE`, `CORS_ORIGIN`, `OAUTH_TOKEN_URL`)

**RBAC:** Job SA needs cluster permission to manage `oauthclients` (same class as Argo CD Operator SSO). Platform team applies the manifest once.

If the portal pod is `CreateContainerConfigError` briefly, wait for the Job to finish, then:

```bash
oc rollout restart deployment/spcg-ui-portal -n spcg-control
```

## Helm chart (later)

Copy `oauth-bootstrap/` into chart templates. Job metadata already includes:

```yaml
helm.sh/hook: post-install,post-upgrade
helm.sh/hook-weight: "10"
helm.sh/hook-delete-policy: hook-succeeded,before-hook-creation
```

Parameterize namespaces and client name via `.Values.oauth.*`.

## Images

| Component | Tag |
|-----------|-----|
| frontend | `quay.io/moby/spcg-frontend:small-20260616` |
| portal | `quay.io/moby/spcg-ui-portal:small-20260616` |
| engine | `quay.io/moby/spcg-backend-engine:small-20260614` |

Rotate Neo4j/graph defaults in `control/secrets.yaml` before production.

See [docs/DEPLOYMENT.md](../../docs/DEPLOYMENT.md) and [docs/openshift-security.md](../../docs/openshift-security.md).
