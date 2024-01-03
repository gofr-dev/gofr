package service

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/log"
)

func TestService_CallGet(t *testing.T) {
	expectedResp := []byte(`{"address":{"city":"Bangalore","postalCode":"67352"}}`)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(expectedResp)
	}))

	s := NewHTTPServiceWithOptions(ts.URL, log.NewLogger(), nil)
	resp, err := s.Get(context.TODO(), "", nil)

	if err != nil {
		t.Errorf("Expected nil err but got %v", err)
	}

	assert.Equal(t, expectedResp, resp.Body, "TEST Failed.\n")
}

func TestBind(t *testing.T) {
	testcases := []struct {
		data        []byte
		contentType string
		err         error
	}{
		{[]byte(`{"name":"name"}`), "application/json", nil},
		{[]byte(`<name>name</name>`), "application/xml", nil},
		{[]byte(`hello`), "text/plain", nil},
	}

	for i, v := range testcases {
		k := i
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("content-type", testcases[k].contentType)
			_, _ = w.Write(testcases[k].data)
		}))

		svc := NewHTTPServiceWithOptions(ts.URL, log.NewMockLogger(io.Discard), nil)
		resp, _ := svc.Get(context.TODO(), "", nil)

		var input interface{}

		err := svc.Bind(resp.Body, &input)
		if !errors.Is(err, v.err) {
			t.Errorf("[TESTCASE %d]Failed. Got %v\tExpected %v\n", i+1, err, v.err)
		}
	}
}

func TestBindStrict(t *testing.T) {
	str := ""
	expected := "hello"
	data := []byte("hello")
	s := &httpService{contentType: TEXT}
	_ = s.BindStrict(data, &str)

	assert.Equal(t, expected, str, "TEST Failed.\n")

	type resp struct {
		Name string `json:"name"`
	}

	x := resp{}
	x1 := resp{}

	var input interface{}
	testcases := []struct {
		i             interface{}
		data          []byte
		expectedError bool
		contentType   string
	}{
		{&x, []byte(`{"Name":"name"}`), false, "application/json"},
		{&x1, []byte(`{"Name1":"Ram"`), true, "application/json"},
		{input, []byte(`<name>name</name>`), false, "application/xml"},
		{input, []byte(`hello`), false, "text/plain"},
	}

	for index, v := range testcases {
		k := index
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("content-type", testcases[k].contentType)
			_, _ = w.Write(testcases[k].data)
		}))

		svc := NewHTTPServiceWithOptions(ts.URL, log.NewMockLogger(io.Discard), nil)
		resp, _ := svc.Get(context.TODO(), "", nil)

		var input interface{}

		err := svc.BindStrict(resp.Body, &input)
		if v.expectedError == false && err != nil {
			t.Errorf("[TESTCASE %d]Failed. Got %v\tExpected %v\n", index+1, err, v.expectedError)
		}
	}
}

func TestBindResponseText(t *testing.T) {
	input := ""
	expected := "hello"
	data := []byte("hello")
	s := &httpService{contentType: TEXT}
	_ = s.Bind(data, &input)

	assert.Equal(t, expected, input, "TEST Failed.\n")
}

func TestBindResponseJSON(t *testing.T) {
	input := map[string]interface{}{}
	expected := map[string]interface{}{"id": "2"}

	data := []byte(`{"id":"2"}`)

	s := &httpService{contentType: JSON}
	_ = s.Bind(data, &input)

	assert.Equal(t, expected, input, "TEST Failed.\n")
}

func TestBindResponseXML(t *testing.T) {
	type resp struct {
		Name      xml.Name `xml:"Name"`
		FirstName string   `xml:"FirstName"`
	}

	b, _ := xml.Marshal(resp{FirstName: "Hello"})

	input := resp{}
	expected := resp{Name: xml.Name{Space: "", Local: "Name"}, FirstName: "Hello"}

	s := &httpService{contentType: XML}
	_ = s.Bind(b, &input)

	assert.Equal(t, expected, input, "TEST Failed.\n")
}

func TestXMLPOST(t *testing.T) {
	type resp struct {
		Name      xml.Name `xml:"Name"`
		FirstName string   `xml:"FirstName"`
	}

	// test server that gives the xml response
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		re := resp{FirstName: "Hello"}
		reBytes, _ := xml.Marshal(re)
		w.Header().Set("Content-type", "application/xml")
		_, _ = w.Write(reBytes)
	}))

	var re resp

	expected := resp{Name: xml.Name{Local: "Name"}, FirstName: "Hello"}
	httpService := NewHTTPServiceWithOptions(ts.URL, log.NewLogger(), nil)
	httpService.SetSurgeProtectorOptions(false, "", 5)

	body, _ := httpService.Post(context.TODO(), "", map[string]interface{}{"d": 1},
		[]byte(`<Name><FirstName>Hello</FirstName></Name>`))

	_ = httpService.Bind(body.Body, &re)

	if expected != re {
		t.Errorf("Failed.Expected %v\tGot %v", expected, re)
	}

	ts.Close()
}

