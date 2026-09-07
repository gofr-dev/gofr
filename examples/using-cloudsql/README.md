# Using Cloud SQL

This GoFr example connects to a Google Cloud SQL instance using the
[`cloudsql`](../../pkg/gofr/datasource/cloudsql) datasource. On GCP it
authenticates with **IAM database authentication** — no static database password
and no Cloud SQL Auth Proxy sidecar — while still supporting ordinary
username/password auth for local development.

> The `cloudsql` datasource supports both Postgres and MySQL. This example is
> written for **Postgres** (it uses a `$1` placeholder and a `SERIAL` schema). To
> run it against MySQL, change the placeholder to `?` and use an `AUTO_INCREMENT`
> schema.

The Cloud SQL datasource is plugged in with `app.AddSQLDB`, so `app.SQL()` /
`ctx.SQL` and all of GoFr's SQL logging, metrics and health checks work exactly
as they do with the built-in SQL datasource.

The same one line works in both environments — only configuration changes. With
`DB_IAM_AUTH=true` it uses IAM auth; otherwise it uses a standard username/password
connection.

```go
app := gofr.New()
app.AddSQLDB(cloudsql.New(app.Config))
```

## Configuration

All settings come from environment variables (see `configs/.env`):

| Variable              | Description                                                        |
|-----------------------|--------------------------------------------------------------------|
| `DB_HOST`             | Instance connection name, `project:region:instance`               |
| `DB_DIALECT`          | `postgres` or `mysql`                                              |
| `DB_NAME`             | Database name                                                     |
| `DB_USER`             | IAM principal (SA email without `.gserviceaccount.com`) or DB user |
| `DB_IAM_AUTH`         | `true` for IAM auth, `false` for username/password                |
| `DB_PASSWORD`         | Only used when `DB_IAM_AUTH=false`                                |
| `DB_CLOUDSQL_IP_TYPE` | `PUBLIC`, `PRIVATE` or `PSC` (defaults to `PUBLIC`)               |

## Prerequisites (IAM auth on GCP)

1. Enable the Cloud SQL Admin API on the project.
2. Create an IAM database user for your service account on the instance.
3. Grant the service account the **Cloud SQL Instance User** and **Cloud SQL
   Client** roles.
4. Make Application Default Credentials available to the process — on GKE/Cloud
   Run this is provided automatically via Workload Identity; locally you can use
   `gcloud auth application-default login`.

## Schema

```sql
CREATE TABLE customers (
    id   SERIAL PRIMARY KEY,
    name TEXT NOT NULL
);
```

## Running

```console
go run main.go
```

Then:

```console
curl -X POST http://localhost:8000/customers -d '{"name":"alice"}'
curl http://localhost:8000/customers
```

For local development without GCP, set `DB_IAM_AUTH=false`, `DB_USER=postgres`
and `DB_PASSWORD=...` in `configs/.env`, pointing `DB_HOST` at a Cloud SQL
instance you can reach.
