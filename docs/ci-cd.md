# CI/CD — GitHub Actions → Docker Hub / Quay

Images built on every push to `main` and on version tags `v*`.

| Image | Dockerfile |
|-------|------------|
| `spcg-backend-engine` | `deploy/Dockerfile.engine` |
| `spcg-ui-portal` | `deploy/Dockerfile.ui` |
| `spcg-frontend` | `deploy/Dockerfile.frontend` |

Published as:

- Docker Hub: `docker.io/<owner>/spcg-<name>:<tag>`
- Quay: `quay.io/<owner>/spcg-<name>:<tag>`

Default owner is **`mothomas`** (override with repo variable `IMAGE_OWNER`).

## One-time GitHub setup

### 1. Create the repository

```bash
cd /path/to/netobserv-spcg
git init
git add .
git commit -m "Initial SPCG scaffold"
gh repo create netobserv-spcg --public --source=. --remote=origin --push
```

Or create an empty repo on GitHub, then:

```bash
git remote add origin git@github.com:mothomas/netobserv-spcg.git
git branch -M main
git push -u origin main
```

### 2. Repository secrets (Settings → Secrets and variables → Actions)

**Docker Hub** (required for default pipeline):

| Secret | Value |
|--------|--------|
| `DOCKERHUB_USERNAME` | Docker Hub user (e.g. `mothomas`) |
| `DOCKERHUB_TOKEN` | Docker Hub access token ([Account Settings → Security](https://hub.docker.com/settings/security)) |

**Quay** (optional — use workflow dispatch “quay” or set variable `DEFAULT_REGISTRY=quay`):

| Secret | Value |
|--------|--------|
| `QUAY_USERNAME` | Quay user or robot account |
| `QUAY_TOKEN` | Quay robot token or CLI password |

Never commit tokens to the repo. Rotate any token that was pasted into chat or logs.

### 3. Optional repository variables

| Variable | Example | Purpose |
|----------|---------|---------|
| `IMAGE_OWNER` | `mothomas` | Namespace on Docker Hub / Quay |
| `DEFAULT_REGISTRY` | `dockerhub` or `quay` | Default push target on branch pushes |

### 4. Create Quay repositories (if using Quay)

Create public or private repos on [quay.io](https://quay.io):

- `spcg-backend-engine`
- `spcg-ui-portal`
- `spcg-frontend`

## Workflow behavior

- **Push to `main`**: build all three images, push `:latest` and `:sha`.
- **Tag `v1.2.3`**: push `:v1.2.3` and `:sha`.
- **Pull request**: build only (no push).
- **Manual run**: choose Docker Hub or Quay.

Workflow file: [`.github/workflows/image-build.yml`](../.github/workflows/image-build.yml).

## Use published images in manifests

After the first successful run, point deployments at your registry:

```yaml
# manifests/deployment-capture.yaml
image: docker.io/mothomas/spcg-backend-engine:latest

# manifests/deployment-frontend.yaml
image: docker.io/mothomas/spcg-ui-portal:latest
image: docker.io/mothomas/spcg-frontend:latest
```

Or with Helm:

```yaml
# charts/spcg/values.yaml
backendEngine:
  image: docker.io/mothomas/spcg-backend-engine
uiPortal:
  image: docker.io/mothomas/spcg-ui-portal
frontend:
  image: docker.io/mothomas/spcg-frontend
```

## Local build (same Dockerfiles as CI)

```bash
make docker
# or
docker build -f deploy/Dockerfile.engine -t spcg-backend-engine:dev .
```
