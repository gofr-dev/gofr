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
# PostgreSQL Advanced Features

## Transaction Handling

### Basic Transaction Example
```sql
-- Start a transaction
BEGIN;

-- Insert data
INSERT INTO customers (name) VALUES ('John Doe');
INSERT INTO orders (customer_id, amount) VALUES (1, 100.00);

-- Commit the transaction
COMMIT;
```

### Transaction with Rollback
```sql
-- Start transaction
BEGIN;

-- Operations
UPDATE accounts SET balance = balance - 100 WHERE id = 1;
UPDATE accounts SET balance = balance + 100 WHERE id = 2;

-- Check if both operations succeeded, otherwise rollback
-- ROLLBACK; -- Use this to undo changes
COMMIT; -- Use this to save changes
```

### Savepoints
```sql
BEGIN;
INSERT INTO customers (name) VALUES ('Alice');

SAVEPOINT sp1;
INSERT INTO orders (customer_id, amount) VALUES (1, 50.00);

-- Rollback to savepoint if needed
ROLLBACK TO SAVEPOINT sp1;

COMMIT;
```

## Join Operations and Complex Queries

### Inner Join
```sql
SELECT c.name, o.amount, o.order_date
FROM customers c
INNER JOIN orders o ON c.id = o.customer_id;
```

### Left Join with Aggregation
```sql
SELECT c.name, COUNT(o.id) as order_count, SUM(o.amount) as total_spent
FROM customers c
LEFT JOIN orders o ON c.id = o.customer_id
GROUP BY c.id, c.name
ORDER BY total_spent DESC;
```

### Complex Query with Multiple Joins
```sql
SELECT 
    c.name as customer_name,
    p.name as product_name,
    oi.quantity,
    oi.price,
    o.order_date
FROM customers c
JOIN orders o ON c.id = o.customer_id
JOIN order_items oi ON o.id = oi.order_id
JOIN products p ON oi.product_id = p.id
WHERE o.order_date >= '2024-01-01'
ORDER BY o.order_date DESC;
```

### Window Functions
```sql
SELECT 
    name,
    salary,
    department,
    AVG(salary) OVER (PARTITION BY department) as dept_avg_salary,
    ROW_NUMBER() OVER (PARTITION BY department ORDER BY salary DESC) as rank_in_dept
FROM employees;
```

## Prepared Statements

### Creating and Using Prepared Statements
```sql
-- Prepare a statement
PREPARE get_customer_orders (int) AS
SELECT c.name, o.amount, o.order_date
FROM customers c
JOIN orders o ON c.id = o.customer_id
WHERE c.id = $1;

-- Execute the prepared statement
EXECUTE get_customer_orders(1);

-- Deallocate when done
DEALLOCATE get_customer_orders;
```

### Bulk Insert with Prepared Statement
```sql
PREPARE insert_product (varchar, decimal, int) AS
INSERT INTO products (name, price, stock) VALUES ($1, $2, $3);

EXECUTE insert_product('Laptop', 999.99, 10);
EXECUTE insert_product('Mouse', 29.99, 100);
EXECUTE insert_product('Keyboard', 79.99, 50);
```

## Migration Examples

### Database Migration Script
```sql
-- Migration: Add email column to customers table
-- File: migrations/001_add_email_to_customers.sql

BEGIN;

-- Add email column
ALTER TABLE customers ADD COLUMN email VARCHAR(255);

-- Add unique constraint
ALTER TABLE customers ADD CONSTRAINT unique_email UNIQUE (email);

-- Create index for better performance
CREATE INDEX idx_customers_email ON customers(email);

-- Update migration tracking
INSERT INTO migrations (version, name, applied_at) 
VALUES ('001', 'add_email_to_customers', NOW());

COMMIT;
```

### Rollback Migration
```sql
-- Rollback: Remove email column from customers table
-- File: migrations/rollback_001_add_email_to_customers.sql

BEGIN;

-- Remove index
DROP INDEX IF EXISTS idx_customers_email;

-- Remove constraint
ALTER TABLE customers DROP CONSTRAINT IF EXISTS unique_email;

-- Remove column
ALTER TABLE customers DROP COLUMN IF EXISTS email;

-- Remove from migration tracking
DELETE FROM migrations WHERE version = '001';

COMMIT;
```

## Connection Pool Configuration

### Docker Compose with Connection Pooling
```yaml
version: '3.8'
services:
  postgres:
    image: postgres:14
    environment:
      POSTGRES_DB: test_db
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    ports:
      - "5432:5432"
    command: >
      postgres
      -c max_connections=200
      -c shared_buffers=256MB
      -c effective_cache_size=1GB
      -c maintenance_work_mem=64MB
      -c checkpoint_completion_target=0.9
      -c wal_buffers=16MB
      -c default_statistics_target=100
      -c random_page_cost=1.1
      -c effective_io_concurrency=200

  pgbouncer:
    image: pgbouncer/pgbouncer:latest
    environment:
      DATABASES_HOST: postgres
      DATABASES_PORT: 5432
      DATABASES_USER: postgres
      DATABASES_PASSWORD: postgres
      DATABASES_DBNAME: test_db
      POOL_MODE: transaction
      MAX_CLIENT_CONN: 100
      DEFAULT_POOL_SIZE: 25
    ports:
      - "6432:5432"
    depends_on:
      - postgres
```

### Application Configuration Example
```bash
# Environment variables for connection pooling
DATABASE_URL=postgresql://postgres:postgres@localhost:6432/test_db
DB_POOL_SIZE=20
DB_MAX_OVERFLOW=30
DB_POOL_TIMEOUT=30
DB_POOL_RECYCLE=3600
```

### Connection Pool Settings
```sql
-- PostgreSQL configuration for connection management
-- Add to postgresql.conf

# Connection settings
max_connections = 200
shared_buffers = 256MB

# Memory settings
work_mem = 4MB
maintenance_work_mem = 64MB
effective_cache_size = 1GB

# WAL settings
wal_buffers = 16MB
checkpoint_completion_target = 0.9

# Query planner settings
random_page_cost = 1.1
effective_io_concurrency = 200
```

## Performance Optimization

### Indexing Strategies
```sql
-- Create composite index for common queries
CREATE INDEX idx_orders_customer_date ON orders(customer_id, order_date);

-- Create partial index for active records
CREATE INDEX idx_active_customers ON customers(created_at) WHERE active = true;

-- Create expression index
CREATE INDEX idx_customer_name_lower ON customers(LOWER(name));
```

### Query Optimization
```sql
-- Use EXPLAIN ANALYZE to understand query performance
EXPLAIN ANALYZE
SELECT c.name, COUNT(o.id) as order_count
FROM customers c
LEFT JOIN orders o ON c.id = o.customer_id
GROUP BY c.id, c.name
HAVING COUNT(o.id) > 5;

-- Optimize with proper indexing and query structure
CREATE INDEX idx_orders_customer_id ON orders(customer_id);
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