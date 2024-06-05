package file

import (
	"bufio"
	"errors"
	"gofr.dev/pkg/gofr/datasource"
	"os"
)

type file struct {
	*os.File
}

type fileRead struct {
	scanner *bufio.Scanner
}

func (f file) ReadAll() datasource.RowReader {
	reader := &fileRead{scanner: bufio.NewScanner(f.File)}

	return reader
}

func (f fileRead) Next() bool {
	return f.scanner.Scan()
}

func (f fileRead) Scan(i ...interface{}) error {
	if len(i) != 1 {
		return errors.New("scan expects a single destination variable")
	}

	switch target := i[0].(type) {
	case *string:
		*target = f.scanner.Text()
		return nil
	default:
		return errors.New("scan destination must be a string pointer")
	}
}
