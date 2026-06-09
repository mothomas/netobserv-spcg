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
# Routes must exist before the Job finishes (same apply includes routes.yaml)
oc get route spcg -n spcg-landing
oc get route spcg-api -n spcg-control
oc wait --for=condition=complete job/spcg-oauth-bootstrap -n spcg-control --timeout=15m
oc logs job/spcg-oauth-bootstrap -n spcg-control
```

**Job stuck on “Waiting for Route spcg-api”?** The router may be slow or `spcg-api` was never applied. Check:

```bash
oc get route -n spcg-control spcg-api
oc get route -n spcg-landing spcg
oc get svc -n spcg-control spcg-ui-portal
```

If `spcg-api` is missing: `oc apply -f manifests/openshift-secure/routes.yaml`. Then delete and re-run the Job:

```bash
oc delete job spcg-oauth-bootstrap -n spcg-control --ignore-not-found
oc apply -k manifests/openshift-secure/oauth-bootstrap
```

The Job waits up to 120s for `spcg-api`, then continues using Route `spcg` only (valid with in-cluster API proxy).

The Job (`oauth-bootstrap/`) runs in-cluster and:

1. Waits for Route `spcg` (required), then `spcg-api` (optional)
2. Creates or syncs **OAuthClient `spcg-ui`** + secret **`spcg-oauth-client`** (secret never logged)
3. Syncs optional `spcg-oauth-serving-ca` ConfigMap
4. Sets Route-derived env on frontend and portal (`SPCG_PUBLIC_API_BASE`, `CORS_ORIGIN`, `OAUTH_TOKEN_URL`)

**RBAC:** Job SA needs cluster permission to manage `oauthclients` (same class as Argo CD Operator SSO). Platform team applies the manifest once.

**Bootstrap image:** `registry.redhat.io/openshift4/ose-cli-rhel9:latest` (cluster needs `registry.redhat.io` pull secret).

Manifests include a **placeholder** `spcg-oauth-client` secret so the portal can start; the Job replaces it and restarts portal/frontend. If you applied an older revision without the placeholder:

```bash
oc create secret generic spcg-oauth-client -n spcg-control --from-literal=client-secret=pending-bootstrap-replace-me
oc delete job spcg-oauth-bootstrap -n spcg-control --ignore-not-found
oc apply -k manifests/openshift-secure
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
| frontend | `quay.io/moby/spcg-frontend:tracer-20260609` |
| portal | `quay.io/moby/spcg-ui-portal:tracer-20260609` |
| engine | `quay.io/moby/spcg-backend-engine:small-20260614` |

Rotate Neo4j/graph defaults in `control/secrets.yaml` before production.

See [docs/DEPLOYMENT.md](../../docs/DEPLOYMENT.md) and [docs/openshift-security.md](../../docs/openshift-security.md).
