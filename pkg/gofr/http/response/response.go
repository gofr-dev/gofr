package response

import (
	"net/http"
)

type Response struct {
	Data          any               `json:"data"`
	CustomHeaders map[string]string `json:"-"`
}

func (resp Response) SetCustomHeaders(w http.ResponseWriter) {
	for key, value := range resp.CustomHeaders {
		if w.Header().Get(key) != "" {
			// do not overwrite existing header
			continue
		}

		w.Header().Set(key, value)
	}
}
