# Startup Hooks

GoFr provides a way to run synchronous jobs when your application starts, before any servers begin handling requests. This is useful for tasks like seeding a database, warming up a cache, or performing other critical setup procedures.

## OnStart

You can register a startup hook using the `a.OnStart()` method on your `app` instance.

## Usage

The method accepts a function with the signature:

The method accepts a function with the signature `func(ctx *gofr.Context) error`.

- The `*gofr.Context` passed to the hook is fully initialized and provides access to all dependency-injection-managed services (e.g., `ctx.Container.SQL`, `ctx.Container.Redis`).
- If any `OnStart` hook returns an error, the application will log the error and refuse to start.


### Example: Warming up a Cache

Here is an example of using `OnStart` to set an initial value in a Redis cache when the application starts.

```go
package main

import (
    "gofr.dev/pkg/gofr"
)

func main() {
    a := gofr.New()

    // Register an OnStart hook to warm up a cache.
    a.OnStart(func(ctx *gofr.Context) error {
        ctx.Logger.Info("Warming up the cache...")

        // In a real app, this might come from a database or another service.
        cacheKey := "initial-data"
        cacheValue := "This is some data cached at startup."

        err := ctx.Redis.Set(ctx, cacheKey, cacheValue, 0).Err()
        if err != nil {
            ctx.Logger.Errorf("Failed to warm up cache: %v", err)
            return err // Return the error to halt startup if caching fails.
        }

        ctx.Logger.Info("Cache warmed up successfully!")

        return nil
    })

    // ... register your routes

    a.Run()
}
```

This ensures that critical startup tasks are completed successfully before the application begins accepting traffic.

