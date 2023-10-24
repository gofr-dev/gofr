package addroute

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gofr.dev/cmd/gofr/helper"
	"gofr.dev/cmd/gofr/migration"
	"gofr.dev/cmd/gofr/validation"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/log"
)

type Handler struct{}

// Match reports whether the byte slice b contains any match of the regular expression pattern
func (h Handler) Match(pattern string, b []byte) (bool, error) {
	return regexp.Match(pattern, b)
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
		Example: `gofr add -methods=GET,POST -path=/person
gofr add -methods=ALL -path=/person/{id}`,
		Flag:        "methods GET,POST,PUT,DELETE or ALL. Comma separated values accepted and ALL accepted",
		Usage:       "add -methods=<GET,POST,PUT,DELETE> -path=<path_name>",
		Description: "add routes and creates a handler template",
	})
}

// AddRoute adds new route with the specified path and method
func AddRoute(c *gofr.Context) (interface{}, error) {
	var h Handler

	validParams := map[string]bool{
		"h":       true,
		"methods": true,
		"path":    true,
	}

	mandatoryParams := []string{"methods", "path"}

	params := c.Params()

	if help := params["h"]; help != "" {
		return h.Help(), nil
	}

	err := validation.ValidateParams(params, validParams, &mandatoryParams)
	if err != nil {
		return nil, err
	}

	methods := params["methods"]
	path := params["path"]

	err = addRoute(h, methods, path)
	if err != nil {
		return nil, err
	}

	return "Added route: " + path, nil
}

type invalidMethodError struct {
	name string
}

// Error generates an error message indicating that the provided method is not valid
func (i invalidMethodError) Error() string {
	return i.name + " is not a valid method"
}

type invalidPathError struct {
	path string
}

// Error generates an error message indicating that the provided path is not valid
func (i invalidPathError) Error() string {
	return i.path + " is an invalid path"
}

type pathExistsError struct {
	path   string
	method string
	line   int
	file   string
}

// Error generates an error message indicating that the provided path already exists
func (i pathExistsError) Error() string {
	err := "route " + i.path + " is already present "

	if i.method != "" {
		err += "for the methods:-  " + i.method
	}

	if i.line != 0 {
		err += " at line number: " + strconv.Itoa(i.line) + " in file: " + i.file
	}

	return err
}

// addRoute adds a new route and Handler
// creates a file for the route and generates a template for the Handler in the file created
func addRoute(f fileSystem, methods, path string) error {
	if methods == "" {
		methods = "ALL"
	}

	methods = strings.ToUpper(methods)
	path = strings.Trim(path, "/")
	pathDirectory := path
	params := strings.Index(path, "{")

	// separates the path params from the path
	if params > -1 {
		pathDirectory = path[0 : params-1]
	}

	path = strings.Trim(path, "/")

	if !validatePath(f, path) {
		return invalidPathError{path: path}
	}

	err := processRoute(f, methods, path, pathDirectory)
	if err != nil {
		return err
	}

	return nil
}

func processRoute(f fileSystem, methodFlag, path, pathDirectory string) error {
	var methods map[string]bool

	if methodFlag != "ALL" {
		inputMethods := strings.Split(methodFlag, ",")
		methods = removeDuplicates(inputMethods) // Remove duplicates methods, if passed in the --methods flag
	}

	readFile, err := f.OpenFile("main.go", os.O_RDONLY, migration.RWMode)
	if err != nil {
		return err
	}

	defer readFile.Close()

	handlerString, mainString, err := validChecks(methods, path, readFile)
	if err != nil {
		return err
	}

	if mainString == "" && handlerString == "" {
		return pathExistsError{path: path}
	}

	err = populateMain(f, mainString, path) // writes the route to main.go
	if err != nil {
		return err
	}

	err = populateHandler(f, pathDirectory, handlerString) // creates the template for the Handler inside http/entity/entity.go
	if err != nil {
		return err
	}

	return nil
}

func validChecks(methods map[string]bool, path string, readFile *os.File) (hs, ms string, err error) {
	logger := log.NewLogger()

	// name of the handler for the method
	methodFuncName := map[string]string{"GET": "Index", "PUT": "Update", "POST": "Create", "DELETE": "Delete"}

	// supported methods
	validMethods := map[string]bool{"GET": true, "PUT": true, "POST": true, "DELETE": true}

	if methods == nil {
		methods = validMethods
	}

	for m := range methods {
		if !validMethods[m] {
			err = invalidMethodError{name: m}
			return "", "", err
		}

		line, present := checkDuplicatePath(readFile, path, m) // checks if the path already exists
		if present {
			err := pathExistsError{path, m, line, readFile.Name()}
			logger.Error(err)

			continue
		}

		// creates the content to be written in main() and also creates the template for the handler function
		h, k := generateFileContents(path, m, methodFuncName[m])
		hs += h
		ms += k
	}

	return hs, ms, nil
}

