---
description: "Run GoFr database migrations in CI/CD: Helm pre-install hooks, Kubernetes Jobs vs init containers, expand-contract for zero-downtime, and rollback strategy."
nextjs:
  metadata:
    title: "GoFr DB Migrations in CI/CD: Helm Hooks, Jobs, and Rollback"
    description: "Run GoFr database migrations in CI/CD: Helm pre-install hooks, Kubernetes Jobs vs init containers, expand-contract for zero-downtime, and rollback strategy."
---

# DB Migrations in CI/CD

{% answer %}
GoFr's built-in migrations run on app start, coordinated by a distributed lock so multi-replica deploys are safe. In CI/CD you have two clean choices: let the app run them on startup, or run them as a separate Helm pre-upgrade Job. The Job pattern is generally preferable because it fails fast, has its own logs, and gates the rollout.
{% /answer %}

## What GoFr provides

GoFr ships a migration system that you wire up via `app.Migrate(migrations.All())`. It supports MySQL, PostgreSQL, Redis, ClickHouse, Cassandra, and Elasticsearch. Records are kept in a `gofr_migrations` table (or Redis hash). A distributed lock (`gofr_migration_locks` table or Redis `SETNX`) prevents two replicas from running the same migration concurrently — see [Handling Data Migrations](/docs/advanced-guide/handling-data-migrations) for the full mechanics.

That means *any* deployment shape works correctness-wise. The CI/CD question is operational: do you want migrations tied to app startup, or separated?

## Option A: Migrations on app start (default)

The simplest setup, and the default GoFr lifecycle. Every replica calls `app.Migrate(...)` in-process before serving traffic — there is no separate migration binary or subcommand. The first replica to acquire the lock runs the migration; the others observe the populated `gofr_migrations` table and no-op. After migrations finish, all replicas continue normal startup.

Pros:
- Zero extra infra. One artifact per service.
- Migrations cannot drift from code — they ship in the same image.
- Idempotent under concurrency: the lock plus the version table guarantee each migration runs exactly once across replicas.

Cons:
- A migration error fails the readiness probe of every replica simultaneously, which can take down healthy old pods if the rollout strategy isn't careful.
- Slow migrations delay every pod's start.
- Logs are mixed with normal application logs.

Use this for small services and early-stage projects.

## Option B: Separate `cmd/migrate` binary as a Helm pre-upgrade Job

For multi-replica production services, run migrations as a Kubernetes Job triggered by Helm before the Deployment rolls forward. **There is no built-in `gofr migrate` CLI or `MIGRATE_ONLY` env mode in the framework.** Instead, organize your application as two binaries built from the same Go module: the serving binary (`cmd/server` or your existing `main.go`) and a small dedicated migration binary (`cmd/migrate`). The migration binary calls `gofr.New()`, registers migrations, calls `app.Migrate(...)` (which is synchronous and runs to completion before returning), and exits without ever calling `app.Run()`. This is application code organization — not a framework knob.

```go
// cmd/migrate/main.go
package main

import (
    "gofr.dev/pkg/gofr"

    "yourmodule/migrations"
)

func main() {
    app := gofr.New()
    // app.Migrate runs migrations synchronously using GoFr's distributed lock
    // and returns once they have completed (or failed). No app.Run() — this
    // binary is intended to be invoked as a one-shot Job.
    app.Migrate(migrations.All())
}
```

Build it as a separate binary in the same image (or a slimmer migrate-only image), and invoke it from the Helm pre-upgrade Job:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .Release.Name }}-migrate-{{ .Values.image.tag | replace ":" "-" }}
  annotations:
    "helm.sh/hook": pre-install,pre-upgrade
    "helm.sh/hook-weight": "-5"
    "helm.sh/hook-delete-policy": before-hook-creation
spec:
  backoffLimit: 0
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: migrate
          image: "{{ .Values.image.repo }}:{{ .Values.image.tag }}"
          # Invoke the dedicated migration binary built from cmd/migrate.
          command: ["/app/migrate"]
          envFrom:
            - secretRef:
                name: {{ .Release.Name }}-db
```

Pros:
- Failed migration fails the Helm release atomically; the rollout never starts.
- Job logs are clean and separately addressable: `kubectl logs job/...`.
- Application pods see the new schema by the time they boot.
- The migration binary is just Go code — no framework feature to learn beyond `app.Migrate`.

Cons:
- One more binary to build and template. The migrator Job's image SHA must always match the Deployment image SHA.

Use this for production.

## Init container: usually not the right tool

It is tempting to put migrations in an `initContainer`. Don't, in multi-replica deploys. Each pod's init container will race for the lock. GoFr's lock makes that *safe*, but it also means N-1 pods just wait for nothing while the rollout takes longer than necessary, and a failing migration manifests as N pods CrashLooping. A pre-upgrade Job centralizes the failure into one Pod and one log stream.

Init containers are fine for single-replica services or local dev.

## Expand-contract for zero downtime

Schema changes that break the previous app version are dangerous during a rolling deploy because both versions run simultaneously. Use the expand-contract pattern:

1. **Expand** — release migration A that *adds* the new column/table without removing the old one. Old code keeps working.
2. **Migrate code** — release the app version that writes both old and new, reads new (or vice versa).
3. **Backfill** — copy data from old to new in a background Job if needed.
4. **Contract** — once the app version is stable, release migration B that drops the old column/table.

This typically means at least two deploys per breaking change. It is the price of zero-downtime.

## Rollback strategy

GoFr currently runs migrations in `UP` mode only (verified against `pkg/gofr/migration` semantics described in the migrations doc). That has implications for rollback:

- App-level rollback (image SHA): always safe if the schema change was expand-only.
- Schema rollback: write a *new* forward migration that reverses the change. Treat the database as append-only history.
- Snapshots before destructive migrations are a safety net for genuine emergencies.

## Idempotency

Write migration SQL so re-running it is harmless: `CREATE TABLE IF NOT EXISTS`, `ALTER TABLE ... ADD COLUMN IF NOT EXISTS` (PostgreSQL), or guarded `IF` checks. The lock prevents concurrent runs, but idempotency protects against partial failures and manual re-runs.

## CI ergonomics

- Run migrations against an ephemeral database in CI on every PR. If the migration fails in CI, it never reaches prod.
- Tag the migration Job with the Helm release name and image SHA so old Jobs are identifiable: `migrate-{{ .Release.Name }}-{{ .Values.image.tag }}`.
- Pin the database driver version in `go.mod` and treat upgrades as their own change.

{% faq %}
{% faq-item question="Are GoFr migrations safe to run from many replicas at once?" %}
Yes. GoFr coordinates with a distributed lock — one replica runs, the others wait. See the multi-instance section in Handling Data Migrations.
{% /faq-item %}
{% faq-item question="Should I use a Helm pre-upgrade Job or let the app run migrations at startup?" %}
For multi-replica production services, prefer the Job. It fails fast, has clean logs, and gates the rollout. App-startup migrations are fine for single-replica or small services.
{% /faq-item %}
{% faq-item question="How do I roll back a schema change?" %}
Write a new forward migration that reverses it. Combine with the expand-contract pattern so each step is reversible and the previous app version keeps working.
{% /faq-item %}
{% /faq %}
