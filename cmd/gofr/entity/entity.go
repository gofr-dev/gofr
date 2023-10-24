package entity

import (
	"fmt"
	"os"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"gofr.dev/cmd/gofr/helper"
	"gofr.dev/cmd/gofr/migration"
	"gofr.dev/cmd/gofr/validation"
	"gofr.dev/pkg/gofr"
)

type Handler struct {
}

// Getwd returns a rooted path name corresponding to the current directory
func (h Handler) Getwd() (string, error) {
	return os.Getwd()
}

// Mkdir creates a new directory with the specified name and permission bits (before umask)
func (h Handler) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(name, perm)
}

// Chdir changes the current working directory to the named directory
func (h Handler) Chdir(dir string) error {
	return os.Chdir(dir)
}

// OpenFile opens the named file with specified flag (O_RDONLY etc.)
func (h Handler) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

// Stat returns a FileInfo describing the named file
func (h Handler) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// IsNotExist returns a boolean indicating whether the error is known to report that a file or directory does not exist
func (h Handler) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

// Help returns a formatted string containing usage instructions, flags, examples and a description
func (h Handler) Help() string {
	return helper.Generate(helper.Help{
		Example: "gofr entity -type=core -name=persons",
		Flag: `type specify the layer: core, composite or consumer
name entity name`,
		Usage:       "entity -type=<layer> -name=<entity_name>",
		Description: "creates a template and interface for an entity",
	})
}

// AddEntity creates a template and interface for an entity
func AddEntity(c *gofr.Context) (interface{}, error) {
	var h Handler

	validParams := map[string]bool{
		"h":    true,
		"name": true,
		"type": true,
	}

	mandatoryParams := []string{"type"}

	params := c.Params()

	if help := params["h"]; help != "" {
		return h.Help(), nil
	}

	err := validation.ValidateParams(params, validParams, &mandatoryParams)
	if err != nil {
		return nil, err
	}

	layer := params["type"]
	name := params["name"]

	err = addEntity(h, layer, name)
	if err != nil {
		return nil, err
	}

	return "Successfully created entity: " + name, nil
}

type invalidTypeError struct{}

// Error generates an error message indicating that the provided method is not valid
func (i invalidTypeError) Error() string {
	return "invalid type"
}

func addEntity(f fileSystem, entityType, entity string) error {
	projectDirectory, err := f.Getwd()
	if err != nil {
		return err
	}

	switch entityType {
	case "core":
		err := addCore(f, projectDirectory, entity)
		if err != nil {
			return err
		}
	case "composite":
		err := addComposite(f, projectDirectory, entity)
		if err != nil {
			return err
		}
	case "consumer":
		err := addConsumer(f, projectDirectory, entity)
		if err != nil {
			return err
		}
	default:
		return invalidTypeError{}
	}

	return nil
}

func addCore(f fileSystem, projectDirectory, entity string) error {
	path := projectDirectory + "/core"

	err := createChangeDir(f, path)
	if err != nil {
		return err
	}
	// create the interfaceFile , interface.go,  for core layer
	interfaceFile, err := f.OpenFile("interface.go", os.O_APPEND|os.O_CREATE|os.O_WRONLY, migration.RWMode)
	if err != nil {
		return err
	}

	defer interfaceFile.Close()

	err = populateInterfaceFiles(cases.Title(language.Und, cases.NoLower).String(entity), projectDirectory, "cores", interfaceFile)
	if err != nil {
		return err
	}

	entityPath := path + "/" + entity

	err = createChangeDir(f, entityPath)
	if err != nil {
		return err
	}

	err = populateEntityFile(f, projectDirectory, entityPath, entity, "core")
	if err != nil {
		return err
	}

	err = createModel(f, projectDirectory, entity)
	if err != nil {
		return err
	}

	return nil
}

func addComposite(f fileSystem, projectDirectory, entity string) error {
	compositePath := projectDirectory + "/composite"

	err := createChangeDir(f, compositePath)
	if err != nil {
		return err
	}

	interfaceFile, err := f.OpenFile("interface.go", os.O_APPEND|os.O_CREATE|os.O_WRONLY, migration.RWMode)
	if err != nil {
		return err
	}

	defer interfaceFile.Close()

	err = populateInterfaceFiles(cases.Title(language.Und, cases.NoLower).String(entity), projectDirectory, "composites", interfaceFile)
	if err != nil {
		return err
	}

	err = createChangeDir(f, compositePath+"/"+entity)
	if err != nil {
		return err
	}

	err = populateEntityFile(f, projectDirectory, compositePath+"/"+entity, entity, "composite")
	if err != nil {
		return err
	}

	return nil
}

func addConsumer(f fileSystem, projectDirectory, entity string) error {
	path := projectDirectory + "/http"

	err := createChangeDir(f, path)
	if err != nil {
		return err
	}

	err = createChangeDir(f, entity)
	if err != nil {
		return err
	}

	filePtr, err := f.OpenFile(entity+".go", os.O_APPEND|os.O_CREATE|os.O_WRONLY, migration.RWMode)
	if err != nil {
		return err
	}

	defer filePtr.Close()

	_, err = fmt.Fprintf(filePtr, "package %s", entity)
	if err != nil {
		return err
	}

	return nil
}
