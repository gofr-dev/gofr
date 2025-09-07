# Key Value Store

A key-value store is a type of NoSQL database that uses a simple data model: each item is stored as a pair consisting of a unique key and a value.
This simplicity offers high performance and scalability, making key-value stores ideal for applications requiring fast and efficient data retrieval and storage.

GoFr supports multiple key-value stores including BadgerDB, NATS-KV, and DynamoDB. Support for other key-value stores will be added in the future.

Keeping in mind the size of the application in the final build, it felt counter-productive to keep the drivers within
the framework itself. GoFr provide the following functionalities for its key-value store.

```go
type KVStore interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	Delete(ctx context.Context, key string) error
}
```

## BadgerDB
GoFr supports injecting BadgerDB that supports the following interface. Any driver that implements the interface can be added
using `app.AddKVStore()` method, and user's can use BadgerDB across application with `gofr.Context`.

User's can easily inject a driver that supports this interface, this provides usability without
compromising the extensibility to use multiple databases.

Import the gofr's external driver for BadgerDB:

```go
go get gofr.dev/pkg/gofr/datasource/kv-store/badger
```

### Example
```go
package main

import (
	"fmt"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/kv-store/badger"
)

type User struct {
	ID   string
	Name string
	Age  string
}

func main() {
	app := gofr.New()

	app.AddKVStore(badger.New(badger.Configs{DirPath: "badger-example"}))

	app.POST("/user", Post)
	app.GET("/user", Get)
	app.DELETE("/user", Delete)

	app.Run()
}

func Post(ctx *gofr.Context) (any, error) {
	err := ctx.KVStore.Set(ctx, "name", "gofr")
	if err != nil {
		return nil, err
	}

	return "Insertion to Key Value Store Successful", nil
}

func Get(ctx *gofr.Context) (any, error) {
	value, err := ctx.KVStore.Get(ctx, "name")
	if err != nil {
		return nil, err
	}

	return value, nil
}

func Delete(ctx *gofr.Context) (any, error) {
	err := ctx.KVStore.Delete(ctx, "name")
	if err != nil {
		return nil, err
	}

	return fmt.Sprintf("Deleted Successfully key %v from Key-Value Store", "name"), nil
}
```
## NATS-KV
GoFr supports injecting NATS-KV that supports the above KVStore interface. Any driver that implements the interface can be added
using `app.AddKVStore()` method, and user's can use NATS-KV across application with `gofr.Context`.

User's can easily inject a driver that supports this interface, this provides usability without
compromising the extensibility to use multiple databases.

Import the gofr's external driver for NATS-KV:

```go
go get gofr.dev/pkg/gofr/datasource/kv-store/nats
```
### Example
```go
package main

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/kv-store/nats"
	"gofr.dev/pkg/gofr/http"
)

type Person struct {
	ID    string `json:"id,omitempty"`
	Name  string `json:"name"`
	Age   int    `json:"age"`
	Email string `json:"email,omitempty"`
}

func main() {
	app := gofr.New()

	app.AddKVStore(nats.New(nats.Configs{
		Server: "nats://localhost:4222",
		Bucket: "persons",
	}))

	app.POST("/person", CreatePerson)
	app.GET("/person/{id}", GetPerson)
	app.PUT("/person/{id}", UpdatePerson)
	app.DELETE("/person/{id}", DeletePerson)

	app.Run()
}

func CreatePerson(ctx *gofr.Context) (any, error) {
	var person Person
	if err := ctx.Bind(&person); err != nil {
		return nil, http.ErrorInvalidParam{Params: []string{"body"}}
	}

	person.ID = uuid.New().String()
	personData, err := json.Marshal(person)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize person")
	}

	if err := ctx.KVStore.Set(ctx, person.ID, string(personData)); err != nil {
		return nil, err
	}

	return person, nil
}

func GetPerson(ctx *gofr.Context) (any, error) {
	id := ctx.PathParam("id")
	if id == "" {
		return nil, http.ErrorInvalidParam{Params: []string{"id"}}
	}

	value, err := ctx.KVStore.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("person not found")
	}

	var person Person
	if err := json.Unmarshal([]byte(value), &person); err != nil {
		return nil, fmt.Errorf("failed to parse person data")
	}

	return person, nil
}

func UpdatePerson(ctx *gofr.Context) (any, error) {
	id := ctx.PathParam("id")
	if id == "" {
		return nil, http.ErrorInvalidParam{Params: []string{"id"}}
	}

	var person Person
	if err := ctx.Bind(&person); err != nil {
		return nil, http.ErrorInvalidParam{Params: []string{"body"}}
	}

	person.ID = id
	personData, err := json.Marshal(person)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize person")
	}

	if err := ctx.KVStore.Set(ctx, id, string(personData)); err != nil {
		return nil, err
	}

	return person, nil
}

func DeletePerson(ctx *gofr.Context) (any, error) {
	id := ctx.PathParam("id")
	if id == "" {
		return nil, http.ErrorInvalidParam{Params: []string{"id"}}
	}

	if err := ctx.KVStore.Delete(ctx, id); err != nil {
		return nil, fmt.Errorf("person not found")
	}

	return map[string]string{"message": "Person deleted successfully"}, nil
}
```

