package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"runtime/debug"
)

func Recover(inner http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer panicRecovery(w)
		inner.ServeHTTP(w, r)
	})
}

func panicRecovery(w http.ResponseWriter) {
	logger := log.New(os.Stderr, "[ERROR] ", log.LstdFlags)
	re := recover()

	if re != nil {
		var e string
		switch t := re.(type) {
		case string:
			e = t
		case error:
			e = t.Error()
		default:
			e = "Unknown panic type"
		}
		logger.Printf("Panicked: %v Trace: %v", e, string(debug.Stack()))

		w.WriteHeader(http.StatusInternalServerError)

		res := map[string]interface{}{"code": http.StatusInternalServerError, "status": "ERROR", "message": "Some unexpected error has occurred"}
		_ = json.NewEncoder(w).Encode(res)
	}
}
