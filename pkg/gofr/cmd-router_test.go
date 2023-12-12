package gofr

import "testing"

func Test_AddRoute(t *testing.T) {
	router := NewCMDRouter()

	h := func(c *Context) (interface{}, error) {
		return "Hi!", nil
	}

	router.AddRoute("/path1", h)

	if len(router.routes) != 1 {
		t.Error("Expected 1 route, got", len(router.routes))
	}
}

func TestHandler(t *testing.T) {
	tests := []struct {
		name        string
		router      CMDRouter
		path        string
		wantErr     bool
		wantHandler Handler
	}{
		{
			name:   "Match simple route",
			router: NewCMDRouter(),
			path:   "ping",
			wantHandler: func(c *Context) (interface{}, error) {
				return "Hi!", nil
			},
			wantErr: false,
		},
		{
			name:   "Match route with parameter",
			router: NewCMDRouter(),
			path:   "user/john",
			wantHandler: func(c *Context) (interface{}, error) {
				return "Hi!", nil
			},
			wantErr: false,
		},
		{
			name:    "No matching route",
			router:  NewCMDRouter(),
			path:    "invalid",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		tc.router.AddRoute(tc.path, tc.wantHandler)
		handler := tc.router.handler(tc.path)

		if tc.wantErr && handler != nil {
			t.Errorf("handler(%q) should have returned nil", tc.path)
		} else if !tc.wantErr && handler == nil {
			t.Errorf("handler(%q) should not have returned nil", tc.path)
		}
	}
}
