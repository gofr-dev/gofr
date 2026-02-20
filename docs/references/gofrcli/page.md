# GoFR Command Line Interface

Managing repetitive tasks and maintaining consistency across large-scale applications is challenging!

**GoFr CLI provides the following:**

* All-in-one command-line tool designed specifically for GoFr applications
* Simplifies **database migrations** management
* **Store Layer Generator** for type-safe data access code from YAML configurations
* Abstracts **tracing**, **metrics** and structured **logging** for GoFr's gRPC server/client
* Enforces standard **GoFr conventions** in new projects

## Prerequisites

- Go 1.22 or above. To check Go version use the following command:
```bash
  go version
```

## **Installation**
To get started with GoFr CLI, use the below commands

```bash
  go install gofr.dev/cli/gofr@latest
```

To check the installation:
```bash
  gofr version
```
---

## Usage

The CLI can be run directly from the terminal after installation. Here’s the general syntax:

```bash
  gofr <subcommand> [flags]=[arguments]
```
---

## **Commands**

## 1. ***`init`***

   The init command initializes a new GoFr project. It sets up the foundational structure for the project and generates a basic "Hello World!" program as a starting point. This allows developers to quickly dive into building their application with a ready-made structure.

### Command Usage
```bash
  gofr init
```
---

## 2. ***`migrate create`***

   The migrate create command generates a migration template file with pre-defined structure in your migrations directory.
   This boilerplate code helps you maintain consistent patterns when writing database schema modifications across your project.


### Command Usage
```bash
  gofr migrate create -name=<migration-name>
```

### Example Usage

```bash
gofr migrate create -name=create_employee_table
```
This command generates a migration directory which has the below files:

1. A new migration file with timestamp prefix (e.g., `20250127152047_create_employee_table.go`) containing:
```go
package migrations

import (
    "gofr.dev/pkg/gofr/migration"
)

func create_employee_table() migration.Migrate {
    return migration.Migrate{
        UP: func(d migration.Datasource) error {
            // write your migrations here
            return nil
        },
    }
}
```
2. An auto-generated all.go file that maintains a registry of all migrations:
```go
// This is auto-generated file using 'gofr migrate' tool. DO NOT EDIT.
package migrations

import (
    "gofr.dev/pkg/gofr/migration"
)

func All() map[int64]migration.Migrate {
    return map[int64]migration.Migrate {
        20250127152047: create_employee_table(),
    }
}
```

