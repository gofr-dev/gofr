package response

type Response struct {
	Data    interface{}       `json:"data"`
	Headers map[string]string `json:"-"`
}
