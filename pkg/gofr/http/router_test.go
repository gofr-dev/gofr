package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoubleSlashRouting(t *testing.T) {
	router := NewRouter()

	getHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("GET"))
	})
	
	postHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("POST"))
	})

	router.Add("GET", "/hello", getHandler)
	router.Add("POST", "/hello", postHandler)

	// Test POST with double slash - should redirect with 301
	req := httptest.NewRequest("POST", "//hello", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Errorf("Expected 301 for POST //hello, got %d", w.Code)
	}

	// Test GET with double slash - should redirect with 301
	req2 := httptest.NewRequest("GET", "//hello", nil)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)

	if w2.Code != http.StatusMovedPermanently {
		t.Errorf("Expected 301 for GET //hello, got %d", w2.Code)
	}
}