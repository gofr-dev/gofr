# GoFr docs MCP server

A [Model Context Protocol](https://modelcontextprotocol.io) server that gives an
AI agent GoFr's documentation as callable tools, instead of leaving it to guess
framework APIs from memory.

## Run it

```bash
go run gofr.dev/mcp/docs-server@latest
```

It speaks MCP over stdio, which is what MCP clients launch by default.

### Claude Code

```bash
claude mcp add gofr-docs -- go run gofr.dev/mcp/docs-server@latest
```

### Any client with a JSON config

```json
{
  "mcpServers": {
    "gofr-docs": {
      "command": "go",
      "args": ["run", "gofr.dev/mcp/docs-server@latest"]
    }
  }
}
```

## Tools

| Tool | Use it for |
| --- | --- |
| `search_docs` | Full-text search across every page. Start here. |
| `get_doc` | One page in full, by site path, when an excerpt is not enough. |
| `list_sections` | The section list, to orient before searching. |

## How it works

Content is fetched once per process from
<https://gofr.dev/llms-full.txt> — the concatenated dump the website regenerates
on every deploy — and cached in memory. Fetching rather than embedding means the
server serves current documentation without needing to be re-released alongside
the docs.

The fetch is deferred until the first tool call. An MCP client starts its servers
eagerly when the editor opens; a fetch failure at startup would surface as "the
server crashed" rather than "that one call could not reach the network".

Implemented with the standard library only. This package lives in the root
`gofr.dev` module, and a documentation tool is not a good reason to add a
dependency that every `go get gofr.dev` would then carry.

## Tests

```bash
go test ./...
```
