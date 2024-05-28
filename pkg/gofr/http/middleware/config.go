package middleware

import "gofr.dev/pkg/gofr/config"

func GetConfigs(c config.Config) map[string]string {
	middlewareConfigs := make(map[string]string)

	allowedCORSHeaders := []string{
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Headers",
		"Access-Control-Allow-Methods",
		"Access-Control-Allow-Credentials",
		"Access-Control-Expose-Headers",
		"Access-Control-Max-Age",
	}

	for _, v := range allowedCORSHeaders {
		if val := c.Get(v); val != "" {
			middlewareConfigs[v] = val
		}
	}

	return middlewareConfigs
}
