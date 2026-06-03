# Lab random-scanner (local test only)

Zmap threat-simulation for SPCG capture testing in **authorized lab clusters only**.

## What is in git

| Path | Tracked |
|------|---------|
| All product code (`cmd/`, `internal/`, `frontend/`, `manifests/base`, overlays, `openshift/`, etc.) | **Yes** on `main` |
| Reference copy | `docs/examples/lab-random-scanner/` |
| Live lab deploy tree | `manifests/lab/`, `scripts/lab/` — **gitignored** |

## One-time local setup

```bash
./docs/examples/lab-random-scanner/setup-local.sh
```

Copies reference YAML and scripts into gitignored paths so you can run the scanner without committing lab config.

## Run / stop

```bash
export KUBECONFIG="$PWD/kubeconfig"
./scripts/lab/random-scanner-start.sh
./scripts/lab/random-scanner-stop.sh
./scripts/lab/random-scanner-teardown.sh
```

Default deployment scale is **0**; start script scales to 1.

## Safety

- Lab use only; do not scan networks you are not authorized to probe.
- Do not merge lab manifests into product overlays (`small`, `openshift-small`, etc.).
