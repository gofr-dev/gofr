package file

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
)

const maxFileSize = 100 * 1024 * 1024 // 100MB

var (
	errMaxFileSize = errors.New("uncompressed file is greater than file size limit of 100MBs")
)

type Zip struct {
	Files map[string]file
}

func GenerateFile(content []byte) (*Zip, error) {
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
			Name:    zrf.Name,
			content: buf.Bytes(),
			isDir:   zrf.FileInfo().IsDir(),
			Size:    zrf.FileInfo().Size(),
		}

		f.Close()
	}

	return &Zip{Files: files}, nil
}

func (z *Zip) CreateLocalCopies(dest string) error {
	for _, zf := range z.Files {
		basePath, _ := os.Getwd()
		destPath := filepath.Join(basePath, dest, zf.Name)

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

func copyToBuffer(f io.ReadCloser, size uint64) (*bytes.Buffer, error) {
	if size > maxFileSize {
		return nil, errMaxFileSize
	}

	buf := new(bytes.Buffer)
	if n, err := io.CopyN(buf, f, maxFileSize); err != nil && err != io.EOF && uint64(n) < size {
		f.Close()

		return nil, err
	}

	return buf, nil
}
