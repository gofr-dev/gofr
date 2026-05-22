package main

import (
	"io"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/service/auth"
)

func main() {
	app := gofr.New()

	fs := file.NewLocalFileSystem(app.Logger())

	tokenCfg, err := auth.NewFileTokenAuthConfig(
		fs,
		app.Logger(),
		auth.DefaultTokenFilePath,
		30*time.Second,
	)
	if err != nil {
		app.Logger().Fatalf("failed to initialize file token auth: %v", err)
	}

	// tokenCfg runs a background refresh goroutine for the lifetime of the
	// process; AddHTTPService keeps it alive until the app exits, so there is no
	// need to call tokenCfg.Close() here. Call Close() only if you build a
	// config that outlives the service that uses it.
	app.AddHTTPService("upstream", "https://example.com", tokenCfg)

	app.GET("/proxy", Proxy)

	app.Run()
}

// Proxy forwards a request to the upstream service. The FileTokenAuthConfig
// option automatically injects an Authorization: Bearer <token> header where
// <token> is read from the configured file and refreshed every 30s.
func Proxy(ctx *gofr.Context) (any, error) {
	resp, err := ctx.GetHTTPService("upstream").Get(ctx, "", nil)
	if err != nil {
		return nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return string(body), nil
}
