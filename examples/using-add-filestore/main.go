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

func main() {
	app := gofr.NewCMD()

	fileSystemProvider := configureFileServer(app)

	app.AddFileStore(fileSystemProvider)

	app.SubCommand("pwd", pwdCommandHandler)
	app.SubCommand("ls", lsCommandHandler)
	app.SubCommand("grep", grepCommandHandler)
	app.SubCommand("createfile", createFileCommandHandler)
	app.SubCommand("rm", rmCommandHandler)

	app.Run()
}

func pwdCommandHandler(c *gofr.Context) (any, error) {
	workingDirectory, err := c.File.Getwd()

	return workingDirectory, err
}

func lsCommandHandler(c *gofr.Context) (any, error) {
	path := c.Param("path")

	files, err := c.File.ReadDir(path)
	if err != nil {
		return nil, err
	}

	printFiles(files)

	return "", err
}

func grepCommandHandler(c *gofr.Context) (any, error) {
	keyword := c.Param("keyword")
	path := c.Param("path")

	files, err := c.File.ReadDir(path)

	if err != nil {
		return nil, err
	}

	grepFiles(files, keyword)

	return "", err
}

func createFileCommandHandler(c *gofr.Context) (any, error) {
	fileName := c.Param("filename")

	_, err := c.File.Create(fileName)
	if err != nil {
		return fmt.Sprintln("File Creation error"), err
	}

	return fmt.Sprintln("Successfully created file:", fileName), nil
}

func rmCommandHandler(c *gofr.Context) (any, error) {
	fileName := c.Param("filename")

	err := c.File.Remove(fileName)
	if err != nil {
		return fmt.Sprintln("File removal error"), err
	}

	return fmt.Sprintln("Successfully removed file:", fileName), nil
}

// This can be a common function to configure both FTP and SFTP server.
func configureFileServer(app *gofr.App) file.FileSystemProvider {
	port, _ := strconv.Atoi(app.Config.Get("PORT"))

	return ftp.New(&ftp.Config{
		Host:      app.Config.Get("HOST"),
		User:      app.Config.Get("USER_NAME"),
		Password:  app.Config.Get("PASSWORD"),
		Port:      port,
		RemoteDir: app.Config.Get("REMOTE_DIR_PATH"),
	})
}

func printFiles(files []file.FileInfo) {
	for _, f := range files {
		fmt.Println(f.Name())
	}
}

func grepFiles(files []file.FileInfo, keyword string) {
	for _, f := range files {
		if strings.HasPrefix(f.Name(), keyword) {
			fmt.Println(f.Name())
		}
	}
}
