# using-file-token-auth

Demonstrates `auth.FileTokenAuthConfig` — an HTTP service option that reads a
bearer token from a file and periodically re-reads it so token rotation is
picked up automatically. The primary use case is Kubernetes projected service
account tokens, where the kubelet writes a fresh JWT to a mounted file every
few minutes.

## How it works

```go
fs := file.NewLocalFileSystem(app.Logger())

tokenCfg, err := auth.NewFileTokenAuthConfig(
    fs,
    auth.DefaultTokenFilePath, // /var/run/secrets/kubernetes.io/serviceaccount/token
    30*time.Second,
)
...
app.AddHTTPService("upstream", "https://example.com", tokenCfg)
```

Every outgoing request to `upstream` gets an `Authorization: Bearer <token>`
header whose value is the current contents of the token file. The file is
re-read every 30s; the header value on in-flight requests uses whatever
token was loaded at send time.

`FileTokenAuthConfig` composes with `ConnectionPoolConfig`,
`CircuitBreakerConfig`, `RetryConfig`, and other `service.Options` — pass them
together to `AddHTTPService`.

## Run locally

The default token path only exists inside a Kubernetes pod. To try the example
on a local machine, point it at a file you control:

```go
tokenCfg, err := auth.NewFileTokenAuthConfig(fs, "/tmp/my-token", 30*time.Second)
```

Then:

```
echo "my-local-token" > /tmp/my-token
go run ./examples/using-file-token-auth
curl http://localhost:9016/proxy
```
