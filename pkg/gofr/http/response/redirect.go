package response

import "net/http"

type Redirect struct {
	URL        string
	StatusCode int
}

// NewRedirect creates a redirect response with the specified URL and status code.
// If statusCode is 0, it defaults to 302 (Found).
func NewRedirect(url string, statusCode int) *Redirect {
	if statusCode == 0 {
		statusCode = http.StatusFound
	}

	return &Redirect{
		URL:        url,
		StatusCode: statusCode,
	}
}
