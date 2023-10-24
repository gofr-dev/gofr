package datastore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.opencensus.io/plugin/ochttp"
)

// Search searches documents in the given collections based on the parameters specified.
// This can be used for making any queries to SOLR
func (c Client) Search(ctx context.Context, collection string, params map[string]interface{}) (interface{}, error) {
	url := c.url + collection + "/select"
	return call(ctx, "GET", url, params, nil)
}

// Create makes documents in the specified collection. params can be used to send parameters like commit=true
func (c Client) Create(ctx context.Context, collection string, document *bytes.Buffer,
	params map[string]interface{}) (interface{}, error) {
	url := c.url + collection + "/update"
	return call(ctx, "POST", url, params, document)
}

// Update updates documents in the specified collection. params can be used to send parameters like commit=true
func (c Client) Update(ctx context.Context, collection string, document *bytes.Buffer,
	params map[string]interface{}) (interface{}, error) {
	url := c.url + collection + "/update"
	return call(ctx, "POST", url, params, document)
}

// Create deletes documents in the specified collection. params can be used to send parameters like commit=true
func (c Client) Delete(ctx context.Context, collection string, document *bytes.Buffer,
	params map[string]interface{}) (interface{}, error) {
	url := c.url + collection + "/update"
	return call(ctx, "POST", url, params, document)
}

// ListFields retrieves all the fields in the schema for the specified collection.
// params can be used to send query parameters like wt, fl, includeDynamic etc.
func (c Client) ListFields(ctx context.Context, collection string, params map[string]interface{}) (interface{}, error) {
	url := c.url + collection + "/schema/fields"
	return call(ctx, "GET", url, params, nil)
}

// Retrieve retrieves the entire schema that includes all the fields,field types,dynamic rules and copy field rules.
// params can be used to specify the format of response
func (c Client) Retrieve(ctx context.Context, collection string, params map[string]interface{}) (interface{}, error) {
	url := c.url + collection + "/schema"
	return call(ctx, "GET", url, params, nil)
}

// AddField adds Field in the schema for the specified collection
func (c Client) AddField(ctx context.Context, collection string, document *bytes.Buffer) (interface{}, error) {
	url := c.url + collection + "/schema"
	return call(ctx, "POST", url, nil, document)
}

// UpdateField updates the field definitions in the schema for the specified collection
func (c Client) UpdateField(ctx context.Context, collection string, document *bytes.Buffer) (interface{}, error) {
	url := c.url + collection + "/schema"
	return call(ctx, "POST", url, nil, document)
}

// DeleteField deletes the field definitions in the schema for the specified collection
func (c Client) DeleteField(ctx context.Context, collection string, document *bytes.Buffer) (interface{}, error) {
	url := c.url + collection + "/schema"
	return call(ctx, "POST", url, nil, document)
}

// Response stores the response from SOLR
type Response struct {
	Code int
	Data interface{}
}

// call forms the http request and makes a call to solr and populates the solr response
func call(ctx context.Context, method, url string, params map[string]interface{}, body io.Reader) (interface{}, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if method != "GET" {
		req.Header.Add("content-type", "application/json")
	}

	q := req.URL.Query()

	for k, val := range params {
		switch v := val.(type) {
		case []string:
			for _, val := range v {
				q.Add(k, val)
			}
		default:
			q.Add(k, fmt.Sprintf("%v", val))
		}
	}

	req.URL.RawQuery = q.Encode()
	// trace the request
	octr := &ochttp.Transport{}
	client := &http.Client{Transport: octr}

	resp, err := client.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var respBody interface{}

	b, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(b, &respBody)

	if err != nil {
		return nil, err
	}

	return Response{resp.StatusCode, respBody}, nil
}
