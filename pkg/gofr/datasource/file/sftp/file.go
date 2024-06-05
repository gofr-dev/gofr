package sftp

import (
	"fmt"
	"gofr.dev/pkg/gofr/datasource"
	"os"
	"strings"

	"github.com/pkg/sftp"
)

type File struct {
	client *sftp.Client
	fd     *sftp.File
	reader *fileRead
}

type fileRead struct {
	content    []byte
	currentPos int64
	offset     int64
}

func fileOpen(s *sftp.Client, name string) (*File, error) {
	fd, err := s.Open(name)
	if err != nil {
		return &File{}, err
	}
	return &File{fd: fd, client: s}, nil
}

func fileCreate(s *sftp.Client, name string) (*File, error) {
	fd, err := s.Create(name)
	if err != nil {
		return &File{}, err
	}
	return &File{fd: fd, client: s}, nil
}

func (f *File) Close() error {
	return f.fd.Close()
}

func (f *File) Name() string {
	return f.fd.Name()
}

func (f *File) Stat() (os.FileInfo, error) {
	return f.fd.Stat()
}

func (f *File) Truncate(size int64) error {
	return f.fd.Truncate(size)
}

func (f *File) Read(b []byte) (n int, err error) {
	return f.fd.Read(b)
}

func (f *File) ReadAll() datasource.RowReader {
	fr := &fileRead{
		content:    nil,
		currentPos: 0,
		offset:     0,
	}

	file, err := f.fd.Stat()
	if err != nil {
		return nil
	}

	fileName := file.Name()
	content := make([]byte, 0, 2048)

	if strings.Contains(fileName, ".csv") {
		//fileType = "csv"
		_, err := f.fd.Read(content)
		if err != nil {
			return nil
		}

		fr.content = content
		fr.offset = 2048

		return fr
	} else if strings.Contains(fileName, ".json") {
		//fileType = "json"
	}

	return nil
}

func (f fileRead) Next() bool {
	//TODO implement me
	panic("implement me")
}

func (f fileRead) Scan(i ...interface{}) error {
	fmt.Println(string(f.content))

	return nil
}

func (f *File) ReadAt(b []byte, off int64) (n int, err error) {
	return f.fd.ReadAt(b, off)
}

func (f *File) Readdir(count int) (res []os.FileInfo, err error) {
	res, err = f.client.ReadDir(f.Name())
	if err != nil {
		return
	}
	if count > 0 {
		if len(res) > count {
			res = res[:count]
		}
	}
	return
}

func (f *File) Readdirnames(n int) (names []string, err error) {
	data, err := f.Readdir(n)
	if err != nil {
		return nil, err
	}
	for _, v := range data {
		names = append(names, v.Name())
	}
	return
}

func (f *File) Seek(offset int64, whence int) (int64, error) {
	return f.fd.Seek(offset, whence)
}

func (f *File) Write(b []byte) (n int, err error) {
	return f.fd.Write(b)
}

// TODO
func (f *File) WriteAt(b []byte, off int64) (n int, err error) {
	return 0, nil
}
