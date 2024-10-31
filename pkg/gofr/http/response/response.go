package response

type Response struct {
	Data    any               `json:"data"`
	Headers map[string]string `json:"-"`
}
