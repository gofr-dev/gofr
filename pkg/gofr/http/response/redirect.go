package response

type Redirect struct {
	URL string
}

// NewRedirect creates a redirect response with the specified URL and status code.
func NewRedirect(url string) *Redirect {
	return &Redirect{
		URL: url,
	}
}
