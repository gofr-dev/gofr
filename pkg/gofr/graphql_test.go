package gofr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

var errWriteFailed = errors.New("write failed")

// errWriter is an http.ResponseWriter whose body writes always fail. It is a synthetic
// failure used to exercise the logging path in respondWithErrors, not a reproduction of a
// client disconnect: net/http buffers these small bodies, so a disconnect does not surface
// as a write error inside the handler.
type errWriter struct {
	header http.Header
	status int
}

func (e *errWriter) Header() http.Header { return e.header }

func (*errWriter) Write([]byte) (int, error) { return 0, errWriteFailed }

func (e *errWriter) WriteHeader(status int) { e.status = status }

func setupSchema(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	configDir := filepath.Join(tmpDir, "configs")
	err := os.MkdirAll(configDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(configDir, "schema.graphqls"), []byte(content), 0600)
	require.NoError(t, err)

	return tmpDir
}

func TestGraphQL_Query(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")
	setupSchema(t, `type Query { hello: String }`)

	app := New()
	tmpDir := setupSchema(t, `type Query { hello: String }`)
	t.Chdir(tmpDir)

	app.GraphQLQuery("hello", func(_ *Context) (any, error) {
		return "world", nil
	})

	reqBody := `{"query": "{ hello }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	err := app.graphqlManager.buildSchema()
	require.NoError(t, err)

	app.graphqlManager.Handle(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result struct {
		Data struct {
			Hello string `json:"hello"`
		} `json:"data"`
	}

	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "world", result.Data.Hello)
}

func TestGraphQL_Mutation(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")

	tmpDir := setupSchema(t, `type User { id: Int name: String } type Query { dummy: String }
		type Mutation { createUser(name: String): User }`)
	t.Chdir(tmpDir)

	app := New()
	app.GraphQLQuery("dummy", func(_ *Context) (any, error) { return "ok", nil })
	app.GraphQLMutation("createUser", func(c *Context) (any, error) {
		var args struct {
			Name string `json:"name"`
		}

		err := c.Bind(&args)
		if err != nil {
			return nil, err
		}

		return map[string]any{"id": 1, "name": args.Name}, nil
	})

	reqBody := `{"query": "mutation { createUser(name: \"test\") { id name } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	err := app.graphqlManager.buildSchema()
	require.NoError(t, err)

	app.graphqlManager.Handle(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	var result struct {
		Data struct {
			CreateUser User `json:"createUser"`
		} `json:"data"`
	}

	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Data.CreateUser.ID)
	assert.Equal(t, "test", result.Data.CreateUser.Name)
}

func TestGraphQL_Playground(t *testing.T) {
	tests := []struct {
		desc         string
		appEnv       string
		expectedCode int
	}{
		{"Development Environment", "development", http.StatusOK},
		{"Production Environment", "production", http.StatusOK},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			t.Setenv("METRICS_PORT", "0")
			t.Setenv("APP_ENV", tc.appEnv)

			tmpDir := setupSchema(t, `type Query { dummy: String }`)
			t.Chdir(tmpDir)

			app := New()
			app.GraphQLQuery("dummy", func(_ *Context) (any, error) { return "ok", nil })

			// Internal call to setup router as App.Run would do
			app.httpServerSetup()

			req := httptest.NewRequest(http.MethodGet, "/.well-known/graphql/ui", http.NoBody)
			resp := httptest.NewRecorder()

			app.httpServer.router.ServeHTTP(resp, req)

			assert.Equal(t, tc.expectedCode, resp.Code)

			if tc.expectedCode == http.StatusOK {
				assert.Contains(t, resp.Body.String(), "GoFr GraphQL Playground")
			}
		})
	}
}

