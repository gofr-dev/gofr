package gofr

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGraphQL_Query(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")

	app := New()

	app.GraphQLQuery("hello", func(_ *Context) (string, error) {
		return "world", nil
	})

	handler := app.graphqlManager.GetHandler()

	reqBody := `{"query": "{ hello }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

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

	app := New()

	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	type CreateUserArgs struct {
		Name string `json:"name"`
	}

	app.GraphQLMutation("createUser", func(_ *Context, args CreateUserArgs) (User, error) {
		return User{ID: 1, Name: args.Name}, nil
	})

	handler := app.graphqlManager.GetHandler()

	reqBody := `{"query": "mutation { createUser(name: \"test\") { id name } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

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

	app := New()
	app.GraphQLQuery("dummy", func(_ *Context) (string, error) { return "ok", nil })

	handler := app.graphqlManager.GetHandler()

	reqBody := `{"query": "{ gofr { status } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

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
	t.Setenv("METRICS_PORT", "0")

	app := New()
	app.GraphQLQuery("dummy", func(_ *Context) (string, error) { return "ok", nil })

	handler := app.graphqlManager.GetHandler()

	req := httptest.NewRequest(http.MethodGet, "/graphql/ui", http.NoBody)
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
	assert.Contains(t, resp.Body.String(), "GoFr GraphQL Playground")
}

func TestGraphQL_ComplexTypes(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")

	app := New()

	type DetailedUser struct {
		ID      int      `json:"id"`
		Score   float64  `json:"score"`
		IsAdmin bool     `json:"isAdmin"`
		Tags    []string `json:"tags"`
	}

	app.GraphQLQuery("complexQuery", func(_ *Context) (DetailedUser, error) {
		return DetailedUser{
			ID:      42,
			Score:   99.5,
			IsAdmin: true,
			Tags:    []string{"admin", "internal"},
		}, nil
	})

	handler := app.graphqlManager.GetHandler()

	reqBody := `{"query": "{ complexQuery { id score isAdmin tags } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result struct {
		Data struct {
			ComplexQuery DetailedUser `json:"complexQuery"`
		} `json:"data"`
	}

	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)

	assert.Equal(t, 42, result.Data.ComplexQuery.ID)
	assert.InDelta(t, 99.5, result.Data.ComplexQuery.Score, 0.001)
	assert.True(t, result.Data.ComplexQuery.IsAdmin)
	assert.Equal(t, []string{"admin", "internal"}, result.Data.ComplexQuery.Tags)
}

func TestGraphQL_ArgumentTypes(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")

	app := New()

	type FilterArgs struct {
		Name   string  `json:"name"`
		Min    float64 `json:"min"`
		Active bool    `json:"active"`
		IDs    []int   `json:"ids"`
	}

	app.GraphQLQuery("filter", func(_ *Context, args FilterArgs) (bool, error) {
		return args.Active && args.Min > 0 && args.Name != "", nil
	})

	handler := app.graphqlManager.GetHandler()

	reqBody := `{"query": "{ filter(name: \"test\", min: 10.5, active: true, ids: [1, 2]) }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result struct {
		Data struct {
			Filter bool `json:"filter"`
		} `json:"data"`
	}

	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.True(t, result.Data.Filter)
}

func TestGraphQL_Errors(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")

	app := New()

	// Register an invalid handler (not a function)
	app.GraphQLQuery("invalid", "not-a-func")

	handler := app.graphqlManager.GetHandler()

	reqBody := `{"query": "{ invalid }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusInternalServerError, resp.Code)
}

func TestGraphQL_NestedTypes(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")

	type Address struct {
		City string `json:"city"`
	}

	type Profile struct {
		Name    string  `json:"name"`
		Address Address `json:"address"`
	}

	app := New()
	app.GraphQLQuery("profile", func(_ *Context) (Profile, error) {
		return Profile{Name: "Aman", Address: Address{City: "Delhi"}}, nil
	})

	handler := app.graphqlManager.GetHandler()

	reqBody := `{"query": "{ profile { name address { city } } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result struct {
		Data struct {
			Profile Profile `json:"profile"`
		} `json:"data"`
	}

	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "Delhi", result.Data.Profile.Address.City)
}

func TestGraphQL_PointerTypes(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")

	type User struct {
		Name *string `json:"name"`
	}

	app := New()
	app.GraphQLQuery("user", func(_ *Context) (*User, error) {
		name := "Pointer"
		return &User{Name: &name}, nil
	})

	handler := app.graphqlManager.GetHandler()

	reqBody := `{"query": "{ user { name } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

	var result struct {
		Data struct {
			User User `json:"user"`
		} `json:"data"`
	}

	err := json.Unmarshal(resp.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "Pointer", *result.Data.User.Name)
}

func TestGraphQL_ResolverError(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")

	app := New()
	app.GraphQLQuery("fail", func(_ *Context) (string, error) {
		return "", assert.AnError
	})

	handler := app.graphqlManager.GetHandler()

	reqBody := `{"query": "{ fail }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)

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

func TestGraphQL_EdgeCases(t *testing.T) {
	t.Setenv("METRICS_PORT", "0")

	type MapArgs struct {
		Data map[string]string `json:"data"`
	}

	app := New()
	// Test map and interface conversion logic
	app.GraphQLQuery("mapQuery", func(_ *Context, _ MapArgs) (any, error) {
		return "ok", nil
	})

	handler := app.graphqlManager.GetHandler()

	reqBody := `{"query": "{ mapQuery(data: \"some-string\") }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()

	handler.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
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
