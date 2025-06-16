# Running startup jobs

GoFr applications can execute initialization logic before serving traffic. Use `AddStartJob` to
register a synchronous function that runs during `app.Run()`. If any job returns an error, the
application exits without starting the servers.

```go
package main

import "gofr.dev/pkg/gofr"

func main() {
    app := gofr.New()

    app.AddStartJob("warm-cache", func(ctx *gofr.Context) error {
        // load data into an in-memory cache
        return nil
    })

    app.Run()
}
```
