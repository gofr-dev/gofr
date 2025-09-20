# HTTP Auth Middleware

This GoFr example demonstrates the usage of auth middlewares in Gofr. Gofr supports the following auth middlewares out of the box:
- API Key Auth
- Basic Auth
- OAuth

## Setup
User can enable requisite auth middleware by adding the respective code snippet

### Basic Auth Middleware Setup

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
    return username == "username" && password == "password" 
	
    // Alternatively, get the expected username/password from any storage and validate
    //expectedPassword, err := c.KVStore.Get(context.Background(), username)
    //if err != nil || expectedPassword != password {
    //	return false
    //}
    //return true
})
```

### API Key Auth Middleware Setup

```go
a := gofr.New()
// OPTION 1
apiKeyProvider, err := middleware.NewAPIKeyAuthProvider([]string{"valid-key-1", "valid-key-2"})
// handle error - typically caused by invalid configuration
apiKeyMiddleware := middleware.AuthMiddleware(apiKeyProvider)
a.UseMiddleware(apiKeyMiddleware)

// OR

// OPTION 2
a.EnableAPIKeyAuthWithValidator(func(c *container.Container, apiKey string) bool {
		// basic validation based on fixed set of credentials
		return apiKey == "valid-api-key"

		// Alternatively, get the expected APIKey from any storage and validate
		//data, err := c.KVStore.Get(context.Background(), apiKey)
		//if err != nil || data == "" {
		//	return false
		//}
		//return true
	})
```

### OAuth Middleware Setup

```go
a := gofr.New()
a.EnableOAuth("<JWKS-Endpoint>", 10)
```

## Execution:
- Enable the desired auth middleware (main.go)
- Run the example using below command :

```console
go run main.go
```

- Call the API on `localhost:8000/test-auth` with credentials in the Auth header   