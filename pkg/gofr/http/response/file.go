package response

import "net/http"

type File struct {
	Content     []byte
	ContentType string
	Cookie      *http.Cookie
	Headers     map[string]string
}

func (file File) SetCustomHeaders(w http.ResponseWriter) {
	for key, value := range file.Headers {
		w.Header().Set(key, value)
	}
}

func (file File) SetCookie(w http.ResponseWriter) {
	http.SetCookie(w, file.Cookie)
}
