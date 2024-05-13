package middleware

import "net/http"

// CORS ia a middleware that adds CORS (Cross-Origin Resource Sharing) headers to the response.
func CORS() func(inner http.Handler) http.Handler {
	return func(inner http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE, PATCH")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			w.Header().Set("Access-Control-Allow-Headers", "content-type")

			inner.ServeHTTP(w, r)
		})
	}
}
