#!/usr/bin/env bash
# Retag local mothomas Docker Hub images and push to quay.io/moby (run after docker login quay.io).
set -euo pipefail
QUAY_ORG="${QUAY_ORG:-moby}"
QUAY="quay.io/${QUAY_ORG}"
TAG="${TAG:-small-20260605}"
ENGINE_TAG="${ENGINE_TAG:-small-20260614}"
# OpenShift worker nodes are amd64; on Apple Silicon always build with:
#   docker buildx build --platform linux/amd64 -f deploy/Dockerfile.ui -t quay.io/${QUAY_ORG}/spcg-ui-portal:${TAG} --push .

for pair in \
  "docker.io/mothomas/spcg-frontend:${TAG} ${QUAY}/spcg-frontend:${TAG}" \
  "docker.io/mothomas/spcg-ui-portal:${TAG} ${QUAY}/spcg-ui-portal:${TAG}" \
  "docker.io/mothomas/spcg-backend-engine:${ENGINE_TAG} ${QUAY}/spcg-backend-engine:${ENGINE_TAG}"; do
  set -- $pair
  echo "Tag $1 -> $2"
  docker tag "$1" "$2"
  docker push "$2"
done
echo "Done. Set manifests/openshift kustomization newName to ${QUAY}/<image>"
