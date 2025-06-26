package file

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const (
	maxFileSize = 100 * 1024 * 1024 // 100MB
)

var (
	errMaxFileSize = errors.New("uncompressed file is greater than file size limit of 100MBs")
)

type Zip struct {
	Files map[string]file
}

func NewZip(content []byte) (*Zip, error) {
	reader := bytes.NewReader(content)

	zipReader, err := zip.NewReader(reader, int64(len(content)))
	if err != nil {
		return nil, err
	}
	// Create a map to store file contents
	files := make(map[string]file)

	for _, zrf := range zipReader.File {
		f, err := zrf.Open()
		if err != nil {
			return nil, err
		}

		buf, err := copyToBuffer(f, zrf.UncompressedSize64)
		if err != nil {
			return nil, err
		}

		files[zrf.Name] = file{
			name:    zrf.Name,
			content: buf.Bytes(),
			isDir:   zrf.FileInfo().IsDir(),
			size:    zrf.FileInfo().Size(),
		}

		f.Close()
	}

	return &Zip{Files: files}, nil
}

func (z *Zip) CreateLocalCopies(dest string) error {
	dest = filepath.Clean(dest)
	for _, zf := range z.Files {
		destPath := filepath.Clean(filepath.Join(dest, zf.name))

		if zf.isDir {
			err := os.MkdirAll(destPath, os.ModePerm)
			if err != nil {
				return err
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

func copyToBuffer(f io.ReadCloser, size uint64) (*bytes.Buffer, error) {
	// check that max file size of unzipped file is less than 100MB
	if size > maxFileSize {
		return nil, errMaxFileSize
	}

	buf := new(bytes.Buffer)
	if n, err := io.CopyN(buf, f, maxFileSize); err != nil && !errors.Is(err, io.EOF) && n < int64(size) {
		f.Close()

		return nil, err
	}

	return buf, nil
}
