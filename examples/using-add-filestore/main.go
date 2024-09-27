package main

import (
	"fmt"
	"strconv"
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

// This can be a common function to configure both FTP and SFTP server
func configureFileServer(app *gofr.App) file.FileSystemProvider {
	port, _ := strconv.Atoi(app.Config.Get("PORT"))
	fileSystemProvider := ftp.New(&ftp.Config{
		Host:      app.Config.Get("HOST"),
		User:      app.Config.Get("USER_NAME"),
		Password:  app.Config.Get("PASSWORD"),
		Port:      port,
		RemoteDir: app.Config.Get("REMOTE_DIR_PATH"),
	})
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

func grepFiles(files []file.FileInfo, keyword string, err error) {
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

func registerPwdCommand(app *gofr.App, fs file.FileSystemProvider) {
	app.SubCommand("pwd", func(c *gofr.Context) (interface{}, error) {
		workingDirectory, error := fs.Getwd()
		return workingDirectory, error
	})
}

func registerLsCommand(app *gofr.App, fs file.FileSystemProvider) {
	app.SubCommand("ls", func(c *gofr.Context) (interface{}, error) {
		path := c.Param("path")
		files, error := fs.ReadDir(path)
		printFiles(files, error)
		return "", error
	})
}

func registerGrepCommand(app *gofr.App, fs file.FileSystemProvider) {
	app.SubCommand("grep", func(c *gofr.Context) (interface{}, error) {
		keyword := c.Param("keyword")
		path := c.Param("path")
		files, error := fs.ReadDir(path)
		grepFiles(files, keyword, error)
		return "", error
	})
}

func registerCreateFileCommand(app *gofr.App, fs file.FileSystemProvider) {
	app.SubCommand("createfile", func(c *gofr.Context) (interface{}, error) {
		fileName := c.Param("filename")
		fmt.Printf("Creating file :%s", fileName)
		_, error := fs.Create(fileName)
		if error == nil {
			fmt.Printf("Succesfully created file:%s", fileName)
		}
		return "", error
	})
}

func registerRmCommand(app *gofr.App, fs file.FileSystemProvider) {
	app.SubCommand("rm", func(c *gofr.Context) (interface{}, error) {
		fileName := c.Param("filename")
		fmt.Printf("Removing file :%s", fileName)
		error := fs.Remove(fileName)
		if error == nil {
			fmt.Printf("Succesfully removed file:%s", fileName)
		}
		return "", error
	})
}

func main() {
	app := gofr.NewCMD()

	fileSystemProvider := configureFileServer(app)

	app.AddFileStore(fileSystemProvider)

	registerPwdCommand(app, fileSystemProvider)

	registerLsCommand(app, fileSystemProvider)

	registerGrepCommand(app, fileSystemProvider)

	registerCreateFileCommand(app, fileSystemProvider)

	registerRmCommand(app, fileSystemProvider)

	app.Run()
}
