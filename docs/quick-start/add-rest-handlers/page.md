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

**NOTE**: The registered routes will have the same name as the given struct, but if you want to change route name, you can implement `RestPath` method in the struct:
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
func (u *user) GetAll(c *gofr.Context) (interface{}, error) {
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

In this example, we define a user struct representing a database entity. The GetAll method in the provided code demonstrates how to override the default behavior for retrieving all entities.
This method can be used to implement custom logic for filtering, sorting, or retrieving additional data along with the entities.


> Few Points to consider:
> 1. Field Naming Convention: GoFr assumes the struct fields in snake-case match the database column names. For example, `IsEmployed` field in the struct matches `is_employed` column in the database, `Age` field matches `age` column, etc.
> 2. Primary Key: The first field of the struct is typically used as the primary key for data operations. However, user can customize this behavior using GoFr's features. 
