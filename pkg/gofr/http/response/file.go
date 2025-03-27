package response

import "net/http"

type File struct {
	Content     []byte
	ContentType string
	Cookie      *http.Cookie
	Headers     map[string]string
}
