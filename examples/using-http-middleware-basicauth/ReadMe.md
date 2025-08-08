# HTTP Middleware - Basic Auth

This GoFr example demonstrates the usage of basic auth middleware in Gofr

User can create a basic auth middleware using the following snippet 

```go
a := gofr.New()

// OPTION 1
basicAuthProvider, err := middleware.NewBasicAuthProvider(map[string]string{"username": "password"})
// handle error - typically caused by invalid configuration
basicAuthMiddleware := middleware.AuthMiddleware(basicAuthProvider)
a.UseMiddleware(basicAuthMiddleware)

// OR

// OPTION 2
a.EnableBasicAuthWithValidator(func(c *container.Container, username, password string) bool {
	// basic validation based on fixed set of credentials 
    if username == "username" && password == "password" {
        return true
    }
    return false
    // Alternatively, get the expected username/password from any storage and validate
    //expectedPassword, err := c.KVStore.Get(context.Background(), username)
    //if err != nil || expectedPassword != password {
    //	return false
    //}
    //return true
})
```


### To run the example follow the below steps:
- Update the credentials in the example (main.go)
- Run the example using below command :

```console
go run main.go
```

- Call the API on `localhost:8000/test-basic-auth` with credentials in the Basic Auth header   