# PostgreSQL

GoFr provides built-in support for PostgreSQL databases, enabling applications to leverage PostgreSQL's advanced features for relational data storage, ACID compliance, and complex queries through `gofr.Context`.

```go
// SQL provides methods for GoFr applications to communicate with PostgreSQL
// through standard database/sql operations.
type SQL interface {
	// HealthChecker verifies if the PostgreSQL server is reachable.
	// Returns an error if the server is unreachable, otherwise nil.
	HealthChecker

	// ExecContext executes a query without returning any rows.
	// The args are for any placeholder parameters in the query.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - query: SQL statement to execute (INSERT, UPDATE, DELETE, etc.).
	// - args: Variable number of arguments for placeholder parameters.
	//
	// Returns:
	// - sql.Result containing information about the execution.
	// - Error if the query fails or connectivity issues occur.
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)

	// QueryContext executes a query that returns rows, typically a SELECT.
	// The args are for any placeholder parameters in the query.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - query: SQL SELECT statement to execute.
	// - args: Variable number of arguments for placeholder parameters.
	//
	// Returns:
	// - *sql.Rows containing the query results.
	// - Error if the query fails or connectivity issues occur.
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)

	// QueryRowContext executes a query that is expected to return at most one row.
	// QueryRowContext always returns a non-nil value. Errors are deferred until Row's Scan method is called.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - query: SQL SELECT statement to execute.
	// - args: Variable number of arguments for placeholder parameters.
	//
	// Returns:
	// - *sql.Row containing the query result.
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row

	// PrepareContext creates a prepared statement for later queries or executions.
	// Multiple queries or executions may be run concurrently from the returned statement.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - query: SQL statement to prepare.
	//
	// Returns:
	// - *sql.Stmt representing the prepared statement.
	// - Error if preparation fails or connectivity issues occur.
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)

	// BeginTx starts a transaction with the provided context and transaction options.
	//
	// Parameters:
	// - ctx: Context for managing request lifetime.
	// - opts: Transaction options (isolation level, read-only, etc.).
	//
	// Returns:
	// - *sql.Tx representing the transaction.
	// - Error if transaction start fails or connectivity issues occur.
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)

	// Begin starts a transaction with default options.
	//
	// Returns:
	// - *sql.Tx representing the transaction.
	// - Error if transaction start fails or connectivity issues occur.
	Begin() (*sql.Tx, error)

	// Ping verifies a connection to the database is still alive,
	// establishing a connection if necessary.
	//
	// Returns:
	// - Error if the database is unreachable, otherwise nil.
	Ping() error

	// Stats returns database statistics.
	//
	// Returns:
	// - sql.DBStats containing connection pool statistics.
	Stats() sql.DBStats

	// SetMaxOpenConns sets the maximum number of open connections to the database.
	//
	// Parameters:
	// - n: Maximum number of open connections (0 means unlimited).
	SetMaxOpenConns(n int)

	// SetMaxIdleConns sets the maximum number of connections in the idle connection pool.
	//
	// Parameters:
	// - n: Maximum number of idle connections (0 means no idle connections).
	SetMaxIdleConns(n int)

	// SetConnMaxLifetime sets the maximum amount of time a connection may be reused.
	//
	// Parameters:
	// - d: Maximum lifetime of a connection (0 means connections are reused forever).
	SetConnMaxLifetime(d time.Duration)
}
```

## Configuration

GoFr applications rely on environment variables to configure and connect to a PostgreSQL server.
These variables are stored in a `.env` file located within the `configs` directory at your project root.

### Required Environment Variables:

| Key | Description |
|-----|-------------|
| DB_HOST | Hostname or IP address of your PostgreSQL server |
| DB_PORT | Port number your PostgreSQL server listens on (default: 5432) |
| DB_USER | PostgreSQL username for authentication |
| DB_PASSWORD | Password for PostgreSQL authentication |
| DB_NAME | Name of the PostgreSQL database to connect to |
| DB_DIALECT | Database dialect, set to "postgres" for PostgreSQL |
| DB_SSL_MODE | SSL mode for connection (optional, defaults to "disable") |

### Example `.env` file:

```env
APP_NAME=test-service
HTTP_PORT=9000

# PostgreSQL Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=test_db
DB_DIALECT=postgres
DB_SSL_MODE=disable
```

### SSL Configuration Options:

| SSL Mode | Description |
|----------|-------------|
| disable | No SSL connection |
| require | SSL connection required, but no certificate verification |
| verify-ca | SSL connection required with CA certificate verification |
| verify-full | SSL connection required with full certificate verification |

## Setup with Docker

You can use Docker to set up a PostgreSQL development environment:

```bash
docker run --name gofr-postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=test_db \
  -p 5432:5432 \
  -d postgres:14
```

Create a sample table for testing:

