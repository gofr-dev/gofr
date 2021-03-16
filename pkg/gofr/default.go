package gofr

const (
	defaultHTTPPort  = 8000
	defaultDBPort    = 3306
	defaultRedisPort = 6379
)

func healthHandler(c *Context) (interface{}, error) {
	return "OK", nil
}
