package service

import (
	"context"
	"io"
"gofr.dev/internal/store"
)

type StudentService interface {
	UploadFile(ctx context.Context, filename string, data io.Reader) error
	ListFiles(ctx context.Context) ([]string, error)
	DownloadFile(ctx context.Context, filename string) ([]byte, error)
	DeleteFile(ctx context.Context, filename string) error
}

type studentService struct {
	store store.StudentStore
}

func New(s store.StudentStore) StudentService {
	return &studentService{store: s}
}

func (s *studentService) UploadFile(ctx context.Context, filename string, data io.Reader) error {
	return s.store.Upload(ctx, filename, data)
}

func (s *studentService) ListFiles(ctx context.Context) ([]string, error) {
	return s.store.List(ctx)
}
func (s *studentService) DownloadFile(ctx context.Context, filename string) ([]byte, error) {
	return s.store.Download(ctx, filename)
}

func (s *studentService) DeleteFile(ctx context.Context, filename string) error {
	return s.store.Delete(ctx, filename)
}

