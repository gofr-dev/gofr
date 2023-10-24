package entity

import "os"

type fileSystem interface {
	Getwd() (string, error)
	Chdir(dir string) error
	Mkdir(name string, perm os.FileMode) error
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	Stat(name string) (os.FileInfo, error)
	IsNotExist(err error) bool
}
