package main

import (
	"encoding/json"
	"flag"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"text/template"

	"gofr.dev/pkg/gofr/assert"
)

const RWXOwner, testProject = 0700, "/testGoProject"

// initializeTest function to create a sample go project in temporary directory
func initializeTest(t *testing.T) {
	dir := t.TempDir()

	_ = os.Mkdir(dir+"/example", RWXOwner)
	_ = os.Chdir(dir + "/example")

	_ = exec.Command("go", "mod", "init", "example").Run()
	_ = exec.Command("go", "mod", "edit", "-go", "1.19").Run()

	file, err := os.Create("main.go")
	if err != nil {
		t.Logf("Unable to create main.go: %v", err)
	}

	_, err = file.WriteString("package main\nfunc main() {}")
	if err != nil {
		t.Logf("Unable to write in main.go: %v", err)
	}

	err = createDockerFile()
	if err != nil {
		t.Logf("Unable to create Dockerfile: %v", err)
	}
}

// createDockerFile function to create DockerFile
func createDockerFile() error {
	dockerFileTemplate := template.Must(template.New("dockerFile").Parse(`FROM alpine:latest

RUN mkdir -p /src/build
WORKDIR  /src/build

RUN apk add --no-cache tzdata ca-certificates

COPY /main /main

EXPOSE 9000
CMD ["/main"]
`))

	file, err := os.OpenFile("Dockerfile", os.O_CREATE|os.O_WRONLY, RWXOwner)
	if err != nil {
		return err
	}

	defer file.Close()

	return dockerFileTemplate.Execute(file, nil)
}

// Test_Dockerize function to test gofr dockerize command
func Test_Dockerize(t *testing.T) {
	initializeTest(t)

	testCases := []struct {
		command string
		expOp   string
	}{
		{"gofr dockerize", "Docker image created"},
		{"gofr dockerize -name=sample-app -tag=version -os=linux -dockerfile=Dockerfile", "Docker image created"},
		{"gofr dockerize -h", "Creates docker image of the app"},
		{"gofr dockerize -namee=test", "unknown parameter(s) [namee]. Run gofr <command_name> -h for help of the command."},
	}

	for _, tc := range testCases {
		assert.CMDOutputContains(t, main, tc.command, tc.expOp)
	}
}

// Test_Dockerize_Fail function to test the gofr dockerize command for failure case
func Test_Dockerize_Fail(t *testing.T) {
	initializeTest(t)

	_ = os.RemoveAll("Dockerfile")

	assert.CMDOutputContains(t, main, "gofr dockerize", "Cannot dockerize")
}

// Test_Dockerize_Build_Fail function to test the gofr dockerize command for failure case of binary build
func Test_Dockerize_Build_Fail(t *testing.T) {
	dir := t.TempDir()
	_ = os.Chdir(dir)

	assert.CMDOutputContains(t, main, "gofr dockerize", "Cannot create binary")
}

// Test_Dockerize function to test gofr dockerize command
func Test_DockerizeRun(t *testing.T) {
	initializeTest(t)

	testCases := []struct {
		command string
		expOp   string
	}{
		{"gofr dockerize run", "Docker image running"},
		{"gofr dockerize run -h", "Creates and run docker container of the app"},
		{"gofr dockerize run -t=testTag", "unknown parameter(s) [t]. Run gofr <command_name> -h for help of the command."},
	}

	for _, tc := range testCases {
		assert.CMDOutputContains(t, main, tc.command, tc.expOp)
	}
}

// Test_Dockerize_Fail function to test the gofr dockerize command for failure case
func Test_DockerizeRun_Fail(t *testing.T) {
	initializeTest(t)

	_ = os.RemoveAll("Dockerfile")

	assert.CMDOutputContains(t, main, "gofr dockerize run", "Cannot dockerize")
}

// Test_Dockerize_Build_Fail function to test the gofr dockerize command for failure case of binary build
func Test_DockerizeRun_Build_Fail(t *testing.T) {
	dir := t.TempDir()
	_ = os.Chdir(dir)

	assert.CMDOutputContains(t, main, "gofr dockerize run", "Cannot create binary")
}

