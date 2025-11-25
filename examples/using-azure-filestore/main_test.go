package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/file/azure"
)

func TestAzureFileStorageHandlers(t *testing.T) {
	// Skip if Azure credentials are not configured
	accountName := getEnv("AZURE_STORAGE_ACCOUNT", "")
	accountKey := getEnv("AZURE_STORAGE_KEY", "")
	shareName := getEnv("AZURE_FILE_SHARE", "")

	if accountName == "" || accountKey == "" || shareName == "" {
		t.Skip("Azure File Storage credentials not configured. " +
			"Set AZURE_STORAGE_ACCOUNT, AZURE_STORAGE_KEY, and AZURE_FILE_SHARE to run integration tests.")
	}

	app := gofr.New()

	config := &azure.Config{
		AccountName: accountName,
		AccountKey:  accountKey,
		ShareName:   shareName,
	}

	fs, err := azure.New(config, app.Logger(), app.Metrics())
	if err != nil {
		t.Fatalf("Failed to initialize Azure File Storage: %v", err)
	}

	app.AddFileStore(fs)

	// Register handlers
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

	t.Run("HomeHandler", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		app.Server.HTTP.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("CreateAndReadFile", func(t *testing.T) {
		testFileName := "test-file.txt"
		testContent := "Hello, Azure File Storage!"

		// Create file
		body := bytes.NewBufferString(testContent)
		req := httptest.NewRequest(http.MethodPost, "/files/"+testFileName, body)
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()

		app.Server.HTTP.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Create file: Expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
		}

		// Read file
		req = httptest.NewRequest(http.MethodGet, "/files/"+testFileName, nil)
		w = httptest.NewRecorder()

		app.Server.HTTP.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Read file: Expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
		}

		// Cleanup
		req = httptest.NewRequest(http.MethodDelete, "/files/"+testFileName, nil)
		w = httptest.NewRecorder()
		app.Server.HTTP.ServeHTTP(w, req)
	})

	t.Run("CreateAndListDirectory", func(t *testing.T) {
		testDirName := "test-dir"

		// Create directory
		req := httptest.NewRequest(http.MethodPost, "/directories/"+testDirName, nil)
		w := httptest.NewRecorder()

		app.Server.HTTP.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Create directory: Expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
		}

		// List directory
		req = httptest.NewRequest(http.MethodGet, "/directories/"+testDirName, nil)
		w = httptest.NewRecorder()

		app.Server.HTTP.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("List directory: Expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
		}

		// Cleanup
		req = httptest.NewRequest(http.MethodDelete, "/directories/"+testDirName, nil)
		w = httptest.NewRecorder()
		app.Server.HTTP.ServeHTTP(w, req)
	})

	t.Run("StatFile", func(t *testing.T) {
		testFileName := "stat-test.txt"
		testContent := "Test content for stat"

		// Create file first
		body := bytes.NewBufferString(testContent)
		req := httptest.NewRequest(http.MethodPost, "/files/"+testFileName, body)
		req.Header.Set("Content-Type", "text/plain")
		w := httptest.NewRecorder()
		app.Server.HTTP.ServeHTTP(w, req)

		// Get stat
		req = httptest.NewRequest(http.MethodGet, "/stat/"+testFileName, nil)
		w = httptest.NewRecorder()

		app.Server.HTTP.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Stat file: Expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
		}

		// Cleanup
		req = httptest.NewRequest(http.MethodDelete, "/files/"+testFileName, nil)
		w = httptest.NewRecorder()
		app.Server.HTTP.ServeHTTP(w, req)
	})
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}