```bash
docker exec -it gofr-postgres psql -U postgres test_db -c \
  "CREATE TABLE customers (id SERIAL PRIMARY KEY, name VARCHAR(255) NOT NULL, email VARCHAR(255), created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);"
```

Insert sample data:

```bash
docker exec -it gofr-postgres psql -U postgres test_db -c \
  "INSERT INTO customers (name, email) VALUES ('John Doe', 'john@example.com'), ('Jane Smith', 'jane@example.com');"
```

## Usage Example

The following example demonstrates using PostgreSQL in a GoFr application for CRUD operations, transactions, and advanced queries.

```go
package main

import (
	"database/sql"
	"fmt"
	"time"

	"gofr.dev/pkg/gofr"
)

type Customer struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type CustomerInput struct {
	Name  string `json:"name" validate:"required,min=2,max=100"`
	Email string `json:"email" validate:"required,email"`
}

func main() {
	app := gofr.New()

	// Register routes
	app.GET("/health", dbHealthCheck)
	app.POST("/customers", createCustomer)
	app.GET("/customers", getAllCustomers)
	app.GET("/customers/:id", getCustomerByID)
	app.PUT("/customers/:id", updateCustomer)
	app.DELETE("/customers/:id", deleteCustomer)
	app.GET("/customers/search", searchCustomers)
	app.POST("/customers/batch", createCustomersBatch)
	app.GET("/stats", getDatabaseStats)

	// Run the app
	app.Run()
}

// Health check for PostgreSQL
func dbHealthCheck(c *gofr.Context) (any, error) {
	err := c.SQL.Ping()
	if err != nil {
		return map[string]string{"status": "unhealthy", "error": err.Error()}, err
	}
	
	return map[string]string{"status": "healthy", "database": "PostgreSQL"}, nil
}

// Create a new customer
func createCustomer(c *gofr.Context) (any, error) {
	var input CustomerInput
	if err := c.Bind(&input); err != nil {
		return nil, err
	}
	
	var customer Customer
	query := `
		INSERT INTO customers (name, email, created_at) 
		VALUES ($1, $2, CURRENT_TIMESTAMP) 
		RETURNING id, name, email, created_at`
	
	err := c.SQL.QueryRowContext(c.Context, query, input.Name, input.Email).Scan(
		&customer.ID, &customer.Name, &customer.Email, &customer.CreatedAt,
	)
	
	if err != nil {
		return nil, err
	}
	
	return map[string]interface{}{
		"message":  "Customer created successfully",
		"customer": customer,
	}, nil
}

// Get all customers with pagination
func getAllCustomers(c *gofr.Context) (any, error) {
	page := c.Param("page")
	limit := c.Param("limit")
	
	// Default values
	if page == "" {
		page = "1"
	}
	if limit == "" {
		limit = "10"
	}
	
	offset := (parseInt(page) - 1) * parseInt(limit)
	
	query := `
		SELECT id, name, email, created_at 
		FROM customers 
		ORDER BY created_at DESC 
		LIMIT $1 OFFSET $2`
	
	rows, err := c.SQL.QueryContext(c.Context, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var customers []Customer
	for rows.Next() {
		var customer Customer
		err := rows.Scan(&customer.ID, &customer.Name, &customer.Email, &customer.CreatedAt)
		if err != nil {
			return nil, err
		}
		customers = append(customers, customer)
	}
	
	// Get total count
	var totalCount int
	countQuery := "SELECT COUNT(*) FROM customers"
	err = c.SQL.QueryRowContext(c.Context, countQuery).Scan(&totalCount)
	if err != nil {
		return nil, err
	}
	
	return map[string]interface{}{
		"customers":   customers,
		"total_count": totalCount,
		"page":        page,
		"limit":       limit,
	}, nil
}

// Get customer by ID
func getCustomerByID(c *gofr.Context) (any, error) {
	id := c.PathParam("id")
	
	var customer Customer
	query := "SELECT id, name, email, created_at FROM customers WHERE id = $1"
	
	err := c.SQL.QueryRowContext(c.Context, query, id).Scan(
		&customer.ID, &customer.Name, &customer.Email, &customer.CreatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("customer with ID %s not found", id)
		}
		return nil, err
	}
	
	return customer, nil
}

// Update customer
func updateCustomer(c *gofr.Context) (any, error) {
	id := c.PathParam("id")
	
	var input CustomerInput
	if err := c.Bind(&input); err != nil {
		return nil, err
	}
	
	query := `
		UPDATE customers 
		SET name = $1, email = $2 
		WHERE id = $3 
		RETURNING id, name, email, created_at`
	
	var customer Customer
	err := c.SQL.QueryRowContext(c.Context, query, input.Name, input.Email, id).Scan(
		&customer.ID, &customer.Name, &customer.Email, &customer.CreatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("customer with ID %s not found", id)
		}
		return nil, err
	}
	
	return map[string]interface{}{
		"message":  "Customer updated successfully",
		"customer": customer,
	}, nil
}

// Delete customer
func deleteCustomer(c *gofr.Context) (any, error) {
	id := c.PathParam("id")
	
	result, err := c.SQL.ExecContext(c.Context, "DELETE FROM customers WHERE id = $1", id)
	if err != nil {
		return nil, err
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	
	if rowsAffected == 0 {
		return nil, fmt.Errorf("customer with ID %s not found", id)
	}
	
	return map[string]interface{}{
		"message": fmt.Sprintf("Customer with ID %s deleted successfully", id),
	}, nil
}

// Search customers by name or email
func searchCustomers(c *gofr.Context) (any, error) {
	searchTerm := c.Param("q")
	if searchTerm == "" {
		return nil, fmt.Errorf("search term 'q' is required")
	}
	
	query := `
		SELECT id, name, email, created_at 
		FROM customers 
		WHERE name ILIKE $1 OR email ILIKE $1 
		ORDER BY created_at DESC`
	
	searchPattern := "%" + searchTerm + "%"
	rows, err := c.SQL.QueryContext(c.Context, query, searchPattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var customers []Customer
	for rows.Next() {
		var customer Customer
		err := rows.Scan(&customer.ID, &customer.Name, &customer.Email, &customer.CreatedAt)
		if err != nil {
			return nil, err
		}
		customers = append(customers, customer)
	}
	
	return map[string]interface{}{
		"customers":   customers,
		"search_term": searchTerm,
		"count":       len(customers),
	}, nil
}

// Create multiple customers in a transaction
func createCustomersBatch(c *gofr.Context) (any, error) {
	var inputs []CustomerInput
	if err := c.Bind(&inputs); err != nil {
		return nil, err
	}
	
	if len(inputs) == 0 {
		return nil, fmt.Errorf("no customers provided")
	}
	
	// Begin transaction
	tx, err := c.SQL.BeginTx(c.Context, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	
	var customers []Customer
	query := `
		INSERT INTO customers (name, email, created_at) 
		VALUES ($1, $2, CURRENT_TIMESTAMP) 
		RETURNING id, name, email, created_at`
	
	for _, input := range inputs {
		var customer Customer
		err := tx.QueryRowContext(c.Context, query, input.Name, input.Email).Scan(
			&customer.ID, &customer.Name, &customer.Email, &customer.CreatedAt,
		)
		if err != nil {
			return nil, err // This will trigger rollback
		}
		customers = append(customers, customer)
	}
	
	// Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	
	return map[string]interface{}{
		"message":   fmt.Sprintf("%d customers created successfully", len(customers)),
		"customers": customers,
	}, nil
}

// Get database statistics
func getDatabaseStats(c *gofr.Context) (any, error) {
	stats := c.SQL.Stats()
	
	// Get additional PostgreSQL-specific statistics
	var dbSize string
	sizeQuery := "SELECT pg_size_pretty(pg_database_size(current_database()))"
	err := c.SQL.QueryRowContext(c.Context, sizeQuery).Scan(&dbSize)
	if err != nil {
		return nil, err
	}
	
	var activeConnections int
	connQuery := "SELECT count(*) FROM pg_stat_activity WHERE state = 'active'"
	err = c.SQL.QueryRowContext(c.Context, connQuery).Scan(&activeConnections)
	if err != nil {
		return nil, err
	}
	
	return map[string]interface{}{
		"max_open_connections":     stats.MaxOpenConnections,
		"open_connections":         stats.OpenConnections,
		"in_use_connections":       stats.InUse,
		"idle_connections":         stats.Idle,
		"wait_count":              stats.WaitCount,
		"wait_duration":           stats.WaitDuration,
		"max_idle_closed":         stats.MaxIdleClosed,
		"max_idle_time_closed":    stats.MaxIdleTimeClosed,
		"max_lifetime_closed":     stats.MaxLifetimeClosed,
		"database_size":           dbSize,
		"active_connections":      activeConnections,
	}, nil
}

// Helper function to parse integer
func parseInt(s string) int {
	if s == "" {
		return 0
	}
	// Simple conversion - in production, add proper error handling
	result := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}
```

