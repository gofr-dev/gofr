# HTTP Authentication
Authentication is a crucial aspect of web applications, controlling access to resources based on user roles or permissions. 
Authentication is the process of verifying a user's identity to grant access to protected resources. It ensures only
authorized users can perform certain actions or access sensitive data within an application.

GoFr offer various approaches to implement authorization.

## 1. HTTP Basic Auth
*Basic Authentication* is a simple HTTP authentication scheme where the user's credentials (username and password) are 
transmitted in the request header in a Base64-encoded format.

Basic auth is the simplest way to authenticate your APIs.  It's built on
[HTTP protocol authentication scheme](https://datatracker.ietf.org/doc/html/rfc7617). It involves sending the term 
`Basic` trailed by the Base64-encoded `<username>:<password>` within the standard `Authorization` header.

### Basic Authentication in GoFr

GoFr offers two ways to implement basic authentication:

**1. Predefined Credentials**

Use `EnableBasicAuth(username, password)` to configure Gofr with pre-defined credentials.

```go
func main() {
	app := gofr.New()
    
	app.EnableBasicAuth("admin", "secret_password") // Replace with your credentials
    
	app.GET("/protected-resource", func(c *gofr.Context) (interface{}, error) {
		// Handle protected resource access 
		return nil, nil
	})
	
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

This code snippet demonstrates how to add basic authentication to an HTTP service in GoFr and make a request with the appropriate Authorization header:

```go
app.AddHTTPService("cat-facts", "https://catfact.ninja",
    &service.Authentication{UserName: "abc", Password: "pass"},
)
```


## 2. API Keys Auth
Users include a unique API key in the request header for validation against a store of authorized keys.

### Usage:
GoFr offers two ways to implement API Keys authentication.

**1. Framework Default Validation**
- Users can select the framework's default validation using **_EnableAPIKeyAuth(apiKeys ...string)_**

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

**2. Custom Validation Function**
- Users can create their own validator function `apiKeyValidator(apiKey string) bool` for validating APIKeys and pass the func in **_EnableAPIKeyAuthWithFunc(validator)_**

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

### Adding Basic Authentication to HTTP Services
This code snippet demonstrates how to add API Key authentication to an HTTP service in GoFr and make a request with the appropriate Authorization header:

```go
app.AddHTTPService("http-server-using-redis", "http://localhost:8000", &service.APIKeyAuth{APIKey: "9221e451-451f-4cd6-a23d-2b2d3adea9cf"})
```
