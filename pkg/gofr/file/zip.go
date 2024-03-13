package file

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
)

const maxFileSize = 100 * 1024 * 1024

type File struct {
	name    string
	isDir   bool
	content []byte
	size    int64
}

type Zip struct {
	files map[string]File
}

func NewZip(content []byte) (*Zip, error) {
	reader := bytes.NewReader(content)

	zipReader, err := zip.NewReader(reader, int64(len(content)))
	if err != nil {
		return nil, err
	}
	// Create a map to store file contents
	files := make(map[string]File)

	for _, file := range zipReader.File {
		f, err := file.Open()
		if err != nil {
			return nil, err
		}

		buf := new(bytes.Buffer)
		if _, err := io.CopyN(buf, f, maxFileSize); err != nil {
			f.Close()
			return nil, err
		}

		files[file.Name] = File{
			name:    file.Name,
			content: buf.Bytes(),
			isDir:   file.FileInfo().IsDir(),
			size:    file.FileInfo().Size(),
		}

		f.Close()
	}

	return &Zip{files: files}, nil
}

func (z *Zip) GetFiles() map[string]File {
	return z.files
}

func (z *Zip) CreateLocalCopies(dest string) error {
	for _, zf := range z.files {
		basePath, _ := os.Getwd()
		destPath := filepath.Join(basePath, dest, zf.name)

		if zf.isDir {
			err := os.MkdirAll(destPath, os.ModePerm)
			if err != nil {
				return nil
			}

			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), os.ModePerm); err != nil {
			return err
		}

		destFile, err := os.Create(destPath)
		if err != nil {
			return err
		}

		if _, err := io.Copy(destFile, bytes.NewReader(zf.content)); err != nil {
			return err
		}

		destFile.Close()
	}

	return nil
}
