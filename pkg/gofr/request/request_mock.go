package request

import (
	"io"
	"net/http"
)

// NewMock returns a httptest request
// adds the mandatory headers to the request
// NOTE: Use only for tests
func NewMock(method, target string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequest(method, target, body)
	if err != nil {
		return nil, err
	}
	// mandatory headers according to v3
	req.Header.Set("X-Zopsmart-Tenant", "good4more")
	req.Header.Set("X-Correlation-ID", "12d4F321S")
	req.Header.Set("True-Client-IP", "127.0.0.1")

	return req, nil
}
