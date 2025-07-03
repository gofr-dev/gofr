package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"gofr.dev/pkg/datasource/file/azureblob"
)

func main() {
	accountName := os.Getenv("AZURE_ACCOUNT_NAME")
	accountKey := os.Getenv("AZURE_ACCOUNT_KEY")
	container := os.Getenv("AZURE_CONTAINER_NAME")

	if strings.TrimSpace(accountName) == "" || strings.TrimSpace(accountKey) == "" || strings.TrimSpace(container) == "" {
		log.Fatal("❌ Please set AZURE_ACCOUNT_NAME, AZURE_ACCOUNT_KEY, and AZURE_CONTAINER_NAME environment variables.")
	}

	client, err := azureblob.NewClient(accountName, accountKey, container)
	if err != nil {
		log.Fatalf("❌ Failed to create Azure Blob client: %v", err)
	}

	// Upload a sample file
	file, err := os.Open("sample.txt")
	if err != nil {
		log.Fatalf("❌ Failed to open sample.txt: %v", err)
	}
	defer file.Close()

	err = client.Upload(context.Background(), "sample.txt", file)
	if err != nil {
		log.Fatalf("❌ Upload failed: %v", err)
	}
	fmt.Println("✅ File uploaded successfully!")

	// List blobs
	files, err := client.List(context.Background())
	if err != nil {
		log.Fatalf("❌ Listing failed: %v", err)
	}

	fmt.Println("📦 Files in Azure Blob container:")
	for _, name := range files {
		fmt.Println(" -", name)
	}
}
