package main

import (
	"fmt"
	"io"
	"os"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/file/azure"
)

func main() {
	app := gofr.New()

	// Configure Azure File Storage
	// Note: Azurite does NOT support Azure File Storage.
	// For local testing, you need actual Azure Storage Account credentials.
	// Set these via environment variables: AZURE_STORAGE_ACCOUNT, AZURE_STORAGE_KEY, AZURE_FILE_SHARE
	accountName := app.Config.Get("AZURE_STORAGE_ACCOUNT")
	accountKey := app.Config.Get("AZURE_STORAGE_KEY")
	shareName := app.Config.Get("AZURE_FILE_SHARE")
	endpoint := app.Config.Get("AZURE_STORAGE_ENDPOINT") // Optional

	if accountName == "" || accountKey == "" || shareName == "" {
		app.Logger().Error("Azure File Storage credentials not configured. " +
			"Please set AZURE_STORAGE_ACCOUNT, AZURE_STORAGE_KEY, and AZURE_FILE_SHARE environment variables.")
		os.Exit(1)
	}

	config := &azure.Config{
		AccountName: accountName,
		AccountKey:  accountKey,
		ShareName:   shareName,
	}

	if endpoint != "" {
		config.Endpoint = endpoint
	}

	// Create Azure File Storage filesystem
	fs, err := azure.New(config, app.Logger(), app.Metrics())
	if err != nil {
		app.Logger().Fatalf("Failed to initialize Azure File Storage: %v", err)
	}

	app.AddFileStore(fs)

	// Register HTTP handlers
	app.GET("/", homeHandler)
	app.GET("/files", listFilesHandler)
	app.GET("/files/{name}", readFileHandler)
	app.POST("/files/{name}", createFileHandler)
	app.PUT("/files/{name}", updateFileHandler)
	app.DELETE("/files/{name}", deleteFileHandler)
	app.POST("/directories/{name}", createDirectoryHandler)
	app.GET("/directories/{name}", listDirectoryHandler)
	app.DELETE("/directories/{name}", deleteDirectoryHandler)
	app.POST("/copy", copyFileHandler)
	app.GET("/stat/{name}", statHandler)

	app.Run()
}

func homeHandler(c *gofr.Context) (interface{}, error) {
	return map[string]string{
		"message": "Azure File Storage Example",
		"endpoints": "GET /files, GET /files/{name}, POST /files/{name}, " +
			"PUT /files/{name}, DELETE /files/{name}, POST /directories/{name}, " +
			"GET /directories/{name}, DELETE /directories/{name}, POST /copy, GET /stat/{name}",
	}, nil
}

func listFilesHandler(c *gofr.Context) (interface{}, error) {
	path := c.PathParam("path")
	if path == "" {
		path = "."
	}

	files, err := c.File.ReadDir(path)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(files))
	for _, file := range files {
		result = append(result, map[string]interface{}{
			"name":         file.Name(),
			"size":         file.Size(),
			"is_directory": file.IsDir(),
			"modified":     file.ModTime(),
		})
	}

	return result, nil
}

func readFileHandler(c *gofr.Context) (interface{}, error) {
	name := c.PathParam("name")
	if name == "" {
		return nil, fmt.Errorf("file name is required")
	}

	file, err := c.File.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"name":    name,
		"content": string(data),
		"size":    len(data),
	}, nil
}

func createFileHandler(c *gofr.Context) (interface{}, error) {
	name := c.PathParam("name")
	if name == "" {
		return nil, fmt.Errorf("file name is required")
	}

	var reqBody struct {
		Content string `json:"content"`
	}

	// Try to bind JSON first, if that fails, read raw body
	if err := c.Bind(&reqBody); err != nil {
		// If binding fails, try to get raw body
		// For raw text/plain content, we'll use a workaround
		// In production, you might want to check Content-Type header
		return nil, fmt.Errorf("unable to read request body: %w", err)
	}

	body := []byte(reqBody.Content)
	if len(body) == 0 {
		body = []byte("") // Empty file
	}

	file, err := c.File.Create(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	_, err = file.Write(body)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"message": fmt.Sprintf("File %s created successfully", name),
		"size":     fmt.Sprintf("%d bytes", len(body)),
	}, nil
}

func updateFileHandler(c *gofr.Context) (interface{}, error) {
	name := c.PathParam("name")
	if name == "" {
		return nil, fmt.Errorf("file name is required")
	}

	var reqBody struct {
		Content string `json:"content"`
	}

	// Try to bind JSON first
	if err := c.Bind(&reqBody); err != nil {
		return nil, fmt.Errorf("unable to read request body: %w", err)
	}

	body := []byte(reqBody.Content)

	file, err := c.File.OpenFile(name, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	_, err = file.Write(body)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"message": fmt.Sprintf("File %s updated successfully", name),
		"size":     fmt.Sprintf("%d bytes", len(body)),
	}, nil
}

func deleteFileHandler(c *gofr.Context) (interface{}, error) {
	name := c.PathParam("name")
	if name == "" {
		return nil, fmt.Errorf("file name is required")
	}

	err := c.File.Remove(name)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"message": fmt.Sprintf("File %s deleted successfully", name),
	}, nil
}

func createDirectoryHandler(c *gofr.Context) (interface{}, error) {
	name := c.PathParam("name")
	if name == "" {
		return nil, fmt.Errorf("directory name is required")
	}

	err := c.File.MkdirAll(name, 0755)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"message": fmt.Sprintf("Directory %s created successfully", name),
	}, nil
}

func listDirectoryHandler(c *gofr.Context) (interface{}, error) {
	name := c.PathParam("name")
	if name == "" {
		name = "."
	}

	files, err := c.File.ReadDir(name)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(files))
	for _, file := range files {
		result = append(result, map[string]interface{}{
			"name":         file.Name(),
			"size":         file.Size(),
			"is_directory": file.IsDir(),
			"modified":     file.ModTime(),
		})
	}

	return result, nil
}

func deleteDirectoryHandler(c *gofr.Context) (interface{}, error) {
	name := c.PathParam("name")
	if name == "" {
		return nil, fmt.Errorf("directory name is required")
	}

	err := c.File.RemoveAll(name)
	if err != nil {
		return nil, err
	}

	return map[string]string{
		"message": fmt.Sprintf("Directory %s deleted successfully", name),
	}, nil
}

func copyFileHandler(c *gofr.Context) (interface{}, error) {
	var req struct {
		Source      string `json:"source"`
		Destination string `json:"destination"`
	}

	if err := c.Bind(&req); err != nil {
		return nil, err
	}

	if req.Source == "" || req.Destination == "" {
		return nil, fmt.Errorf("source and destination are required")
	}

	// Read source file
	srcFile, err := c.File.Open(req.Source)
	if err != nil {
		return nil, fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := c.File.Create(req.Destination)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy content
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return nil, fmt.Errorf("failed to copy file: %w", err)
	}

	return map[string]string{
		"message": fmt.Sprintf("File copied from %s to %s successfully", req.Source, req.Destination),
	}, nil
}

func statHandler(c *gofr.Context) (interface{}, error) {
	name := c.PathParam("name")
	if name == "" {
		return nil, fmt.Errorf("file or directory name is required")
	}

	info, err := c.File.Stat(name)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"name":         info.Name(),
		"size":         info.Size(),
		"is_directory": info.IsDir(),
		"modified":     info.ModTime(),
		"mode":         info.Mode().String(),
	}, nil
}

