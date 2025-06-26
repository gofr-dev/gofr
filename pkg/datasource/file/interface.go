package file

import "context"

type File interface {
    Upload(ctx context.Context, fileName string, data []byte) error
    Download(ctx context.Context, fileName string) ([]byte, error)
    Delete(ctx context.Context, fileName string) error
    List(ctx context.Context) ([]string, error)
}
