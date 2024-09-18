package main

import (
	"fmt"
	"strings"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/datasource/file"
	"gofr.dev/pkg/gofr/datasource/file/ftp"
)

type FileServerType int

const (
	FTP FileServerType = iota
	SFTP
)

// this can be a common function to configure both ftp and SFTP server
func configureFTPServer(fileServerType FileServerType) file.FileSystemProvider {
	var fileSystemProvider file.FileSystemProvider
	if fileServerType == FTP {
		fileSystemProvider = ftp.New(&ftp.Config{
			Host:      "localhost",
			User:      "anonymous",
			Password:  "",
			Port:      21,
			RemoteDir: "/",
		})
	} else {
		// fileSystemProvider = sftp.New(&ftp.Config{
		// 	Host:      "localhost",
		// 	User:      "anonymous",
		// 	Password:  "",
		// 	Port:      21,
		// 	RemoteDir: "/",
		// })
	}
	return fileSystemProvider
}

func printFiles(files []file.FileInfo, err error) {
	if err != nil {
		fmt.Println(err)
	} else {
		for _, f := range files {
			fmt.Println(f.Name())
		}
	}
}

func filterFiles(files []file.FileInfo, keyword string, err error) {
	if err != nil {
		fmt.Println(err)
	} else {
		for _, f := range files {
			if strings.HasPrefix(f.Name(), keyword) {
				fmt.Println(f.Name())
			}
		}
	}
}

func main() {
	app := gofr.NewCMD()

	fileSystemProvider := configureFTPServer(FTP)

	app.AddFileStore(fileSystemProvider)

	app.SubCommand("pwd", func(c *gofr.Context) (interface{}, error) {
		workingDirectory, error := fileSystemProvider.Getwd()
		return workingDirectory, error
	})

	app.SubCommand("ls", func(c *gofr.Context) (interface{}, error) {
		files, error := fileSystemProvider.ReadDir("/")
		printFiles(files, error)
		return "", nil
	})

	app.SubCommand("grep", func(c *gofr.Context) (interface{}, error) {
		keyword := c.Param("keyword")
		files, error := fileSystemProvider.ReadDir("/")
		filterFiles(files, keyword, error)
		return "", nil
	})

	app.SubCommand("createfile", func(c *gofr.Context) (interface{}, error) {
		fileName := c.Param("filename")
		fmt.Printf("Creating file :%s", fileName)
		_, error := fileSystemProvider.Create(fileName)
		return "", error
	})

	app.SubCommand("rm", func(c *gofr.Context) (interface{}, error) {
		fileName := c.Param("filename")
		fmt.Printf("Removing file :%s", fileName)
		error := fileSystemProvider.Remove(fileName)
		return "", error
	})

	app.Run()
}
