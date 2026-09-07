---
description: "Connect GoFr to Google Cloud SQL (Postgres & MySQL) with IAM database authentication. The same code works locally with username/password and on GCP with IAM — no Cloud SQL Auth Proxy sidecar required."
nextjs:
  metadata:
    title: "Cloud SQL in GoFr — IAM-authenticated SQL Datasource"
    description: "Connect GoFr to Google Cloud SQL (Postgres & MySQL) with IAM database authentication. The same code works locally with username/password and on GCP with IAM — no Cloud SQL Auth Proxy sidecar required."
---

# Cloud SQL

The `cloudsql` datasource connects GoFr to **Google Cloud SQL** (Postgres and MySQL)
using **IAM database authentication** via the
[Cloud SQL Go Connector](https://github.com/GoogleCloudPlatform/cloud-sql-go-connector).

The connector mints short-lived credentials and opens a secure tunnel to the
instance, so **no static database password and no Cloud SQL Auth Proxy sidecar** are
required. Credentials resolve via Application Default Credentials, which supports
Workload Identity Federation on GKE and Cloud Run.

It is a separate, opt-in module, so you only add the GCP dependencies when you
actually use Cloud SQL. Once added, it behaves like any other GoFr SQL connection —
`ctx.SQL`, query logging, metrics, health checks and transactions all work the same.

## The same code locally and on GCP

A single configuration switch, `DB_IAM_AUTH`, selects the connection mode, so your
application code is identical in both environments — there is **no conditional to
write**:

- `DB_IAM_AUTH=true` → connect through the Cloud SQL connector using IAM auth.
- otherwise → use GoFr's standard SQL datasource (host/port with username/password),
  exactly as `gofr.New()` would on its own.

Set username/password locally and `DB_IAM_AUTH=true` on GCP; the code does not
change. In both cases `ctx.SQL` and all of GoFr's SQL logging, metrics, health checks
and transactions behave identically.

## Configuration

| Variable              | Description                                                          |
|-----------------------|----------------------------------------------------------------------|
| `DB_HOST`             | Cloud SQL instance connection name, `project:region:instance`        |
| `DB_DIALECT`          | `postgres` or `mysql`                                                 |
| `DB_NAME`             | Database name                                                        |
| `DB_USER`             | IAM principal — a user email, or a service-account email with the `.gserviceaccount.com` suffix removed (e.g. `app-sa@my-proj.iam`) |
| `DB_IAM_AUTH`         | `true` enables IAM auth; otherwise standard username/password is used |
| `DB_PASSWORD`         | Only used when `DB_IAM_AUTH` is not `true`                            |
| `DB_CLOUDSQL_IP_TYPE` | `PUBLIC` (default), `PRIVATE` or `PSC`                                |

## Setup

Import GoFr's external driver for Cloud SQL:

```shell
go get gofr.dev/pkg/gofr/datasource/cloudsql@latest
```

The datasource is plugged in with `app.AddSQLDB`, so `app.SQL()` / `ctx.SQL` work
like any other GoFr SQL connection.

### Example

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/cloudsql"
)

type customer struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func main() {
	app := gofr.New()

	// One call handles both environments. With DB_IAM_AUTH=true it connects to
	// Cloud SQL using IAM auth; otherwise it uses a standard username/password
	// connection. The code is identical either way — only configuration changes.
	app.AddSQLDB(cloudsql.New(app.Config))

	app.GET("/customers", func(ctx *gofr.Context) (any, error) {
		rows, err := ctx.SQL.QueryContext(ctx, "SELECT id, name FROM customers ORDER BY id")
		if err != nil {
			return nil, err
		}
		defer func() { _ = rows.Close() }()

		customers := make([]customer, 0)
		for rows.Next() {
			var c customer
			if err := rows.Scan(&c.ID, &c.Name); err != nil {
				return nil, err
			}
			customers = append(customers, c)
		}

		return customers, rows.Err()
	})

	app.Run()
}
```

## IAM authentication prerequisites (on GCP)

1. Enable the Cloud SQL Admin API on the project.
2. Create an IAM database user for your service account on the instance.
3. Grant the service account the **Cloud SQL Instance User** and **Cloud SQL Client**
   roles.
4. Make Application Default Credentials available to the process — on GKE/Cloud Run
   this is provided automatically via Workload Identity; locally you can run
   `gcloud auth application-default login`.

A runnable example is available at
[`examples/using-cloudsql`](https://github.com/gofr-dev/gofr/tree/development/examples/using-cloudsql).

---

Contributing to GoFr and want to add another cloud provider's managed SQL (AWS
RDS/Aurora, Azure Database)? See the developer guide in
[`pkg/gofr/datasource/cloudsql/doc.go`](https://github.com/gofr-dev/gofr/blob/development/pkg/gofr/datasource/cloudsql/doc.go).
