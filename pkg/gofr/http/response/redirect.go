package response

type Redirect struct {
	URL string
}

// NewRedirect creates a redirect response with the specified URL and status code.
// If statusCode is 0, it defaults to 302 (Found).
func NewRedirect(url string) *Redirect {
	return &Redirect{
		URL: url,
	}
}
