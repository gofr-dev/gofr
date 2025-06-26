package azureblob

import (
	"context"
	"fmt"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
)

type AzureBlob struct {
	client        *azblob.Client
	containerName string
}

func NewAzureBlob(accountName, accountKey, container string) (*AzureBlob, error) {
	cred, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://%s.blob.core.windows.net/", accountName)
	client, err := azblob.NewClientWithSharedKeyCredential(url, cred, nil)
	if err != nil {
		return nil, err
	}

	return &AzureBlob{
		client:        client,
		containerName: container,
	}, nil
}

func (a *AzureBlob) Upload(ctx context.Context, fileName string, data []byte) error {
	_, err := a.client.UploadBuffer(ctx, a.containerName, fileName, data, nil)
	return err
}

func (a *AzureBlob) Download(ctx context.Context, fileName string) ([]byte, error) {
	resp, err := a.client.DownloadStream(ctx, a.containerName, fileName, nil)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(resp.Body)
}

func (a *AzureBlob) Delete(ctx context.Context, fileName string) error {
	_, err := a.client.DeleteBlob(ctx, a.containerName, fileName, nil)
	return err
}

func (a *AzureBlob) List(ctx context.Context) ([]string, error) {
	pager := a.client.NewListBlobsFlatPager(a.containerName, nil)
	var files []string
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, blob := range page.Segment.BlobItems {
			files = append(files, *blob.Name)
		}
	}
	return files, nil
}
