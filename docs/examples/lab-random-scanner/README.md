# Lab random-scanner (reference copy)

Reference manifests and scripts for **local threat-simulation testing** only. They are **not** applied by product overlays.

## Install locally (gitignored paths)

From repo root:

```bash
./docs/examples/lab-random-scanner/setup-local.sh
```

This copies files into `manifests/lab/random-scanner/` and `scripts/lab/` (both gitignored).

## Run

```bash
export KUBECONFIG="$PWD/kubeconfig"
./scripts/lab/random-scanner-start.sh
./scripts/lab/random-scanner-stop.sh      # scale to 0
./scripts/lab/random-scanner-teardown.sh # delete namespace
```

See [docs/lab-random-scanner.md](../../lab-random-scanner.md).