> **💡 Best Practice:** Learn about [organizing migrations by feature](../../docs/advanced-guide/handling-data-migrations#organizing-migrations-by-feature) to avoid creating one migration per table or operation.

For detailed instructions on handling database migrations, see the [handling-data-migrations documentation](../../docs/advanced-guide/handling-data-migrations)
For more examples, see the [using-migrations](https://github.com/gofr-dev/gofr/tree/main/examples/using-migrations)
---

## 3. ***`wrap grpc`***

   * The gofr wrap grpc command streamlines gRPC integration in a GoFr project by generating GoFr's context-aware structures.
   * It simplifies setting up gRPC handlers with minimal steps, and accessing datasources, adding tracing as well as custom metrics. Based on the proto file it creates the handler/client with GoFr's context.
   For detailed instructions on using grpc with GoFr see the [gRPC documentation](../../advanced-guide/grpc/page.md)

### Command Usage
**gRPC Server**
```bash
  gofr wrap grpc server --proto=<path_to_the_proto_file>
```
### Generated Files
**Server**
- ```{serviceName}_gofr.go (auto-generated; do not modify)```
- ```{serviceName}_server.go (example structure below)```

### Example Usage
**gRPC Server**

The command generates a server implementation template similar to this:
```go
package server

import (
   "gofr.dev/pkg/gofr"
)

// Register the gRPC service in your app using the following code in your main.go:
//
// service.Register{ServiceName}ServerWithGofr(app, &server.{ServiceName}Server{})
//
// {ServiceName}Server defines the gRPC server implementation.
// Customize the struct with required dependencies and fields as needed.
type {ServiceName}Server struct {
}

// Example method (actual methods will depend on your proto file)
func (s *MyServiceServer) MethodName(ctx *gofr.Context) (any, error) {
   // Replace with actual logic if needed
   return &ServiceResponse{
   }, nil
}
```
For detailed instruction on setting up a gRPC server with GoFr see the [gRPC Server Documentation](https://gofr.dev/docs/advanced-guide/grpc#generating-g-rpc-server-handler-template-using)

**gRPC Client**
```bash
  gofr wrap grpc client --proto=<path_to_the_proto_file>
```

**Client**
- ```{serviceName}_client.go (example structure below)```


### Example Usage:
Assuming the service is named hello, after generating the hello_client.go file, you can seamlessly register and access the gRPC service using the following steps:

```go
type GreetHandler struct {
	helloGRPCClient client.HelloGoFrClient
}

func NewGreetHandler(helloClient client.HelloGoFrClient) *GreetHandler {
    return &GreetHandler{
        helloGRPCClient: helloClient,
    }
}

func (g GreetHandler) Hello(ctx *gofr.Context) (any, error) {
    userName := ctx.Param("name")
    helloResponse, err := g.helloGRPCClient.SayHello(ctx, &client.HelloRequest{Name: userName})
    if err != nil {
        return nil, err
    }

    return helloResponse, nil
}

func main() {
    app := gofr.New()

// Create a gRPC client for the Hello service
    helloGRPCClient, err := client.NewHelloGoFrClient(app.Config.Get("GRPC_SERVER_HOST"), app.Metrics())
    if err != nil {
		app.Logger().Errorf("Failed to create Hello gRPC client: %v", err)
    return
}

    greetHandler := NewGreetHandler(helloGRPCClient)

    // Register HTTP endpoint for Hello service
    app.GET("/hello", greetHandler.Hello)

    // Run the application
    app.Run()
}
```
For detailed instruction on setting up a gRPC server with GoFr see the [gRPC Client Documentation](https://gofr.dev/docs/advanced-guide/grpc#generating-tracing-enabled-g-rpc-client-using)
For more examples refer [gRPC Examples](https://github.com/gofr-dev/gofr/tree/main/examples/grpc)

---
## 4. ***`store`***

The `gofr store` command is a code generator that creates type-safe data access layers from YAML configuration files. It eliminates boilerplate code while maintaining GoFr's best practices for observability and context management.

### **Features**

* **YAML-Driven Configuration**: Define your data models and queries in a simple, declarative format.
* **Type-Safe Code Generation**: Generates Go interfaces and implementation boilerplates.
* **GoFr Context Integration**: Generated methods work with `*gofr.Context` for built-in observability.
* **Multiple Stores**: Define all stores in a single YAML file — each gets its own directory.
* **Store Registry**: Centralized factory management of all generated stores via `stores/all.go`.

### **Commands**

#### **Initialize Store Configuration**

Create a new store directory and a `store.yaml` configuration template. **The `-name` flag is required.**

```bash
gofr store init -name=<store-name>
```

**Example:**
```bash
gofr store init -name=user
```

This creates the following structure:
- `stores/store.yaml` — Configuration file template (shared across all stores).
- `stores/all.go` — Store registry factory (auto-generated, DO NOT EDIT).
- `stores/user/interface.go` — Initial interface stub (DO NOT EDIT — regenerated by `generate`).
- `stores/user/user.go` — Initial implementation stub (editable — add your SQL logic here).

#### **Generate Store Code**

Generate or update Go code from your store configuration file.

```bash
gofr store generate
```

> **💡 Note:** By default, this command looks for the configuration at **`stores/store.yaml`**. To use a different path, use the `-config` flag:
> ```bash
> gofr store generate -config=path/to/store.yaml
> ```

---

### **Quick Start Example**

**Step 1: Initialize Configuration**
```bash
gofr store init -name=user
```

**Step 2: Define Your Store in `stores/store.yaml`**
```yaml
version: "1.0"

stores:
  - name: "user"
    package: "user"
    output_dir: "stores/user"
    interface: "UserStore"
    implementation: "userStore"
    queries:
      - name: "GetUserByID"
        sql: "SELECT id, name, email FROM users WHERE id = ?"
        type: "select"
        model: "User"
        returns: "single"
        params:
          - name: "id"
            type: "int64"
        description: "Retrieves a user by their ID"

      - name: "GetAllUsers"
        sql: "SELECT id, name, email FROM users"
        type: "select"
        model: "User"
        returns: "multiple"
        description: "Retrieves all users"

models:
  - name: "User"
    fields:
      - name: "ID"
        type: "int64"
        tag: 'db:"id" json:"id"'
      - name: "Name"
        type: "string"
        tag: 'db:"name" json:"name"'
      - name: "Email"
        type: "string"
        tag: 'db:"email" json:"email"'
```

**Step 3: Generate Store Code**
```bash
gofr store generate
```

This generates:
```
stores/
├── store.yaml          # Central Configuration
├── all.go              # Store registry factory (auto-generated)
└── user/
    ├── interface.go    # UserStore interface definition
    ├── userStore.go    # userStore implementation boilerplate
    └── user.go         # User model struct
```

**Step 4: Use in Your Application**
```go
package main

import (
    "gofr.dev/pkg/gofr"
    "your-project/stores/user"
)

func main() {
    app := gofr.New()

    userStore := user.NewUserStore()

    app.GET("/users/{id}", func(ctx *gofr.Context) (interface{}, error) {
        id, _ := strconv.ParseInt(ctx.PathParam("id"), 10, 64)
        return userStore.GetUserByID(ctx, id)
    })

    app.GET("/users", func(ctx *gofr.Context) (interface{}, error) {
        return userStore.GetAllUsers(ctx)
    })

    app.Run()
}
```

---

### **Multiple Stores in One File**

You can define all stores in a single YAML file. Each store gets its own output directory and all are registered into the same `stores/all.go` registry.

```yaml
version: "1.0"

stores:
  - name: "user"
    package: "user"
    output_dir: "stores/user"
    interface: "UserStore"
    implementation: "userStore"
    queries: [...]

  - name: "product"
    package: "product"
    output_dir: "stores/product"
    interface: "ProductStore"
    implementation: "productStore"
    queries: [...]

models:
  - name: "User"
    fields: [...]
  - name: "Product"
    fields: [...]
```

**Generated structure:**
```
stores/
├── all.go
├── user/
│   ├── interface.go
│   ├── userStore.go
│   └── user.go
└── product/
    ├── interface.go
    ├── productStore.go
    └── product.go
```

**Using the registry with multiple stores:**
```go
import (
    "your-project/stores"
    "your-project/stores/user"
    "your-project/stores/product"
)

// stores.GetStore returns a factory-created instance
userStore    := stores.GetStore("user").(user.UserStore)
productStore := stores.GetStore("product").(product.ProductStore)
```

> **💡 Note:** `stores.All()` returns a `map[string]func() any` — a map of **factory functions**, not active instances. `stores.GetStore(name)` calls the factory for you and returns the instance.

---

### **Configuration Reference**

#### **Store Configuration**

| Field | Description | Required |
|-------|-------------|----------|
| `name` | Store identifier used in the registry key. | **Yes** |
| `package` | Go package name for generated code. | **Yes** |
| `output_dir` | Directory path where files will be generated. | Optional (defaults to `stores/<name>`) |
| `interface` | Interface name — **recommended: `<Name>Store`** (e.g., `UserStore`). | Optional (defaults to `<Name>Store`) |
| `implementation` | Private struct name for the implementation (e.g., `userStore`). | Optional (defaults to `<name>Store`) |
| `queries` | List of database queries. | Optional |

> **⚠️ Naming Convention:** The registry (`stores/all.go`) uses a hardcoded `<Name>Store` pattern when generating constructor calls (e.g., `NewUserStore()`). Always name your interface as `<Name>Store` to avoid compilation errors.

#### **Query Types**

* **`select`** — SELECT queries.
* **`insert`** — INSERT queries.
* **`update`** — UPDATE queries.
* **`delete`** — DELETE queries.

#### **Return Types**

* **`single`** — Returns `(Model, error)`.
* **`multiple`** — Returns `([]Model, error)`.
* **`count`** — Returns `(int64, error)`.
* **`custom`** — Returns `(any, error)`.

#### **Query Parameters**

```yaml
params:
  - name: "id"
    type: "int64"
  - name: "email"
    type: "string"
```

Supported parameter types include all Go primitive types, `time.Time`, and pointer types (e.g., `*int64`).

---

### **Model Generation**

#### **Generate New Models**

```yaml
models:
  - name: "User"
    fields:
      - name: "ID"
        type: "int64"
        tag: 'db:"id" json:"id"'
      - name: "Name"
        type: "string"
        tag: 'db:"name" json:"name"'
      - name: "CreatedAt"
        type: "time.Time"
        tag: 'db:"created_at" json:"created_at"'
```

This generates:
```go
type User struct {
    ID        int64     `db:"id" json:"id"`
    Name      string    `db:"name" json:"name"`
    CreatedAt time.Time `db:"created_at" json:"created_at"`
}

func (User) TableName() string {
    return "user"
}
```

#### **Reference Existing Models**

If you already have models defined elsewhere:

```yaml
models:
  - name: "User"
    path: "../models/user.go"
    package: "your-project/models"
```

---

### **Generated Code Structure**

#### **Interface (`interface.go`)**

```go
// Code generated by gofr.dev/cli/gofr. DO NOT EDIT.
package user

import "gofr.dev/pkg/gofr"

type UserStore interface {
    GetUserByID(ctx *gofr.Context, id int64) (User, error)
    GetAllUsers(ctx *gofr.Context) ([]User, error)
}
```

#### **Implementation (`userStore.go`)**

```go
// Code generated by gofr.dev/cli/gofr.
package user

type userStore struct{}

func NewUserStore() UserStore {
    return &userStore{}
}

func (s *userStore) GetUserByID(ctx *gofr.Context, id int64) (User, error) {
    // TODO: Implement using ctx.SQL()
    var result User
    // err := ctx.SQL().QueryRowContext(ctx, sql, id).Scan(&result.ID, ...)
    return result, nil
}

func (s *userStore) GetAllUsers(ctx *gofr.Context) ([]User, error) {
    // TODO: Implement using ctx.SQL()
    return []User{}, nil
}
```

---

### **Best Practices**

1. **Implement the TODOs**: The generator creates method **signatures and boilerplate only**. You must fill in the `// TODO: Implement` sections with actual SQL execution using `ctx.SQL()` methods.
2. **Use `<Name>Store` Interface Names**: The registry assumes this convention. E.g., `interface: "UserStore"` results in the constructor `NewUserStore()` and type assertion `.(user.UserStore)`.
3. **One YAML, Many Stores**: Define all your stores in a single `store.yaml` to keep your data access layer centrally configured.
4. **Know Which Files Are Auto-Generated**: Only `interface.go` and `all.go` are marked `DO NOT EDIT` and are overwritten on every `gofr store generate`. The implementation stub (`<name>.go`) created by `gofr store init` is editable — this is where you add your SQL logic. The `userStore.go` generated by `gofr store generate` is also editable boilerplate.
5. **Version Control**: Always commit your `store.yaml`. Re-run `gofr store generate` after any configuration change to sync the generated interfaces.

---

### **Complete Example**

For a complete working example of the store generator, see the [store example](https://github.com/gofr-dev/gofr-cli/tree/main/store/example.yaml) in the gofr-cli repository.

For detailed configuration options and advanced usage, refer to the [Store Generator README](https://github.com/gofr-dev/gofr-cli/blob/main/store/README.md).
