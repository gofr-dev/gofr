# Key Value Store

A key-value store is a type of NoSQL database that uses a simple data model: each item is stored as a pair consisting of a unique key and a value.
This simplicity offers high performance and scalability, making key-value stores ideal for applications requiring fast and efficient data retrieval and storage.

GoFr supports BadgerDB as a key value store. Support for other key-value store will be added in the future.

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





