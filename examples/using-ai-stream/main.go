package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/ai"
	"gofr.dev/pkg/gofr/ai/llm"
	"gofr.dev/pkg/gofr/http/response"
)

func main() {
	app := gofr.New()

	app.AddLLM(&llm.Client{Provider: llm.Groq, Model: "llama-3.3-70b-versatile"})

	app.POST("/chat", chat)

	app.Run()
}

func chat(c *gofr.Context) (any, error) {
	var in struct {
		Prompt string `json:"prompt"`
	}

	if err := c.Bind(&in); err != nil {
		return nil, err
	}

	stream, err := c.LLM().Stream(c, []ai.Message{{Role: ai.RoleUser, Content: in.Prompt}})
	if err != nil {
		return nil, err
	}

	// The tokens are streamed to the client as server-sent events as the model produces them.
	return response.Stream{Source: stream}, nil
}
