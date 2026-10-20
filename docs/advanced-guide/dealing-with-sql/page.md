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

## Transactions

`ctx.SQL.Begin()` starts a transaction. Every statement run on the returned transaction is logged and
recorded in the `app_sql_stats` metric, the same as a statement run on `ctx.SQL` directly. Besides
`Exec`, `Query`, `QueryRow` and `Prepare`, the transaction has context-aware variants
(`ExecContext`, `QueryContext`, `QueryRowContext`, `PrepareContext`) and a `Select` that binds rows
into a slice or struct exactly like `ctx.SQL.Select`.

```go
func Transfer(ctx *gofr.Context) (any, error) {
	tx, err := ctx.SQL.Begin()
	if err != nil {
		return nil, err
	}

	defer tx.Rollback() // no-op once Commit has succeeded

	if _, err = tx.ExecContext(ctx, "UPDATE accounts SET balance = balance - ? WHERE id = ?", 100, 1); err != nil {
		return nil, err
	}

	if _, err = tx.ExecContext(ctx, "UPDATE accounts SET balance = balance + ? WHERE id = ?", 100, 2); err != nil {
		return nil, err
	}

	var balances []int

	tx.Select(ctx, &balances, "SELECT balance FROM accounts WHERE id IN (?, ?)", 1, 2)

	return balances, tx.Commit()
}
```

`Begin` starts the transaction with `context.Background()`, so the request's deadline does not end
the transaction itself; only the statements given `ctx` are bound to it. To tie the whole
transaction to the request, or to set the isolation level or a read-only transaction, use `BeginTx`
on the SQL datasource. If the request context is canceled before `Commit`, the driver rolls the
transaction back.

```go
import (
	"database/sql"

	gofrSQL "gofr.dev/pkg/gofr/datasource/sql"
)

db, ok := ctx.SQL.(*gofrSQL.DB)
if !ok {
	return nil, errors.New("BeginTx needs the built-in SQL datasource")
}

tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
```

The type assertion works when `ctx.SQL` is the built-in SQL datasource. It fails when the
datasource is wrapped, for example by the [DBResolver](/docs/quick-start/connecting-mysql), and with
`container.NewMockContainer` in tests.

## Related production guides

- **Connection Pooling**: [Tune `DB_MAX_OPEN_CONNECTION` and friends for production load](/docs/guides/connection-pooling) — sizing pools to match traffic and DB capacity.
