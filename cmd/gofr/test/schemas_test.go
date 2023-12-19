package test

import (
	"sort"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
)

func TestSwagger_ConvertIntoIntegrationTestSchema(t *testing.T) {
	// Create a Swagger instance with test data
	swagger := &Swagger{
		&openapi3.T{
			Paths: &openapi3.Paths{
				Extensions: nil,
			},
		},
	}

	swagger.openapiSwagger.Paths.Set("/api/v1/users", &openapi3.PathItem{Get: &openapi3.Operation{OperationID: "getUser"},
		Post: &openapi3.Operation{OperationID: "createUser"}})

	swagger.openapiSwagger.Paths.Set("/api/v1/users/{id}", &openapi3.PathItem{Put: &openapi3.Operation{OperationID: "deleteUser"},
		Delete: &openapi3.Operation{OperationID: "deleteUser"}})

	// Convert Swagger into IntegrationTestSchema
	integrationSchema := swagger.convertIntoIntegrationTestSchema()

	// Assert the generated IntegrationTestSchema
	expectedSchema := IntegrationTestSchema{
		TestCases: []IntegrationTestCase{
			{
				Endpoint:             "/api/v1/users",
				Method:               "GET",
				ParamsJSONString:     "`{}`",
				ExpectedResponseCode: "",
			},
			{
				Endpoint:             "/api/v1/users",
				Method:               "POST",
				ParamsJSONString:     "`{}`",
				ExpectedResponseCode: "",
			},
			{
				Endpoint:             "/api/v1/users/{id}",
				Method:               "PUT",
				ParamsJSONString:     "`{}`",
				ExpectedResponseCode: "",
			},
			{
				Endpoint:             "/api/v1/users/{id}",
				Method:               "DELETE",
				ParamsJSONString:     "`{}`",
				ExpectedResponseCode: "",
			},
		},
	}

	// Sort expected and actual schemas
	sortMaps(expectedSchema)
	sortMaps(integrationSchema)

	assert.Equal(t, expectedSchema, integrationSchema, "IntegrationTestSchema should match the expected result")
}

func Test_populateSlice(t *testing.T) {
	inputSlice := []IntegrationTestCase{
		{
			Endpoint:             "/api/v1/users/{id}",
			Method:               "PUT",
			ParamsJSONString:     "`{}`",
			ExpectedResponseCode: "",
		},
	}

	value := IntegrationTestCase{
		Endpoint:             "/api/v1/users/{id}",
		Method:               "DELETE",
		ParamsJSONString:     "`{}`",
		ExpectedResponseCode: "",
	}

	expRes := []IntegrationTestCase{
		{Endpoint: "/api/v1/users/{id}", Method: "PUT", ParamsJSONString: "`{}`", Header: nil},
		{Endpoint: "/api/v1/users/{id}", Method: "DELETE", ParamsJSONString: "`{}`", Header: nil},
	}

	res := populateSlice(inputSlice, value)

	assert.Equal(t, expRes, res, "IntegrationTestSchema should match the expected result")
}

func sortMaps(schema IntegrationTestSchema) {
	var (
		headerKeys   []string
		sortedHeader = make(map[string][]string)
	)

	for i := range schema.TestCases {
		// Sort headers by keys
		for key := range schema.TestCases[i].Header {
			headerKeys = append(headerKeys, key)
		}

		sort.Strings(headerKeys)

		// sorted map
		for _, key := range headerKeys {
			sortedHeader[key] = schema.TestCases[i].Header[key]
		}

		// Assign the sorted map back to the test case
		schema.TestCases[i].Header = sortedHeader
	}

	sort.Slice(schema.TestCases, func(i, j int) bool {
		return schema.TestCases[i].Endpoint < schema.TestCases[j].Endpoint
	})
}