func TestCLI(t *testing.T) {
	dir := t.TempDir()
	_ = os.Chdir(dir)

	flag.String("name", "", "")
	flag.String("methods", "", "")
	flag.String("path", "", "")
	flag.String("type", "", "")

	assert.CMDOutputContains(t, main, "gofr init -name=testGoProject", "Successfully created project: testGoProject")

	_ = os.Chdir(dir + testProject)

	assert.CMDOutputContains(t, main, "gofr init -namee=testGoProject",
		"unknown parameter(s) [namee]. Run gofr <command_name> -h for help of the command.")

	_ = os.Chdir(dir + testProject)

	assert.CMDOutputContains(t, main, "gofr add -methods=all -path=/foo", "Added route: /foo")

	_ = os.Chdir(dir + testProject)

	assert.CMDOutputContains(t, main, "gofr add -method=all -path=/foo",
		"unknown parameter(s) [method]. Run gofr <command_name> -h for help of the command.")

	_ = os.Chdir(dir + testProject)

	assert.CMDOutputContains(t, main, "gofr add -methods= -path=/foo",
		"Parameter methods is required for this request")

	_ = os.Chdir(dir + testProject)

	assert.CMDOutputContains(t, main, "gofr add -methods=all -path=", "Parameter path is required for this request")

	_ = os.Chdir(dir + testProject)

	assert.CMDOutputContains(t, main, "gofr add -methods=all -path=/foo", "route foo is already present")

	_ = os.Chdir(dir + testProject)

	assert.CMDOutputContains(t, main, "gofr entity -type=core -name=person", "Successfully created entity: person")

	_ = os.Chdir(dir + testProject)

	assert.CMDOutputContains(t, main, "gofr entity -type= -name=person",
		"Parameter type is required for this request")

	_ = os.Chdir(dir + testProject)

	assert.CMDOutputContains(t, main, "gofr entity -typee=core -namee=person",
		"unknown parameter(s) [namee,typee]. Run gofr <command_name> -h for help of the command.")
}

func Test_Migrate(t *testing.T) {
	currDir := t.TempDir()
	_ = os.Chdir(currDir)

	assert.CMDOutputContains(t, main, "gofr migrate -method=ABOVE -database=gorm", "migrations do not exists! "+
		"If you have created migrations please run the command from the project's root directory")
	assert.CMDOutputContains(t, main, "gofr migrate -method=UP -database=gorm", "migrations do not exists")
	assert.CMDOutputContains(t, main, "gofr migrate -method=UP -database=", "Parameter database is required for this request")

	path, _ := os.MkdirTemp(currDir, "migrateCreateTest")
	defer os.RemoveAll(path)

	_ = os.Chdir(path)

	assert.CMDOutputContains(t, main, "gofr migrate create", "Parameter name is required for this request")
	assert.CMDOutputContains(t, main, "gofr migrate create -name=testMigration", "Migration created")

	assert.CMDOutputContains(t, main, "gofr migrate create -name=migrationTest", "Migration created")

	assert.CMDOutputContains(t, main, "gofr migrate -method=UP -database=gorm", "migrations do not exists")

	assert.CMDOutputContains(t, main, "gofr migrate -method=DOWN -database=gorm", "migrations do not exists")

	assert.CMDOutputContains(t, main, "gofr migrate -method=DOWN -database=mongo", "migrations do not exists")

	assert.CMDOutputContains(t, main, "gofr migrate -method=DOWN -database=cassandra", "migrations do not exists")

	assert.CMDOutputContains(t, main, "gofr migrate -method=DOWN -database=ycql", "migrations do not exists")

	assert.CMDOutputContains(t, main, "gofr migrate -method=DOWN -database=redis -tag=20200123143215", "migrations do not exists")

	assert.CMDOutputContains(t, main, "gofr migrate -methods=DOWN -databases=redis -tagged",
		"unknown parameter(s) [databases,methods,tagged]. Run gofr <command_name> -h for help of the command.")
}

func Test_CreateMigration(t *testing.T) {
	path, _ := os.MkdirTemp("", "migrationTest")

	defer os.RemoveAll(path)

	err := os.Chdir(path)
	if err != nil {
		t.Errorf("Error while changing directory:\n%+v", err)
	}

	assert.CMDOutputContains(t, main, "gofr migrate create -name=removeColumn", "Migration created: removeColumn")

	assert.CMDOutputContains(t, main, "gofr migrate create", "Parameter name is required for this request")

	assert.CMDOutputContains(t, main, "gofr migrate create -namee=test",
		"unknown parameter(s) [namee]. Run gofr <command_name> -h for help of the command.")
}

func Test_Integration(t *testing.T) {
	assert.CMDOutputContains(t, main, "gofr help", "Available Commands")
}

func Test_HelpGenerate(t *testing.T) {
	assert.CMDOutputContains(t, main, "gofr init -h", "creates a project structure inside the directory specified in the name flag")
	assert.CMDOutputContains(t, main, "gofr entity -h", "creates a template and interface for an entity")
	assert.CMDOutputContains(t, main, "gofr add -h", "add routes and creates a handler template")
	assert.CMDOutputContains(t, main, "gofr migrate -h", "usage: gofr migrate")
	assert.CMDOutputContains(t, main, "gofr migrate create -h", "usage: gofr migrate create")
}

