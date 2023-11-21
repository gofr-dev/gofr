# Migration

Migrations are used to alter the state of a database, be it adding a new column to existing table or modifying the data type of an existing column or adding constraints to an existing table, etc.

**How Migrations help?**

Suppose you manually edit fragments of SQL, and now it's your responsibility to inform other developers to execute them. Additionally, you need to keep track of which changes should be applied to production machines in the next deployment.

The Database table called **migration** helps in such case. This table tracks which migrations have already been executed and ensures that only migrations that have never been run are executed. This way, you only need to ensure that your migrations are properly in place. ([Learn more](https://cloud.google.com/architecture/database-migration-concepts-principles-part-1))

**Migration file names**

In GoFr each Migration is run in numeric order forward or backward depending on the method taken. Each migration is numbered using the timestamp when the migration was created, in **YYYYMMDDHHIISS** format (e.g., **20210517142839**). This helps prevent numbering conflicts when working in a team environment. Every time we make migrations, it is advised to create a new migration file.

By default, a prefix (of the timestamp) is added to the migration file name which we have created.

**For example :**

- `20210517142839_create.go`, where _20210517142839_ is timestamp and _create_ is name of the migration.

## Migration Template

The migration templates are created in _PROJECT_DIR/migrations_ directory. The name of the migration is in the format :- _k<'timestamp'>_, the timestamp when the migration was created.

Below is an example of a migration template created at _2020/03/20 at 09:53:52 hrs_:-

```go
type K20200320095352 struct {
}

func (k K20200320095352) Up(d *datastore.DataStore, logger log.Logger) error {
	return nil
}

func (k K20200320095352) Down(d *datastore.DataStore, logger log.Logger) error {
	return nil
}
```

Both Up and Down method accept datastore as a parameter which gives access to the database on which the migration is to be run.

**Note:**

- Using the `gofr migrate create` command of the **gofr-cli** tool, one can create a migration template.
- One can install gofr-cli through following command
  `go install gofr.dev/cmd/gofr`

Example: `gofr migrate create -name=<name of your migration>`

## Usage

```go
package migrations

import (
	"fmt"

	"gofr.dev/gofr/pkg/datastore"
	"gofr.dev/gofr/pkg/log"
)

type K20210517154650 struct {
}

func (k K20210517154650) Up(d *datastore.DataStore, logger log.Logger) error {
	fmt.Println("Running migration up: 20210517153524_create.go")

	_,err:=d.DB().Exec("CREATE TABLE IF NOT EXISTS `users` "+
	    "(`id` int NOT NULL AUTO_INCREMENT, "+
		"`first_name` varchar(50) NOT NULL,"+
		"`last_name` varchar(50) NOT NULL,"+
		"`email_id` varchar(150) NOT NULL, "+
		"PRIMARY KEY (`id`))")

	if err != nil {
		return err
	}

	return nil
}

func (k K20210517154650) Down(d *datastore.DataStore, logger log.Logger) error {
	fmt.Println("Running migration down: 20210517153524_create.go")

	_,err:=d.DB().Exec("Drop table If EXISTS `users`")

	if err != nil {
		return err
	}

	return nil
}
```

To run migrations from `main.go`, it will look like the following.

```go
package main

import (
	"fmt"
	"net/http"

	"gofr.dev/cmd/gofr/migration"
	"gofr.dev/cmd/gofr/migration/dbMigration"
	"gofr.dev/pkg/gofr"

	"github.com/example/migrations"
)

func main() {
	// initialise gofr object
	app := gofr.New()

	// get app name from configs
	appName := app.Config.Get("APP_NAME")

	err := migration.Migrate(appName, dbmigration.NewGorm(app.GORM()),
		migrations.All(), dbmigration.UP, app.Logger)
	if err != nil {
		fmt.Println(err)
	}

	//register middlewares, routes after running migration


	// Starts the server, it will listen on the default port 8000.
	// it can be over-ridden through configs
	app.Start()
}
```

## How to Run Migrations from the Command line?

We can run migrations from CLI as :

`gofr migrate -method=<method> -database=<database> -tag=<migration_to_run>`

- **method**: it can be UP or DOWN.
- **database**: Cassandra, Mongo, Redis, YCQL, GORM (MySQL, MSSQL, Postgres, Sqlite).
- **tag**: when the method is DOWN, specify the migrations to run in comma-separated format. If tag is not specified, then the DOWN version of all the UP migrations runs.

### Usage

```bash
# Apply Migrations
# To run all the UP migrations which ran not yet, run the following command

gofr migrate -method=UP -database=gorm

# Rollback Migrations
# To run all the Down migrations which ran not yet, run the following command
gofr migrate -method=DOWN -database=gorm

# To run some specific Down migrations which ran not yet, run the following command
gofr migrate -method=DOWN -database=gorm -tag=20200301123212,20200403231214
```
