package entity

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"gofr.dev/cmd/gofr/migration"
)

func createChangeDir(f fileSystem, name string) error {
	if _, err := f.Stat(name); f.IsNotExist(err) {
		if err := f.Mkdir(name, os.ModePerm); err != nil {
			return err
		}
	}

	err := f.Chdir(name)

	return err
}

func createModel(f fileSystem, projectDirectory, entity string) error {
	err := createChangeDir(f, projectDirectory+"/models")
	if err != nil {
		return err
	}

	modelFile, err := f.OpenFile(entity+".go", os.O_APPEND|os.O_CREATE|os.O_WRONLY, migration.RWMode)
	if err != nil {
		return err
	}

	fi, err := modelFile.Stat()
	if err != nil {
		return err
	}

	if fi.Size() == 0 {
		modelString := fmt.Sprintf(`package models

type %s struct {
		// Add the required fields
	}`, cases.Title(language.Und, cases.NoLower).String(entity))

		_, err = modelFile.WriteString(modelString)
		if err != nil {
			return err
		}
	}

	return nil
}

func populateInterfaceFiles(entity, projectDirectory, types string, interfaceFile *os.File) error {
	fi, err := interfaceFile.Stat()
	if err != nil {
		return err
	}

	var modelDirectory string

	if filepath.Base(projectDirectory) == "gofr" {
		modelDirectory = ""
	} else {
		modelDirectory = filepath.Base(projectDirectory) + "/"
	}

	fileContent := ""
	if fi.Size() < 1 {
		fileContent += fmt.Sprintf(`package %s`, types)
	}

	fileContent += fmt.Sprintf(`

import (
	"`+modelDirectory+`models"
	"gofr.dev/pkg/gofr"
)

type %s interface {
	Find(ctx *gofr.Context, model *models.%s) ([]*models.%s, error)
	Retrieve(ctx *gofr.Context, id string) (*models.%s, error)
	Update(ctx *gofr.Context, model *models.%s) (*models.%s, error)
	Create(ctx *gofr.Context, model *models.%s) (*models.%s, error)
	DeleteById(ctx *gofr.Context, id string) error
}
`, cases.Title(language.Und, cases.NoLower).String(entity), entity, entity, entity, entity, entity, entity, entity)

	_, err = interfaceFile.WriteString(fileContent)
	if err != nil {
		return err
	}

	return nil
}

func populateEntityFile(f fileSystem, projectDirectory, layerDirectory, entity, types string) error {
	err := f.Chdir(layerDirectory)
	if err != nil {
		return err
	}

	entityFile, err := f.OpenFile(entity+".go", os.O_APPEND|os.O_CREATE|os.O_WRONLY, migration.RWOwner)
	if err != nil {
		return err
	}

	defer entityFile.Close()

	M := cases.Title(language.Und, cases.NoLower).String(entity)

	fi, err := entityFile.Stat()
	if err != nil {
		return err
	}

	var modelDirectory string

	if filepath.Base(projectDirectory) == "gofr" {
		modelDirectory = ""
	} else {
		modelDirectory = filepath.Base(projectDirectory) + "/"
	}

	fileContent := ""
	if fi.Size() < 1 {
		fileContent += fmt.Sprintf(`package %s`, entity)
	}

	fileContent += fmt.Sprintf(`

import (
	"database/sql"
	"`+modelDirectory+`models"
	"gofr.dev/pkg/gofr"
)

type %s struct {
	DB *sql.DB
}

func New(db *sql.DB) *%s {
	return &%s{DB: db}
}

// Find fetch the expected parameters for all the entities present
func (%s *%s) Find(ctx *gofr.Context, model *models.%s) ([]*models.%s, error) {
	// Your code goes here
	return nil, nil
}

// Create store a new entity entry in database
func (%s *%s) Create(ctx *gofr.Context, model *models.%s) (b *models.%s, err error) {
	// Your code goes here
	return nil, nil
}

// Retrieve fetch an entry from database with particular id
func (%s *%s) Retrieve(ctx *gofr.Context, id string) (b *models.%s, err error) {
	// Your code goes here
	return nil, nil
}

// Update edit few parameters of the entity in database
func (%s *%s) Update(ctx *gofr.Context, model *models.%s) (b *models.%s, err error) {
	// Your code goes here
	return nil, nil
}

// Delete delete and entry from database whose id is provided
func (%s *%s) Delete(ctx *gofr.Context, id string) error {
	// Your code goes here
	return nil
}
`, entity, entity, entity, types, entity, M, M, types, entity, M, M, types, entity, M, types, entity, M, M, types, entity)

	_, err = entityFile.WriteString(fileContent)
	if err != nil {
		return err
	}

	return nil
}
