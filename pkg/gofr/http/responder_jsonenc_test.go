package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	resTypes "gofr.dev/pkg/gofr/http/response"
)

var errJSONEnc = errors.New("partial <failure> & more")

type jsonencCustomError struct{}

func (jsonencCustomError) Error() string { return "custom" }

func (jsonencCustomError) StatusCode() int { return http.StatusConflict }

func (jsonencCustomError) Response() map[string]any {
	return map[string]any{"code": 7, "fields": []string{"a", "b"}, "message": "ignored"}
}

type jsonencSelfMarshal struct{}

func (jsonencSelfMarshal) MarshalJSON() ([]byte, error) { return []byte(`{ "spaced" : "<x>" }`), nil }

// TestRespond_JSONEncMatchesEncoder checks that, for the result shapes a handler returns, Respond
// writes the bytes json.Encoder writes for the same envelope, with the same status: the values
// jsonenc covers and the ones it hands back to encoding/json alike.
func TestRespond_JSONEncMatchesEncoder(t *testing.T) {
	type user struct {
		ID      int               `json:"id"`
		Name    string            `json:"name"`
		Email   string            `json:"email,omitempty"`
		Created time.Time         `json:"created"`
		Labels  map[string]string `json:"labels"`
		Score   float64           `json:"score"`
		Parent  *user             `json:"-"`
	}

	when := time.Date(2026, 11, 6, 3, 20, 0, 5, time.UTC)

	cases := []struct {
		name string
		data any
		err  error
	}{
		{"struct", user{ID: 1, Name: "a<b>", Created: when, Labels: map[string]string{"z": "1", "a": "2"}, Score: 1e21}, nil},
		{"pointer", &user{ID: 2}, nil},
		{"slice of structs", []user{{ID: 1}, {ID: 2, Email: "x@y"}}, nil},
		{"map", map[string]any{"b": []any{1, 2.5, "x"}, "a": nil}, nil},
		{"number", 3.25, nil},
		{"bool", true, nil},
		{"empty string", "", nil},
		{"partial content", map[string]int{"n": 1}, errJSONEnc},
		{"error only", nil, errJSONEnc},
		{"custom error", nil, jsonencCustomError{}},
		{"marshaler", jsonencSelfMarshal{}, nil},
		{"non-ascii", map[string]string{"k": "é "}, nil},
		{"bytes", []byte("raw"), nil},
		{"nan", math.NaN(), nil},
		{"metadata", resTypes.Response{Data: user{ID: 3}, Metadata: map[string]any{"page": 1, "<": ">"}}, nil},
		{"raw", resTypes.Raw{Data: []user{{ID: 4}}}, nil},
		{"raw nil", resTypes.Raw{Data: nil}, nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			NewResponder(rec, http.MethodGet).Respond(tc.data, tc.err)

			wantStatus, wantBody := encoderReference(t, tc.data, tc.err)

			assert.Equal(t, wantStatus, rec.Code)
			assert.Equal(t, wantBody, rec.Body.String())
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		})
	}
}

// encoderReference is what Respond wrote before jsonenc: the envelope through json.Encoder, or the
// fixed encode-failure body with a 500.
func encoderReference(t *testing.T, data any, err error) (status int, body string) {
	t.Helper()

	statusCode, errorObj := Responder{method: http.MethodGet}.determineResponse(data, err)

	var resp any

	switch v := data.(type) {
	case resTypes.Raw:
		resp = v.Data
	case resTypes.Response:
		resp = &response{Data: v.Data, Metadata: v.Metadata, Error: errorObj}
	default:
		resp = &response{Data: data, Error: errorObj}
	}

	var b bytes.Buffer

	if encErr := json.NewEncoder(&b).Encode(resp); encErr != nil {
		return http.StatusInternalServerError, `{"error":{"message": "failed to encode response as JSON"}}` + "\n"
	}

	return statusCode, b.String()
}

// TestRespond_JSONEncNoLeakAcrossResponses responds from many goroutines with different values and
// checks every body is its own, for the race detector: the encoders are shared across requests.
func TestRespond_JSONEncNoLeakAcrossResponses(t *testing.T) {
	type item struct {
		N    int      `json:"n"`
		Tags []string `json:"tags"`
	}

	done := make(chan struct{})

	for g := range 8 {
		go func() {
			defer func() { done <- struct{}{} }()

			for i := range 300 {
				v := item{N: g*1000 + i, Tags: []string{string(rune('a' + g))}} //nolint:gosec // g is below 8.

				rec := httptest.NewRecorder()
				NewResponder(rec, http.MethodGet).Respond(v, nil)

				want, err := json.Marshal(&response{Data: v})
				if !assert.NoError(t, err) {
					return
				}

				if !assert.Equal(t, string(want)+"\n", rec.Body.String()) {
					return
				}
			}
		}()
	}

	for range 8 {
		<-done
	}
}
