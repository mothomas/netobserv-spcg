# Quay.io images (avoid Docker Hub rate limits)

Use **quay.io/moby/** repositories (adjust org if your Quay namespace differs).

## 1. Create repositories on Quay (required before first push)

On [quay.io](https://quay.io) → organization **moby** → **Create repository** (public), for each:

- `spcg-frontend`
- `spcg-ui-portal`
- `spcg-backend-engine`

Grant robot **`moby+robo`** *Write* (or Admin) on all three. Without this, `docker push` returns **401 UNAUTHORIZED**.

## 2. Push from your workstation

```bash
# Login (use a robot account token — never commit tokens to git)
docker login quay.io -u 'moby+robo' -p '<token-from-quay-robot-settings>'

export QUAY=quay.io/moby
export TAG=small-20260624

docker tag docker.io/mothomas/spcg-frontend:${TAG}      ${QUAY}/spcg-frontend:${TAG}
docker tag docker.io/mothomas/spcg-ui-portal:${TAG}     ${QUAY}/spcg-ui-portal:${TAG}
docker tag docker.io/mothomas/spcg-backend-engine:small-20260614 ${QUAY}/spcg-backend-engine:small-20260614

docker push ${QUAY}/spcg-frontend:${TAG}
docker push ${QUAY}/spcg-ui-portal:${TAG}
docker push ${QUAY}/spcg-backend-engine:small-20260614
```

Or run: `./scripts/push-images-quay.sh`

## 3. OpenShift pull secret (private repos or higher limits)

```bash
oc create secret docker-registry spcg-quay \
  -n pcap-frontend \
  --docker-server=quay.io \
  --docker-username='moby+robo' \
  --docker-password='<token>' \
  --dry-run=client -o yaml | oc apply -f -

oc patch serviceaccount default -n pcap-frontend --type=merge -p \
  '{"imagePullSecrets":[{"name":"spcg-quay"}]}'
```

## 4. Deploy

```bash
oc apply -k manifests/overlays/openshift-small
PORTAL_IMAGE=quay.io/moby/spcg-ui-portal:small-20260624 \
FRONTEND_IMAGE=quay.io/moby/spcg-frontend:small-20260624 \
  bash scripts/openshift-force-auth-fix.sh
```

Verify:

```bash
oc get pods -n pcap-frontend -l app=spcg-ui-portal \
  -o jsonpath='{.items[0].spec.containers[0].image}{"\n"}'
```
