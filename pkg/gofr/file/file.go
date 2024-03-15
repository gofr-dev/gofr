package file

type File struct {
	Name    string
	content []byte
	Size    int64
	isDir   bool
}

func (f *File) IsDir() bool {
	return f.isDir
}

func (f *File) Bytes() []byte {
	return f.content
}
