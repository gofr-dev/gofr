package azureblob

import (
    "github.com/PAVAN2627/gofr/pkg/datasource/file"
)

func New(accountName, accountKey, container string) file.File {
    az, err := NewAzureBlob(accountName, accountKey, container)
    if err != nil {
        panic(err) // or return nil
    }
    return az
}
