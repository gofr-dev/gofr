# Authorization Support
Authorization is a crucial aspect of web applications, controlling access to resources based on user roles or permissions. 
Authentication is the process of verifying a user's identity to grant access to protected resources. It ensures only
authorized users can perform certain actions or access sensitive data within an application.

GoFr offer various approaches to implement authorization.

## 1. HTTP Basic Auth
*Basic Authentication* is a simple HTTP authentication scheme where the user's credentials (username and password) are 
transmitted in the request header in a Base64-encoded format.

Basic auth is the simplest way to authenticate your APIs.  It's built on
[HTTP protocol authentication scheme](https://datatracker.ietf.org/doc/html/rfc7617). It involves sending the term 
`Basic` trailed by the Base64-encoded `<username>:<password>` within the standard `Authorization` header.

### Basic Authentication in Gofr

Gofr offers two ways to implement basic authentication:

**1. Predefined Credentials**

Use `EnableBasicAuth(username, password)` to configure Gofr with pre-defined credentials.

```go
func main() { 
	app := gofr.New() 
	app.EnableBasicAuth("admin", "secret_password") // Replace with your credentials 
	app.GET("/protected-resource", func(c *gofer.Context) error { 
		// Handle protected resource access  
	return  nil }) 
	app.Run() 
}
```

**2. Custom Validation Function**

Use `EnableBasicAuthWithFunc(validationFunc)` to implement your own validation logic for credentials. The `validationFunc` takes the username and password as arguments and returns true if valid, false otherwise.

```go
func validateUser(username string, password string) bool { 
// Implement your credential validation logic here  
// This example uses hardcoded credentials for illustration only   
	return username == "john" && password == "doe123" 
} 

func main() { 
	app := gofr.New() 
	
	app.EnableBasicAuthWithFunc(validateUser) 

	app.GET("/secure-data", func(c *gofer.Context) error { 
		// Handle access to secure data  
	return  nil }) 
	app.Run()
}
```

### Adding Basic Authentication to HTTP Services

This code snippet demonstrates how to add basic authentication to an HTTP service in Gofr and make a request with the appropriate Authorization header:

```go
a.AddHTTPService("cat-facts", "https://catfact.ninja",
    &service.Authentication{UserName: "abc", Password: "pass"},
)
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
