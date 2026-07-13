package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/ai/llm"
)

const maxTurns = 5

func main() {
	app := gofr.New()

	app.AddLLM(&llm.Client{Provider: llm.Groq, Model: "llama-3.3-70b-versatile"})

	// The agent's tools are this service's own read-only handlers.
	app.GET("/inventory/{sku}", inventory)
	app.EnableMCP()

	app.POST("/agent", runAgent)

	app.Run()
}

func inventory(c *gofr.Context) (any, error) {
	return map[string]any{"sku": c.PathParam("sku"), "in_stock": 12}, nil
}

// runAgent shows the loop GoFr leaves to the application: call the model with the available tools,
// run any tool it picks, feed the result back, and repeat until it answers.
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
		messages = append(messages, runToolCalls(c, tools, resp.ToolCalls)...)
	}

	return "agent did not converge", nil
}

func runToolCalls(c *gofr.Context, tools ai.Tools, calls []ai.ToolCall) []ai.Message {
	out := make([]ai.Message, 0, len(calls))

	for _, call := range calls {
		content := ""

		result, err := tools.Call(c, call.Name, call.Args)
		if err != nil {
			content = err.Error()
		} else if data, jErr := result.JSON(); jErr == nil {
			content = string(data)
		}

		out = append(out, ai.Message{Role: ai.RoleTool, ToolCallID: call.ID, Content: content})
	}

	return out
}
