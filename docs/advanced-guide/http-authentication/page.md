# HTTP Authentication

Authentication is a crucial aspect of web applications, controlling access to resources based on user roles or permissions. 
It is the process of verifying a user's identity to grant access to protected resources. It ensures that only authenticated
users can perform actions or access data within an application.

GoFr offers various approaches to implement authorization.

## 1. HTTP Basic Auth
*Basic Authentication* is a simple HTTP authentication scheme where the user's credentials (username and password) are 
transmitted in the request header in a Base64-encoded format.

Basic auth is the simplest way to authenticate your APIs. It's built on
{% new-tab-link title="HTTP protocol authentication scheme" href="https://datatracker.ietf.org/doc/html/rfc7617" /%}.
It involves sending the prefix `Basic` trailed by the Base64-encoded `<username>:<password>` within the standard `Authorization` header.

### Basic Authentication in GoFr

GoFr offers two ways to implement basic authentication:

**1. Predefined Credentials**

Use `EnableBasicAuth(username, password)` to configure GoFr with pre-defined credentials.

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

Use `EnableBasicAuthWithValidator(validationFunc)` to implement your own validation logic for credentials.
The `validationFunc` takes the username and password as arguments and returns true if valid, false otherwise.

```go
func validateUser(c *container.Container, username, password string) bool {
	// Implement your credential validation logic here 
	// This example uses hardcoded credentials for illustration only   
	return username == "john" && password == "doe123" 
} 

func main() { 
	app := gofr.New() 

	app.EnableBasicAuthWithValidator(validateUser) 

	app.GET("/secure-data", func(c *gofr.Context) (interface{}, error) { 
		// Handle access to secure data 
		return nil, nil
	})

	app.Run()
}
```

### Adding Basic Authentication to HTTP Services
This code snippet demonstrates how to add basic authentication to an HTTP service in GoFr and make a request with the appropriate Authorization header:

```go
app.AddHTTPService("order", "https://localhost:2000",
    &service.Authentication{UserName: "abc", Password: "pass"},
)
```

## 2. API Keys Auth
*API Key Authentication* is an HTTP authentication scheme where a unique API key is included in the request header for validation against a store of authorized keys.

### Usage:
GoFr offers two ways to implement API Keys authentication.

**1. Framework Default Validation**
- GoFr's default validation can be selected using **_EnableAPIKeyAuth(apiKeys ...string)_**

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
- GoFr allows a custom validator function `apiKeyValidator(apiKey string) bool` for validating APIKeys and pass the func in **_EnableAPIKeyAuthWithValidator(validator)_**

```go
package main

func apiKeyValidator(c *container.Container, apiKey string) bool {
	validKeys := []string{"f0e1dffd-0ff0-4ac8-92a3-22d44a1464e4", "d7e4b46e-5b04-47b2-836c-2c7c91250f40"}

	return slices.Contains(validKeys, apiKey)
}

func main() {
	// initialise gofr object
	app := gofr.New()

	app.EnableAPIKeyAuthWithValidator(apiKeyValidator)

	app.GET("/customer", Customer)

	app.Run()
}
```

### Adding API-KEY Authentication to HTTP Services
This code snippet demonstrates how to add API Key authentication to an HTTP service in GoFr and make a request with the appropriate Authorization header:

```go
app.AddHTTPService("http-server-using-redis", "http://localhost:8000", &service.APIKeyConfig{APIKey: "9221e451-451f-4cd6-a23d-2b2d3adea9cf"})
```

## 3. OAuth 2.0
{% new-tab-link title="OAuth" href="https://www.rfc-editor.org/rfc/rfc6749" /%} 2.0 is the industry-standard protocol for authorization. 
It focuses on client developer simplicity while providing specific authorization flows for web applications, desktop applications, mobile phones, and living room devices.

It involves sending the prefix `Bearer` trailed by the encoded token within the standard `Authorization` header.

### OAuth Authentication in GoFr

GoFr supports authenticating tokens encoded by algorithm `RS256/384/512`. 

### App level Authentication
Enable OAuth 2.0 with three-legged flow to authenticate requests

Use `EnableOAuth(jwks-endpoint,refresh_interval)` to configure GoFr with pre-defined credentials.

```go
func main() {
	app := gofr.New()

	app.EnableOAuth("http://jwks-endpoint", 20) 
    
	app.GET("/protected-resource", func(c *gofr.Context) (interface{}, error) {
		// Handle protected resource access 
		return nil, nil
	})

	app.Run()
}
```

### Adding OAuth Authentication to HTTP Services
For server-to-server communication it follows two-legged OAuth, also known as "client credentials" flow,
where the client application directly exchanges its own credentials (ClientID and ClientSecret)
for an access token without involving any end-user interaction.

This code snippet demonstrates how two-legged OAuth authentication is added to an HTTP service in GoFr and make a request with the appropriate Authorization header:

```go
app.AddHTTPService("orders", "http://localhost:9000",
    &service.OAuthConfig{   // Replace with your credentials
        ClientID:     "0iyeGcLYWudLGqZfD6HvOdZHZ5TlciAJ",
        ClientSecret: "GQXTY2f9186nUS3C9WWi7eJz8-iVEsxq7lKxdjfhOJbsEPPtEszL3AxFn8k_NAER",
        TokenURL:     "https://dev-zq6tvaxf3v7p0g7j.us.auth0.com/oauth/token",
        Scopes:       []string{"read:order"},
        EndpointParams: map[string][]string{
            "audience": {"https://dev-zq6tvaxf3v7p0g7j.us.auth0.com/api/v2/"},
    },
})
```
