package test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

func getIntergrationTestSchema(values *openapi3.Operation, method, endpoint string) IntegrationTestCase {
	result := IntegrationTestCase{}

	if values == nil {
		return result
	}

	result.Method = method

	result.Endpoint = replaceEndpointString(endpoint, values.Parameters) + getQueryParameterString(values.Parameters)
	jsonData, err := getBodyParameterString(values.Parameters)
	headers := getHeaderParameterString(values.Parameters)
	result.ParamsJSONString = jsonData
	result.Header = headers

	if err != nil {
		result.ParamsJSONString = ""
	}

	for k := range values.Responses {
		result.ExpectedResponseCode = k
		// Taking only the first response as a test case
		break
	}

	return result
}

func replaceEndpointString(endpoint string, params openapi3.Parameters) string {
	for _, param := range params {
		if param.Value.In == "path" {
			endpoint = strings.Replace(endpoint, "{"+param.Value.Name+"}", getStringValue(param.Value.Example), 1)
		}
	}

	return endpoint
}

func getQueryParameterString(params openapi3.Parameters) string {
	result := ""

	for _, param := range params {
		if param.Value.In == "query" {
			strVal := getStringValue(param.Value.Example)

			if result == "" {
				result = result + "?" + param.Value.Name + "=" + strVal
			} else {
				result = result + "&" + param.Value.Name + "=" + strVal
			}
		}
	}

	return result
}

func getBodyParameterString(params openapi3.Parameters) (string, error) {
	resultData := make(map[string]interface{})

	for _, param := range params {
		if param.Value.In == "body" {
			data := param.Value.Example
			if param.Value.Schema.Value.Type == "object" {
				data = createJSONString(param.Value.Example)
			}

			resultData[param.Value.Name] = data
		}
	}

	paramsJSON, err := json.Marshal(resultData)
	if err != nil {
		return "", err
	}

	return "`" + string(paramsJSON) + "`", nil
}

func getHeaderParameterString(params openapi3.Parameters) map[string][]string {
	headers := make(map[string][]string)

	for _, param := range params {
		if param.Value.In == "header" {
			val := getStringValue(param.Value.Example)
			headers[param.Value.Name] = strings.Split(val, ",")
		}
	}
	// if no header is present that return nil
	if len(headers) == 0 {
		return nil
	}

	return headers
}

func getStringValue(data interface{}) string {
	strVal := ""

	switch data := data.(type) {
	case []interface{}:
		for _, v := range data {
			if strVal == "" {
				strVal = getStringValue(v)
			} else {
				strVal = strVal + "," + getStringValue(v)
			}
		}
	default:
		strVal = fmt.Sprint(data)
	}

	return strVal
}

func createJSONString(data interface{}) string {
	result := ""
	marshalData, _ := json.Marshal(data)
	result = string(marshalData)

	return result
}

func runTests(host string, testcase IntegrationTestSchema) error {
	for _, tc := range testcase.TestCases {
		resp, err := makeRequest(host, tc)
		if err != nil {
			return err
		}

		respCode, err := strconv.Atoi(tc.ExpectedResponseCode)
		if err != nil {
			return err
		}

		if resp == nil {
			return fmt.Errorf("could not get any response from host: %v", host+tc.Endpoint)
		}

		if resp.StatusCode != respCode {
			return fmt.Errorf("failed.\tExpected %v\tGot %v", tc.ExpectedResponseCode, resp.StatusCode)
		}

		_ = resp.Body.Close()
	}

	return nil
}

func makeRequest(host string, tc IntegrationTestCase) (*http.Response, error) {
	req, err := http.NewRequest(tc.Method, host+tc.Endpoint, bytes.NewBuffer([]byte(tc.ParamsJSONString)))

	for key, vals := range tc.Header {
		for _, val := range vals {
			req.Header.Add(key, val)
		}
	}

	if err != nil {
		return nil, err
	}

	c := http.Client{}

	return c.Do(req)
}
