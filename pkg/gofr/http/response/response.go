package response

import (
	"net/http"
)

type Response struct {
	Data    any               `json:"data"`
	Headers map[string]string `json:"-"`
}

func (resp Response) SetCustomHeaders(w http.ResponseWriter) {
	for key, value := range resp.Headers {
		if w.Header().Get(key) != "" {
			// do not overwrite existing header
			continue
		} else {
			w.Header().Set(key, value)
		}
	}
}
