# Cloud SQL datasource

`cloudsql` is a GoFr SQL datasource for **Google Cloud SQL** (Postgres and MySQL).
It is built on top of GoFr's standard SQL datasource — it does not reimplement any
SQL behavior — and adds **IAM database authentication** via the
[Cloud SQL Go Connector](https://github.com/GoogleCloudPlatform/cloud-sql-go-connector).

It supports, with a single `DB_IAM_AUTH` switch and no code change:

- **IAM database authentication** on the cloud (no static password, no Cloud SQL
  Auth Proxy sidecar) — credentials resolve via Application Default Credentials,
  which works with Workload Identity Federation.
- **Username/password** for local development — when IAM auth is off the datasource
  is GoFr's standard SQL connection.

It is published as a separate module so the GCP SDK dependencies are only pulled in
by applications that actually use Cloud SQL — the GoFr core stays lean.

## Install

```bash
go get gofr.dev/pkg/gofr/datasource/cloudsql
```

## Usage

The same one line works locally and on GCP — only configuration changes:

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/cloudsql"
)

func main() {
	app := gofr.New()

	app.AddSQLDB(cloudsql.New(app.Config))

	app.Run()
}
```

Once added, `app.SQL()` / `ctx.SQL` behave like any other GoFr SQL connection,
including query logging, metrics and health checks.

## Configuration

| Env var               | Description                                                          |
|-----------------------|----------------------------------------------------------------------|
| `DB_HOST`             | Instance connection name `project:region:instance` (IAM), or host (standard) |
| `DB_DIALECT`          | `postgres` or `mysql`                                                |
| `DB_NAME`             | Database name                                                        |
| `DB_USER`             | IAM principal (SA email without `.gserviceaccount.com`), or DB user  |
| `DB_IAM_AUTH`         | `true` enables IAM auth; otherwise standard username/password is used |
| `DB_PASSWORD`         | Only used when `DB_IAM_AUTH` is not `true`                            |
| `DB_CLOUDSQL_IP_TYPE` | `PUBLIC` (default), `PRIVATE` or `PSC`                                |
| `DB_MAX_IDLE_CONNECTION` / `DB_MAX_OPEN_CONNECTION` | Pool sizing; zero uses GoFr defaults    |

## How it works

`New` returns a `*Client` that GoFr instruments via `AddSQLDB`: it injects the
logger and metrics and calls `Connect`. When `DB_IAM_AUTH` is not `true`, `Connect`
delegates to GoFr's standard SQL datasource (`sql.NewSQL`). When IAM auth is
requested, it registers the Cloud SQL connector driver, opens the connection and
wraps it with that same standard datasource via `sql.NewSQLFromDB` — so no SQL
behavior is duplicated. `Close` closes the connection and tears down the connector
registration (stopping its background credential refresh).

## Extending to other cloud providers

This module is the reference implementation of a GoFr "managed SQL" provider. AWS
RDS/Aurora and Azure Database can be added as their own leaf modules without any
change to GoFr core, following the same contract. See the developer guide in
[`doc.go`](./doc.go) for the AWS/Azure token-refresh pattern.

See the runnable [example](../../../../examples/using-cloudsql).
