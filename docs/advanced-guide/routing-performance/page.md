---
description: "Speed up route matching in GoFr with the opt-in trie router. Set GOFR_ROUTER=trie to make matching cost O(path length) instead of scaling with your route count."
nextjs:
  metadata:
    title: "Routing Performance in GoFr — The Opt-In Trie Router"
    description: "Speed up route matching in GoFr with the opt-in trie router. Set GOFR_ROUTER=trie to make matching cost O(path length) instead of scaling with your route count."
---

# Routing Performance

GoFr routes on `gorilla/mux`, which finds a handler by walking the registered routes in order and
testing each one against the request path. That is O(n) in the number of routes, so a service pays a
little more per request for every route it adds.

Setting `GOFR_ROUTER=trie` swaps the matching step for a segment trie, making it O(path length) —
flat as the route table grows. Everything else is unchanged: `mux` is still the route registry, and
`mux` still makes the final decision about which route matches.

```bash
# configs/.env
GOFR_ROUTER=trie
```

It is **off by default**. Leave it unset and your service behaves exactly as it always has.

## Whether it will help you

The win scales with the size of your route table, so it is worth being concrete about where the line
is. Measured on an Apple M4, with the request hitting the middle of the table:

| Routes | Default (mux) | `GOFR_ROUTER=trie` | Speedup |
| -----: | ------------: | -----------------: | ------: |
|      1 |        431 ns |             447 ns |   0.96x |
|     10 |        506 ns |             461 ns |    1.1x |
|     50 |       1007 ns |             550 ns |    1.8x |
|    100 |       1642 ns |             548 ns |    3.0x |
|    200 |       2918 ns |             515 ns |    5.7x |

The crossover is around 5–10 routes. Below that the trie is marginally slower, so a small service
gains nothing by turning it on.

Two further caveats worth setting expectations against:

- **Matching is a minority of a request.** The middleware chain — tracing, logging, metrics, CORS —
  dominates. So end-to-end throughput moves by less than the table above, approaching it only as the
  route count grows.
- **The trie allocates slightly more.** Two extra allocations per matched request, for restoring the
  path params and route template. This is a CPU and scaling win, not an allocation win.

## What stays the same

Routing behaviour is unchanged, and that is a property the framework tests for rather than a hope.
The trie only *narrows* the set of routes worth considering; `mux`'s own `Route.Match` still decides
every request, so method matching, `{id:[0-9]+}` constraints, header and query matchers, route
ordering and path cleaning all behave exactly as they do by default. Anything the trie cannot index
— `PathPrefix` routes, static file handlers, slash-spanning parameters like `{path:.*}` — is handled
by `mux` directly. Requests that match nothing are handed to `mux` in full.

Path parameters are unaffected: `ctx.PathParam("id")` and `mux.Vars(r)` work identically.

## The one thing to check in your own code

The trie serves matched requests without going through `mux`'s own `ServeHTTP`, which is what
populates `mux.CurrentRoute`. If any of your handlers or middleware calls it:

```go
// Returns nil when GOFR_ROUTER=trie.
route := mux.CurrentRoute(r)
tmpl, _ := route.GetPathTemplate()
```

use GoFr's accessor instead. It resolves the template under both routers, so it is safe to adopt
before you flip the flag:

```go
import gofrHTTP "gofr.dev/pkg/gofr/http"

tmpl := gofrHTTP.RouteTemplate(r) // "/users/{id}", or "" if nothing matched
```

`mux.Vars(r)` is **not** affected and needs no change.

## Confirming which matcher is active

GoFr logs the matcher at startup whenever `GOFR_ROUTER` is set:

```
INFO  HTTP route matcher: trie
```

A value it does not recognize falls back to `mux` and says so, so a typo does not cost you the opt-in
silently:

```
WARN  unrecognized GOFR_ROUTER value "tri", using the "mux" router; valid values are "mux" and "trie"
```

## A note on registering routes late

The index is built once, from the routes present when the first request arrives. GoFr registers every
route during startup, before the server begins accepting requests, so this holds for all framework
code paths. A route added after the server is already serving would not be indexed — it would still
be served correctly, via `mux`, just without the speedup.
