// Package test provides a command line interface for running tests for a given openapi specification.
// You can run it `gofr genit -source=path/to/openapispec.yml -host=host:port`
package test

import (
	"strings"

	"github.com/getkin/kin-openapi/openapi3"

	"gofr.dev/cmd/gofr/helper"
	"gofr.dev/cmd/gofr/validation"
	"gofr.dev/pkg/gofr"
)

func testHelp() string {
	return helper.Generate(helper.Help{
		Example:     "gofr test -host=localhost:9000 -source=/path/to/file.yml",
		Flag:        `host provide the host along with the port, source provide the path to the yml file`,
		Usage:       "test -host=<host:port> -source=</path/to/file>",
		Description: "runs integration test for a given configuration from an yml file",
	})
}

// GenerateIntegrationTest  generates an integration test based on parameters and swagger definitions, run tests against the provided host.
func GenerateIntegrationTest(c *gofr.Context) (interface{}, error) {
	validParams := map[string]bool{
		"h":      true,
		"source": true,
		"host":   true,
	}

	mandatoryParams := []string{"source", "host"}

	params := c.Params()

	if help := params["h"]; help != "" {
		return testHelp(), nil
	}

	err := validation.ValidateParams(params, validParams, &mandatoryParams)
	if err != nil {
		return nil, err
	}

	sourceFile := params["source"]
	host := params["host"]

	if !strings.Contains(host, "http://") {
		host = "http://" + host
	}

	swaggerLoader := openapi3.NewLoader()
	swaggerLoader.IsExternalRefsAllowed = true

	v, err := swaggerLoader.LoadFromFile(sourceFile)

	if err != nil {
		return nil, err
	}

	s := Swagger{openapiSwagger: v}

	err = runTests(host, s.convertIntoIntegrationTestSchema())
	if err != nil {
		return "Test Failed!", err
	}

	return "Test Passed!", nil
}
