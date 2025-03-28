package response

import "net/http"

type Raw struct {
	Cookie  *http.Cookie
	Headers map[string]string
	Data    any
}

func (raw Raw) SetCustomHeaders(w http.ResponseWriter) {
	for key, value := range raw.Headers {
		w.Header().Set(key, value)
	}
}

func (raw Raw) SetCookie(w http.ResponseWriter) {
	http.SetCookie(w, raw.Cookie)
}
