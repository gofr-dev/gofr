package dockerize

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/request"
)

const RWXOwner = 0700

// initializeTest function to create a sample go project in temporary directory
func initializeTest(t *testing.T) {
	dir := t.TempDir()
	defer os.Remove(dir)

	_ = os.Chdir(dir)

	_ = exec.Command("go", "mod", "init", "example").Run()
	_ = exec.Command("go", "mod", "edit", "-go", "1.19").Run()

	file, err := os.Create("main.go")
	if err != nil {
		t.Fatalf("Unable to create main.go: %v", err)
	}

	_, err = file.WriteString("package main\nfunc main() {}")
	if err != nil {
		t.Fatalf("Unable to write in main.go: %v", err)
	}

	err = createDockerFile()
	if err != nil {
		t.Fatalf("Unable to create Dockerfile: %v", err)
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

// setQueryParams utility function to set query param in url
func setQueryParams(params map[string]string) string {
	url := "/dummy?"

	for key, value := range params {
		url = fmt.Sprintf("%s%s=%s&", url, key, value)
	}

	return strings.TrimSuffix(url, "&")
}

//nolint:gocritic  //this function is used to initialize the git
func initialiseGit(t *testing.T) {
	// initialize git and commit go.mod file
	out, err := exec.Command("git", "init").CombinedOutput()
	if err != nil {
		t.Fatal(string(out), err)
	}

	// Check if the Git repository was initialized successfully
	if _, err = os.Stat(".git"); os.IsNotExist(err) {
		t.Fatalf("Git repository not initialized")
	}

	_ = exec.Command("git", "config", "user.name", "test").Run()
	_ = exec.Command("git", "config", "user.email", "test@example.com").Run()
	_ = exec.Command("git", "add", ".").Run()
	_ = exec.Command("git", "commit", "-m", "Initial commit").Run()
}

func Test_Dockerize(t *testing.T) {
	initializeTest(t)
	initialiseGit(t)

	h := New("gofr-app", "1.0.0")

	app := gofr.New()
	testCases := []struct {
		desc   string
		params map[string]string
		expRes string
		expErr error
	}{
		{"Success case, with default app name", map[string]string{}, "Docker image created", nil},
		{"Success case, with invalid tag to check if timestamp is used", map[string]string{"tag": "invalid"},
			"Docker image created", nil},
		{"Success case, with tag commit", map[string]string{"tag": "commit"}, "Docker image created", nil},
		{"Success case, should create docker image with name custom-app", map[string]string{"name": "custom-app"},
			"Docker image created", nil},
		{"Success case, with os linux", map[string]string{"os": "linux"}, "Docker image created", nil},
		{"Success case, with default os", map[string]string{"os": ""}, "Docker image created", nil},
		{"Success case, with help", map[string]string{"h": "true"}, Help(), nil},
	}

	for i, tc := range testCases {
		req := httptest.NewRequest("", setQueryParams(tc.params), http.NoBody)
		ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), app)

		resp, err := h.Dockerize(ctx)

		assert.Contains(t, resp, tc.expRes, "[TESTCASE %d] Failed Desc: %v\nexpected %v\tgot %v\n", i+1, tc.desc, tc.expRes, resp)
		assert.Equal(t, tc.expErr, err, "[TESTCASE %d]failed Desc: %v\nexpected %v\tgot %v\n", i+1, tc.desc, tc.expErr, err)
	}
}

func TestDockerize_Error(t *testing.T) {
	initializeTest(t)
	initialiseGit(t)

	h := New("gofr-app", "1.0.0")

	app := gofr.New()
	expErr := &errors.Response{Reason: fmt.Sprintf(`unknown parameter(s) [` + strings.Join([]string{"tags"}, ",") + `]. ` +
		`Run gofr <command_name> -h for help of the command.`)}

	req := httptest.NewRequest("", setQueryParams(map[string]string{"tags": "commit"}), http.NoBody)
	ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), app)

	resp, err := h.Dockerize(ctx)

	assert.Nil(t, resp, "TEST Failed.")
	assert.Equal(t, expErr, err, "TEST Failed.")
}

func TestDockerize_Fail(t *testing.T) {
	h := New("gofr-app", "1.0.0")

	desc := "Failure case, with invalid argument"
	params := map[string]string{"invalid": "testTag"}
	expErr := &errors.Response{Reason: "unknown parameter(s) [invalid]. Run gofr <command_name> -h for help of the command."}

	req := httptest.NewRequest("", setQueryParams(params), http.NoBody)
	ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

	resp, err := h.Run(ctx)

	assert.Nil(t, resp, "[TESTCASE] Failed Desc: %v\nexpected: Nil \tgot %v\n", desc, resp)
	assert.Equalf(t, expErr, err, "[TESTCASE]failed Desc: %v\nexpected %v\tgot %v\n", desc, expErr, err)
}

