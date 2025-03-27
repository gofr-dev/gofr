package response

import "net/http"

type Raw struct {
	Cookie  *http.Cookie
	Headers map[string]string
	Data    any
}
