# Using OpenAI Api

GoFr provides an injectable module that integrates OpenAI's API into the GoFr applications. Since it doesn’t come bundled with the framework, this wrapper can be injected seamlessly to extend Gofr's capabilities, enabling developers to utilize OpenAI's powerful AI models effortlessly while maintaining flexibility and scalability.

GoFr supports any OpenAI API wrapper that implements the following interface. Any other wrapper that implements the interface can be added using `app.AddOpenAI()` method, and user's can use openai across application with `gofr.Context`.

```go
type OpenAI interface {
	// implementation of chat endpoint of openai api
	CreateCompletions(ctx context.Context, r any) (any, error)
}
```

### Example
```go
package main

import (
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/service/openai"
)

func main() {
	app := gofr.New()

	config := openai.Config{
		APIKey: app.Config.Get("OPENAI_API_KEY"),
		Model:  "gpt-3.5-turbo",

		// optional config parameters
		// BaseURL: "https://api.custom.com",
		// Timeout: 10 * time.Second,
		// MaxIdleConns: 10,
	}

	openAIClient, err := openai.NewClient(&config)
	if err != nil {
		return
	}

	app.AddOpenAI(openAIClient)

	app.POST("/chat", Chat)

	app.Run()
}

func Chat(ctx *gofr.Context) (any, error) {

	var req *openai.CreateCompletionsRequest

	if err := ctx.Bind(&req); err != nil {
		return nil, err
	}

	println(req)

	resp, err := ctx.Openai.CreateCompletions(ctx, req)
	if err != nil {
		return nil, err
	}

	return resp, nil
}
```

