# Authentication

Authentication is a crucial aspect of web applications, controlling access to resources based on user roles or permissions. 
It is the process of verifying a user's identity to grant access to protected resources. It ensures that only authenticated
users can perform actions or access data within an application.

GoFr offers a **Unified Authentication** model, meaning that once you enable an authentication method, it automatically 
applies to both your HTTP and gRPC services.

## Exempted Paths

By default, the authentication middleware exempts the following paths from authentication:

- `/.well-known/alive`: Used for liveness probes, should be publicly accessible for health checks.

The health check endpoint `/.well-known/health` is exempted by default, but as it may contain sensitive information about the service and its dependencies, it is recommended to require authentication for it.

## 1. Basic Auth
*Basic Authentication* is a simple authentication scheme where the user's credentials (username and password) are 
transmitted in the request header in a Base64-encoded format.

Basic auth is the simplest way to authenticate your APIs. It's built on
{% new-tab-link title="HTTP protocol authentication scheme" href="https://datatracker.ietf.org/doc/html/rfc7617" /%}.
It involves sending the prefix `Basic` trailed by the Base64-encoded `<username>:<password>` within the standard `Authorization` header.

### Usage in GoFr

GoFr offers two ways to implement basic authentication:

**1. Predefined Credentials**

Use `EnableBasicAuth(username, password)` to configure GoFr with pre-defined credentials.

```go
func main() {
	app := gofr.New()

	app.EnableBasicAuth("admin", "secret_password") // Replace with your credentials

	app.GET("/protected-resource", func(c *gofr.Context) (any, error) {
		return "Success", nil
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
	return username == "john" && password == "doe123"
}

func main() {
	app := gofr.New()

	app.EnableBasicAuthWithValidator(validateUser)

	app.Run()
}
```

## 2. API Keys Auth
*API Key Authentication* is an authentication scheme where a unique API key is included in the request header `X-Api-Key` for validation against a store of authorized keys.

### Usage in GoFr

GoFr offers two ways to implement API Keys authentication.

**1. Framework Default Validation**
- GoFr's default validation can be selected using **_EnableAPIKeyAuth(apiKeys ...string)_**

```go
func main() {
	app := gofr.New()

	app.EnableAPIKeyAuth("9221e451-451f-4cd6-a23d-2b2d3adea9cf", "0d98ecfe-4677-48aa-b463-d43505766915")

	app.Run()
}
```

**2. Custom Validation Function**
- GoFr allows a custom validator function for validating APIKeys using **_EnableAPIKeyAuthWithValidator(validator)_**

```go
func apiKeyValidator(c *container.Container, apiKey string) bool {
	validKeys := []string{"f0e1dffd-0ff0-4ac8-92a3-22d44a1464e4"}

	return slices.Contains(validKeys, apiKey)
}

func main() {
	app := gofr.New()

	app.EnableAPIKeyAuthWithValidator(apiKeyValidator)

	app.Run()
}
```

## 3. OAuth 2.0
{% new-tab-link title="OAuth" href="https://www.rfc-editor.org/rfc/rfc6749" /%} 2.0 is the industry-standard protocol for authorization. 
It involves sending the prefix `Bearer` trailed by the encoded token within the standard `Authorization` header.

### Usage in GoFr

Enable OAuth 2.0 to authenticate requests. Use `EnableOAuth(jwks-endpoint, refresh_interval, options ...jwt.ParserOption)` to configure GoFr.

```go
func main() {
	app := gofr.New()

	app.EnableOAuth("http://jwks-endpoint", 3600)

	app.Run()
}
```

### Available JWT Claim Validations

- **Expiration (`exp`)**: Validated by default if present. Use `jwt.WithExpirationRequired()` to make it mandatory.
- **Audience (`aud`)**: `jwt.WithAudience("https://api.example.com")`
- **Issuer (`iss`)**: `jwt.WithIssuer("https://auth.example.com")`
- **Subject (`sub`)**: `jwt.WithSubject("user@example.com")`

## Accessing Auth Info in Handlers

Once authenticated, you can retrieve the authentication information from the context using the `GetAuthInfo()` method. This works identically for both HTTP and gRPC handlers.

```go
func MyHandler(ctx *gofr.Context) (any, error) {
    authInfo := ctx.GetAuthInfo()

    // For Basic Auth
    username := authInfo.GetUsername()
    
    // For API Key
    apiKey := authInfo.GetAPIKey()

    // For OAuth
    claims := authInfo.GetClaims()
    
    return "Success", nil
}
```

## Security Best Practices

*   **Timing Attacks**: GoFr's Basic Auth and API Key interceptors use `subtle.ConstantTimeCompare` to prevent timing attacks.
*   **TLS**: Always use TLS in production to encrypt the authentication credentials and tokens transmitted over the network.