// generateFileContents creates the content to be added in main.go and creates the Handler template
func generateFileContents(path, method, funcName string) (handlerTemplate, mainTemplate string) {
	handlerTemplate = fmt.Sprintf(`
func %s(c *gofr.Context) (interface{}, error) {
	// your logic here

	return nil, nil
}

`, funcName)
	mainTemplate = fmt.Sprintf(`    k.%s("/%s", %s.%s)`, method, path, path, funcName) + "\n"

	return
}

// removeDuplicates removes all the from the elem slice
func removeDuplicates(elem []string) map[string]bool {
	tempMap := make(map[string]bool)

	for _, e := range elem {
		if !tempMap[e] {
			tempMap[e] = true
		}
	}

	return tempMap
}

// populateHandler writes the handlerString into http/path/path.go file
func populateHandler(f fileSystem, path, handlerString string) error {
	err := createChangeDir(f, "http")
	if err != nil {
		return err
	}

	err = createChangeDir(f, path)
	if err != nil {
		return err
	}

	handlerFile, err := f.OpenFile(path+".go", os.O_APPEND|os.O_WRONLY|os.O_CREATE, migration.RWMode)
	if err != nil {
		return err
	}

	defer handlerFile.Close()

	fi, err := handlerFile.Stat()
	if err != nil {
		return err
	}

	if fi.Size() == 0 {
		handlerString = fmt.Sprintf(`package %s

import (
       "gofr.dev/pkg/gofr"
)

%s`, path, handlerString)
	}

	_, err = handlerFile.WriteString(handlerString)
	if err != nil {
		return err
	}

	return nil
}

// populateMain writes the mainString into main.go file
func populateMain(f fileSystem, mainString, path string) error {
	currDir, err := f.Getwd()
	if err != nil {
		return err
	}

	err = f.Chdir(currDir)
	if err != nil {
		return err
	}

	return processMainFile(f, mainString, path)
}

func processMainFile(f fileSystem, mainString, path string) error {
	mainFile, err := f.OpenFile("main.go", os.O_RDWR, migration.RWMode)
	if err != nil {
		return err
	}

	defer mainFile.Close()

	fileContent := ""
	lineString := ""
	line := 1

	// reads the file line by line and checks for .Start() because all the routes need to be added before the call to .Start(),
	// where .Start() is the function which starts the server
	if mainFile != nil {
		scanner := bufio.NewScanner(mainFile)

		for scanner.Scan() {
			lineString = scanner.Text()

			if strings.Contains(lineString, ".Start()") {
				lineString = mainString + "\n" + lineString
			}

			fileContent += lineString + "\n"
			line++
		}
	}

	_, err = mainFile.Seek(0, 0)
	if err != nil {
		return err
	}

	_, err = mainFile.WriteString(fileContent)
	if err != nil {
		return err
	}

	currDir, _ := os.Getwd()
	parentDir := filepath.Base(currDir)

	err = addHandlerImport(f, parentDir, path)
	if err != nil {
		return err
	}

	return nil
}

// checkDuplicatePath checks whether a route is already present in the mainFile
func checkDuplicatePath(mainFile io.ReadSeeker, route, method string) (int, bool) {
	// if method = put and route = /hello then in main.go we will have
	// ".PUT("/hello", " and hence match this string to check if duplicate exists
	routeString := "." + method + "(\"/" + route + "\","
	line, present := existCheck(mainFile, routeString)

	return line, present
}

// reads the file content and checks if elem exists in the file. If exists, returns the line number
func existCheck(file io.ReadSeeker, elem string) (int, bool) {
	present := false
	line := 0

	if file == nil {
		return line, false
	}

	_, err := file.Seek(0, 0)
	if err != nil {
		return 0, false
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), elem) {
			present = true
			line++

			break
		}
		line++
	}

	return line, present
}

func validatePath(f fileSystem, path string) bool {
	// /, {, } are added in regex pattern to accept pathParams in the format of gorilla mux, example:- /order/{id}
	pattern := `^[a-zA-Z/{}.~_-]+$`

	ok, err := f.Match(pattern, []byte(path))
	if err != nil || !ok {
		return false
	}

	return true
}

func addHandlerImport(f fileSystem, parentDirectory, path string) error {
	mainFile, err := f.OpenFile("main.go", os.O_RDWR, migration.RWMode)
	if err != nil {
		return err
	}

	fileContent := ""
	lineString := ""
	line := 1

	if mainFile != nil {
		scanner := bufio.NewScanner(mainFile)

		for scanner.Scan() {
			lineString = scanner.Text()

			if strings.Contains(lineString, "gofr.dev/pkg/gofr") {
				lineString = importSortCheck(parentDirectory+"/http/"+path, lineString)
			}

			fileContent += lineString + "\n"
			line++
		}
	}

	_, err = mainFile.Seek(0, 0)
	if err != nil {
		return err
	}

	_, err = mainFile.WriteString(fileContent)
	if err != nil {
		return err
	}

	return nil
}

func importSortCheck(directory, lineString string) string {
	if directory < "gofr.dev/pkg/gofr" {
		lineString = `	"` + directory + `"` + "\n" + lineString
	} else {
		lineString = lineString + "\n" + `	"` + directory + `"`
	}

	return lineString
}
