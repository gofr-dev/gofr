package initialize

import "os"

type fileSystem interface {
	Chdir(dir string) error
	Mkdir(name string, perm os.FileMode) error
	Create(name string) (*os.File, error)
}
