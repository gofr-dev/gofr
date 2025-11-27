# Connecting to MySQL

Just like Redis, GoFr supports connection to various SQL-compatible databases (MySQL, MariaDB, PostgreSQL, and Supabase) based on configuration variables.

## MySQL/MariaDB

### Setup

Users can run MySQL/MariaDB and create a database locally using the following Docker command:

```bash
docker run --name gofr-mysql -e MYSQL_ROOT_PASSWORD=root123 -e MYSQL_DATABASE=test_db -p 3306:3306 -d mysql:8.0.30
```

Access the `test_db` database and create a table customer with columns `id` and `name`. Change MySQL to MariaDB as needed: 

```bash
docker exec -it gofr-mysql mysql -uroot -proot123 test_db -e "CREATE TABLE customers (id INT AUTO_INCREMENT PRIMARY KEY, name VARCHAR(255) NOT NULL);"
```

Now that the database with the table is ready, we can connect our GoFr server to MySQL/MariaDB. 

### Configuration & Usage

After adding MySQL/MariaDB configs `.env` will be updated to the following. Use ```DB_DIALECT=mysql``` for both MySQL and MariaDB.

```dotenv
# configs/.env
APP_NAME=test-service
HTTP_PORT=9000

REDIS_HOST=localhost
REDIS_PORT=6379

DB_HOST=localhost
DB_USER=root
DB_PASSWORD=root123
DB_NAME=test_db
DB_PORT=3306
DB_DIALECT=mysql
DB_CHARSET=utf8 #(optional)
```

## PostgreSQL

### Setup

Users can run PostgreSQL and create a database locally using the following Docker command:

```bash
docker run --name gofr-postgres -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=test_db -p 5432:5432 -d postgres:14
```

Access `test_db` database and create a table customer with columns `id` and `name`:

```bash
docker exec -it gofr-postgres psql -U postgres test_db -c "CREATE TABLE customers (id SERIAL PRIMARY KEY, name VARCHAR(255) NOT NULL);"
```

### Configuration & Usage

After adding PostgreSQL configs, `.env` will be updated to the following:

```dotenv
# configs/.env
APP_NAME=test-service
HTTP_PORT=9000

REDIS_HOST=localhost
REDIS_PORT=6379

DB_HOST=localhost
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=test_db
DB_PORT=5432
DB_DIALECT=postgres
DB_SSL_MODE=disable #(optional, defaults to disable)
```

## Supabase