func Test_Dockerize_getImageFailure(t *testing.T) {
	initializeTest(t)

	h := New("gofr-app", "1.0.0")

	req := httptest.NewRequest("", setQueryParams(map[string]string{"tag": "commit"}), http.NoBody)
	ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

	resp, err := h.Dockerize(ctx)

	assert.Nil(t, resp, "[TESTCASE]failed Desc: get image failure in dockerize\n got %v", resp)
	assert.NotNil(t, err, "[TESTCASE]failed Desc: get image failure in dockerize\n got %v", err)
}

func Test_Dockerize_buildFailure(t *testing.T) {
	dir := t.TempDir()
	defer os.Remove(dir)

	_ = os.Chdir(dir)

	h := New("gofr-app", "1.0.0")

	req := httptest.NewRequest("", "/dummy", http.NoBody)
	ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

	resp, err := h.Dockerize(ctx)

	assert.Nil(t, resp, "[TESTCASE]failed Desc: unable to build image\n got %v", resp)
	assert.NotNil(t, err, "[TESTCASE]failed Desc: unable to build image\n got %v", err)
}

func Test_Dockerize_buildDockerFailure(t *testing.T) {
	initializeTest(t)

	os.Remove("Dockerfile")

	h := New("gofr-app", "1.0.0")

	req := httptest.NewRequest("", "/dummy", http.NoBody)
	ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

	resp, err := h.Dockerize(ctx)

	assert.Nil(t, resp, "[TESTCASE]failed Desc: get image failure in dockerize\n got %v", resp)
	assert.NotNil(t, err, "[TESTCASE]failed Desc: get image failure in dockerize\n got %v", err)
}

func TestRun(t *testing.T) {
	initializeTest(t)
	initialiseGit(t)

	h := New("gofr-app", "1.0.0")

	app := gofr.New()
	testCases := []struct {
		desc   string
		params map[string]string
		expRes string
		expErr error
	}{
		{"Success case, with invalid tag such that timestamp is used", map[string]string{"tag": "xyznjkfd"},
			"Docker image running", nil},
		{"Success case, with help", map[string]string{"h": "true"}, RunHelp(), nil},
		{"Success case, commit tag", map[string]string{"tag": "commit"}, "Docker image running", nil},
		{"Success case, create docker image with name ", map[string]string{"name": "sample-app"},
			"Docker image running", nil},
		{"Success case, with os", map[string]string{"os": "linux"}, "Docker image running", nil},
		{"Success case, with default os", map[string]string{"os": ""}, "Docker image running", nil},
		{"Success case, with default values", map[string]string{}, "Docker image running", nil},
	}

	for i, tc := range testCases {
		req := httptest.NewRequest("", setQueryParams(tc.params), http.NoBody)
		ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), app)

		resp, err := h.Run(ctx)

		assert.Contains(t, resp, tc.expRes, "[TESTCASE %d] Failed Desc: %v\nexpected %v\tgot %v\n", i+1, tc.desc, tc.expRes, resp)
		assert.Equalf(t, tc.expErr, err, "[TESTCASE %d]failed Desc: %v\nexpected %v\tgot %v\n", i+1, tc.desc, tc.expErr, err)
	}
}

func TestRun_Fail(t *testing.T) {
	h := New("gofr-app", "1.0.0")

	desc := "Failure case, with invalid argument"
	params := map[string]string{"invalid": "testTag"}
	expErr := &errors.Response{Reason: "unknown parameter(s) [invalid]. Run gofr <command_name> -h for help of the command."}

	req := httptest.NewRequest("", setQueryParams(params), http.NoBody)
	ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

	resp, err := h.Run(ctx)

	assert.Nil(t, resp, "[TESTCASE] Failed Desc: %v\nexpected: Nil \tgot %v\n", desc, resp)
	assert.Equalf(t, expErr, err, "[TESTCASE]failed Desc: %v\nexpected %v\tgot %v\n", desc, expErr, err)
}

func Test_populateFlags(t *testing.T) {
	testcases := []struct {
		desc   string
		params map[string]string
		expRes flags
	}{
		{"Success case, default values", map[string]string{}, flags{os: "linux",
			tag: "timestamp", name: "gofr-app", port: "8050:8000"}},
		{"Success case, with valid fields", map[string]string{"name": "custom-app", "tag": "commit", "os": "linux",
			"port": "8050:8000", "binary": "main", "image": "sample", "dockerfile": "Dockerfile1"}, flags{name: "custom-app",
			tag: "commit", os: "linux", port: "8050:8000", binary: "main", image: "sample", dockerfile: "Dockerfile1"}},
	}

	for i, tc := range testcases {
		req := httptest.NewRequest("", setQueryParams(tc.params), http.NoBody)
		ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

		res := populateFlags(ctx, "gofr-app")

		assert.Equal(t, tc.expRes, res, "[TESTCASE %d]failed Desc: %v\nexpected %v\tgot %v\n", i+1, tc.desc, tc.expRes, res)
	}
}

