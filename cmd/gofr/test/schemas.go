package test

import (
	"net/http"
	"reflect"

	"github.com/getkin/kin-openapi/openapi3"
)

type APIValues struct {
	Tags        []string            `yaml:"tags"`
	Summary     string              `yaml:"summary"`
	Description string              `yaml:"description"`
	Parameters  []Parameter         `yaml:"parameters"`
	Responses   map[string]Response `yaml:"responses"`
}

type API struct {
	Get    APIValues `yaml:"get"`
	Post   APIValues `yaml:"post"`
	Put    APIValues `yaml:"put"`
	Delete APIValues `yaml:"delete"`
}

type Response struct {
	Description string `yaml:"description"`
	Content     struct {
		ApplicationJSON interface{} `yaml:"application/json"`
	}
}

type Parameter struct {
	Name        string `yaml:"name"`
	In          string `yaml:"in"`
	Description string `yaml:"description"`
	Schema      struct {
		Type string `yaml:"type"`
	} `yaml:"schema"`
	Example interface{} `yaml:"example"`
}

type IntegrationTestCase struct {
	Endpoint             string
	Method               string
	ParamsJSONString     string
	Header               map[string][]string
	ExpectedResponseCode string
}

type IntegrationTestSchema struct {
	TestCases []IntegrationTestCase
}

type Swagger struct {
	openapiSwagger *openapi3.T
}

func (s *Swagger) convertIntoIntegrationTestSchema() IntegrationTestSchema {
	resultSchema := IntegrationTestSchema{}

	for k := range s.openapiSwagger.Paths {
		v := s.openapiSwagger.Paths[k]

		getVal := getIntergrationTestSchema(v.Get, http.MethodGet, k)
		resultSchema.TestCases = populateSlice(resultSchema.TestCases, getVal)

		postVal := getIntergrationTestSchema(v.Post, http.MethodPost, k)
		resultSchema.TestCases = populateSlice(resultSchema.TestCases, postVal)

		putVal := getIntergrationTestSchema(v.Put, http.MethodPut, k)
		resultSchema.TestCases = populateSlice(resultSchema.TestCases, putVal)

		deleteVal := getIntergrationTestSchema(v.Delete, http.MethodDelete, k)
		resultSchema.TestCases = populateSlice(resultSchema.TestCases, deleteVal)
	}

	return resultSchema
}

func populateSlice(inputSlice []IntegrationTestCase, value IntegrationTestCase) []IntegrationTestCase {
	if reflect.DeepEqual(IntegrationTestCase{}, value) {
		return inputSlice
	}

	inputSlice = append(inputSlice, value)

	return inputSlice
}
