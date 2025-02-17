# Add REST Handlers

GoFr simplifies the process of implementing CRUD (Create, Read, Update, Delete) operations by enabling the automatic generation of handlers directly from Go structs.
This feature eliminates the need for writing repetitive boilerplate code, allowing developers to focus on application logic.

## Default Behaviour

If the custom handlers ain't implemented on the struct, GoFr provides default handlers for each CRUD operation. These handlers handle basic database interactions:

- **Create**: `/entity` Inserts a new record based on data provided in a JSON request body.
- **Read**:
  - **GET**:  `/entity` Retrieves all entities of the type specified by the struct.
  - **GET**:  `/entity/{id}` Retrieves a specific entity identified by the {id} path parameter.
- **Update**: `/entity/{id}` Updates an existing record identified by the {id} path parameter, based on data provided in a JSON request body.
- **Delete**  `/entity/{id}` Deletes an existing record identified by the {id} path parameter.

**NOTE**: The registered routes will have the same name as the given struct, but if we want to change route name, we can implement `RestPath` method in the struct:
```go
type userEntity struct {
	Id         int    `json:"id"`
	Name       string `json:"name"`
	Age        int    `json:"age"`
	IsEmployed bool   `json:"isEmployed"`
}

func (u *userEntity) RestPath() string {
	return "users"
}
```

## Overriding Default Handlers

While the default handlers provide basic functionality, user might want to customize their behavior for specific use cases. 
The AddRESTHandlers feature allows user to override these handlers by implementing methods within the struct itself.

## Database Table Name
By default, GoFr assumes the struct name in snake-case matches the database table name for querying data. For example, `UserEntity` struct matches `user_entity` database table, `cardConfig` struct matches `card_config` database table, etc.
To change table name, you need to implement `TableName` method in the struct:
```go
type userEntity struct {
	Id         int    `json:"id"`
	Name       string `json:"name"`
	Age        int    `json:"age"`
	IsEmployed bool   `json:"isEmployed"`
}

func (u *userEntity) TableName() string {
	return "user"
}
```

## Adding Database Constraints
By default, GoFr assumes to have manual insertion of id for a given struct, but to support SQL constraints like `auto-increment`,
`not-null` user can use the `sql` tag while declaring the struct fields.

```go
type user struct {
	ID         int    `json:"id"  sql:"auto_increment"`
	Name       string `json:"name"  sql:"not_null"`
	Age        int    `json:"age"`
	IsEmployed bool   `json:"isEmployed"`
}
```

Now when posting data for the user struct, the `Id` we be auto-incremented and the `Name` will be a not-null field in table.

## Benefits of Adding REST Handlers of GoFr

1. Reduced Boilerplate Code: Eliminate repetitive code for CRUD operations, freeing user to focus on core application logic.
2. Consistency: Ensures consistency in CRUD operations across different entities by using a standardized approach.
3. Flexibility: Allows developers to customize CRUD behavior as per application requirements, providing flexibility and extensibility.

## Example

```go
package main

import (
	"gofr.dev/examples/using-crud-from-struct/migrations"
	"gofr.dev/pkg/gofr"
)

type user struct {
	Id         int    `json:"id"`
	Name       string `json:"name"`
	Age        int    `json:"age"`
	IsEmployed bool   `json:"isEmployed"`
}

// GetAll : User can overwrite the specific handlers by implementing them like this
func (u *user) GetAll(c *gofr.Context) (any, error) {
	return "user GetAll called", nil
}

func main() {
	// Create a new application
	a := gofr.New()

	// Add migrations to run
	a.Migrate(migrations.All())

	// AddRESTHandlers creates CRUD handles for the given entity
	err := a.AddRESTHandlers(&user{})
	if err != nil {
		return
	}

	// Run the application
	a.Run()
}
```

In this example, we define a user struct representing a database entity. The `GetAll` method in the provided code demonstrates how to override the default behavior for retrieving all entities.
This method can be used to implement custom logic for filtering, sorting, or retrieving additional data along with the entities.


## Few Points to Consider:

**1. Passing Struct by Reference**

The struct should always be passed by reference in the method `AddRESTHandlers`.

**2. Field Naming Convention**

GoFr assumes that struct fields in snake_case match the database column names.

* For example, the `IsEmployed` field in the struct matches the `is_employed` column in the database.
* Similarly, the `Age` field matches the `age` column.

**3. Primary Key**

The first field of the struct is typically used as the primary key for data operations. However, this behavior can be customized using GoFr's features.

**4. Datatype Conversions**

| Go Type | SQL Type | Description |
|---|---|---|
| `uuid.UUID` (from `github.com/google/uuid` or `github.com/satori/go.uuid`) | `CHAR(36)` or `VARCHAR(36)` | UUIDs are typically stored as 36-character strings in SQL databases. |
| `string` | `VARCHAR(n)` or `TEXT` | Use `VARCHAR(n)` for fixed-length strings, while `TEXT` is for longer, variable-length strings. |
| `int`, `int32`, `int64`, `uint`, `uint32`, `uint64` | `INT`, `BIGINT`, `SMALLINT`, `TINYINT`, `INTEGER` | Use `INT` for general integer values, `BIGINT` for large values, and `SMALLINT` or `TINYINT` for smaller ranges. |
| `bool` | `BOOLEAN` or `TINYINT(1)` | Use `BOOLEAN` (supported by most SQL databases like PostgreSQL, MySQL) or `TINYINT(1)` in MySQL (where `0` is false, and `1` is true). |
| `float32`, `float64` | `FLOAT`, `DOUBLE`, `DECIMAL` | Use `DECIMAL` for precise decimal numbers (e.g., financial data), `FLOAT` or `DOUBLE` for approximate floating-point numbers. |
| `time.Time` | `DATE`, `TIME`, `DATETIME`, `TIMESTAMP` | Use `DATE` for just the date, `TIME` for the time of day, and `DATETIME` or `TIMESTAMP` for both date and time. |
> #### Check out the example on how to add REST Handlers in GoFr: [Visit GitHub](https://github.com/gofr-dev/gofr/tree/main/examples/using-add-rest-handlers)