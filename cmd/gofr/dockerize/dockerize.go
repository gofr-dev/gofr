package dockerize

import (
	"fmt"
	"os/exec"
	"time"

	"gofr.dev/cmd/gofr/helper"
	"gofr.dev/cmd/gofr/validation"
	"gofr.dev/pkg/errors"
	"gofr.dev/pkg/gofr"
)

// Help returns a formatted string containing usage instructions, flags, examples and a description
func Help() string {
	return helper.Generate(helper.Help{
		Example: `gofr dockerize -name=sample-app -tag=version -os=darwin -binary=sample -dockerfile=Dockerfile
gofr dockerize`,
		Flag: `-name: Name of the docker image. Default is app name.
-tag: Tag associated with the image name. It can be commit, version, timestamp. Default is timestamp.
-os: OS to be used for building binary. Default is 'linux'.
-binary: Binary to be used. Default is 'main'.
-dockerfile: Path to the Dockerfile. Default is 'DOCKERFILE'.`,
		Usage:       "dockerize -name=<image_name> -tag=<image-tag> -binary=<binary_name> -dockerfile=<dockerfile_path> -os=<os_name>",
		Description: "Creates docker image of the app, requires Dockerfile to be present in the current directory or specified by the user.",
	})
}

// RunHelp returns a formatted string containing usage instructions, flags, examples and a description for run command
func RunHelp() string {
	return helper.Generate(helper.Help{
		Example: `gofr dockerize run -name=gofr-app -tag=commit -os=linux -binary=main -dockerfile=Dockerfile -port=8000:9000
gofr dockerize run -port=8000:9000 -image=gofr-app:abcdef
gofr dockerize run -image=gofr-app:abcdef
gofr dockerize run -port=8000:9000
gofr dockerize run`,
		Flag: `-name: Name of the docker image. Default is app name.
-tag: Tag associated with the image name. It can be commit, version, timestamp. Default is timestamp.
-os: OS to be used for building binary. Default is 'linux'.
-binary: Binary to be used. Default is 'main'.
-dockerfile: Path to the Dockerfile. Default is 'DOCKERFILE'.
-port: Port mapping between host and container. Default is '8080:8080'.
-image: Name of the image to be used for running the container.By Default it will create and run the image.`,
		Usage: "gofr dockerize run -name=<image_name> -tag=<image-tag> -os=<build-os> -binary=<binary_name> " +
			"-dockerfile=<dockerfile_path> -port=<host>:<container> -image=<image_name>",
		Description: "Creates and run docker container of the app. \n All the fields are optional but if the image name " +
			"is provided it will run that image instead of creating a new one.",
	})
}

type flags struct {
	name       string `flag:"name"`
	tag        string `flag:"tag"`
	os         string `flag:"os"`
	binary     string `flag:"binary"`
	dockerfile string `flag:"dockerfile"`
	image      string `flag:"image"`
	port       string `flag:"port"`
}

type handler struct {
	AppName    string
	AppVersion string
}

// New is factory function for Handler layer
//
//nolint:revive // handler should not be used without proper initilization with required dependency
func New(appName, appVersion string) *handler {
	return &handler{
		AppName:    appName,
		AppVersion: appVersion,
	}
}

// Dockerize creates binary and docker image of the project
func (h handler) Dockerize(ctx *gofr.Context) (interface{}, error) {
	var validParams = map[string]bool{
		"h":          true,
		"name":       true,
		"tag":        true,
		"os":         true,
		"binary":     true,
		"dockerfile": true,
		"image":      true,
		"port":       true,
	}

	// there are no mandatory params
	var mandatoryParams []string

	params := ctx.Params()

	if help := params["h"]; help != "" {
		return Help(), nil
	}

	err := validation.ValidateParams(params, validParams, &mandatoryParams)
	if err != nil {
		return nil, err
	}

	flag := populateFlags(ctx, h.AppName)

	if flag.binary == "" {
		err = buildBinary(&flag)
		if err != nil {
			return nil, err
		}
	}

	imageName, err := getImageName(&flag, h.AppVersion)
	if err != nil {
		return nil, err
	}

	err = buildDockerImage(ctx, imageName, flag.dockerfile)
	if err != nil {
		return nil, err
	}

	return "Docker image created", nil
}

