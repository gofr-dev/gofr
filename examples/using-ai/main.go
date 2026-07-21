// This example shows the whole GoFr AI surface in one service:
//   - app.AddLLM        register an LLM (POST /ask calls it)
//   - response.Stream   stream tokens to the client (POST /stream)
//   - app.EnableMCP     expose the GET /inventory/{sku} handler as an agent tool
//   - ctx.LLM().Tools() drive an agent loop over that tool (POST /agent)
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/ai/llm"
	"gofr.dev/pkg/gofr/http/response"
)

const maxTurns = 5

func main() {
	app := gofr.New()

	// LLM_API_KEY (or GROQ_API_KEY) is read from the environment.
	app.AddLLM(&llm.Client{Provider: llm.Groq, Model: "llama-3.3-70b-versatile"})

	// Enable MCP up front; it discovers handlers lazily, so routes registered below are exposed too.
	app.EnableMCP()

	// A plain data endpoint — also exposed to agents as a read-only tool.
	app.GET("/inventory/{sku}", inventory)

	app.POST("/ask", ask)       // one-shot completion
	app.POST("/stream", stream) // streamed completion
	app.POST("/agent", agent)   // agent loop using the service's own tools

	app.Run()
}

func inventory(c *gofr.Context) (any, error) {
	return map[string]any{"sku": c.PathParam("sku"), "in_stock": 12}, nil
}

func ask(c *gofr.Context) (any, error) {
	var in struct {
		Prompt string `json:"prompt"`
	}

	if err := c.Bind(&in); err != nil {
		return nil, err
	}

	return c.LLM().Generate(c, in.Prompt)
}

func stream(c *gofr.Context) (any, error) {
	var in struct {
		Prompt string `json:"prompt"`
	}

	if err := c.Bind(&in); err != nil {
		return nil, err
	}

	s, err := c.LLM().Stream(c, []ai.Message{{Role: ai.RoleUser, Content: in.Prompt}})
	if err != nil {
		return nil, err
	}

	return response.Stream{Source: s}, nil
}

func agent(c *gofr.Context) (any, error) {
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
			result, callErr := tools.Call(c, call.Name, call.Args)
			content := ""

			if callErr != nil {
				content = callErr.Error()
			} else if data, jErr := result.JSON(); jErr == nil {
				content = string(data)
			}

			messages = append(messages, ai.Message{Role: ai.RoleTool, ToolCallID: call.ID, Content: content})
		}
	}

	return "agent did not converge", nil
}
