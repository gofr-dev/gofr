package datasource

import "io/fs"

type File interface {
	// CreateDir creates a new directory with the specified named name,
	// along with any necessary parents with fs.ModeDir FileMode.
	// If directory already exist it will do nothing and return nil.
	// name contains the file name along with the path.
	CreateDir(name string, option ...Option) error

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
	Stat(name string) (fs.FileInfo, error)
}

type Option func(Setter)

type Setter interface {
	Apply()
}
