# networkplot integration

Visual style and layout ported from [networkplot-openshift](https://github.com/mothomas/networkplot-openshift) `networkplot/html.py`:

- Cytoscape.js + dagre (TB layout)
- Official Kubernetes icons (unlabeled SVG set)
- Edge types: direct, scheduled, snat, https, egressservice

SPCG maps **NetObserv flow topology** into this format. Tenant isolation is enforced in:

1. Backend `FilterTopologyToSelection` (capture session tracked pods)
2. Frontend `isolateToTrackedPods()` (defense in depth)

Only **selected pods** render as solid (`directPod`); one-hop flow peers are dashed.
