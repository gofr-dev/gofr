# Handling Data Migrations

Gofr supports data migrations for MySQL and Redis which allows to alter the state of a database, be it adding a new column to existing table or modifying the data type of an existing column or adding constraints to an existing table, setting and removing keys etc.

**How Migrations help?**

Suppose you manually edit fragments of your database, and now it's your responsibility to inform other developers to execute them. Additionally, you need to keep track of which changes should be applied to production machines in the next deployment.

GoFr maintains the table called **gofr_migration** which helps in such case. This table tracks which migrations have already been executed and ensures that only migrations that have never been run are executed. This way, you only need to ensure that your migrations are properly in place. ([Learn more](https://cloud.google.com/architecture/database-migration-concepts-principles-part-1))

## Usage

We will create an employee table using migrations.

### Creating Migration Files

It is recommended to maintain a migrations directory in your project root to enhance readability and maintainability.

**Migration file names**

It is recommended that each migration file should be numbered using the unix timestamp when the migration was created, This helps prevent numbering conflicts when working in a team environment.

Create the following file in migrations directory.

**Filename : 1708322067_create_employee_table.go**
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
			_, err := d.DB.Exec(createTable)
			if err != nil {
				return err
			}
			return nil
		},
	}
}
```

`migration.Datasource` have the datasources whose migrations are supported which are Redis and MySQL. 
All the migrations run in transactions by default.

For MySQL it is highly recommended to use `IF EXISTS` and `IF NOT EXIST` in DDL commands as MySQL implicitly commits these Commands.

**Create a function which returns all the migrations in a map**

**Filename : all.go**
```go
package migrations

import "gofr.dev/pkg/gofr/migration"

func All() map[int64]migration.Migrate {
	return map[int64]migration.Migrate{
		1708322067: createTableEmployee(),
	}
}
```

Migrations will run in ascending order of keys in this map.

### Initialisation from main.go 
```go
package main

import (
	"errors"
	"fmt"

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
INFO [16:55:46] Migration 1708322067 ran successfully
```




