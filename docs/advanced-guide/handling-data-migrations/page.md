# Handling Data Migrations

Suppose you manually make changes to your database, and now it's your responsibility to inform other developers to execute them. Additionally, you need to keep track of which changes should be applied to production machines in the next deployment.
GoFr supports data migrations for MySQL, Postgres, Redis, ClickHouse & Cassandra which allows altering the state of a database, be it adding a new column to existing table or modifying the data type of existing column or adding constraints to an existing table, setting and removing keys etc.

## Usage

### Creating Migration Files

It is recommended to maintain a `migrations` directory in your project root to enhance readability and maintainability.

**Migration file names**

It is recommended that each migration file should be numbered in the format of _YYYYMMDDHHMMSS_ when the migration was created.
This helps prevent numbering conflicts and allows for maintaining the correct sort order by name in different filesystem views.

Run the following commands to create a migration file

```shell
  # Install GoFr CLI
  go install gofr.dev/cli/gofr@latest

  # Create migration
  gofr migrate create -name=create_employee_table
```

Add the `createTableEmployee` function given below in the created file in `migrations` directory.

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

For MySQL, it is highly recommended to use `IF EXISTS` and `IF NOT EXIST` in DDL commands as MySQL implicitly commits these commands.

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

> **Best Practice:** Before creating multiple migrations, learn about [organizing migrations by feature](#organizing-migrations-by-feature) to avoid creating one migration per table or operation.

### Initialization from main.go

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

## Organizing Migrations by Feature

**Important:** Migrations should be organized by **feature**, not by individual database operations. The migration history should tell the story of feature evolution, not database operation granularity.

### Bad Practice: One Migration Per Operation

A common mistake is to create one migration for each table or operation, even when they're part of the same feature:

```go
func All() map[int64]migration.Migrate {
    return map[int64]migration.Migrate{
        20251114000001: createTableUsers(),
        20251114000002: createTableMonitors(),
        20251114000003: createTableCheckResults(),
        20251114000004: createTableIncidents(),
    }
}
```

**Why this is problematic:**
- When reverting a feature, you want to revert all related changes together
- When deploying, you want to deploy the entire feature atomically
- Having multiple migrations for a single feature creates unnecessary complexity and potential inconsistencies

### Good Practice: One Migration Per Feature

Instead, group all database operations related to a single feature into one migration:

```go
func All() map[int64]migration.Migrate {
    return map[int64]migration.Migrate{
        20251114000001: addMonitoringFeature(), // Creates all 4 tables together
    }
}

func addMonitoringFeature() migration.Migrate {
    return migration.Migrate{
        UP: func(d migration.Datasource) error {
            // Create all tables for the monitoring feature
            if _, err := d.SQL.Exec(createTableUsers); err != nil {
                return err
            }
            if _, err := d.SQL.Exec(createTableMonitors); err != nil {
                return err
            }
            if _, err := d.SQL.Exec(createTableCheckResults); err != nil {
                return err
            }
            if _, err := d.SQL.Exec(createTableIncidents); err != nil {
                return err
            }
            return nil
        },
    }
}
```

**Benefits of this approach:**
- **Atomic deployment:** The entire feature is deployed or reverted together
- **Clear history:** Migration history reflects feature evolution, not granular operations
- **Easier rollback:** Reverting a feature means reverting one migration, not tracking multiple related migrations
- **Better organization:** Related changes stay together, making the codebase easier to understand

## Multi-Instance Deployments

When running multiple instances of your application (e.g., in Kubernetes or Docker Swarm), GoFr automatically coordinates migrations to ensure only one instance runs them at a time.

### How It Works

1. **Automatic Coordination:** When multiple instances start simultaneously, they coordinate using distributed locks
2. **One Runs, Others Wait:** The first instance to acquire the lock runs migrations, while others wait
3. **Fast Path:** If migrations are already complete, instances return immediately without acquiring locks

### Lock Mechanism

**SQL (MySQL/PostgreSQL/SQLite):**
- Uses a dedicated `gofr_migration_locks` table
- Lock TTL: 10 seconds
- Heartbeat: Refreshes every 5 seconds for long migrations

**Redis:**
- Uses `SETNX` with TTL
- Lock TTL: 10 seconds  
- Heartbeat: Refreshes every 5 seconds for long migrations

**Retry Behavior:**
- Max retries: 140 attempts
- Retry interval: 500ms
- Total wait time: ~70 seconds

### What This Means for You

**✅ No code changes needed** - Locking happens automatically

**✅ Safe deployments** - Multiple instances won't corrupt data

**✅ Long migrations supported** - Locks are automatically extended via heartbeat

**✅ Crash recovery** - Locks auto-expire after 10 seconds if a pod crashes

### Example Deployment

\`\`\`yaml
# docker-compose.yaml or Kubernetes deployment
services:
  app:
    image: myapp:latest
    replicas: 3  # All 3 instances coordinate automatically
\`\`\`

When you deploy:
- Instance 1: Acquires lock → Runs migrations → Releases lock
- Instance 2: Waits for lock → Sees migrations complete → Continues startup
- Instance 3: Waits for lock → Sees migrations complete → Continues startup

> **Note:** Single-instance deployments work exactly as before with no performance impact.

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

### Migrations in Cassandra

`GoFr` provides support for migrations in Cassandra but does not guarantee atomicity for individual Data Manipulation Language (DML) commands. To achieve atomicity during migrations, users can leverage batch operations using the `NewBatch`, `BatchQuery`, and `ExecuteBatch` methods. These methods allow multiple queries to be executed as a single atomic operation.

Alternatively, users can construct their batch queries using the `BEGIN BATCH` and `APPLY BATCH` statements to ensure that all the commands within the batch are executed successfully or not at all. This is particularly useful for complex migrations involving multiple inserts, updates, or schema changes in a single transaction-like operation.

When using batch operations, consider using a `LoggedBatch` for atomicity or an `UnloggedBatch` for improved performance where atomicity isn't required. This approach provides a way to maintain data consistency during complex migrations.

> Note: The following example assumes that user has already created the `KEYSPACE` in cassandra. A `KEYSPACE` in Cassandra is a container for tables that defines data replication settings across the cluster.


```go
package migrations

import (
	"gofr.dev/pkg/gofr/migration"
)

const (
	createTableCassandra = `CREATE TABLE IF NOT EXISTS employee (
                            id int PRIMARY KEY,
                            name text,
                            gender text,
                            number text
                            );`

	addCassandraRecords = `BEGIN BATCH
                           INSERT INTO employee (id, name, gender, number) VALUES (1, 'Alison', 'F', '1234567980');
                           INSERT INTO employee (id, name, gender, number) VALUES (2, 'Alice', 'F', '9876543210');
                           APPLY BATCH;
                           `

	employeeDataCassandra = `INSERT INTO employee (id, name, gender, number) VALUES (?, ?, ?, ?);`
)

func createTableEmployeeCassandra() migration.Migrate {
	return migration.Migrate{
		UP: func(d migration.Datasource) error {
			// Execute the create table statement
			if err := d.Cassandra.Exec(createTableCassandra); err != nil {
				return err
			}

			// Batch processes can also be executed in Exec as follows:
			if err := d.Cassandra.Exec(addCassandraRecords); err != nil {
				return err
			}

			// Create a new batch operation
			batchName := "employeeBatch"
			if err := d.Cassandra.NewBatch(batchName, 0); err != nil { // 0 for LoggedBatch
				return err
			}

			// Add multiple queries to the batch
			if err := d.Cassandra.BatchQuery(batchName, employeeDataCassandra, 1, "Harry", "M", "1234567980"); err != nil {
				return err
			}

			if err := d.Cassandra.BatchQuery(batchName, employeeDataCassandra, 2, "John", "M", "9876543210"); err != nil {
				return err
			}

			// Execute the batch operation
			if err := d.Cassandra.ExecuteBatch(batchName); err != nil {
				return err
			}

			return nil
		},
	}
}
```

## Migrations in ElasticSearch

GoFr allows Elasticsearch document migrations, focusing on **single document** and **bulk operations**.

### Single Document Migration

```go  
func addSingleProduct() migration.Migrate {  
 return migration.Migrate{ 
	 UP: func(d migration.Datasource) error { 
			 product := map[string]any{ 
			 "title": "Laptop", 
			 "price": 999.99, 
			 "category": "electronics", 
			 } 
			 
		return d.Elasticsearch.IndexDocument( context.Background(), "products", "1", product, ) }, }
		}  
```  

### Bulk Operation Migration

```go  
func bulkProducts() migration.Migrate {  
 return migration.Migrate{ 
 UP: func(d migration.Datasource) error { 
		operations := []map[string]any{ 
			{"index": map[string]any{"_index": "products", "_id": "1"}}, 
			{"title": "Phone", "price": 699.99, "category": "electronics"}, 
			{"index": map[string]any{"_index": "products", "_id": "2"}}, 
			{"title": "Mug", "price": 12.99, "category": "kitchen"}, 
			 }
		
		_, err := d.Elasticsearch.Bulk(context.Background(), operations) return err },}
	}  
``` 

> ##### Check out the example to add and run migrations in GoFr: [Visit GitHub](https://github.com/gofr-dev/gofr/blob/main/examples/using-migrations/main.go)