func TestGraphQL_ArgumentTypes(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")

	tmpDir := setupSchema(t, `type Query { 
		user(id: Int, score: Float, isAdmin: Boolean, tags: [String]): DetailedUser 
	} 
	type DetailedUser { id: Int score: Float isAdmin: Boolean tags: [String] }`)
	t.Chdir(tmpDir)

	app := New()
	app.GraphQLQuery("user", func(c *Context) (any, error) {
		var args struct {
			ID      int      `json:"id"`
			Score   float64  `json:"score"`
			IsAdmin bool     `json:"isAdmin"`
			Tags    []string `json:"tags"`
		}

		err := c.Bind(&args)
		if err != nil {
			return nil, err
		}

		return args, nil
	})

	reqBody := `{"query": "{ user(id: 1, score: 9.5, isAdmin: true, tags: [\"a\", \"b\"]) { id score isAdmin tags } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	err := app.graphqlManager.buildSchema()
	require.NoError(t, err)

	app.graphqlManager.Handle(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	type DetailedUser struct {
		ID      int      `json:"id"`
		Score   float64  `json:"score"`
		IsAdmin bool     `json:"isAdmin"`
		Tags    []string `json:"tags"`
	}

	var result struct {
		Data struct {
			User DetailedUser `json:"user"`
		} `json:"data"`
	}

	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Data.User.ID)
	assert.InDelta(t, 9.5, result.Data.User.Score, 0.001)
	assert.True(t, result.Data.User.IsAdmin)
	assert.Equal(t, []string{"a", "b"}, result.Data.User.Tags)
}

func TestGraphQL_BuildFailure(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")
	// No schema file setup, should fail
	app := New()
	app.GraphQLQuery("hello", func(_ *Context) (any, error) { return "world", nil })

	err := app.graphqlManager.buildSchema()
	assert.Error(t, err)
}

func TestGraphQL_ResolverError(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")

	tmpDir := setupSchema(t, `type Query { fail: String }`)
	t.Chdir(tmpDir)

	app := New()
	app.GraphQLQuery("fail", func(_ *Context) (any, error) {
		return nil, assert.AnError
	})

	reqBody := `{"query": "{ fail }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	err := app.graphqlManager.buildSchema()
	require.NoError(t, err)

	app.graphqlManager.Handle(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result struct {
		Data   any `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Errors)
	assert.Contains(t, result.Errors[0].Message, assert.AnError.Error())
}

func TestGraphQL_RequestMethods(t *testing.T) {
	params := map[string]any{"id": 1, "name": "test"}
	req := &graphQLRequest{ctx: context.Background(), params: params}

	assert.Equal(t, "1", req.Param("id"))
	assert.Equal(t, "test", req.Param("name"))
	assert.Empty(t, req.Param("invalid"))
	assert.Empty(t, req.PathParam("any"))
	assert.Empty(t, req.HostName())
	assert.Nil(t, req.Params("any"))
	assert.NotNil(t, req.Context())

	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	var u User

	err := req.Bind(&u)
	require.NoError(t, err)
	assert.Equal(t, 1, u.ID)
	assert.Equal(t, "test", u.Name)
}

func TestGraphQL_Enums(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")

	tmpDir := setupSchema(t, `
		enum Role { ADMIN USER }
		type User { id: Int role: Role }
		type Query { user(role: Role): User }
	`)
	t.Chdir(tmpDir)

	app := New()
	app.GraphQLQuery("user", func(c *Context) (any, error) {
		var args struct {
			Role string `json:"role"`
		}

		err := c.Bind(&args)
		if err != nil {
			return nil, err
		}

		return map[string]any{"id": 1, "role": args.Role}, nil
	})

	reqBody := `{"query": "{ user(role: ADMIN) { id role } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	err := app.graphqlManager.buildSchema()
	require.NoError(t, err)

	app.graphqlManager.Handle(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result struct {
		Data struct {
			User struct {
				ID   int    `json:"id"`
				Role string `json:"role"`
			} `json:"user"`
		} `json:"data"`
	}

	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "ADMIN", result.Data.User.Role)
}

func TestGraphQL_OperationName(t *testing.T) {
	tmpDir := setupSchema(t, `type Query { a: String, b: String }`)
	t.Chdir(tmpDir)

	app := New()
	app.GraphQLQuery("a", func(_ *Context) (any, error) { return "valA", nil })
	app.GraphQLQuery("b", func(_ *Context) (any, error) { return "valB", nil })

	// Document with multiple named operations
	reqBody := `{"query": "query QueryA { a } query QueryB { b }", "operationName": "QueryB"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	err := app.graphqlManager.buildSchema()
	require.NoError(t, err)

	app.graphqlManager.Handle(resp, req)

	var result struct {
		Data struct {
			B string `json:"b"`
		} `json:"data"`
	}

	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)

	assert.Equal(t, "valB", result.Data.B)
}

func TestGraphQL_Variables(t *testing.T) {
	tmpDir := setupSchema(t, `type Query { user(id: Int): User } type User { id: Int }`)
	t.Chdir(tmpDir)

	app := New()
	app.GraphQLQuery("user", func(c *Context) (any, error) {
		var args struct {
			ID int `json:"id"`
		}

		err := c.Bind(&args)
		if err != nil {
			return nil, err
		}

		return args, nil
	})

	reqBody := `{"query": "query GetUser($id: Int) { user(id: $id) { id } }", "variables": {"id": 123}}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	err := app.graphqlManager.buildSchema()
	require.NoError(t, err)

	app.graphqlManager.Handle(resp, req)

	var result struct {
		Data struct {
			User struct {
				ID int `json:"id"`
			} `json:"user"`
		} `json:"data"`
	}

	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)

	assert.Equal(t, 123, result.Data.User.ID)
}

func TestGraphQL_MalformedQuery(t *testing.T) {
	tmpDir := setupSchema(t, `type Query { hello: String }`)
	t.Chdir(tmpDir)

	app := New()
	app.GraphQLQuery("hello", func(_ *Context) (any, error) { return "ok", nil })

	reqBody := `{"query": "{ malformed "}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	err := app.graphqlManager.buildSchema()
	require.NoError(t, err)

	app.graphqlManager.Handle(resp, req)

	var result struct {
		Errors []any `json:"errors"`
	}

	err = json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Errors)
}

func Test_respondWithErrors(t *testing.T) {
	tests := []struct {
		desc    string
		status  int
		message string
	}{
		{"internal server error", http.StatusInternalServerError, "Internal Server Error"},
		{"unsupported media type", http.StatusUnsupportedMediaType, "Content-Type must be application/json"},
		{"bad request", http.StatusBadRequest, "invalid JSON request body"},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			resp := httptest.NewRecorder()

			logs := testutil.StderrOutputForFunc(func() {
				m := &graphQLManager{container: &container.Container{Logger: logging.NewMockLogger(logging.ERROR)}}
				m.respondWithErrors(resp, tc.status, tc.message)
			})

			assert.Empty(t, logs)
			assert.Equal(t, tc.status, resp.Code)
			assert.Equal(t, "application/json", resp.Header().Get("Content-Type"))

			var body struct {
				Errors []struct {
					Message string `json:"message"`
				} `json:"errors"`
			}

			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
			require.Len(t, body.Errors, 1)
			assert.Equal(t, tc.message, body.Errors[0].Message)
		})
	}
}

func Test_respondWithErrors_WriteFailureIsLogged(t *testing.T) {
	w := &errWriter{header: http.Header{}}

	logs := testutil.StderrOutputForFunc(func() {
		m := &graphQLManager{container: &container.Container{Logger: logging.NewMockLogger(logging.ERROR)}}
		m.respondWithErrors(w, http.StatusBadRequest, "invalid JSON request body")
	})

	assert.Contains(t, logs, "error encoding GraphQL error response")
	assert.Contains(t, logs, errWriteFailed.Error())
	assert.Equal(t, http.StatusBadRequest, w.status)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestGraphQL_RequestErrors(t *testing.T) {
	tests := []struct {
		desc        string
		contentType string
		body        string
		status      int
		message     string
	}{
		{"unsupported content type", "text/plain", `{"query": "{ hello }"}`,
			http.StatusUnsupportedMediaType, "Content-Type must be application/json"},
		{"invalid JSON body", "application/json", `{bad`,
			http.StatusBadRequest, "invalid JSON request body"},
	}

	tmpDir := setupSchema(t, `type Query { hello: String }`)
	t.Chdir(tmpDir)

	app := New()
	app.GraphQLQuery("hello", func(_ *Context) (any, error) { return "ok", nil })

	require.NoError(t, app.graphqlManager.buildSchema())

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/graphql", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", tc.contentType)

			resp := httptest.NewRecorder()

			app.graphqlManager.GetHandler().ServeHTTP(resp, req)

			var result struct {
				Errors []struct {
					Message string `json:"message"`
				} `json:"errors"`
			}

			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
			assert.Equal(t, tc.status, resp.Code)
			require.Len(t, result.Errors, 1)
			assert.Equal(t, tc.message, result.Errors[0].Message)
		})
	}
}
