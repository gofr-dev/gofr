package response

import (
	"net/http"
)

type Response struct {
	Data     any               `json:"data"`
	Metadata map[string]any    `json:"metadata,omitempty"`
	Headers  map[string]string `json:"-"`
}

func (resp Response) SetCustomHeaders(w http.ResponseWriter) {
	for key, value := range resp.Headers {
		w.Header().Set(key, value)
	}
}
