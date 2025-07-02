# PostgreSQL

PostgreSQL is a powerful, open source object-relational database system with over 35 years of active development.

## Docker

To run PostgreSQL using Docker, use the following command:

```bash
docker run --name gofr-postgres -e POSTGRES_PASSWORD=password -p 5432:5432 -d postgres:15-alpine
```

## Connecting to PostgreSQL

GoFr provides built-in database support. To connect to PostgreSQL, provide the following environment variables:

```bash
DB_HOST=localhost
DB_USER=postgres  
DB_PASSWORD=password
DB_NAME=test_db
DB_PORT=5432
DB_DIALECT=postgres
```

### Example

```go
package main

import "gofr.dev/pkg/gofr"

func main() {
    app := gofr.New()

    app.GET("/users", func(ctx *gofr.Context) (interface{}, error) {
        rows, err := ctx.DB.Query("SELECT id, name FROM users")
        if err != nil {
            return nil, err
        }
        defer rows.Close()

        var users []map[string]interface{}
        for rows.Next() {
            var id int
            var name string
            rows.Scan(&id, &name)
            users = append(users, map[string]interface{}{
                "id": id, "name": name,
            })
        }
        return users, nil
    })

    app.Start()
}
```

## CRUD Operations

### Create

```go
// Insert single record
func createUser(ctx *gofr.Context) (interface{}, error) {
    query := `INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id`
    
    var userID int
    err := ctx.DB.QueryRow(query, "John Doe", "john@example.com").Scan(&userID)
    if err != nil {
        return nil, err
    }
    
    return map[string]int{"id": userID}, nil
}
```

### Read

```go
// Get single user
func getUser(ctx *gofr.Context) (interface{}, error) {
    userID := ctx.PathParam("id")
    
    query := `SELECT id, name, email FROM users WHERE id = $1`
    row := ctx.DB.QueryRow(query, userID)
    
    var user struct {
        ID    int    `json:"id"`
        Name  string `json:"name"`
        Email string `json:"email"`
    }
    
    err := row.Scan(&user.ID, &user.Name, &user.Email)
    if err != nil {
        return nil, err
    }
    
    return user, nil
}
```

### Update

```go
func updateUser(ctx *gofr.Context) (interface{}, error) {
    userID := ctx.PathParam("id")
    
    var reqBody struct {
        Name  string `json:"name"`
        Email string `json:"email"`
    }
    ctx.Bind(&reqBody)
    
    query := `UPDATE users SET name = $1, email = $2 WHERE id = $3`
    _, err := ctx.DB.Exec(query, reqBody.Name, reqBody.Email, userID)
    
    return "User updated", err
}
```

### Delete

```go
func deleteUser(ctx *gofr.Context) (interface{}, error) {
    userID := ctx.PathParam("id")
    
    query := `DELETE FROM users WHERE id = $1`
    _, err := ctx.DB.Exec(query, userID)
    
    return "User deleted", err
}
```

## Transactions

```go
func transferAmount(ctx *gofr.Context) (interface{}, error) {
    tx, err := ctx.DB.Begin()
    if err != nil {
        return nil, err
    }
    defer tx.Rollback()
    
    // Debit from account 1
    _, err = tx.Exec("UPDATE accounts SET balance = balance - $1 WHERE id = $2", 100, 1)
    if err != nil {
        return nil, err
    }
    
    // Credit to account 2
    _, err = tx.Exec("UPDATE accounts SET balance = balance + $1 WHERE id = $2", 100, 2)
    if err != nil {
        return nil, err
    }
    
    return "Transfer completed", tx.Commit()
}
```

## Joins

```go
func getUsersWithPosts(ctx *gofr.Context) (interface{}, error) {
    query := `
        SELECT u.id, u.name, p.title 
        FROM users u 
        LEFT JOIN posts p ON u.id = p.user_id
    `
    
    rows, err := ctx.DB.Query(query)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var result []map[string]interface{}
    for rows.Next() {
        var userID int
        var userName, postTitle sql.NullString
        
        rows.Scan(&userID, &userName, &postTitle)
        
        result = append(result, map[string]interface{}{
            "user_id":    userID,
            "user_name":  userName.String,
            "post_title": postTitle.String,
        })
    }
    
    return result, nil
}
```

## Prepared Statements

```go
func searchUsers(ctx *gofr.Context) (interface{}, error) {
    stmt, err := ctx.DB.Prepare("SELECT id, name FROM users WHERE name ILIKE $1")
    if err != nil {
        return nil, err
    }
    defer stmt.Close()
    
    searchTerm := "%" + ctx.Param("search") + "%"
    rows, err := stmt.Query(searchTerm)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var users []map[string]interface{}
    for rows.Next() {
        var id int
        var name string
        rows.Scan(&id, &name)
        users = append(users, map[string]interface{}{
            "id": id, "name": name,
        })
    }
    
    return users, nil
}
```

## Migration Example

```go
func createTables(ctx *gofr.Context) error {
    queries := []string{
        `CREATE TABLE IF NOT EXISTS users (
            id SERIAL PRIMARY KEY,
            name VARCHAR(100) NOT NULL,
            email VARCHAR(100) UNIQUE NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
        `CREATE TABLE IF NOT EXISTS posts (
            id SERIAL PRIMARY KEY,
            user_id INTEGER REFERENCES users(id),
            title VARCHAR(200) NOT NULL,
            content TEXT,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )`,
    }
    
    for _, query := range queries {
        _, err := ctx.DB.Exec(query)
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

## Connection Pool Configuration

```bash
DB_HOST=localhost
DB_USER=postgres
DB_PASSWORD=password
DB_NAME=test_db
DB_PORT=5432
DB_DIALECT=postgres
DB_MAX_OPEN_CONNS=25    # Optional
DB_MAX_IDLE_CONNS=5     # Optional
DB_CONN_MAX_LIFETIME=5m # Optional
```