[Supabase](https://supabase.com) is an open-source Firebase alternative that provides a PostgreSQL database with additional features. GoFr supports connecting to Supabase databases with specialized configuration.

### Setup

To use Supabase with GoFr:

1. Sign up for a [Supabase account](https://supabase.com)
2. Create a new project
3. Get your connection information from the Supabase dashboard:
   - Project Reference ID
   - Database Password
   - Region (for pooled connections)

### Configuration & Usage

GoFr provides three connection types for Supabase:

1. **Direct Connection**: Standard connection to the database
2. **Session Pooler**: Connection via Supabase's connection pooler (maintains session variables)
3. **Transaction Pooler**: Connection via Supabase's transaction pooler (resets session variables)

Add Supabase configuration to your `.env` file:

```dotenv
# configs/.env
APP_NAME=test-service
HTTP_PORT=9000

# Supabase configuration
DB_DIALECT=supabase
DB_USER=postgres
DB_PASSWORD=your_database_password
DB_NAME=postgres
DB_PORT=5432      # Optional, defaults based on connection type
DB_SSL_MODE=require  # Optional, always forced to "require" for Supabase

# Supabase-specific configs
SUPABASE_PROJECT_REF=your_project_ref_id
SUPABASE_CONNECTION_TYPE=direct  # Options: direct, session, transaction
SUPABASE_REGION=us-east-1  # Required for pooled connections
```

Alternatively, you can provide a full connection string:

```dotenv
DB_DIALECT=supabase
DB_URL=postgresql://postgres:your_password@db.your_project_ref.supabase.co:5432/postgres
```

#### Connection Types

- **Direct** (`SUPABASE_CONNECTION_TYPE=direct`): Connects directly to your database at `db.[PROJECT_REF].supabase.co:5432`
- **Session Pooler** (`SUPABASE_CONNECTION_TYPE=session`): Uses Supabase's connection pooler at `aws-0-[REGION].pooler.supabase.co:5432`
- **Transaction Pooler** (`SUPABASE_CONNECTION_TYPE=transaction`): Uses Supabase's transaction pooler at `aws-0-[REGION].pooler.supabase.co:6543`

**Note:** For pooled connections, the `SUPABASE_REGION` parameter is required.

## Database Usage Example

For all supported SQL databases, GoFr provides a consistent API to interact with your data.

Now, in the following example, we'll store customer data using **POST** `/customer` and then use **GET** `/customer` to retrieve the same.
We will be storing the customer data with `id` and `name`.

After adding code to add and retrieve data from the SQL datastore, `main.go` will be updated to the following.
```go
package main

import (
	"errors"

	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr"
)

type Customer struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

func main() {
	// initialize gofr object
	app := gofr.New()

	app.GET("/redis", func(ctx *gofr.Context) (any, error) {
		// Get the value using the Redis instance

		val, err := ctx.Redis.Get(ctx.Context, "test").Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			// If the key is not found, we are not considering this an error and returning ""
			return nil, err
		}

		return val, nil
	})

	app.POST("/customer/{name}", func(ctx *gofr.Context) (any, error) {
		name := ctx.PathParam("name")

		// Inserting a customer row in database using SQL
		_, err := ctx.SQL.ExecContext(ctx, "INSERT INTO customers (name) VALUES (?)", name)

		return nil, err
	})

	app.GET("/customer", func(ctx *gofr.Context) (any, error) {
		var customers []Customer

		// Getting the customer from the database using SQL
		rows, err := ctx.SQL.QueryContext(ctx, "SELECT * FROM customers")
		if err != nil {
			return nil, err
		}

		for rows.Next() {
			var customer Customer
			if err := rows.Scan(&customer.ID, &customer.Name); err != nil {
				return nil, err
			}

			customers = append(customers, customer)
		}

		// return the customer
		return customers, nil
	})

	app.Run()
}
```

To update the database with the customer data access, use this curl command through the terminal

```bash
# here abc and xyz after /customer are the path parameters
curl --location --request POST 'http://localhost:9000/customer/abc'

curl --location --request POST 'http://localhost:9000/customer/xyz'
```

Now when we access {% new-tab-link title="http://localhost:9000/customer" href="http://localhost:9000/customer" /%} we should see the following output:

```json
{
  "data": [
    {
      "id": 1,
      "name": "abc"
    },
    {
      "id": 2,
      "name": "xyz"
    }
  ]
}
```

**Note:** When using PostgreSQL or Supabase, you may need to use `$1` instead of `?` in SQL queries, depending on your driver configuration.

### Enabling Read/Write Splitting in MySQL (DBResolver)
GoFr provides built-in support for read/write splitting using its `DBRESOLVER` module for **MySQL**.
This feature automatically routes requests to the **primary database** or **read replicas** based on:

- **HTTP Method**: Write operations (`POST`, `PUT`, `PATCH`, `DELETE`) → Primary | Read operations (`GET`, `HEAD`, `OPTIONS`) → Replicas
- **Route Configuration**: Force specific routes to always use the primary database for strong consistency

#### Installation

Import the GoFr's dbresolver for MySQL:

```shell
go get gofr.dev/pkg/gofr/datasource/dbresolver@latest
```

#### Configuration

**1. Environment Variables**

Configure the primary database in your .env file:

```editorconfig
# Primary database
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=root123
DB_NAME=test_db
DB_DIALECT=mysql
```

**2. Initialize DBResolver**

After importing the package, you can configure the DBResolver in your GoFr application using the `AddDBResolver` method. 
You can choose the load balancing strategy and enable fallback to primary:

```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/dbresolver"
)

type Customer struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

func main() {
	a := gofr.New()

	// Initialize DB resolver with default settings
	err := dbresolver.InitDBResolver(a, &dbresolver.Config{
		Strategy:      dbresolver.StrategyRoundRobin, // use round-robin strategy or random strategy
		ReadFallback:  true, // allow reads on primary if all replicas are down
		MaxFailures:   3, 			  // number of allowed failures before marking a replica as down
		TimeoutSec:    30, // timeout for marking a replica as down
		PrimaryRoutes: []string{"/admin", "/api/payments/*"},

		Replicas: []dbresolver.ReplicaCredential{
			{
				Host:     "localhost:3307",
				User:     "replica_user1",
				Password: "pass1",
			},
			{
				Host:     "replica2.example.com:3308",
				User:     "replica_user2",
				Password: "pass2",
			},
			{
				Host:     "replica3.example.com:3309",
				User:     "replica_user3",
				Password: "pass3",
			},
		},// routes that should go to primary
	})
	if err != nil {
		a.Logger().Errorf("failed to initialize db resolver: %v", err)
	}

	// Read endpoint - goes to replica
	a.GET("/customers", func(c *gofr.Context) (interface{}, error) {
		var customers []Customer

		c.SQL.Select(c, &customers, "SELECT id, name FROM customers")

		return customers, err
	})

	// Write endpoint - goes to primary
	a.POST("/customers", func(c *gofr.Context) (interface{}, error) {
		var customer Customer

		c.Bind(&customer)

		_, err := c.SQL.Exec("INSERT INTO customers (name) VALUES (?)", customer.Name)

		return customer, err
	})

	// Admin endpoint - forced to primary
	a.GET("/admin/customers", func(c *gofr.Context) (interface{}, error) {
		var customers []Customer

		c.SQL.Select(c, &customers, "SELECT id, name FROM customers")

		return customers, err
	})

	a.Run()
}
```

**3. Connection Pool Tuning (Optional)**

By default, replica pools are auto-scaled based on primary settings:

```
# Defaults (automatically calculated)
DB_MAX_IDLE_CONNECTION=2    → Replicas: 8 (2 × 4)
DB_MAX_OPEN_CONNECTION=20   → Replicas: 40 (20 × 2)
```

Override with:

```
DB_REPLICA_MAX_IDLE_CAP=100
DB_REPLICA_MIN_IDLE=5
DB_REPLICA_DEFAULT_IDLE=15

DB_REPLICA_MAX_OPEN_CAP=500
DB_REPLICA_MIN_OPEN=20
DB_REPLICA_DEFAULT_OPEN=150

```

**Benefits**
- Performance: Offloads read traffic from the primary, reducing latency.
- Scalability: Easily scale reads by adding more replicas.
- Resilience: Ensures high availability through automatic fallback.