package main

import (
	"encoding/json"
	"io"
	"time"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/service/auth"
)

func main() {
	a := gofr.New()

	tokenPath := a.Config.GetOrDefault("FILE_TOKEN_PATH", auth.DefaultTokenFilePath)

	tokenAuth, err := auth.NewFileTokenAuthConfig(
		file.NewLocalFileSystem(a.Logger()),
		tokenPath,
		30*time.Second,
	)
	if err != nil {
		a.Logger().Fatalf("failed to create file token auth: %v", err)
	}

	defer tokenAuth.Close()

	a.AddHTTPService("k8s-api", "https://kubernetes.default.svc", tokenAuth)

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
