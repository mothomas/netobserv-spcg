# Neo4j graph store + Sigma.js UI

## Architecture

1. **Capture** streams flows into the portal (`pcap.Session` in memory).
2. On each `/api/v1/ai/context` refresh, topology is merged and written to **Neo4j** keyed by `captureId` + `authSessionId`.
3. The UI loads the graph via `POST /api/v1/graph/topology` and renders with **Sigma.js** (Graphology).
4. On session teardown or logout, all nodes/relationships for that `captureId` are deleted.

## Tenant isolation

- Each capture session is a separate subgraph (`captureId` on every node/relationship).
- Reads require matching `authSessionId` on `CaptureSession`.
- Sensitive labels and edge detail JSON are encrypted with a per-auth-session key derived from `GRAPH_MASTER_KEY`.

## Kubernetes

- `spcg-neo4j` in `pcap-frontend` (Bolt `7687`, emptyDir data — in-memory style for lab; replace with PVC for production).
- `spcg-ui-portal` env: `NEO4J_URI`, `NEO4J_USER`, `NEO4J_PASSWORD`, `GRAPH_MASTER_KEY`.

## Local dev

```bash
docker run -d --name neo4j -p 7687:7687 -e NEO4J_AUTH=neo4j/test neo4j:5-community
export NEO4J_URI=bolt://localhost:7687
export NEO4J_USER=neo4j
export NEO4J_PASSWORD=test
export GRAPH_MASTER_KEY=dev-master-key-change-in-prod
```
