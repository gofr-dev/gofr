package handler

import "os"

type FSCreate interface {
	Chdir(dir string) error
	Mkdir(name string, perm os.FileMode) error
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	ReadDir(dir string) ([]os.DirEntry, error)
	Create(name string) (*os.File, error)
	Stat(name string) (os.FileInfo, error)
	IsNotExist(err error) bool
}

type FSMigrate interface {
	Getwd() (string, error)
	Chdir(dir string) error
	Mkdir(name string, perm os.FileMode) error
	OpenFile(name string, flag int, perm os.FileMode) (*os.File, error)
	Stat(name string) (os.FileInfo, error)
	IsNotExist(err error) bool
}
