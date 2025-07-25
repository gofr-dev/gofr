/*
Package file provides unified access to various file operations, such as creating, reading, writing files across :
- S3
- FTP
- SFTP
- Local FileSystem
*/
package file

type file struct {
	name    string
	content []byte
	size    int64
	isDir   bool
}

func (f file) GetName() string {
	return f.name
}

func (f file) GetSize() int64 {
	return f.size
}

func (f file) Bytes() []byte {
	return f.content
}

func (f file) IsDir() bool {
	return f.isDir
}
