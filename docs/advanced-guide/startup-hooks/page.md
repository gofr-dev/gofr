---
description: "Register synchronous startup hooks in GoFr to seed databases, warm caches, or run setup tasks before the HTTP, gRPC, and pub/sub servers begin handling traffic."
nextjs:
  metadata:
    title: "Startup Hooks in GoFr — Run Jobs Before Server Starts"
    description: "Register synchronous startup hooks in GoFr to seed databases, warm caches, or run setup tasks before the HTTP, gRPC, and pub/sub servers begin handling traffic."
---

# Startup Hooks

GoFr provides a way to run synchronous jobs when your application starts, before any servers begin handling requests. This is useful for tasks like seeding a database, warming up a cache, or performing other critical setup procedures.

## OnStart

You can register a startup hook using the `a.OnStart()` method on your `app` instance.

## Usage

The method accepts a function with the signature:

The method accepts a function with the signature `func(ctx *gofr.Context) error`.

- The `*gofr.Context` passed to the hook is fully initialized and provides access to all dependency-injection-managed services (e.g., `ctx.Container.SQL`, `ctx.Container.Redis`).
- If any `OnStart` hook returns an error, the application will log the error and refuse to start.

### What "refuse to start" does

A hook that returns an error abandons the run, and the process **exits with status 1**.

1. The error is logged.
2. Everything startup has already opened is released — the container's datasources are live by the time hooks run, so they are closed rather than left to process exit.
3. No server is started, and `app.Run()` does not return.

Two consequences worth planning for:

- **Code after `app.Run()` in `main` does not execute** on this path. `Run` is meant to be the last call in `main`; anything you need to happen on a failed start belongs in the hook itself or in an `OnShutdown` hook.
- **A test that calls `app.Run()` with a failing hook terminates the test binary.** Exercise the hook function directly rather than through `Run`.

Exiting non-zero is deliberate: a Kubernetes `restartPolicy`, a `CrashLoopBackOff` and a systemd restart all need a non-zero status. A service that refused to start but reported success is the failure this avoids.

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


## Related production guides

- **Graceful Shutdown**: [Mirror your startup work on the way down](/docs/guides/graceful-shutdown) — close pools, drain queues, and finish in-flight requests cleanly.