## DynamoDB

GoFr supports injecting DynamoDB as a key-value store that implements the standard KVStore interface. Any driver that implements the interface can be added using `app.AddKVStore()` method, and users can use DynamoDB across application with `gofr.Context`.

DynamoDB is a fully managed NoSQL database service that provides fast and predictable performance with seamless scalability. It's ideal for applications that need consistent, single-digit millisecond latency at any scale.

Import the gofr's external driver for DynamoDB:

```shell
go get gofr.dev/pkg/gofr/datasource/kv-store/dynamodb@latest
```

### Configuration

```go
type Configs struct {
    Table            string // DynamoDB table name
    Region           string // AWS region (e.g., "us-east-1")
    Endpoint         string // Leave empty for real AWS; set for local DynamoDB
    PartitionKeyName string // Default is "pk" if not specified
}
```

### Local Development Setup

For local development, you can use DynamoDB Local with Docker:

```bash
# Start DynamoDB Local
docker run --name dynamodb-local -d -p 8000:8000 amazon/dynamodb-local

# Create a table
aws dynamodb create-table \
    --table-name gofr-kv-store \
    --attribute-definitions AttributeName=pk,AttributeType=S \
    --key-schema AttributeName=pk,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST \
    --endpoint-url http://localhost:8000 \
    --region us-east-1
```

### JSON Helper Functions

The DynamoDB package provides helper functions for JSON serialization/deserialization that work with the standard KVStore interface:

```go
// ToJSON converts any struct to JSON string
func ToJSON(value any) (string, error)

// FromJSON converts JSON string to struct
func FromJSON(jsonData string, dest any) error
```

### Example

```go
package main

import (
	"fmt"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/kv-store/dynamodb"
)

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	app := gofr.New()

	// Create DynamoDB client with configuration
	db := dynamodb.New(dynamodb.Configs{
		Table:            "gofr-kv-store",
		Region:           "us-east-1",
		Endpoint:         "http://localhost:8000", // For local DynamoDB
		PartitionKeyName: "pk",
	})

	// Connect to DynamoDB
	db.Connect()

	// Inject the DynamoDB into gofr
	app.AddKVStore(db)

	app.POST("/user", CreateUser)
	app.GET("/user/{id}", GetUser)
	app.PUT("/user/{id}", UpdateUser)
	app.DELETE("/user/{id}", DeleteUser)

	app.Run()
}

func CreateUser(ctx *gofr.Context) (any, error) {
	var user User
	if err := ctx.Bind(&user); err != nil {
		return nil, err
	}

	user.ID = fmt.Sprintf("user_%d", time.Now().UnixNano())
	user.CreatedAt = time.Now()

	// Convert struct to JSON string using helper function
	userData, err := dynamodb.ToJSON(user)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize user: %w", err)
	}

	// Store using standard KVStore interface
	if err := ctx.KVStore.Set(ctx, user.ID, userData); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

func GetUser(ctx *gofr.Context) (any, error) {
	id := ctx.PathParam("id")
	if id == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	// Get JSON string from KVStore
	userData, err := ctx.KVStore.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Convert JSON string to struct using helper function
	var user User
	if err := dynamodb.FromJSON(userData, &user); err != nil {
		return nil, fmt.Errorf("failed to parse user data: %w", err)
	}

	return user, nil
}

func UpdateUser(ctx *gofr.Context) (any, error) {
	id := ctx.PathParam("id")
	if id == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	var user User
	if err := ctx.Bind(&user); err != nil {
		return nil, err
	}

	user.ID = id

	// Convert struct to JSON string using helper function
	userData, err := dynamodb.ToJSON(user)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize user: %w", err)
	}

	// Update in DynamoDB using standard KVStore interface
	if err := ctx.KVStore.Set(ctx, id, userData); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return user, nil
}

func DeleteUser(ctx *gofr.Context) (any, error) {
	id := ctx.PathParam("id")
	if id == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	// Delete from DynamoDB using standard KVStore interface
	if err := ctx.KVStore.Delete(ctx, id); err != nil {
		return nil, fmt.Errorf("failed to delete user: %w", err)
	}

	return map[string]string{"message": "User deleted successfully"}, nil
}
```

### Production Configuration

For production use, remove the `Endpoint` field to connect to real AWS DynamoDB:

```go
db := dynamodb.New(dynamodb.Configs{
    Table:            "gofr-kv-store",
    Region:           "us-east-1",
    // Endpoint: "", // Remove this for production
    PartitionKeyName: "pk",
})
```

### AWS Credentials

For production, ensure your AWS credentials are configured through:
- AWS IAM roles (recommended for EC2/ECS/Lambda)
- AWS credentials file (`~/.aws/credentials`)
- Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)




