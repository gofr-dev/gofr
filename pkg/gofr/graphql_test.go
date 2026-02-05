package gofr

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSchema(t *testing.T, content string) {
	t.Helper()

	err := os.MkdirAll("configs", 0755)
	require.NoError(t, err)

	err = os.WriteFile("configs/schema.graphqls", []byte(content), 0600)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = os.RemoveAll("configs")
	})
}

func TestGraphQL_Query(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")
	setupSchema(t, `type Query { hello: String }`)

	app := New()
	app.GraphQLQuery("hello", func(_ *Context) (any, error) {
		return "world", nil
	})

	reqBody := `{"query": "{ hello }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	app.graphqlManager.Handle(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result struct {
		Data struct {
			Hello string `json:"hello"`
		} `json:"data"`
	}

	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "world", result.Data.Hello)
}

func TestGraphQL_Mutation(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")
	setupSchema(t, `type User { id: Int name: String } type Query { hello: String } type Mutation { createUser(name: String): User }`)

	app := New()
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

	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Data.CreateUser.ID)
	assert.Equal(t, "test", result.Data.CreateUser.Name)
}

func TestGraphQL_Health(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")
	setupSchema(t, `type Query { dummy: String gofr: GofrInfo } type GofrInfo { status: String }`)

	app := New()
	app.GraphQLQuery("dummy", func(_ *Context) (any, error) { return "ok", nil })

	reqBody := `{"query": "{ gofr { status } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	app.graphqlManager.Handle(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result struct {
		Data struct {
			Gofr struct {
				Status string `json:"status"`
			} `json:"gofr"`
		} `json:"data"`
	}

	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Data.Gofr.Status)
}

func TestGraphQL_Playground(t *testing.T) {
	tests := []struct {
		desc         string
		appEnv       string
		expectedCode int
	}{
		{"Development Environment", "development", http.StatusOK},
		{"Production Environment", "production", http.StatusNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			t.Setenv("METRICS_PORT", "0")
			t.Setenv("APP_ENV", tc.appEnv)

			setupSchema(t, `type Query { dummy: String }`)

			app := New()
			app.GraphQLQuery("dummy", func(_ *Context) (any, error) { return "ok", nil })

			// Internal call to setup router as App.Run would do
			app.httpServerSetup()

			req := httptest.NewRequest(http.MethodGet, "/graphql/ui", http.NoBody)
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
	setupSchema(t, `type Query { 
		user(id: Int, score: Float, isAdmin: Boolean, tags: [String]): DetailedUser 
	} 
	type DetailedUser { id: Int score: Float isAdmin: Boolean tags: [String] }`)

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

	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Data.User.ID)
	assert.InDelta(t, 9.5, result.Data.User.Score, 0.001)
	assert.True(t, result.Data.User.IsAdmin)
	assert.Equal(t, []string{"a", "b"}, result.Data.User.Tags)
}

func TestGraphQL_Errors(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")
	// No schema file setup, should fail
	app := New()
	app.GraphQLQuery("hello", func(_ *Context) (any, error) { return "world", nil })

	reqBody := `{"query": "{ hello }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	app.graphqlManager.Handle(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestGraphQL_ResolverError(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")
	setupSchema(t, `type Query { fail: String }`)

	app := New()
	app.GraphQLQuery("fail", func(_ *Context) (any, error) {
		return nil, assert.AnError
	})

	reqBody := `{"query": "{ fail }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	app.graphqlManager.Handle(resp, req)

	assert.Equal(t, http.StatusUnprocessableEntity, resp.Code)

	var result struct {
		Data   any `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	err := json.Unmarshal(resp.Body.Bytes(), &result)
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
