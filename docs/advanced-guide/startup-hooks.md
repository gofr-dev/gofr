# Synchronous Startup Hooks with `OnStart`

GoFr now supports synchronous startup hooks using the `OnStart` API. This feature allows you to run blocking initialization logic (such as preloading caches, running migrations, or fetching data from external services) **before** your application starts serving traffic.

## Why Use Startup Hooks?
- Ensure critical data is loaded before handling requests
- Avoid race conditions or partial initialization
- Keep all dependency management within GoFr's DI system

## Usage Example
```go
app := gofr.New()

app.OnStart(func(ctx *gofr.StartupContext) error {
    // Use ctx.Container to access DB, Redis, HTTP clients, etc.
    // e.g., preload cache, run migrations, etc.
    return nil // or return error to abort startup
})

app.GET("/greet", func(ctx *gofr.Context) (any, error) {
    return "Hello World!", nil
})

app.Run()
```

## How It Works
- All registered startup hooks are executed in order before the server starts.
- Each hook receives a `*StartupContext` with access to DI-managed services (DB, HTTP, etc.).
- If any hook returns an error, the app logs the error and exits without serving traffic.

## Best Practices
- **Do not** manually construct DB or HTTP clients; always use the DI system via `ctx.Container`.
- **Do not** expose or use the raw container or app in hooks—use only the `StartupContext`.
- Use startup hooks for blocking, critical initialization only. For background or periodic jobs, use cron jobs instead.

## FAQ
**Q: Why not just run code before `app.Run()`?**
A: Startup hooks ensure all dependencies are managed by GoFr, keep your code consistent, and allow for proper error handling and lifecycle management.

**Q: Can I access request-specific data in a startup hook?**
A: No. Startup hooks are for application-wide initialization, not per-request logic.

---

For more details, see the [GoFr documentation](../README.md) or ask in the community! 