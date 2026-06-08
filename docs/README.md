# SPCG documentation index

| Document | Audience | Contents |
|----------|----------|----------|
| [ARCHITECTURE.md](./ARCHITECTURE.md) | Architects, security reviewers | Design ideology, concepts, component model, end-to-end data flows, decision log |
| [DEPLOYMENT.md](./DEPLOYMENT.md) | Platform / SRE | Manifest layout, Small/Medium/Peak overlays, vanilla K8s vs OpenShift, SCC, Routes, network policy map |
| [CODE-STRUCTURE.md](./CODE-STRUCTURE.md) | Developers | Repository layout, package boundaries, conventions, security patterns in code |
| [TRACE-ROADMAP.md](./TRACE-ROADMAP.md) | Trace feature | Packet Trace branch strategy, phases, deploy workflow (`tracer` branch) |
| [TRACE-GRAPH-PLAN.md](./TRACE-GRAPH-PLAN.md) | Trace UX / graph | Sigma.js path-first model, ingress/egress swimlanes, cluster path inventory |
| [architecture-tiers.md](./architecture-tiers.md) | Capacity planning | Tier sizing matrix, scaling levers, roadmap items |
| [kubernetes-vs-openshift.md](./kubernetes-vs-openshift.md) | Quick platform compare | Auth, NodePort vs Route, PSS |
| [neo4j-graph.md](./neo4j-graph.md) | Graph / multi-tenant | Neo4j pipeline, tenant crypto, query isolation |
| [ci-cd.md](./ci-cd.md) | Release engineering | Image build and registry |
| [lab-random-scanner.md](./lab-random-scanner.md) | Lab only | Local threat-sim (gitignored; reference in `examples/`) |

Start with **ARCHITECTURE.md** and **DEPLOYMENT.md** for a full picture.
