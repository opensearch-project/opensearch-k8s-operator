# Helm Chart Push to AWS ECR

This document describes how to package and push OpenSearch Helm charts to AWS ECR as OCI artifacts.

## Charts

| Chart | Path | Description |
|-------|------|-------------|
| `opensearch-operator` | `charts/opensearch-operator/` | The OpenSearch Kubernetes Operator |
| `opensearch-cluster` | `charts/opensearch-cluster/` | OpenSearch Cluster custom resource |

## CI/CD (GitHub Actions)

The workflow at `.github/workflows/TMDC-HELM-chart-push.yaml` is triggered by pushing a tag with the appropriate prefix.

### Tag Convention

```
<chart-prefix>-<semver>-d<build>
```

| Tag Pattern | Chart Pushed | Example |
|-------------|--------------|---------|
| `operator-*` | `opensearch-operator` | `operator-3.0.3-d1` |
| `cluster-*` | `opensearch-cluster` | `cluster-3.3.1-d1` |

The version is extracted from the tag by stripping the prefix (e.g., `operator-3.0.3-d1` becomes version `3.0.3-d1`).

### Push a Chart via CI

```bash
# Push opensearch-operator chart
git tag operator-3.0.3-d1
git push origin operator-3.0.3-d1

# Push opensearch-cluster chart
git tag cluster-3.3.1-d1
git push origin cluster-3.3.1-d1

# Push both charts (two tags)
git tag operator-3.0.3-d1 && git tag cluster-3.3.1-d1
git push origin operator-3.0.3-d1 cluster-3.3.1-d1
```

### Required GitHub Secrets

| Secret | Description |
|--------|-------------|
| `OIDC_ROLE_ARN` | AWS IAM role ARN for OIDC authentication |
| `AWS_ECR_REGION` | AWS region where ECR repositories are hosted |
| `DOCKER_HUB_USERNAME` | Docker Hub username for pulling the builder image |
| `DOCKER_HUB_PASSWORD` | Docker Hub password for pulling the builder image |

## Local / Manual Push

### Prerequisites

- AWS CLI configured with ECR permissions
- Helm v3.16.4+ (binary named `helm3.16.4`)
- `make`

### Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `TAG` | Chart version to package | `3.0.3-d1` |
| `ECR_HOST` | ECR registry host | `123456789.dkr.ecr.us-east-1.amazonaws.com` |
| `AWS_DEFAULT_REGION` | AWS region | `us-east-1` |

### Push a Single Chart

```bash
export AWS_DEFAULT_REGION=us-east-1
export ECR_HOST=123456789.dkr.ecr.us-east-1.amazonaws.com
export TAG=3.0.3-d1

# Push only opensearch-operator
make push-oci-chart DIR=opensearch-operator

# Push only opensearch-cluster
make push-oci-chart DIR=opensearch-cluster
```

### Push All Charts

```bash
export AWS_DEFAULT_REGION=us-east-1
export ECR_HOST=123456789.dkr.ecr.us-east-1.amazonaws.com
export TAG=3.0.3-d1

make push-all-charts
```

## Makefile Targets

| Target | Description |
|--------|-------------|
| `push-all-charts` | Package and push all charts (login, push each, logout) |
| `push-oci-chart` | Push a single chart (requires `DIR=<chart-name>`) |
| `ecr-login` | Login to the OCI registry |
| `ecr-logout` | Logout from the OCI registry |

## How It Works

1. **Login** — Authenticates to ECR using `aws ecr get-login-password`
2. **Package** — Runs `helm package` to create a `.tgz` archive with the specified version
3. **Create Repo** — Creates the ECR repository if it doesn't already exist
4. **Push** — Pushes the packaged chart as an OCI artifact to ECR
5. **Logout** — Cleans up the registry session

## Pulling Charts from ECR

After pushing, consumers can pull the chart using:

```bash
# Login to ECR
aws ecr get-login-password --region us-east-1 | helm registry login 123456789.dkr.ecr.us-east-1.amazonaws.com --username AWS --password-stdin

# Pull the chart
helm pull oci://123456789.dkr.ecr.us-east-1.amazonaws.com/opensearch-operator --version 3.0.3-d1

# Install directly from OCI
helm install opensearch-operator oci://123456789.dkr.ecr.us-east-1.amazonaws.com/opensearch-operator --version 3.0.3-d1
```