## Advanced Features

### 1. Connection Pooling Configuration

Configure connection pooling in your application:

```go
func configureDatabase(app *gofr.App) {
	// Set maximum number of open connections
	app.SQL.SetMaxOpenConns(25)
	
	// Set maximum number of idle connections
	app.SQL.SetMaxIdleConns(5)
	
	// Set maximum lifetime of connections
	app.SQL.SetConnMaxLifetime(5 * time.Minute)
}
```

### 2. Prepared Statements

Use prepared statements for better performance with repeated queries:

```go
func getCustomersByEmailDomain(c *gofr.Context) (any, error) {
	domain := c.Param("domain")
	
	// Prepare statement
	stmt, err := c.SQL.PrepareContext(c.Context, 
		"SELECT id, name, email FROM customers WHERE email LIKE $1")
	if err != nil {
		return nil, err
	}
	defer stmt.Close()
	
	// Execute prepared statement
	rows, err := stmt.QueryContext(c.Context, "%@"+domain)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var customers []Customer
	for rows.Next() {
		var customer Customer
		err := rows.Scan(&customer.ID, &customer.Name, &customer.Email)
		if err != nil {
			return nil, err
		}
		customers = append(customers, customer)
	}
	
	return customers, nil
}
```

### 3. Complex Queries with Joins

