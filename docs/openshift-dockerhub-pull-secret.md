# Docker Hub pull secret (OpenShift)

Cluster nodes pull `docker.io/mothomas/*` **without auth** and hit Docker Hub rate limits (`toomanyrequests`). Create a pull secret and attach it to the namespace or deployments.

## 1. Create secret (use a [Docker Hub access token](https://hub.docker.com/settings/security))

```bash
NS=pcap-frontend
oc create secret docker-registry spcg-dockerhub \
  -n "$NS" \
  --docker-server=https://index.docker.io/v1/ \
  --docker-username='YOUR_DOCKERHUB_USER' \
  --docker-password='YOUR_ACCESS_TOKEN' \
  --dry-run=client -o yaml | oc apply -f -
```

## 2. Attach to default ServiceAccount (all pods in namespace)

```bash
oc patch serviceaccount default -n pcap-frontend --type=merge -p \
  '{"imagePullSecrets":[{"name":"spcg-dockerhub"}]}'
```

## 3. Set images and roll out (after secret exists)

```bash
oc set image deployment/spcg-frontend -n pcap-frontend \
  frontend=docker.io/mothomas/spcg-frontend:small-20260624
oc set image deployment/spcg-ui-portal -n pcap-frontend \
  ui-portal=docker.io/mothomas/spcg-ui-portal:small-20260624

oc delete pod -n pcap-frontend -l 'app in (spcg-frontend,spcg-ui-portal)' \
  --field-selector=status.phase!=Running

oc rollout status deployment/spcg-frontend deployment/spcg-ui-portal -n pcap-frontend
```

## Alternative: mirror to OpenShift internal registry

Push images once from a machine with Docker Hub access, then point Deployments at `image-registry.openshift-image-registry.svc:5000/...`.
