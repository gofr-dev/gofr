---
description: "Connect a GoFr service to MySQL, PostgreSQL, or SQLite via env-var configs. The framework manages the connection and exposes ctx.SQL with traces and metrics."
nextjs:
  metadata:
    title: "Dealing with SQL in GoFr — MySQL, Postgres, SQLite"
    description: "Connect a GoFr service to MySQL, PostgreSQL, or SQLite via env-var configs. The framework manages the connection and exposes ctx.SQL with traces and metrics."
---

# Dealing with SQL

GoFr simplifies the process of connecting to SQL databases where one needs to add respective configs in .env,
which allows connecting to different SQL dialects(MySQL, PostgreSQL, SQLite) without going into complexity of configuring connections.

With GoFr, connecting to different SQL databases is as straightforward as setting the DB_DIALECT environment variable to the respective dialect.

## Usage for PostgreSQL and MySQL
To connect with PostgreSQL, set `DB_DIALECT` to `postgres`. Similarly, To connect with MySQL, simply set `DB_DIALECT` to `mysql`.

```dotenv
DB_HOST=localhost
DB_USER=root
DB_PASSWORD=root123
DB_NAME=test_db
DB_PORT=3306

DB_DIALECT=postgres
```

## Usage for SQLite
To connect with SQLite, set `DB_DIALECT` to `sqlite` and `DB_NAME` to the name of your DB File. If the DB file already exists then it will be used otherwise a new one will be created.

```dotenv
DB_NAME=test.db

DB_DIALECT=sqlite
```

## Setting Max open and Idle Connections

To set max open and idle connection for any MySQL, PostgreSQL, SQLite.
Add the following configs in `.env` file.

```dotenv
DB_MAX_IDLE_CONNECTION=5 // Default 2
DB_MAX_OPEN_CONNECTION=5 // Default unlimited
```
> ##### Check out the example on how to add configuration for SQL in GoFr: [Visit GitHub](https://github.com/gofr-dev/gofr/blob/main/examples/http-server/configs/.env)

## Keeping query arguments out of logs and traces

At `LOG_LEVEL=DEBUG`, GoFr logs every query together with its arguments, which can include personal data or
secrets. Set `DB_RECORD_ARGS=false` to replace each argument with `*****`, so the log still shows how many
arguments a query had but not their values:

```dotenv
DB_RECORD_ARGS=false // Default true
```

The query text itself is still logged and traced, so pass sensitive values as arguments (`?` / `$1`) rather than
building them into the query string. Trace spans carry only the query text, never its arguments.

## Related production guides

- **Connection Pooling**: [Tune `DB_MAX_OPEN_CONNECTION` and friends for production load](/docs/guides/connection-pooling) — sizing pools to match traffic and DB capacity.
