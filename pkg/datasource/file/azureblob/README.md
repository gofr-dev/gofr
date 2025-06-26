# Azure Blob File Module for GoFr

This module adds Azure Blob Storage support to the GoFr framework under `pkg/datasource/file`.

## ✅ Features

- Upload files to Azure Blob
- Download files
- Delete files
- List blobs
- Clean Go interface, easy to integrate with GoFr apps

---

## 🧰 Usage

```go
import (
    "context"
    "log"

    "github.com/yourusername/gofr/pkg/datasource/file/azureblob"
)

func main() {
    az, err := azureblob.NewAzureBlob("<account-name>", "<account-key>", "<container>")
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    // Upload a file
    err = az.Upload(ctx, "folder/file.txt", []byte("Hello from Go!"))

    // Download the file
    data, err := az.Download(ctx, "folder/file.txt")

    // Delete the file
    err = az.Delete(ctx, "folder/file.txt")

    // List files in container
    files, err := az.List(ctx)
}
