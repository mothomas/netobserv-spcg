# Quay.io images (public — no cluster pull secret)

Images: **quay.io/moby/** (public repositories).

| Component | Image |
|-----------|--------|
| Frontend | `quay.io/moby/spcg-frontend:small-20260625` |
| UI portal | `quay.io/moby/spcg-ui-portal:small-20260624` |
| Backend | `quay.io/moby/spcg-backend-engine:small-20260614` |

## Push from your workstation (maintainers)

```bash
docker login quay.io -u 'moby+robo' -p '<token>'
./scripts/push-images-quay.sh
docker push quay.io/moby/spcg-frontend:small-20260625
```

## Deploy on OpenShift

No pull secret needed for public Quay repos.

```bash
git pull origin main
oc apply -k manifests/overlays/openshift-small
oc delete pod -n pcap-frontend -l 'app in (spcg-frontend,spcg-ui-portal)'
oc rollout status deployment/spcg-ui-portal deployment/spcg-frontend -n pcap-frontend
```

Or:

```bash
bash scripts/openshift-force-auth-fix.sh
```

Verify:

```bash
oc get pods -n pcap-frontend -l app=spcg-ui-portal \
  -o jsonpath='{.items[0].spec.containers[0].image}{"\n"}'
```

Must show `quay.io/moby/...`, not `docker.io/mothomas/...`.
