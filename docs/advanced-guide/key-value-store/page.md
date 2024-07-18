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

func Post(ctx *gofr.Context) (interface{}, error) {
	err := ctx.KVStore.Set(ctx, "name", "gofr")
	if err != nil {
		return nil, err
	}

	return "Insertion to Key Value Store Successful", nil
}

func Get(ctx *gofr.Context) (interface{}, error) {
	value, err := ctx.KVStore.Get(ctx, "name")
	if err != nil {
		return nil, err
	}

	return value, nil
}

func Delete(ctx *gofr.Context) (interface{}, error) {
	err := ctx.KVStore.Delete(ctx, "name")
	if err != nil {
		return nil, err
	}

	return fmt.Sprintf("Deleted Successfully key %v from Key-Value Store", "name"), nil
}
```
