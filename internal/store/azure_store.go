package store

import (
	"context"
	"io"
	"os"

	"gofr.dev/pkg/datasource/file/azureblob"
)

type StudentStore interface {
	Upload(ctx context.Context, name string, file io.Reader) error
	List(ctx context.Context) ([]string, error)
	Download(ctx context.Context, filename string) ([]byte, error)
	Delete(ctx context.Context, filename string) error
}


type studentStore struct {
	client *azureblob.Client
}

func New() StudentStore {
	client, _ := azureblob.NewClient(
		os.Getenv("AZURE_ACCOUNT_NAME"),
		os.Getenv("AZURE_ACCOUNT_KEY"),
		os.Getenv("AZURE_CONTAINER_NAME"),
	)

	return &studentStore{client: client}
}

func (s *studentStore) Upload(ctx context.Context, name string, file io.Reader) error {
	return s.client.Upload(ctx, name, file)
}

func (s *studentStore) List(ctx context.Context) ([]string, error) {
	return s.client.List(ctx)
}
func (s *studentStore) Download(ctx context.Context, filename string) ([]byte, error) {
	blobClient := s.client.GetContainerClient().NewBlobClient(filename)

	downloadResp, err := blobClient.DownloadStream(ctx, nil)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(downloadResp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (s *studentStore) Delete(ctx context.Context, filename string) error {
	blobClient := s.client.GetContainerClient().NewBlobClient(filename)
	_, err := blobClient.Delete(ctx, nil)
	return err
}
