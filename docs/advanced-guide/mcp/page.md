---
description: "Expose your GoFr handlers to AI agents with app.EnableMCP. Handlers become Model Context Protocol tools, dispatched through the real router so binding, validation and auth all run — plus a userland agent loop with ctx.LLM().Tools()."
nextjs:
  metadata:
    title: "Building AI Agents in GoFr — MCP Tools from Handlers"
    description: "Expose your GoFr handlers to AI agents with app.EnableMCP. Handlers become Model Context Protocol tools, dispatched through the real router so binding, validation and auth all run — plus a userland agent loop with ctx.LLM().Tools()."
---

# Building AI Agents

GoFr turns the handlers you already wrote into tools an AI agent can call, and gives you the
primitives to build an agent loop — while leaving the loop itself in your code, where the turns,
stopping conditions and memory belong.

## Exposing handlers as tools

`app.EnableMCP()` publishes your registered handlers over a
[Model Context Protocol](https://modelcontextprotocol.io) server on its own port, so an MCP client
(Claude Code, Claude Desktop, or any agent) can discover and call them.

```go
func main() {
	app := gofr.New()

	// Enable MCP up front — handlers are discovered lazily, so routes registered afterwards are
	// exposed too. The server runs on MCP_PORT (default 8200).
	app.EnableMCP()

	app.GET("/products/{id}", getProduct)
	app.POST("/products", createProduct)

	app.Run()
}
```

The input schema for each tool is derived from the route's path parameters.

When a tool is called, its arguments are dispatched like a real request: path parameters fill the
route and the remaining arguments become query values.

### Read-only

Only safe-to-retry handlers (`GET`, `HEAD`, `OPTIONS`) are exposed as tools. Write handlers
(`POST`/`PUT`/`PATCH`/`DELETE`) are never exposed, so an agent cannot mutate state through this
surface.

Drop specific routes with `gofr.WithExcludedRoutes("/internal/{id}")`. Framework `/.well-known/*` probes are
never exposed. The read-only rule is enforced at call time, not just in the tool listing — a write
route cannot be invoked by guessing its tool name.

### Safety

The MCP server binds to loopback (`127.0.0.1`) so it does not become a second network-reachable
entry point to your handlers. When a tool is invoked, GoFr rebuilds the request and dispatches it
**through the same router**, so your binding, validation, RBAC and auth middleware all run exactly
as for a normal HTTP call. The caller's `Authorization` / `X-Api-Key` headers propagate, so
`ctx.GetAuthInfo()` works inside a tool call and a secured endpoint stays secured. Set `MCP_PORT=0`
to disable the server while keeping in-process tools available.

### If the port is unavailable

Calling `app.EnableMCP()` says you want the MCP transport, so GoFr treats a port it cannot claim as
a startup failure rather than coming up without it. `app.Run()` reports the problem and returns
instead of serving:

```
ERROR  MCP server cannot start on port 8200: listen tcp 127.0.0.1:8200: bind: address already in
       use. Set MCP_PORT to a free port, or MCP_PORT=0 to run without the MCP transport while
       keeping tools available in-process.
```

A service that started successfully and quietly lacked a transport it was configured to expose would
be the worse outcome — an orchestrator would report it healthy while agents could not reach it.

Note the default is `8200`, which is also [Vault](https://developer.hashicorp.com/vault)'s default
port. If you run Vault locally, set `MCP_PORT` to something else.

The port is claimed before any server starts, so this decision is made while nothing is yet serving.
The startup hooks have already run by then, so GoFr runs shutdown on the way out to release the
datasources they opened. If you would rather run without the transport, `MCP_PORT=0` is the explicit
way to say so — tools stay callable in-process through `ctx.LLM().Tools()`, because registration is
independent of the transport.

`MCP_PORT` is read as a number, so `0`, `00`, `+0` and ` 0 ` all disable the transport. A value that
is not a number, or one outside the valid port range `1`–`65535`, is reported and falls back to the
default `8200` — an unbindable port would otherwise stop the whole service, every transport
included, with a message about occupancy that does not describe the problem.

## Building an agent

`ctx.LLM().Tools()` gives a handler the same tools — your own service's endpoints — so you can run
an agent loop: call the model with the available tools, run whatever it picks, feed the result back,
and repeat until it answers.

```go
func runAgent(c *gofr.Context) (any, error) {
	var in struct {
		Task string `json:"task"`
	}

	if err := c.Bind(&in); err != nil {
		return nil, err
	}

	model, tools := c.LLM(), c.LLM().Tools()
	messages := []ai.Message{{Role: ai.RoleUser, Content: in.Task}}

	for range maxTurns {
		resp, err := model.Chat(c, messages, ai.WithTools(tools.List()))
		if err != nil {
			return nil, err
		}

		if len(resp.ToolCalls) == 0 {
			return resp.Content, nil
		}

		messages = append(messages, ai.Message{Role: ai.RoleAssistant, ToolCalls: resp.ToolCalls})

		for _, call := range resp.ToolCalls {
			result, _ := tools.Call(c, call.Name, call.Args)
			data, _ := result.JSON()
			messages = append(messages, ai.Message{Role: ai.RoleTool, ToolCallID: call.ID, Content: string(data)})
		}
	}

	return "agent did not converge", nil
}
```

Narrow the tools an agent may use with `tools.Only("get_products_id")`. Every tool call flows the
request ID into the same trace, metric and log spine as the LLM call and any database query, so one
agent request produces one coherent trace.

> **Why no built-in agent loop?** Turns, stopping conditions, streaming and memory vary too much per
> application. GoFr ships the primitives — `ctx.LLM()`, `ctx.LLM().Tools()` — the observability and
> the plumbing; the decision logic stays in your code.

> #### Check out the example on how to expose handlers to agents and build an agent loop in GoFr: [Visit GitHub](https://github.com/gofr-dev/gofr/blob/main/examples/using-ai/main.go)
