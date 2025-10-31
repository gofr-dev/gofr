//go:generate mockgen -source=interface.go -destination=mock_interface.go -package=gcs

package gcs

import (
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/file"
)

// Logger interface redefines the logger interface for the package.
type Logger interface {
	datasource.Logger
}

type Metrics interface {
	file.StorageMetrics
}
