# Handling Data Migrations

Suppose you manually make changes to your database, and now it's your responsibility to inform other developers to execute them. Additionally, you need to keep track of which changes should be applied to production machines in the next deployment.
GoFr supports data migrations for MySQL, Postgres and Redis which allows altering the state of a database, be it adding a new column to existing table or modifying the data type of existing column or adding constraints to an existing table, setting and removing keys etc.

## Usage

### Creating Migration Files

It is recommended to maintain a `migrations` directory in your project root to enhance readability and maintainability.

**Migration file names**

It is recommended that each migration file should be numbered in the format of _YYYYMMDDHHMMSS_ when the migration was created.
This helps prevent numbering conflicts and allows for maintaining the correct sort order by name in different filesystem views.

Create the following file in `migrations` directory.

**Filename : 20240226153000_create_employee_table.go**

```go
package migrations

import "gofr.dev/pkg/gofr/migration"


const createTable = `CREATE TABLE IF NOT EXISTS employee
(
    id             int         not null
        primary key,
    name           varchar(50) not null,
    gender         varchar(6)  not null,
    contact_number varchar(10) not null
);`

func createTableEmployee() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			_, err := d.SQL.Exec(createTable)
			if err != nil {
				return err
			}
			return nil
		},
	}
}
```

`migration.Datasource` have the datasources whose migrations are supported i.e., Redis and SQL (MySQL and PostgreSQL).
All migrations always run in a transaction.

For MySQL it is highly recommended to use `IF EXISTS` and `IF NOT EXIST` in DDL commands as MySQL implicitly commits these commands.

**Create a function which returns all the migrations in a map**

**Filename : all.go**

```go
package migrations

import "gofr.dev/pkg/gofr/migration"

func All() map[int64]migration.Migrate {
	return map[int64]migration.Migrate{
		20240226153000: createTableEmployee(),
	}
}
```

Migrations run in ascending order of keys in this map.

### Initialisation from main.go

```go
package main

import (
	"gofr.dev/examples/using-migrations/migrations"
	"gofr.dev/pkg/gofr"
)

func main() {
	// Create a new application
	a := gofr.New()

	// Add migrations to run
	a.Migrate(migrations.All())

	// Run the application
	a.Run()
}

```

When we run the app we will see the following logs for migrations which ran successfully.

```bash
INFO [16:55:46] Migration 20240226153000 ran successfully
```

GoFr maintains the records in the database itself which helps in tracking which migrations have already been executed and ensures that only migrations that have never been run are executed.

## Migration Records

**SQL**

Migration records are stored and maintained in **gofr_migrations** table which has the following schema:

{% table %}

- Field
- Type

---

- version
- bigint

---

- method
- varchar(4)

---

- start_time
- timestamp

---

- duration
- bigint

---

{% /table %}

**REDIS**

Migration records are stored and maintained in a Redis Hash named **gofr_migrations** where key is the version and value contains other details in JSON format.

Example :

Key : 20240226153000

Value : {"method":"UP","startTime":"2024-02-26T15:03:46.844558+05:30","duration":0}

Where,

**Version** : Migration version is the number provided in the map, i.e., sequence number.

**Start Time** : Time when Migration Started in UTC.

**Duration** : Time taken by Migration since it started in milliseconds.

**Method** : It contains the method(UP/DOWN) in which migration ran.
(For now only method UP is supported)
