---
description: "CI/CD pipelines for GoFr: GitHub Actions workflow, Go build caching, image tagging, OIDC for cloud auth, and short pointers for GitLab CI and CircleCI."
nextjs:
  metadata:
    title: "GoFr CI/CD Recipes: GitHub Actions, Build & Deploy Pipelines"
    description: "CI/CD pipelines for GoFr: GitHub Actions workflow, Go build caching, image tagging, OIDC for cloud auth, and short pointers for GitLab CI and CircleCI."
---

# CI/CD Recipes

{% answer %}
A GoFr CI pipeline is a standard Go pipeline plus a container build: lint, `go test`, build a versioned Docker image, push to a registry, then deploy via Helm or `kubectl apply`. Use OIDC for cloud auth, cache Go modules and build output, and tag images with both a short SHA and a semver.
{% /answer %}

## GitHub Actions: end-to-end workflow

```yaml
name: ci-cd
on:
  push:
    branches: [main]
  pull_request:

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

permissions:
  contents: read
  id-token: write   # for OIDC to cloud
  packages: write   # for GHCR push

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25'   # gofr.dev requires Go >= 1.25 (per its go.mod). Alternatively use `go-version-file: go.mod` to auto-track.
          cache: true       # caches modules + build cache automatically
      - run: go vet ./...
      - run: go test -race -coverprofile=cover.out ./...

  build-push:
    needs: test
    if: github.ref == 'refs/heads/main'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: docker/setup-buildx-action@v3
      - uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - id: meta
        uses: docker/metadata-action@v5
        with:
          images: ghcr.io/${{ github.repository }}
          tags: |
            type=sha,prefix=,format=short
            type=semver,pattern={{version}}
            type=raw,value=latest,enable={{is_default_branch}}
      - uses: docker/build-push-action@v5
        with:
          context: .
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          cache-from: type=gha
          cache-to: type=gha,mode=max

  deploy:
    needs: build-push
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: aws-actions/configure-aws-credentials@v4
        with:
          role-to-assume: arn:aws:iam::123456789012:role/gha-deployer
          aws-region: us-east-1
      - run: aws eks update-kubeconfig --name prod-cluster
      - run: |
          helm upgrade --install gofr-api ./chart \
            --set image.tag=${{ github.sha }} \
            --wait --timeout 5m
```

Key points:

- `actions/setup-go` with `cache: true` caches `~/go/pkg/mod` and the build cache between runs.
- `concurrency` cancels superseded runs on the same branch — prevents two deploys from racing.
- The `id-token: write` permission plus `aws-actions/configure-aws-credentials` uses GitHub OIDC to assume an IAM role with no long-lived keys.

## Image tagging strategy

Pick a tag scheme that gives you both *traceability* and *promotability*:

- `git-sha-short` (e.g., `a1b2c3d`) for every commit — unique, immutable, easy to roll back to.
- Semver (`1.4.2`) on tagged releases for human-friendly references and Helm chart values.
- `latest` only on the default branch and never used in production manifests — it makes rollbacks ambiguous.

Helm values should pin to the SHA tag, not `latest`.

## Secrets in CI

In order of preference:

1. **OIDC** to AWS / GCP / Azure / Vault — no static secret stored in CI.
2. Encrypted CI variables scoped to a single environment.
3. Long-lived API tokens — last resort, rotate often.

Never echo secrets into logs. Mask them by setting them as masked variables.

## Database migrations

Run migrations as part of deploy, not as part of the image build. See [DB Migrations in CI/CD](/docs/guides/db-migrations-in-cicd) for the Helm pre-install hook and Job patterns.

## GitLab CI

The shape is identical: a `test` job, a `build` job using `kaniko` or `buildah`, and a `deploy` job using `helm`. Use GitLab's OIDC support (`CI_JOB_JWT_V2`) for cloud auth.

```yaml
build:
  image: gcr.io/kaniko-project/executor:debug
  script:
    - /kaniko/executor --dockerfile=Dockerfile --destination=$CI_REGISTRY_IMAGE:$CI_COMMIT_SHORT_SHA
```

## CircleCI

Use orbs (`circleci/aws-cli`, `circleci/kubernetes`) and CircleCI's OIDC tokens (`oidc_token`) to assume cloud roles. Cache Go with `restore_cache` keyed on `go.sum`.

## Gotchas

- A failed migration should fail the deploy. Always set `--wait` on Helm and a non-zero exit on the migration Job.
- Never run integration tests against a shared production database. Spin up an ephemeral DB in the CI runner.
- The Go test race detector (`-race`) catches subtle data races in handlers — keep it on for the unit test stage.
- If you build a single multi-arch image, use `docker/build-push-action` with `platforms: linux/amd64,linux/arm64`.

{% faq %}
{% faq-item question="Should I run go test inside Docker or on the runner?" %}
On the runner. It is faster (Go module cache reuse) and gives clearer logs. The Docker build only happens once tests pass.
{% /faq-item %}
{% faq-item question="How do I authenticate to AWS or GCP from GitHub Actions without storing keys?" %}
Use OIDC. Configure a trust policy on the cloud role that trusts GitHub's OIDC issuer, then use `aws-actions/configure-aws-credentials` or `google-github-actions/auth` with the `id-token: write` permission.
{% /faq-item %}
{% faq-item question="What image tag should production manifests reference?" %}
The short Git SHA. It is unique, immutable, and makes rollbacks unambiguous. Reserve `latest` for development environments only.
{% /faq-item %}
{% /faq %}
