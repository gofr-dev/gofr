package errors

type Raw struct {
	StatusCode int   `json:"status_code"`
	Err        error `json:"error"`
}

// Error returns the underlying error message or "Unknown Error" if no specific error is available.
func (r Raw) Error() string {
	if r.Err == nil {
		return "Unknown Error"
	}

	return r.Err.Error()
}
