package addroute

import "os"

func createChangeDir(f fileSystem, name string) error {
	if _, err := f.Stat(name); f.IsNotExist(err) {
		if err := f.Mkdir(name, os.ModePerm); err != nil {
			return err
		}
	}

	err := f.Chdir(name)

	return err
}
