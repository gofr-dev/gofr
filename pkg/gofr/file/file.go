package file

type file struct {
	Name    string
	content []byte
	Size    int64
	isDir   bool
}

func (f file) IsDir() bool {
	return f.isDir
}

func (f file) Bytes() []byte {
	return f.content
}
