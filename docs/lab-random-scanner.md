# Lab random-scanner (local branch)

The zmap threat-simulation manifests are **not** part of the product tree on `main`. They live on a dedicated local branch so they never ship upstream.

## Cluster

The `random-scanner` namespace has been removed from the lab cluster. Nothing runs until you explicitly start it.

## Use the lab branch

```bash
git fetch origin   # optional
git checkout lab/random-scanner
export KUBECONFIG="$PWD/kubeconfig"
./scripts/lab/random-scanner-start.sh
```

Stop without deleting config:

```bash
./scripts/lab/random-scanner-stop.sh
```

Full cleanup:

```bash
./scripts/lab/random-scanner-teardown.sh
```

Return to product work:

```bash
git checkout main
```

## Upstream policy

- `manifests/lab/` and `scripts/lab/` are **gitignored on `main`**.
- Do **not** merge `lab/random-scanner` into `main`.
- Do **not** push `lab/random-scanner` to shared/upstream remotes (keep the branch local).

To block accidental pushes of the lab branch:

```bash
git config branch.lab/random-scanner.pushRemote no_push
```

Create the `no_push` remote only as a dummy if you use the config above, or simply never `git push` that branch.
