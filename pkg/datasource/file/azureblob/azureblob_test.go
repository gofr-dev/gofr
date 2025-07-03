package azureblob

import (
	"context"
	"os"
	"testing"
)

func TestAzureBlobIntegration(t *testing.T) {
	// Load credentials from environment variables
	account := os.Getenv("AZURE_ACCOUNT")
	key := os.Getenv("AZURE_KEY")
	container := os.Getenv("AZURE_CONTAINER")

	if account == "" || key == "" || container == "" {
		t.Fatal("❌ Missing AZURE_ACCOUNT, AZURE_KEY, or AZURE_CONTAINER environment variables")
	}

	az, err := NewAzureBlob(account, key, container)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	ctx := context.Background()
	path := "test/test.txt"
	content := []byte("Hello Azure!")

	// Upload
	err = az.Upload(ctx, path, content)
	if err != nil {
		t.Errorf("Upload failed: %v", err)
	}

	// Download
	data, err := az.Download(ctx, path)
	if err != nil {
		t.Errorf("Download failed: %v", err)
	} else if string(data) != string(content) {
		t.Errorf("Download mismatch. Got: %s", string(data))
	}

	// List
	files, err := az.List(ctx)
	if err != nil {
		t.Errorf("List failed: %v", err)
	} else if len(files) == 0 {
		t.Errorf("List returned no files")
	}

	// Delete
	err = az.Delete(ctx, path)
	if err != nil {
		t.Errorf("Delete failed: %v", err)
	}
}