func Test_getImageName(t *testing.T) {
	initializeTest(t)
	initialiseGit(t)

	testCases := []struct {
		desc   string
		flag   flags
		expRes string
	}{
		{"Success case, should return image name docker with timestamp", flags{tag: "timestamp", name: "docker"}, "docker"},
		{"Success case, should return image name with version", flags{tag: "version", name: "docker"}, "docker"},
		{"Success case, return image name with default timestamp", flags{tag: "", name: "docker"}, "docker"},
	}

	for i, tc := range testCases {
		testFlag := tc.flag
		imageName, err := getImageName(&testFlag, "dev")

		assert.Contains(t, imageName, tc.expRes, "[TESTCASE %d] Failed Desc: %v\nexpected %v\tgot %v\n", i+1, tc.desc, tc.expRes, imageName)
		assert.Nil(t, err, "[TESTCASE %d] Failed Desc: %v\nexpected %v\tgot %v\n", i+1, tc.desc, nil, err)
	}
}

func Test_getImageName_Fail(t *testing.T) {
	initializeTest(t)

	imageName, err := getImageName(&flags{tag: "commit"}, "dev")

	assert.Equal(t, "", imageName, "Testcase Failed: Git is not initialized so should be empty")
	assert.NotNil(t, err, "Testcase Failed: Git is not initialized so should return error")
}

func Test_buildDockerImage(t *testing.T) {
	initializeTest(t)

	ctx := gofr.NewContext(nil, nil, gofr.New())

	err := buildDockerImage(ctx, "xyz", "invalid")

	assert.NotNil(t, err, "Testcase Failed: invalid docker file")
}

func Test_buildBinary(t *testing.T) {
	initializeTest(t)

	err := buildBinary(&flags{os: "xyz"})

	assert.NotNil(t, err, "Testcase Failed: invalid os")
}

func Test_runDockerImage(t *testing.T) {
	initializeTest(t)

	ctx := gofr.NewContext(nil, nil, gofr.New())

	err := runDockerImage(ctx, &flags{})

	assert.NotNil(t, err, "Testcase Failed: invalid port")
}

func Test_Run_getImageFailure(t *testing.T) {
	initializeTest(t)

	h := New("gofr-app", "1.0.0")

	req := httptest.NewRequest("", setQueryParams(map[string]string{"tag": "commit"}), http.NoBody)
	ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

	resp, err := h.Run(ctx)

	assert.Nil(t, resp, "[TESTCASE]failed Desc: get image failure in dockerize\n got %v", resp)
	assert.NotNil(t, err, "[TESTCASE]failed Desc: get image failure in dockerize\n got %v", err)
}

func Test_Run_buildFailure(t *testing.T) {
	dir := t.TempDir()
	defer os.Remove(dir)

	_ = os.Chdir(dir)

	h := New("gofr-app", "1.0.0")

	req := httptest.NewRequest("", "/dummy", http.NoBody)
	ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

	resp, err := h.Run(ctx)

	assert.Nil(t, resp, "[TESTCASE]failed Desc: unable to build image\n got %v", resp)
	assert.NotNil(t, err, "[TESTCASE]failed Desc: unable to build image\n got %v", err)
}

func Test_Run_buildDockerFailure(t *testing.T) {
	initializeTest(t)

	os.Remove("Dockerfile")

	h := New("gofr-app", "1.0.0")

	req := httptest.NewRequest("", "/dummy", http.NoBody)
	ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

	resp, err := h.Run(ctx)

	assert.Nil(t, resp, "[TESTCASE]failed Desc: get image failure in run\n got %v", resp)
	assert.NotNil(t, err, "[TESTCASE]failed Desc: get image failure in run\n got %v", err)
}

func Test_Run_runDockerFailure(t *testing.T) {
	initializeTest(t)

	h := New("gofr-app", "1.0.0")

	req := httptest.NewRequest("", setQueryParams(map[string]string{"image": "invalid"}), http.NoBody)
	ctx := gofr.NewContext(nil, request.NewHTTPRequest(req), gofr.New())

	resp, err := h.Run(ctx)

	assert.Nil(t, resp, "[TESTCASE]failed Desc: run docker failure\n got %v", resp)
	assert.NotNil(t, err, "[TESTCASE]failed Desc: run docker failure\n got %v", err)
}
