# MCP Mode — Expose your GoFr endpoints to AI assistants

GoFr can stand up a Model Context Protocol (MCP) server alongside the
HTTP server with a single environment variable. Every registered route
becomes a callable MCP tool, with the same auth, observability, and
handler logic as the HTTP path. AI coding assistants (Claude Desktop,
Cursor, Continue, Aider, and any other MCP client) can connect, list
tools, and invoke them — all without changing a line of handler code.

## Why this exists

MCP is the protocol AI assistants use to talk to external systems.
Wiring a Go service into MCP normally means hand-writing a separate
server: tool definitions, JSON Schemas, request adapters, auth
plumbing. GoFr already has all of that — routes, request binding,
middleware, OTel spans — so the framework can derive the MCP surface
automatically. Setting `MCP_ENABLED=true` flips that on.

## Turning it on

```env
# configs/.env
MCP_ENABLED=true
```

Run the app. The framework registers `POST /mcp` next to your usual
endpoints and the logs show:

```text
MCP mode enabled at /mcp (mutations=false)
```

That's it. `tools/list` returns one MCP tool per `GET`/`HEAD` route,
`tools/call` invokes the route through the existing router (so all
middleware applies). No new port, no second server.

## Two-value env var

```text
MCP_ENABLED=true   # GET + HEAD only (safe default)
MCP_ENABLED=full   # also expose POST/PUT/PATCH/DELETE
```

`true` is the right default for almost every deployment — read-only
exposure means an LLM can browse your data but cannot mutate it.
`full` is the explicit opt-in for agentic write workflows ("create a
ticket", "update this record"). Use it deliberately.

## How tools are named

Tool names are derived from the HTTP method and path so they're
predictable and don't require any annotations:

| Route                               | Tool name                              |
|-------------------------------------|----------------------------------------|
| `GET /users`                        | `get_users`                            |
| `GET /users/{id}`                   | `get_users_id`                         |
| `POST /users`                       | `post_users`                           |
| `PUT /users/{id}`                   | `put_users_id`                         |
| `DELETE /users/{id}`                | `delete_users_id`                      |
| `GET /orders/{orderID}/items/{id}`  | `get_orders_orderID_items_id`          |

## How tool schemas are learned

When `MCP_ENABLED` is set, GoFr wraps the `Request` interface that
every handler receives. On every call:

- `c.Bind(&u)` — the wrapper reflects on the bound struct (including
  `json` tags, `omitempty`, nested types, slices, maps, embedded
  structs) and emits a JSON Schema. That schema becomes the
  `body` field on the corresponding tool's `inputSchema`.
- `c.Param("limit")` — the wrapper records the query key. The next
  `tools/list` call lists `limit` as an accepted query parameter.
- The handler's return value — after the handler succeeds, the
  wrapper reflects on the returned value and records the output
  schema.

You don't write any annotations. You don't run any codegen. The
schemas are exact — they match the actual Go types your handlers
work with — and they sharpen as endpoints get exercised.

Schemas are persisted to `./.gofr-mcp-schemas.json` at shutdown so the
next process boot has full schemas before any traffic. Set
`MCP_PERSIST_PATH=-` to disable persistence, or pass any other path
to relocate the file.

## Auth, RBAC, and observability — all inherited

The `/mcp` endpoint is mounted as a regular route inside the main
mux router. Two consequences:

1. **Whatever middleware protects your service protects `/mcp`.** If
   you have a global JWT middleware, the MCP endpoint requires a
   valid token. If you have a global rate limiter, MCP traffic
   counts against it.

2. **Per-route middleware runs on every tool call.** When the bridge
   translates an MCP `tools/call` into a synthetic HTTP request, that
   request goes through the same router. RBAC checks, per-route auth,
   request timeouts — everything runs unchanged. The handler does
   not know the call originated from MCP.

The bridge forwards `Authorization`, `X-Api-Key`, `X-Forwarded-For`,
`X-Request-Id`, and `Cookie` from the inbound MCP request onto the
synthetic request, so the credentials that opened the MCP session
are the credentials your existing auth layer validates.

OpenTelemetry spans, Prometheus metrics, and structured logs all
behave identically — you'll see the same trace tree you'd see for
an HTTP call, plus a parent `/mcp` span.

## Connecting from Claude Desktop

Claude Desktop reads server configs from
`~/Library/Application Support/Claude/claude_desktop_config.json` on
macOS. Bridge HTTP through `mcp-proxy` (or your client's preferred
HTTP transport):

```jsonc
{
  "mcpServers": {
    "my-gofr-service": {
      "command": "npx",
      "args": ["-y", "mcp-proxy", "http://localhost:9100/mcp"]
    }
  }
}
```

Restart Claude Desktop and your routes appear as callable tools.
Other clients (Cursor, Continue, Aider) accept similar config — point
them at `/mcp` and they discover tools automatically.

## What is intentionally not exposed

- **Built-in routes.** `/.well-known/...`, `/swagger`, `/favicon.ico`,
  `/static/...`, and `/mcp` itself are filtered out — they exist for
  framework purposes, not for LLM consumption.
- **Streaming endpoints.** `gofr.SSE` returns and WebSocket upgrades
  don't translate to a single tool call. They're skipped by the
  manifest in v1; revisit if there's demand.
- **gRPC, GraphQL, Pub/Sub.** MCP exposes HTTP routes only. gRPC
  surfaces require a separate adapter; GraphQL has its own
  introspection that LLMs already understand; Pub/Sub is fire-and-
  forget and doesn't fit the request/response tool model.

## What you should still do

- **Document your handlers.** Tool names plus learned schemas are a
  strong start, but a one-line description still beats no
  description. (A future opt-in API will let you attach descriptions
  per route. Today, the description is `METHOD path`.)
- **Be deliberate with `MCP_ENABLED=full`.** An LLM with a hammer
  finds every nail. Confirm you want every mutation accessible
  before flipping the switch.
- **Monitor the `/mcp` endpoint.** Treat it like any other public
  surface — rate-limit it, audit-log it, alert on anomalies.

## Worked example

A complete service is in
[`examples/using-mcp/`](https://github.com/gofr-dev/gofr/tree/main/examples/using-mcp).
The handlers there have no MCP-specific code; only the env var
toggles MCP behavior. Run it, then talk to it as MCP:

```bash
curl -s -X POST http://localhost:9100/mcp \
     -H 'Content-Type: application/json' \
     -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq
```
