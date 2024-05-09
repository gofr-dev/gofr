package file

import (
	"io"
	"io/fs"
	"os"

	"gofr.dev/pkg/gofr/datasource"
)

type local struct {
	datasource.Logger
}

func New(logger datasource.Logger) local {
	return local{logger}
}

func (c local) CreateDir(name string) error {
	return os.MkdirAll(name, fs.ModePerm)
}

// Create creates the file named path along with any necessary parents,
// and writes the given data to it.
// If file exists, error is returned.
// If file does not exist, it is created with mode 0666
// Error return are of type *fs.PathError.
// name contains the file name along with the path.
func (c local) Create(name string, data []byte) error {
	// Open the file for writing with exclusive creation flag
	// os.O_WRONLY: Opens the file for writing only.
	// os.O_CREATE: Creates the file if it doesn't exist.
	// os.O_EXCL: Fails if the file already exists (exclusive creation).
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if err != nil {
		return err
	}

	defer f.Close()

	// Write data to the file
	_, err = f.Write(data)
	if err != nil {
		c.Logger.Errorf("error writing data to file: %v", err)

		return err
	}

	return nil
}

// Read reads the content of file and writes it in data.
// If there is an error, it will be of type *fs.PathError.
// name contains the file name along with the path.
func (c local) Read(path string) ([]byte, error) {
	// Open the file for reading
	f, err := os.Open(path)
	if err != nil {
		return nil, err // Return error if opening fails
	}
	defer f.Close() // Close the file even on errors

	// Allocate buffer for reading the file
	data, err := io.ReadAll(f)
	if err != nil {
		c.Logger.Errorf("error reading file: %w", err)
		return nil, err
	}

	return data, nil
}

func (c local) Update(name string, data []byte) error {
	// Open the file for writing with truncation
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	// Write data to the file
	_, err = f.Write(data)
	if err != nil {
		return err
	}

	// Success
	return nil
}

func (c local) Delete(path string) error {
	return os.RemoveAll(path)
}

func (c local) Move(src string, dest string) error {
	return os.Rename(src, dest)
}

func (c local) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}
