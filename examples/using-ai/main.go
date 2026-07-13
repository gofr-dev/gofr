package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/ai/llm"
)

func main() {
	app := gofr.New()

	// The API key is read from GROQ_API_KEY (or <PROVIDER>_API_KEY) in the environment.
	app.AddLLM(&llm.Client{Provider: llm.Groq, Model: "llama-3.3-70b-versatile"})

	app.POST("/summarize", summarize)

	app.Run()
}

func summarize(c *gofr.Context) (any, error) {
	var in struct {
		Text string `json:"text"`
	}

	if err := c.Bind(&in); err != nil {
		return nil, err
	}

	// One call — the trace span, token metrics and structured log happen automatically.
	return c.LLM().Generate(c, "Summarize the following:\n"+in.Text)
}
