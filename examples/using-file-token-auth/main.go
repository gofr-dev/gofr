package main

import (
	"encoding/json"
	"io"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/service"
	"gofr.dev/pkg/gofr/service/auth"
)

func main() {
	a := gofr.New()

	logger := a.Logger()
	tokenPath := a.Config.GetOrDefault("FILE_TOKEN_PATH", auth.DefaultTokenFilePath)

	tokenAuth, err := auth.NewFileTokenAuthConfig(
		file.NewLocalFileSystem(logger),
		tokenPath,
		30*time.Second,
	)
	if err != nil {
		logger.Fatalf("failed to create file token auth: %v", err)
	}

	defer tokenAuth.(io.Closer).Close()

	// Option 1: Pass as option to AddHTTPService.
	// Logger and metrics are injected automatically via the Observable interface.
	a.AddHTTPService("k8s-api", "https://kubernetes.default.svc", tokenAuth)

	// Option 2: Call AddOption directly on an existing HTTP service.
	// Logger and metrics must be set manually since AddOption does not inject them.
	svc := service.NewHTTPService("https://api.example.com", logger, a.Metrics())
	tokenAuth.(service.Observable).UseLogger(logger)
	tokenAuth.(service.Observable).UseMetrics(a.Metrics())

	svc = tokenAuth.AddOption(svc)

	_ = svc

	a.GET("/pods", PodHandler)

	a.Run()
}

func PodHandler(c *gofr.Context) (any, error) {
	k8sAPI := c.GetHTTPService("k8s-api")

	resp, err := k8sAPI.Get(c, "api/v1/namespaces/default/pods", nil)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]any

	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
