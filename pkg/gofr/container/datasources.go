package container

import (
	"context"
	"database/sql"
	"io/fs"

	"github.com/redis/go-redis/v9"

	"gofr.dev/pkg/gofr/datasource"
	gofrSQL "gofr.dev/pkg/gofr/datasource/sql"
)

type DB interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
	QueryRow(query string, args ...interface{}) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	Exec(query string, args ...interface{}) (sql.Result, error)
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	Prepare(query string) (*sql.Stmt, error)
	Begin() (*gofrSQL.Tx, error)
	Select(ctx context.Context, data interface{}, query string, args ...interface{})
	HealthCheck() *datasource.Health
	Dialect() string
}

type Redis interface {
	redis.Cmdable
	redis.HashCmdable
	HealthCheck() datasource.Health
}

type File interface {
	// CreateDir creates a new directory with the specified named name,
	// along with any necessary parents with fs.ModeDir FileMode.
	// If directory already exist it will do nothing and return nil.
	// name contains the file name along with the path.
	CreateDir(name string) error

	// Create creates the file named path along with any necessary parents,
	// and writes the given data to it.
	// If file exists, error is returned.
	// If file does not exist, it is created with mode 0666
	// Error return are of type *fs.PathError.
	// name contains the file name along with the path.
	Create(name string, data []byte) error

	// Read reads the content of file and writes it in data.
	// If there is an error, it will be of type *fs.PathError.
	// name contains the file name along with the path.
	Read(name string) ([]byte, error)

	// Move moves the file from src to dest, along with any necessary parents for dest location.
	// If there is an error, it will be of type *fs.PathError.
	// src and dest contains the filename along with path
	Move(src string, dest string) error

	// Update rewrites file named path with data, if file doesn't exist, error is returned.
	// name contains the file name along with the path.
	Update(name string, data []byte) error

	// Delete deletes the file at given path, if no file/directory exist nil is returned.
	// name contains the file name along with the path.
	Delete(name string) error

	// Stat returns stat for the file.
	// name contains the file name along with the path.
	// os.IsExist() can
	Stat(name string) (fs.FileInfo, error)
}
