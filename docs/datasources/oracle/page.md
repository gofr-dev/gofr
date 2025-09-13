# OracleDB

## Configuration
To connect to `OracleDB`, you need to provide the following environment variables:
- `HOST`: The hostname or IP address of your OracleDB server.
- `PORT`: The port number.
- `USERNAME`: The username for connecting to the database.
- `PASSWORD`: The password for the specified user.
- `SERVICE`: The specific Oracle database instance or service on the server that the client should connect to.

## Setup
GoFr supports injecting OracleDB as a relational datasource through a clean, extensible interface. Any driver that implements the following interface can be added using the `app.AddOracle()` method, and users can access OracleDB throughout their application via `gofr.Context`.

```go
type Oracle interface {
 Exec(ctx context.Context, query string, args ...any) error
 Select(ctx context.Context, dest any, query string, args ...any) error
}
```

This approach allows users to easily inject any compatible Oracle driver, providing both usability and the flexibility to use multiple databases in a GoFr application.

## ⚠️ Important: Oracle Database Must Exist

**Before running your GoFr application, you must ensure that the Oracle database and the required schema (such as the `users` table) are already created.**

- Oracle does not allow creating a database (PDB or CDB) via a simple SQL query from a standard client connection.
- You must use Oracle tools (like DBCA, SQL\*Plus as SYSDBA, or Docker container initialization) to create the database and pluggable database (PDB) before connecting your app.
- Your application can create tables within an existing schema, but the database itself must be provisioned in advance.

## Setting Up OracleDB with Docker

To help new users, the following steps outline how to quickly set up an OracleDB instance using Docker.

### 1. Prerequisites

- **Docker** installed on your system.
- An **Oracle account** (free) with access to the Oracle Container Registry.

### 2. Create Your Oracle Account

Visit the Oracle Container Registry and create or sign in to your account:

👉 [https://container-registry.oracle.com/ords/f?p=113:10:14574461221664:::::](https://container-registry.oracle.com/ords/f?p=113:10:14574461221664:::::)

### 3. Pull the Oracle Free Database Docker Image

In your terminal:

1. Log in to the Oracle Container Registry using your Oracle account credentials:

```sh
docker login container-registry.oracle.com
```

2. After login, pull the Oracle Free Database image:

```sh
docker pull container-registry.oracle.com/database/free:latest
```

### 4. Run the Oracle Database Container

You can now run the OracleDB container (replace `YourPasswordHere` with a suitable strong password):

```sh
docker run -d --name oracle-free -p 1521:1521 -e ORACLE_PWD=YourPasswordHere container-registry.oracle.com/database/free:latest

```

- The database will be available on port **1521**
- The default Pluggable Database (PDB) is **FREEPDB1**
- The `system` user password is your `ORACLE_PWD`
- The service name for connecting is `FREEPDB1`

You can verify the container is running:

```sh
docker ps
```

### 5. Connect to the Oracle Database

Option 1: Direct SQL\*Plus session from within the container:

```sh
docker exec -it oracle-free sqlplus system/YourPasswordHere@localhost:1521/FREEPDB1
```

Option 2: Open bash shell inside the container and use SQL\*Plus from there:

```sh
docker exec -it oracle-free bash
sqlplus system/YourPasswordHere@localhost:1521/FREEPDB1
```

### 6. Create the `users` Table

Based on the Go struct:

```go
type User struct {
 Id   string `db:"ID"`
 Name string `db:"NAME"`
 Age  int    `db:"AGE"`
}
```

Run the following SQL command in SQL\*Plus:

```sql
CREATE TABLE users (
 id   VARCHAR2(36) PRIMARY KEY,
 name VARCHAR2(100),
 age  NUMBER
);
```

This will create the required table for the GoFr application to interact with.

### 7. Sample OracleDB Config for GoFr

| Setting     | Value              |
| :---------- | :----------------- |
| host        | `localhost`        |
| port        | `1521`             |
| username    | `system`           |
| password    | `YourPasswordHere` |
| service/SID | `FREEPDB1`         |

## Import the GoFr External Driver for OracleDB

```bash
go get gofr.dev/pkg/gofr/datasource/oracle@latest
```

## Example

```go
package main

import (
 "gofr.dev/pkg/gofr"
 "gofr.dev/pkg/gofr/datasource/oracle"
 "os"
)

type User struct {
 Id   string `db:"ID"`
 Name string `db:"NAME"`
 Age  int    `db:"AGE"`
}

func main() {
 app := gofr.New()

 app.AddOracle(oracle.New(oracle.Config{
  Host:     os.Getenv("HOST"),
  Port:     os.Getenv("PORT"),
  Username: os.Getenv("USERNAME"),
  Password: os.Getenv("PASSWORD")
  Service:  os.Getenv("SERVICE"),
 }))

 app.POST("/user", Post)
 app.GET("/user", Get)

 app.Run()
}

func Post(ctx *gofr.Context) (any, error) {
 err := ctx.Oracle.Exec(ctx, "INSERT INTO users (id, name, age) VALUES (:1, :2, :3)",
  "8f165e2d-feef-416c-95f6-913ce3172e15", "aryan", 10)
 if err != nil {
  return nil, err
 }
 return "successfully inserted", nil
}

func Get(ctx *gofr.Context) (any, error) {
 var users []map[string]any
 err := ctx.Oracle.Select(ctx, &users, "SELECT id, name, age FROM users")
 if err != nil {
  return nil, err
 }
 return users, nil
}
```

## Example API Usage

You can create a user and get users using the following commands on the command prompt:

- **Create a user:**

```sh
curl -X POST http://localhost:8000/user
```

- **Get all users:**

```sh
curl http://localhost:8000/user
```
