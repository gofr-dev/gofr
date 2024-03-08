# Authorization Support

Authorization is a crucial aspect of web applications, controlling access to resources based on user roles or permissions.
GoFr offer various approaches to implement authorization.

## 1. HTTP Basic Auth
Users provide credentials in the request header for verification.

### Usage:
Users need to implement **_ValidateUser(username, password string) bool_** method on a defined type. The method will be responsible for the
validations of username and password.

```go
package main

type UserPassValidator struct {}

func (v UserPassValidator) ValidateUser(username, password string) bool {
	if username == "test-user" && password == "valid-pass" {
		return true
	}

	return false
}

func main() {
	// initialise gofr object
	app := gofr.New()

	app.BasicAuth(UserPassValidator{})

	app.GET("/customer", Customer)

	app.Run()
}
```

## 2. API Keys
Users include a unique API key in the request header for validation against a store of authorized keys.

### Usage:
Users need to implement **_ValidateKey(apiKey string) bool_** method on a defined type. The method will be responsible for the
validations of API key.

```go
package main

type APIKeyValidator struct{}

func (v APIKeyValidator) ValidateKey(apiKey string) bool {
	if apiKey == "testing-api-key" {
		return true
	}

	return false
}

func main() {
	// initialise gofr object
	app := gofr.New()

	app.APIKeyAuth(APIKeyValidator{})

	app.GET("/customer", Customer)

	app.Run()
}
```

### NOTE:
To add a downstream service with auth enabled, user can pass the auth as options in `AddHTTPService` function.

**Example:**
```go
app.AddHTTPService("http-server-using-redis", "http://localhost:8000", &service.APIKeyAuth{APIKey: "testing-api-key"})
```