func TestJSONPUT(t *testing.T) {
	type resp struct {
		FirstName string `json:"FirstName"`
	}

	var reBytes []byte

	res := resp{FirstName: "Hello"}
	reBytes, _ = json.Marshal(res)
	// test server that gives the json response
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json")
		_, _ = w.Write(reBytes)
	}))

	var re Response

	expected := &Response{
		Body:       reBytes,
		StatusCode: 200,
	}
	s := NewHTTPServiceWithOptions(ts.URL, log.NewLogger(), nil)
	s.SetSurgeProtectorOptions(false, "", 5)

	body, _ := s.Put(context.TODO(), "", map[string]interface{}{"d": 1}, []byte(`{"name":"hdhd"}`))

	_ = s.Bind(body.Body, &re)

	expected.headers = body.headers // needed to test the header fields, since the Date is not a constant.

	assert.Equal(t, expected, body, "TEST Failed.\n")

	ts.Close()
}

func TestJSONPatch(t *testing.T) {
	type resp struct {
		FirstName string `json:"FirstName"`
		LastName  string `json:"LastName"`
		Age       string `json:"Age"`
	}

	var reBytes []byte

	res := resp{FirstName: "Hello", LastName: "Buddy", Age: "23"}
	reBytes, _ = json.Marshal(res)

	// test server that gives the json response
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json")
		_, _ = w.Write(reBytes)
	}))

	defer ts.Close()

	var re Response

	expected := &Response{
		Body:       reBytes,
		StatusCode: 200,
	}
	s := NewHTTPServiceWithOptions(ts.URL, log.NewLogger(), nil)
	s.SetSurgeProtectorOptions(false, "", 5)

	body, _ := s.Patch(context.TODO(), "", map[string]interface{}{"d": 1}, []byte(`{"FirstName" : "Hello", LastName:"Buddy", Age:"23" }`))

	_ = s.Bind(body.Body, &re)

	expected.headers = body.headers // needed to test the header fields, since the Date is not a constant.

	assert.Equal(t, expected, body, "TEST Failed.\n")
}

//nolint:gocognit,gocyclo // breaking down function will reduce readability and reduce cognitive complexity
func TestHTTPMethodWithHeaders(t *testing.T) {
	expectedResp := []byte(`{"entity":"test"}`)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("entity") == "test" {
			_, _ = w.Write(expectedResp)
		}
		_, _ = w.Write(nil)
	}))

	s := NewHTTPServiceWithOptions(ts.URL, log.NewLogger(), &Options{Headers: map[string]string{"id": "1234"}})

	// Test GetWithHeaders
	resp, err := s.GetWithHeaders(context.TODO(), "", nil, map[string]string{"entity": "test"})
	if err != nil {
		t.Errorf("Expected nil err but got %v", err)
	}

	assert.Equal(t, expectedResp, resp.Body, "TEST Failed.\n")

	// Test PostWithHeaders
	resp, err = s.PostWithHeaders(context.TODO(), "", nil, nil, map[string]string{"entity": "test"})
	if err != nil {
		t.Errorf("Expected nil err but got %v", err)
	}

	assert.Equal(t, expectedResp, resp.Body, "TEST Failed.\n")

	// Test PutWithHeaders
	resp, err = s.PutWithHeaders(context.TODO(), "", nil, nil, map[string]string{"entity": "test"})
	if err != nil {
		t.Errorf("Expected nil err but got %v", err)
	}

	assert.Equal(t, expectedResp, resp.Body, "TEST Failed.\n")

	// Test PatchWithHeaders
	resp, err = s.PatchWithHeaders(context.TODO(), "", nil, nil, map[string]string{"entity": "test"})
	if err != nil {
		t.Errorf("Expected nil err but got %v", err)
	}

	assert.Equal(t, expectedResp, resp.Body, "TEST Failed.\n")

	// Test DeleteWithHeaders
	resp, err = s.DeleteWithHeaders(context.TODO(), "", nil, map[string]string{"entity": "test"})
	if err != nil {
		t.Errorf("Expected nil err but got %v", err)
	}

	assert.Equal(t, expectedResp, resp.Body, "TEST Failed.\n")
}