// Run executes the Docker image. If the image does not exist, it will first build it.
func (h handler) Run(ctx *gofr.Context) (interface{}, error) {
	var validParams = map[string]bool{
		"h":          true,
		"name":       true,
		"tag":        true,
		"os":         true,
		"binary":     true,
		"dockerfile": true,
		"image":      true,
		"port":       true,
	}

	// there are no mandatory params
	var (
		mandatoryParams []string
		imageName       string
	)

	params := ctx.Params()

	if help := params["h"]; help != "" {
		return RunHelp(), nil
	}

	err := validation.ValidateParams(params, validParams, &mandatoryParams)
	if err != nil {
		return nil, err
	}

	flag := populateFlags(ctx, h.AppName)

	if flag.image == "" {
		err = buildBinary(&flag)
		if err != nil {
			return nil, err
		}

		imageName, err = getImageName(&flag, h.AppVersion)
		if err != nil {
			return nil, err
		}

		err = buildDockerImage(ctx, imageName, flag.dockerfile)
		if err != nil {
			return nil, err
		}

		flag.image = imageName
	}

	err = runDockerImage(ctx, &flag)
	if err != nil {
		return nil, err
	}

	return "Docker image running: " + flag.image, nil
}

func runDockerImage(ctx *gofr.Context, flag *flags) error {
	dockerRunCMD := "docker run -d -p" + flag.port + " " + flag.image

	cmd := exec.Command("bash", "-c", dockerRunCMD)

	ctx.Logger.Infof("Running docker image %v", flag.image)

	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("%v\nCannot run docker image: %v\n", string(out), err)

		return errors.Error(errMsg)
	}

	ctx.Logger.Infof("Docker image running: %v", string(out))

	return nil
}

func buildBinary(flag *flags) error {
	createBinaryCMD := "GOOS=" + flag.os + " go build -o main"

	cmd := exec.Command("bash", "-c", createBinaryCMD)

	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("%v\nCannot create binary: %v\n", string(out), err)

		return errors.Error(errMsg)
	}

	return nil
}

func buildDockerImage(ctx *gofr.Context, imageName, dockerFile string) error {
	cmd := exec.Command("docker", "build", "-t", imageName, "-f", dockerFile, ".")

	ctx.Logger.Info("Creating docker image: ", imageName)

	out, err := cmd.CombinedOutput()
	if err != nil {
		errMsg := fmt.Sprintf("%v\nCannot dockerize: %v", string(out), err)

		return errors.Error(errMsg)
	}

	return nil
}

// getImageName returns the image name based on the tag
func getImageName(flag *flags, appVersion string) (string, error) {
	switch flag.tag {
	case "commit":
		cmd := exec.Command("bash", "-c", "git log --oneline -1")

		output, err := cmd.CombinedOutput()
		if err != nil {
			return "", err
		}

		return fmt.Sprintf("%v:%v", flag.name, string(output)[:7]), nil

	case "version":
		return fmt.Sprintf("%v:%v", flag.name, appVersion), nil
	default:
		return fmt.Sprintf("%v:%v", flag.name, time.Now().Format("20060102150405")), nil
	}
}

func populateFlags(ctx *gofr.Context, appName string) flags {
	params := ctx.Params()
	flag := flags{}

	var knownOS = map[string]bool{
		"darwin":  true,
		"linux":   true,
		"windows": true,
	}

	var validTag = map[string]bool{
		"commit":    true,
		"version":   true,
		"timestamp": true,
	}

	flag.binary = params["binary"]
	flag.image = params["image"]

	flag.name = params["name"]
	if flag.name == "" {
		flag.name = appName
	}

	flag.tag = params["tag"]
	if !validTag[flag.tag] {
		flag.tag = "timestamp"
	}

	flag.dockerfile = params["dockerfile"]
	if flag.dockerfile == "" {
		params["dockerfile"] = "Dockerfile"
	}

	flag.os = params["os"]
	if !knownOS[flag.os] {
		flag.os = "linux"
	}

	flag.port = params["port"]
	if flag.port == "" {
		flag.port = "8050:8000"
	}

	return flag
}
