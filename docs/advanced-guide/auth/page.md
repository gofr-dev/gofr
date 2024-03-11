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
There are two methods to enable API Keys authentication. 
- User can either select the framework's default validation using **_EnableAPIKeyAuth(apiKeys ...string)_**
```go
package main

func main() {
	// initialise gofr object
	app := gofr.New()

	app.EnableAPIKeyAuth("9221e451-451f-4cd6-a23d-2b2d3adea9cf", "0d98ecfe-4677-48aa-b463-d43505766915")

	app.GET("/customer", Customer)

	app.Run()
}
```

- User can create their own validator function `apiKeyValidator(apiKey string) bool` for validating APIKeys and pass the func in **_EnableAPIKeyAuthWithFunc(validator)_**

```go
package main

func apiKeyValidator(apiKey string) bool {
  validKeys := []string{"f0e1dffd-0ff0-4ac8-92a3-22d44a1464e4", "d7e4b46e-5b04-47b2-836c-2c7c91250f40"}

  return slices.Contains(validKeys, apiKey)
}

func main() {
	// initialise gofr object
	app := gofr.New()

	app.EnableAPIKeyAuthWithFunc(apiKeyValidator)

	app.GET("/customer", Customer)

	app.Run()
}
```

### NOTE:
To add a downstream service with auth enabled, user can pass the auth as options in `AddHTTPService` function.

**Example:**
```go
app.AddHTTPService("http-server-using-redis", "http://localhost:8000", &service.APIKeyAuth{APIKey: "9221e451-451f-4cd6-a23d-2b2d3adea9cf"})
```