Example of complex queries with table joins:

```go
func getCustomersWithOrders(c *gofr.Context) (any, error) {
	query := `
		SELECT 
			c.id, c.name, c.email,
			COUNT(o.id) as order_count,
			COALESCE(SUM(o.total_amount), 0) as total_spent
		FROM customers c
		LEFT JOIN orders o ON c.id = o.customer_id
		GROUP BY c.id, c.name, c.email
		ORDER BY total_spent DESC`
	
	rows, err := c.SQL.QueryContext(c.Context, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	type CustomerWithOrders struct {
		Customer
		OrderCount int     `json:"order_count"`
		TotalSpent float64 `json:"total_spent"`
	}
	
	var results []CustomerWithOrders
	for rows.Next() {
		var result CustomerWithOrders
		err := rows.Scan(
			&result.ID, &result.Name, &result.Email,
			&result.OrderCount, &result.TotalSpent,
		)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}
	
	return results, nil
}
```

### 4. JSON Operations (PostgreSQL-specific)

PostgreSQL supports JSON operations natively:

```go
func searchCustomersByMetadata(c *gofr.Context) (any, error) {
	// Assuming customers table has a 'metadata' JSONB column
	query := `
		SELECT id, name, email, metadata
		FROM customers 
		WHERE metadata ->> 'city' = $1
		ORDER BY metadata ->> 'signup_date' DESC`
	
	city := c.Param("city")
	rows, err := c.SQL.QueryContext(c.Context, query, city)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	type CustomerWithMetadata struct {
		Customer
		Metadata string `json:"metadata"`
	}
	
	var customers []CustomerWithMetadata
	for rows.Next() {
		var customer CustomerWithMetadata
		err := rows.Scan(&customer.ID, &customer.Name, &customer.Email, &customer.Metadata)
		if err != nil {
			return nil, err
		}
		customers = append(customers, customer)
	}
	
	return customers, nil
}
```

## Data Migrations

GoFr supports database migrations for PostgreSQL, allowing you to manage schema changes across different environments.

Create migration files in the `migrations` directory:

```go
// migrations/20240101120000_create_customers_table.go
package migrations

import "gofr.dev/pkg/gofr"

func up(c *gofr.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS customers (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			email VARCHAR(255) UNIQUE NOT NULL,
			phone VARCHAR(20),
			address TEXT,
			metadata JSONB,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE INDEX IF NOT EXISTS idx_customers_email ON customers(email);
		CREATE INDEX IF NOT EXISTS idx_customers_created_at ON customers(created_at);
		CREATE INDEX IF NOT EXISTS idx_customers_metadata ON customers USING GIN(metadata);`
	
	_, err := c.SQL.ExecContext(c.Context, query)
	return err
}

func down(c *gofr.Context) error {
	_, err := c.SQL.ExecContext(c.Context, "DROP TABLE IF EXISTS customers CASCADE")
	return err
}
```

## Performance Tips

1. **Use Connection Pooling**: Configure appropriate pool sizes based on your application load.

2. **Prepare Statements**: Use prepared statements for frequently executed queries.

3. **Use Transactions**: Group related operations in transactions for consistency and performance.

4. **Indexes**: Create appropriate indexes for frequently queried columns.

5. **Query Optimization**: Use EXPLAIN ANALYZE to optimize slow queries.

6. **Batch Operations**: Use batch inserts/updates when dealing with multiple records.

7. **Pagination**: Always use LIMIT and OFFSET for large result sets.

## Error Handling

```go
func handleDatabaseErrors(err error) (int, string) {
	if err == sql.ErrNoRows {
		return 404, "Record not found"
	}
	
	// Check for PostgreSQL-specific errors
	if pqErr, ok := err.(*pq.Error); ok {
		switch pqErr.Code {
		case "23505": // unique_violation
			return 409, "Record already exists"
		case "23503": // foreign_key_violation
			return 400, "Referenced record does not exist"
		case "23514": // check_violation
			return 400, "Data validation failed"
		default:
			return 500, fmt.Sprintf("Database error: %s", pqErr.Message)
		}
	}
	
	return 500, "Internal server error"
}
```