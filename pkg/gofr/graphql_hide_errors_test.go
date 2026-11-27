//go:build !gofr_nographql

package gofr

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gofrHTTP "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/internal/errhide"
	"gofr.dev/pkg/gofr/testutil"
)

type gqlHideCase struct {
	desc    string
	hide    string
	err     error
	message string // errors[0].message the client receives
}

func TestGraphQL_HideInternalErrors(t *testing.T) {
	tests := []gqlHideCase{
		{"on: raw resolver error is hidden", "true", errDriverDetail, errhide.Message},
		{"on: error with a status code is kept", "true", gofrHTTP.ErrorEntityNotFound{Name: "id", Value: "7"},
			"No entity found with id: 7"},
		{"off: raw resolver error is unchanged", "false", errDriverDetail, errDriverDetail.Error()},
	}

	runGraphQLHideCases(t, tests, func(err error) Handler {
		return func(*Context) (any, error) { return nil, err }
	})
}

// TestGraphQL_HideInternalErrors_Panic: graphql-go recovers a resolver panic itself and puts the
// panic value in errors[].message, so the resolver must recover first when hiding is on.
func TestGraphQL_HideInternalErrors_Panic(t *testing.T) {
	tests := []gqlHideCase{
		{"on: panic text is hidden", "true", errDriverDetail, errhide.Message},
		{"off: panic text is unchanged", "false", errDriverDetail, errDriverDetail.Error()},
	}

	runGraphQLHideCases(t, tests, func(err error) Handler {
		return func(*Context) (any, error) { panic(err) }
	})
}

func runGraphQLHideCases(t *testing.T, tests []gqlHideCase, resolver func(error) Handler) {
	t.Helper()

	hideErrors(t, false)

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			t.Setenv("METRICS_PORT", "0")
			t.Setenv("HIDE_INTERNAL_ERRORS", tc.hide)
			errhide.Enable(false) // New only turns the switch on, so start each case from off
			t.Chdir(setupSchema(t, `type Query { fail: String }`))

			req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/graphql",
				bytes.NewBufferString(`{"query": "{ fail }"}`))
			req.Header.Set("Content-Type", "application/json")

			resp := httptest.NewRecorder()

			// The app is built inside the capture: its logger binds stderr when it is created.
			logs := testutil.StderrOutputForFunc(func() {
				app := New()
				app.GraphQLQuery("fail", resolver(tc.err))
				require.NoError(t, app.graphqlManager.buildSchema())

				gqlManager(t, app).Handle(resp, req)
			})

			var result struct {
				Errors []struct {
					Message string `json:"message"`
				} `json:"errors"`
			}

			require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &result))
			require.Len(t, result.Errors, 1)
			assert.Equal(t, tc.message, result.Errors[0].Message)

			if tc.message == errhide.Message {
				assert.Contains(t, logs, tc.err.Error(), "a hidden error must be logged")
			}
		})
	}
}
