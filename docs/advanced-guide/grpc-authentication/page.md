# gRPC Authentication

GoFr provides built-in support for securing your gRPC services using common authentication methods. These interceptors ensure that your gRPC services have the same level of security and parity as your HTTP services.

The authentication interceptors are located in the `gofr.dev/pkg/gofr/grpc/middleware` package.

## 1. gRPC Basic Auth

Basic Authentication validates credentials (username and password) sent in the `authorization` header. The header should follow the format: `Basic <base64-encoded-credentials>`.

### Usage

To enable Basic Auth, use `BasicAuthUnaryInterceptor` for unary calls and `BasicAuthStreamInterceptor` for streaming calls.

```go
import (
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/grpc/middleware"
)

func main() {
    app := gofr.New()

    users := map[string]string{
        "admin": "password123",
    }

    // Add Unary Interceptor
    app.AddGRPCUnaryInterceptors(middleware.BasicAuthUnaryInterceptor(users))
    
    // Add Stream Interceptor
    app.AddGRPCServerStreamInterceptors(middleware.BasicAuthStreamInterceptor(users))

    // Register your gRPC services...
    app.Run()
}
```

## 2. gRPC API Key Auth

API Key Authentication validates a unique key sent in the `x-api-key` header.

### Usage

Use `APIKeyAuthUnaryInterceptor` and `APIKeyAuthStreamInterceptor` to secure your service with one or more valid API keys.

```go
import (
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/grpc/middleware"
)

func main() {
    app := gofr.New()

    validKeys := []string{"key-1", "key-2"}

    // Add Unary Interceptor
    app.AddGRPCUnaryInterceptors(middleware.APIKeyAuthUnaryInterceptor(validKeys...))
    
    // Add Stream Interceptor
    app.AddGRPCServerStreamInterceptors(middleware.APIKeyAuthStreamInterceptor(validKeys...))

    // Register your gRPC services...
    app.Run()
}
```

## 3. gRPC OAuth (JWT)

OAuth authentication validates JWT tokens sent in the `authorization` header with the `Bearer` prefix. It uses a `PublicKeyProvider` to fetch the public keys required for token verification.

### Usage

Use `OAuthUnaryInterceptor` and `OAuthStreamInterceptor`. These interceptors also inject the decoded JWT claims into the context, which can be accessed in your handlers.

```go
import (
    "gofr.dev/pkg/gofr"
    "gofr.dev/pkg/gofr/grpc/middleware"
    httpMiddleware "gofr.dev/pkg/gofr/http/middleware"
)

func main() {
    app := gofr.New()

    // You can use the same PublicKeyProvider used for HTTP OAuth
    keyProvider := httpMiddleware.NewPublicKeyProvider("http://jwks-endpoint", 3600)

    // Add Unary Interceptor
    app.AddGRPCUnaryInterceptors(middleware.OAuthUnaryInterceptor(keyProvider))
    
    // Add Stream Interceptor
    app.AddGRPCServerStreamInterceptors(middleware.OAuthStreamInterceptor(keyProvider))

    // Register your gRPC services...
    app.Run()
}
```

### Accessing Auth Info in Handlers

Once authenticated, you can retrieve the authentication information from the context using the idiomatic `GetAuthInfo()` method, just like in HTTP handlers:

```go
func (s *Server) MyMethod(ctx *gofr.Context, req *MyRequest) (*MyResponse, error) {
    // For OAuth
    claims := ctx.GetAuthInfo().GetClaims()
    
    // For Basic Auth
    username := ctx.GetAuthInfo().GetUsername()
    
    // For API Key
    apiKey := ctx.GetAuthInfo().GetAPIKey()
    
    return &MyResponse{}, nil
}
```

## Security Best Practices

*   **Timing Attacks**: GoFr's Basic Auth and API Key interceptors use `subtle.ConstantTimeCompare` to prevent timing attacks.
*   **TLS**: Always use TLS in production to encrypt the authentication credentials and tokens transmitted over the network. You can enable TLS using `app.AddGRPCServerOptions(grpc.Creds(creds))`.
