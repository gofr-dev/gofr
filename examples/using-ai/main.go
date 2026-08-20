// This example shows the whole GoFr AI surface in one service:
//   - app.AddLLM        register an LLM (POST /ask calls it)
//   - response.Stream   stream tokens to the client (POST /stream)
//   - app.EnableMCP     expose the GET /inventory/{sku} handler as an agent tool
//   - ctx.LLM().Tools() drive an agent loop over that tool (POST /agent)
//   - ai.EmbeddingLLM   turn text into vectors (POST /embed)
package main

import (
	"errors"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/ai/llm"
	"gofr.dev/pkg/gofr/http/response"
)

const maxTurns = 5

var errEmbeddingsUnavailable = errors.New("embeddings unavailable")

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
	app.POST("/embed", embed)   // text -> vectors

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

// embed turns text into vectors — the primitive behind semantic search and agent memory. Embeddings
// are usually a different model from the chat one, so a real service registers a second LLM with
// gofr.WithName("embed") and selects it per call; this example has a single model, so it asserts on
// the default. Whether the model can actually embed is reported by Embed itself
// (ai.ErrEmbedNotSupported), not by the assertion — the assertion only checks the LLM carries the
// capability at all.
func embed(c *gofr.Context) (any, error) {
	var in struct {
		Inputs []string `json:"inputs"`
	}

	if err := c.Bind(&in); err != nil {
		return nil, err
	}

	model, ok := c.LLM().(ai.EmbeddingLLM)
	if !ok {
		return nil, errEmbeddingsUnavailable
	}

	resp, err := model.Embed(c, in.Inputs)
	if err != nil {
		return nil, err
	}

	// One vector per input, in input order: resp.Embeddings[i] belongs to in.Inputs[i].
	return resp.Embeddings, nil
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
