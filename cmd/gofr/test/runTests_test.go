package test

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
)

func TestReplaceEndpointString(t *testing.T) {
	endpoint := "/api/v1/users/{id}/profile"
	params := openapi3.Parameters{
		&openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				In:      "path",
				Name:    "id",
				Example: "123",
			},
		},
		&openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				In:      "query",
				Name:    "filter",
				Example: "true",
			},
		},
	}

	testcases := []struct {
		desc   string
		params openapi3.Parameters
		expRes string
	}{
		{"success: when param is passed", params, "/api/v1/users/123/profile"},
		{"success: when param is not passed", openapi3.Parameters{}, "/api/v1/users/{id}/profile"},
	}

	for i, tc := range testcases {
		result := replaceEndpointString(endpoint, tc.params)

		assert.Equalf(t, tc.expRes, result, "Test[%d] Failed. Replaced endpoint should match the expected result", i)
	}
}

func TestGetQueryParameterString(t *testing.T) {
	params := openapi3.Parameters{
		&openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				In:      "query",
				Name:    "filter",
				Example: "true",
			},
		},
		&openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				In:      "query",
				Name:    "limit",
				Example: "10",
			},
		},
		&openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				In:      "path",
				Name:    "id",
				Example: "123",
			},
		},
	}

	result := getQueryParameterString(params)
	expectedResult := "?filter=true&limit=10"

	assert.Equal(t, expectedResult, result, "Query parameter string should match the expected result")
}

func TestGetBodyParameterString(t *testing.T) {
	params := openapi3.Parameters{
		&openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				In:      "body",
				Name:    "user",
				Example: map[string]interface{}{"name": "John Doe", "age": 30},
				Schema: &openapi3.SchemaRef{
					Value: &openapi3.Schema{
						Type:    "object",
						Example: openapi3.NewObjectSchema(),
					},
				},
			},
		},
		&openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				In:      "query",
				Name:    "filter",
				Example: "true",
			},
		},
	}

	expectedResult := "`{\"user\":\"{\\\"age\\\":30,\\\"name\\\":\\\"John Doe\\\"}\"}`"

	result, err := getBodyParameterString(params)

	assert.Nil(t, err, "Error should be nil")
	assert.Equal(t, expectedResult, result, "Body parameter string should match the expected result")
}

func TestGetHeaderParameterString(t *testing.T) {
	params := openapi3.Parameters{
		&openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				In:      "header",
				Name:    "Authorization",
				Example: "Bearer abcdef123456",
			},
		},
		&openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				In:      "header",
				Name:    "Content-Type",
				Example: "application/json",
			},
		},
		&openapi3.ParameterRef{
			Value: &openapi3.Parameter{
				In:      "query",
				Name:    "filter",
				Example: "true",
			},
		},
	}

	result := getHeaderParameterString(params)
	expectedResult := map[string][]string{
		"Authorization": {"Bearer abcdef123456"},
		"Content-Type":  {"application/json"},
	}

	assert.Equal(t, expectedResult, result, "Header parameter map should match the expected result")
}

func TestGetStringValue(t *testing.T) {
	testCases := []struct {
		desc          string
		data          interface{}
		expectedValue string
	}{
		{"string value", "Hello World", "Hello World"},
		{"integer value", 123, "123"},
		{"interface: slice of string", []interface{}{"apple", "banana", "cherry"}, "apple,banana,cherry"},
		{"interface: slice of int", []interface{}{1, 2, 3}, "1,2,3"},
		{"interface: slice of boolean", []interface{}{true, false}, "true,false"},
	}

	for i, testCase := range testCases {
		result := getStringValue(testCase.data)

		assert.Equalf(t, testCase.expectedValue, result, "Test[%d] Failed. String value should match the expected result", i)
	}
}

func TestCreateJSONString(t *testing.T) {
	testCases := []struct {
		data          interface{}
		expectedValue string
	}{
		{
			data:          map[string]interface{}{"name": "John Doe", "age": 30},
			expectedValue: `{"age":30,"name":"John Doe"}`,
		},
		{
			data:          []string{"apple", "banana", "cherry"},
			expectedValue: `["apple","banana","cherry"]`,
		},
		{
			data:          123,
			expectedValue: `123`,
		},
		{
			data:          true,
			expectedValue: `true`,
		},
	}

	for i, testCase := range testCases {
		result := createJSONString(testCase.data)

		assert.Equal(t, testCase.expectedValue, result, "Test[%d] failed. JSON string should match the expected result", i)
	}
}

func TestRunTests(t *testing.T) {

	_, _ = http.NewRequest(http.MethodGet, "%%", http.NoBody)
	testCases := IntegrationTestSchema{
		TestCases: []IntegrationTestCase{
			{
				Endpoint:             "/api/v1/users/123",
				Method:               "GET",
				ParamsJSONString:     "",
				ExpectedResponseCode: "200",
			},
		},
	}

	// Mock the server using httptest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id": 123, "name": "John Doe"}`)
	}))

	defer server.Close()

	host := server.URL
	err := runTests(host, testCases)

	assert.Nil(t, err, "No error should occur during test execution")
}

func TestRunTestErrorRespCode(t *testing.T) {
	testCases := []struct {
		tests  IntegrationTestSchema
		expErr error
	}{
		{
			tests: IntegrationTestSchema{
				[]IntegrationTestCase{
					{
						Endpoint:             "/api/v1/users/123",
						Method:               "GET",
						ParamsJSONString:     "",
						ExpectedResponseCode: "200.%",
					},
				},
			},
			expErr: &strconv.NumError{Func: "Atoi", Num: "200.%", Err: errors.New("invalid syntax")},
		},
		{
			tests: IntegrationTestSchema{
				[]IntegrationTestCase{
					{
						Endpoint:             "/api/v1/users/123",
						Method:               "GET",
						ParamsJSONString:     "",
						ExpectedResponseCode: "201",
					},
				},
			},
			expErr: errors.New("failed.\tExpected 201\tGot 200"),
		},
	}

	// Mock the server using httptest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id": 123, "name": "John Doe"}`)
	}))

	defer server.Close()

	host := server.URL
	for i, tc := range testCases {
		err := runTests(host, tc.tests)
		assert.Equal(t, tc.expErr.Error(), err.Error(), "Failed [%d] testcase: Expected [%v], Got [%v]", i, tc.expErr, err)
	}

}

func TestMakeRequest(t *testing.T) {
	testCase := IntegrationTestCase{
		Endpoint:             "/api/v1/users",
		Method:               "POST",
		ParamsJSONString:     `{"name":"John Doe","age":30}`,
		Header:               map[string][]string{"Content-Type": {"application/json"}, "Authorization": {"Bearer token"}},
		ExpectedResponseCode: "201",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate the request
		assert.Equal(t, "/api/v1/users", r.URL.Path, "Endpoint should match")
		assert.Equal(t, "POST", r.Method, "HTTP method should match")

		// Validate the request payload
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err, "No error should occur while reading the request body")
		assert.Equal(t, []byte(`{"name":"John Doe","age":30}`), body, "Request payload should match")

		// Validate the headers
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"), "Content-Type header should match")
		assert.Equal(t, "Bearer token", r.Header.Get("Authorization"), "Authorization header should match")

		// Send the response
		w.WriteHeader(http.StatusCreated)
	}))

	defer server.Close()

	host := server.URL

	resp, err := makeRequest(host, testCase)

	assert.Nil(t, err, "No error should occur during request execution")
	assert.NotNil(t, resp, "Response should not be nil")
	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Response status code should match")
	resp.Body.Close()
}
