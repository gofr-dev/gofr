# File Token Auth Example

This GoFr example demonstrates file-based bearer token authentication with automatic token rotation,
designed for Kubernetes service account tokens.

The token is read from a file and periodically refreshed in the background, so rotated tokens
are picked up automatically.

### Usage Options

There are two ways to use `auth.NewFileTokenAuthConfig`:

#### Option 1: Pass as an option to `AddHTTPService`

Logger and metrics are injected automatically via the `Observable` interface.

```go
tokenAuth, err := auth.NewFileTokenAuthConfig(
    file.NewLocalFileSystem(logger),
    tokenPath,
    30*time.Second,
)

a.AddHTTPService("k8s-api", "https://kubernetes.default.svc", tokenAuth)
```

#### Option 2: Call `AddOption` directly on an existing HTTP service

When using `service.NewHTTPService` directly, logger and metrics must be set manually.

```go
tokenAuth, err := auth.NewFileTokenAuthConfig(
    file.NewLocalFileSystem(logger),
    tokenPath,
    30*time.Second,
)

svc := service.NewHTTPService("https://api.example.com", logger, metrics)
tokenAuth.(service.Observable).UseLogger(logger)
tokenAuth.(service.Observable).UseMetrics(metrics)

svc = tokenAuth.AddOption(svc)
```

### To run the example, follow the steps below:

The default token file path is `/var/run/secrets/kubernetes.io/serviceaccount/token` (the standard
Kubernetes projected service account token mount). To override it, set `FILE_TOKEN_PATH` in
`configs/.env`:

```
FILE_TOKEN_PATH=/path/to/your/token
```

Then run the example:

```console
go run main.go
```
