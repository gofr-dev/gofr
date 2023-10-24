package handler

import (
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	"gofr.dev/cmd/gofr/helper"
	"gofr.dev/cmd/gofr/migration"
	"gofr.dev/cmd/gofr/validation"
	"gofr.dev/pkg/gofr"
)

type Create struct {
}

// Mkdir creates a new directory with the specified name and permission bits (before umask)
func (c Create) Mkdir(name string, perm os.FileMode) error {
	return os.Mkdir(name, perm)
}

// Chdir changes the current working directory to the named directory
func (c Create) Chdir(dir string) error {
	return os.Chdir(dir)
}

// OpenFile opens the named file with specified flag (O_RDONLY etc.)
func (c Create) OpenFile(name string, flag int, perm os.FileMode) (*os.File, error) {
	return os.OpenFile(name, flag, perm)
}

// ReadDir reads the named directory, returning all its directory entries sorted by filename
func (c Create) ReadDir(dir string) ([]os.DirEntry, error) {
	return os.ReadDir(dir)
}

// Create creates or truncates the named file. If the file already exists, it is truncated
func (c Create) Create(name string) (*os.File, error) {
	return os.Create(name)
}

// Stat returns a FileInfo describing the named file
func (c Create) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// IsNotExist returns a boolean indicating whether the error is known to report that a file or directory does not exist
func (c Create) IsNotExist(err error) bool {
	return os.IsNotExist(err)
}

// Help returns a formatted string containing usage instructions, flags, examples and a description
func (c Create) Help() interface{} {
	return helper.Generate(helper.Help{
		Example:     `gofr migrate create -name=AddForeignKey`,
		Flag:        `name: name of the migration`,
		Usage:       "gofr migrate create -name=<migration_name>",
		Description: "creates a migration template inside migrations folder and the name of the file is the name provided in the `name` flag",
	})
}

// CreateMigration generates a migration file with a timestamp prefix in the format YYYYMMDDHHIISS, followed by the provided name
func CreateMigration(c *gofr.Context) (interface{}, error) {
	h := Create{}

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

	migrationName := params["name"]

	err = create(h, migrationName)
	if err != nil {
		return nil, err
	}

	return "Migration created: " + migrationName, nil
}

func create(f FSCreate, name string) error {
	err := createMigrationFile(f, name)
	if err != nil {
		return err
	}

	prefixes, err := getPrefixes(f)
	if err != nil {
		return err
	}

	sort.Strings(prefixes)

	err = createAllFile(f, prefixes)
	if err != nil {
		return err
	}

	return nil
}

// getPrefixes extracts the timestamp/prefix from every migration files and returns the timestamp/prefix
func getPrefixes(f FSCreate) ([]string, error) {
	var prefixes = make([]string, 0)

	files, err := f.ReadDir("./")

	if err != nil {
		return nil, err
	}

	for _, file := range files {
		fileParts := strings.Split(file.Name(), "_")
		if len(fileParts) < 2 || file.Name() == "000_all.go" || fileParts[len(fileParts)-1] == "test.go" {
			continue
		}

		prefixes = append(prefixes, fileParts[0])
	}

	return prefixes, nil
}

// createAllFile creates the file which stores all the migrations of the project in the form of a map
func createAllFile(f FSCreate, prefixes []string) error {
	// Write 000_all.go
	fAll, err := f.Create("000_all.go")
	if err != nil {
		return err
	}

	defer fAll.Close()

	var allTemplate = template.Must(template.New("000_all").Parse(
		`// This is auto-generated file using 'gofr migrate' tool. DO NOT EDIT.
package migrations

import (
	"gofr.dev/cmd/gofr/migration/dbMigration"
)

func All() map[string]dbmigration.Migrator{
	return map[string]dbmigration.Migrator{
{{range $key, $value := .}}	
		"{{ $value }}": K{{ $value }}{},{{end}}
	}
}
`))

	err = allTemplate.Execute(fAll, prefixes)
	if err != nil {
		return err
	}

	return nil
}

// createMigrationFile creates a .go file which contains the template for writing up and down migration
func createMigrationFile(f FSCreate, migrationName string) error {
	if _, err := f.Stat("migrations"); f.IsNotExist(err) {
		if er := f.Mkdir("migrations", migration.RWXMode); er != nil {
			return er
		}
	}

	if err := f.Chdir("migrations"); err != nil {
		return err
	}

	currTimeStamp := time.Now().Format("20060102150405")

	migrationName = currTimeStamp + "_" + migrationName

	migrationTemplate := template.Must(template.New("migration").Parse(`package migrations

import (
	"gofr.dev/pkg/datastore"
	"gofr.dev/pkg/log"
)

type K{{.Timestamp}} struct {
}

func (k K{{.Timestamp}}) Up(d *datastore.DataStore, logger log.Logger) error {
	return nil
}

func (k K{{.Timestamp}}) Down(d *datastore.DataStore, logger log.Logger) error {
	return nil
}
`))

	file, err := f.OpenFile(migrationName+".go", os.O_CREATE|os.O_WRONLY, migration.RWMode)
	if err != nil {
		return err
	}

	defer file.Close()

	tData := struct {
		Timestamp         string
		MigrationFileName string
	}{currTimeStamp, migrationName}

	err = migrationTemplate.Execute(file, tData)
	if err != nil {
		return err
	}

	return nil
}