//nolint:funlen // reducing the function length reduces readability
func Test_test_Success(t *testing.T) {
	const ymlStr = `openapi: 3.0.1
info:
  title: LogisticsAPI
  version: '0.1'
servers:
  - url: 'http://api.staging.gofr.dev'
paths:
  /hello-world:
    get:
      tags:
        - Hello
      description: Sample API Hello
      responses:
        '200':
          description: Sample API Hello
  /hello:
    get:
      tags:
        - Hello
      description: Sample API Hello with name
      parameters:
        - name: X-Correlation-ID
          in: header
          schema:
            type: string
            format: uuid
          example: 
        - name: custom-header
          in: header
          schema:
            type: string
            format: uuid
          example: 'abc,xyz,ijk'
        - name: name
          in: query
          schema:
            type: string
          example: 'Roy'
        - name: age
          in: body
          schema:
            type: float
          example: 32189.5
        - name: hasAcc
          in: query
          schema:
            type: bool
          example: true
        - name: nick_names
          in: query
          schema:
            type: array
          example: [abc, def, ghi]
      responses:
        '200':
          description: Sample API Hello with name
    post:
      tags:
        - Hello
      description: Sample API Hello with name
      parameters:
        - name: X-Correlation-ID
          in: header
          schema:
            type: string
            format: uuid
          example: 
        - name: custom-header
          in: header
          schema:
            type: string
            format: uuid
          example: 'abc,xyz,ijk'
        - name: id
          in: path
          schema:
            type: int
          example: 5
        - name: catalog_item
          in: body
          schema:
            type: object
            properties:
              id:
                type: integer
              name:
                type: string
            required:
              - id
              - name
          example:
            id: 38
            name: T-shirt
            salary: 452.05
      responses:
        '200':
          description: Sample API Hello with name`

	d1 := []byte(ymlStr)

	tempFile, err := os.CreateTemp(t.TempDir(), "dat1.yml")
	if err != nil {
		t.Error(err)
	}

	_, err = tempFile.Write(d1)
	if err != nil {
		t.Error(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode("{}")
	}))
	hostPort := strings.Replace(server.URL, "http://", "", 1)
	assert.CMDOutputContains(t, main, "gofr test -host="+hostPort+" -source="+tempFile.Name(), "Test Passed!")
}

func Test_test_Error(t *testing.T) {
	const ymlStr = `openapi: 3.0.1
info:
  title: LogisticsAPI
  version: '0.1'
servers:
  - url: 'http://api.staging.gofr.dev'
paths:
  /hello/{id}:
    put:
      tags:
        - Hello
      description: Sample API Hello with name
      parameters:
        - name: id
          in: path
          schema:
            type: int
          example: 5
        - name: catalog_item
          in: body
          schema:
            type: object
            properties:
              id:
                type: integer
              name:
                type: string
            required:
              - id
              - name
          example:
            id: 38
            name: T-shirt
            salary: 452.05
      responses:
        '403':
          description: Sample API Hello with name
    delete:
      tags:
        - Post Hello
      description: Sample API Hello with name
      parameters:
        - name: id
          in: path
          schema:
            type: int
          example: 5
      responses:
        '400':
          description: Sample API Hello`

	const gofrTestHost = "gofr test -host="

	d1 := []byte(ymlStr)

	tempFile, err := os.CreateTemp(t.TempDir(), "dat1.yml")
	if err != nil {
		t.Error(err)
	}

	_, err = tempFile.Write(d1)
	if err != nil {
		t.Error(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode("{}")
	}))
	hostPort := strings.Replace(server.URL, "http://", "", 1)
	assert.CMDOutputContains(t, main, gofrTestHost+hostPort+" -source="+tempFile.Name(), "failed")

	// case to check test help
	assert.CMDOutputContains(t, main, "gofr test -h", "runs integration test for a given configuration")

	// case when wrong argument is passed
	assert.CMDOutputContains(t, main, "gofr test -hosts=test",
		"unknown parameter(s) [hosts]. Run gofr <command_name> -h for help of the command.")

	// case when source not specified
	assert.CMDOutputContains(t, main, gofrTestHost+hostPort, "Parameter source is required for this request")

	// case when host not specified
	assert.CMDOutputContains(t, main, "gofr test -source="+tempFile.Name(), "Parameter host is required for this request")

	// case when source is incorrect
	assert.CMDOutputContains(t, main, gofrTestHost+hostPort+" -source=/some/fake/path/data.yml", "no such file or directory")
}
