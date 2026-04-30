# MCP mode

This example shows how a single environment variable turns a normal
GoFr service into a Model Context Protocol (MCP) server. The handlers
in `main.go` are vanilla — they have no MCP-specific code. With
`MCP_ENABLED=true` (or `full`) set, the framework also serves an MCP
endpoint at `POST /mcp` that exposes the same routes as MCP tools.

AI coding assistants (Claude Desktop, Cursor, Continue, etc.) can
connect and call those tools the same way an HTTP client would — with
the same auth, the same observability, and the same handler logic.

## Run it

```bash
cd examples/using-mcp
go run .
```

The service listens on `:9100`. Try the HTTP endpoints first:

```bash
curl http://localhost:9100/users
curl http://localhost:9100/users/1
curl -X POST http://localhost:9100/users \
     -H 'Content-Type: application/json' \
     -d '{"name":"Grace Hopper","role":"engineer"}'
```

Each call sharpens the MCP tool schema for that route — the schema
emitted by the manifest now reflects the actual struct types the
handler bound or returned, learned at runtime via reflection.

## Talk to it as MCP

Point an MCP client at `http://localhost:9100/mcp`. For a quick
manual test:

```bash
# tools/list
curl -s -X POST http://localhost:9100/mcp \
     -H 'Content-Type: application/json' \
     -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq

# tools/call — get a single user
curl -s -X POST http://localhost:9100/mcp \
     -H 'Content-Type: application/json' \
     -d '{
       "jsonrpc":"2.0",
       "id":2,
       "method":"tools/call",
       "params":{"name":"get_users_id","arguments":{"id":"1"}}
     }' | jq
```

## Wire it into Claude Desktop

Claude Desktop reads MCP server configs from
`~/Library/Application Support/Claude/claude_desktop_config.json` on
macOS. Add an entry that proxies stdio to the HTTP endpoint via
`mcp-proxy` (or whichever http-bridge your client supports):

```jsonc
{
  "mcpServers": {
    "gofr-example": {
      "command": "npx",
      "args": ["-y", "mcp-proxy", "http://localhost:9100/mcp"]
    }
  }
}
```

Restart Claude Desktop and the GoFr routes appear as callable tools.

## What gets exposed

`MCP_ENABLED=true` exposes `GET` (and `HEAD`) routes only — read-only
by default so an LLM can't accidentally mutate your data. To opt into
mutations, set `MCP_ENABLED=full`. Built-in routes (`/.well-known/...`,
`/swagger`, `/favicon.ico`, `/static/...`, `/mcp` itself) are never
exposed.

## How schemas get learned

The framework wraps the `Request` interface that handlers receive. On
every `c.Bind(&u)` and `c.Param("foo")` call, the wrapper reflects on
the bound type or records the query key. The result is a JSON Schema
that matches the developer's actual Go struct — including `json` tags,
`omitempty`, nested types, slices, maps — without any annotations or
codegen.

Schemas persist to `./.gofr-mcp-schemas.json` at shutdown so the next
process boot has full schemas before any traffic. Set
`MCP_PERSIST_PATH=-` to disable persistence.
