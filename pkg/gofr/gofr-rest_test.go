package gofr

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"gofr.dev/pkg/gofr/request"
	"gofr.dev/pkg/gofr/types"
)

type testController struct{}

func (t *testController) Index(*Context) (interface{}, error) {
	return "Index OK!", nil
}

func (t *testController) Read(c *Context) (interface{}, error) {
	return "Read id: " + c.PathParam("id"), nil
}

func (t *testController) Update(c *Context) (interface{}, error) {
	return "Put id: " + c.PathParam("id"), nil
}

func (t *testController) Create(*Context) (interface{}, error) {
	return "Post OK!", nil
}

func (t *testController) Delete(c *Context) (interface{}, error) {
	return "Delete id: " + c.PathParam("id"), nil
}

func (t *testController) Patch(c *Context) (interface{}, error) {
	return "Patch id: " + c.PathParam("id"), nil
}

func TestGofr_REST(t *testing.T) {
	testCases := []struct {
		// Given
		method string
		target string
		// Expectations
		response string
	}{
		{http.MethodGet, "/person", "Index OK!"},
		{http.MethodGet, "/person/12", "Read id: 12"},
		{http.MethodPut, "/person/12", "Put id: 12"},
		{http.MethodPost, "/person", "Post OK!"},
		{http.MethodDelete, "/person/12", "Delete id: 12"},
		{http.MethodPatch, "/person/12", "Patch id: 12"},
	}

	k := New()
	k.REST("person", &testController{})
	// Added contextInjector middleware
	k.Server.Router.Use(k.Server.contextInjector)

	for _, tc := range testCases {
		w := httptest.NewRecorder()
		r, _ := request.NewMock(tc.method, tc.target, nil)

		r.Header.Set("content-type", "text/plain")

		k.Server.Router.ServeHTTP(w, r)

		expectedResp := fmt.Sprintf("%v", &types.Response{Data: tc.response})

		if resp := w.Body.String(); resp != expectedResp {
			t.Errorf("Unexpected response for %s %s. \t expected: %s \t got: %s", tc.method, tc.target, expectedResp, resp)
		}
	}
}
