package azureblob

import (
	"context"
	"fmt"
	"io"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
)

// Client handles interaction with Azure Blob Storage.
type Client struct {
	containerClient *container.Client
}

// NewClient creates a new Azure Blob client using account credentials and container name.
func NewClient(accountName, accountKey, containerName string) (*Client, error) {
	cred, err := azblob.NewSharedKeyCredential(accountName, accountKey)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", accountName)
	svcClient, err := service.NewClientWithSharedKeyCredential(serviceURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create service client: %w", err)
	}

	containerClient := svcClient.NewContainerClient(containerName)
	return &Client{containerClient: containerClient}, nil
}

// GetContainerClient returns the container client.
func (c *Client) GetContainerClient() *container.Client {
	return c.containerClient
}

// Upload uploads a blob to the container.
func (c *Client) Upload(ctx context.Context, blobName string, data io.Reader) error {
	blobClient := c.containerClient.NewBlockBlobClient(blobName)
	_, err := blobClient.UploadStream(ctx, data, nil)
	return err
}

// Download downloads a blob and returns a reader.
func (c *Client) Download(ctx context.Context, blobName string) (io.ReadCloser, error) {
	blobClient := c.containerClient.NewBlobClient(blobName)
	resp, err := blobClient.DownloadStream(ctx, nil)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Delete deletes a blob from the container.
func (c *Client) Delete(ctx context.Context, blobName string) error {
	blobClient := c.containerClient.NewBlobClient(blobName)
	_, err := blobClient.Delete(ctx, nil)
	return err
}

// List returns the names of all blobs in the container.
func (c *Client) List(ctx context.Context) ([]string, error) {
	pager := c.containerClient.NewListBlobsFlatPager(nil)
	var names []string

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, blob := range page.Segment.BlobItems {
			names = append(names, *blob.Name)
		}
	}
	return names, nil
}
