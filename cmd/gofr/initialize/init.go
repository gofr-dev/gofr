package initialize

import (
	"os"

	"gofr.dev/cmd/gofr/helper"
	"gofr.dev/cmd/gofr/migration"
	"gofr.dev/cmd/gofr/validation"
	"gofr.dev/pkg/gofr"
)

type Handler struct {
}

// Mkdir creates a new directory with the specified name
func (h Handler) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(name, perm)
}

// Chdir changes the current working directory to the named directory
func (h Handler) Chdir(dir string) error {
	return os.Chdir(dir)
}

// Create creates or truncates a file  the specified file name
func (h Handler) Create(name string) (*os.File, error) {
	return os.Create(name)
}

// Help returns a formatted string containing usage instructions, flags, examples and a description
func (h Handler) Help() string {
	return helper.Generate(helper.Help{
		Example:     "gofr init -name=testProject",
		Flag:        "name provide the name of the project",
		Usage:       "init -name=<project_name>",
		Description: "creates a project structure inside the directory specified in the name flag",
	})
}

// Init creates a basic layout for a project
func Init(c *gofr.Context) (interface{}, error) {
	var h Handler

	validParams := map[string]bool{
		"h":    true,
		"name": true,
	}

	mandatoryParams := []string{"name"}

	params := c.Params()

	if help := params["h"]; help != "" {
		return h.Help(), nil
	}

	err := validation.ValidateParams(params, validParams, &mandatoryParams)
	if err != nil {
		return nil, err
	}

	projectName := params["name"]

	err = createProject(h, projectName)
	if err != nil {
		return nil, err
	}

	return "Successfully created project: " + projectName, nil
}

func createProject(f fileSystem, projectName string) error {
	standardDirectories := []string{
		"cmd",
		"configs",
		"internal",
	}

	standardEnvFiles := []string{
		".env",
		".test.env",
	}

	err := f.Mkdir(projectName, os.ModePerm)
	if err != nil {
		return err
	}

	err = f.Chdir(projectName)
	if err != nil {
		return err
	}

	for _, name := range standardDirectories {
		if er := f.Mkdir(name, migration.RWXMode); er != nil {
			return er
		}
	}

	mainFile, err := f.Create("main.go")
	if err != nil {
		return err
	}

	mainString := `package main

import (
	"gofr.dev/pkg/gofr"
)

func main() {
	k := gofr.New()

	// Sample Route
	k.GET("/hello", func(c *gofr.Context) (interface{}, error) {
		return "Hello World!!!", nil
	})

	// Add the routes here

	k.Start()
}
`

	_, err = mainFile.WriteString(mainString)
	if err != nil {
		_ = os.Remove(projectName)
		return err
	}

	err = createEnvFiles(f, standardEnvFiles)
	if err != nil {
		return err
	}

	return nil
}

func createEnvFiles(f fileSystem, envFiles []string) error {
	err := f.Chdir("configs")
	if err != nil {
		return err
	}

	for _, fileName := range envFiles {
		_, err = f.Create(fileName)
		if err != nil {
			return err
		}
	}

	return nil
}
