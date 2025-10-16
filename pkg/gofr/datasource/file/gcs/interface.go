//go:generate mockgen -source=interface.go -destination=mock_interface.go -package=gcs

package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"

	"cloud.google.com/go/storage"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/file"
	"google.golang.org/api/iterator"
)

// Logger interface redefines the logger interface for the package.
type Logger interface {
	datasource.Logger
}
type gcsClientImpl struct {
	client *storage.Client
	bucket *storage.BucketHandle
}

// TODO: Future improvement - Refactor both S3 and GCS implementations to use a
// common CloudStorageClient interface for better abstraction. This implementation
// currently follows the same pattern as S3 to maintain consistency with existing code.
type gcsClient interface {
	NewWriter(ctx context.Context, name string) io.WriteCloser
	NewReader(ctx context.Context, name string) (io.ReadCloser, error)
	DeleteObject(ctx context.Context, name string) error
	CopyObject(ctx context.Context, src, dst string) error
	ListObjects(ctx context.Context, prefix string) ([]string, error)
	ListDir(ctx context.Context, prefix string) ([]*storage.ObjectAttrs, []string, error)
	StatObject(ctx context.Context, name string) (*storage.ObjectAttrs, error)
}

type Metrics interface {
	file.StorageMetrics
}

func (g *gcsClientImpl) NewWriter(ctx context.Context, name string) io.WriteCloser {
	return g.bucket.Object(name).NewWriter(ctx)
}

func (g *gcsClientImpl) NewReader(ctx context.Context, name string) (io.ReadCloser, error) {
	return g.bucket.Object(name).NewReader(ctx)
}

func (g *gcsClientImpl) DeleteObject(ctx context.Context, name string) error {
	attrs, err := g.bucket.Object(name).Attrs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get object attributes: %w", err)
	}

	err = g.bucket.Object(name).If(storage.Conditions{GenerationMatch: attrs.Generation}).Delete(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	return nil
}

func (g *gcsClientImpl) CopyObject(ctx context.Context, src, dst string) error {
	srcObj := g.bucket.Object(src)
	dstObj := g.bucket.Object(dst)
	_, err := dstObj.CopierFrom(srcObj).Run(ctx)

	return err
}

func (g *gcsClientImpl) ListObjects(ctx context.Context, prefix string) ([]string, error) {
	var objects []string

	it := g.bucket.Objects(ctx, &storage.Query{Prefix: prefix})

	for {
		obj, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}

		if err != nil {
			return nil, err
		}

		objects = append(objects, obj.Name)
	}

	return objects, nil
}

func (g *gcsClientImpl) ListDir(ctx context.Context, prefix string) ([]*storage.ObjectAttrs, []string, error) {
	var attrs []*storage.ObjectAttrs

	var prefixes []string

	it := g.bucket.Objects(ctx, &storage.Query{
		Prefix:    prefix,
		Delimiter: "/",
	})

	for {
		obj, err := it.Next()

		if errors.Is(err, iterator.Done) {
			break
		} else if err != nil {
			return nil, nil, err
		}

		if obj.Prefix != "" {
			prefixes = append(prefixes, obj.Prefix)
		} else {
			attrs = append(attrs, obj)
		}
	}

	return attrs, prefixes, nil
}

func (g *gcsClientImpl) StatObject(ctx context.Context, name string) (*storage.ObjectAttrs, error) {
	return g.bucket.Object(name).Attrs(ctx)
}
