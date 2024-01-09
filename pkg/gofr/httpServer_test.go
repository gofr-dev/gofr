package gofr

import (
	"net/http"
	"os"
	"testing"
	"time"

	http2 "gofr.dev/pkg/gofr/http"
	"gofr.dev/pkg/gofr/logging"

	"github.com/go-redis/redis/v8"
)

func TestRun_ServerStartsListening(t *testing.T) {
	// Create a mock router and add a new route
	router := &http2.Router{}
	router.Add(http.MethodGet, "/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Create a mock container
	container := &Container{
		Logger: logging.NewMockLogger(os.Stdout),
		Redis:  &redis.Client{},
		DB:     &DB{},
	}

	// Create an instance of httpServer
	server := &httpServer{
		router: router,
		port:   8080,
	}

	// Start the server
	go server.Run(container)

	// Wait for the server to start listening
	time.Sleep(1 * time.Second)

	// Send a GET request to the server
	resp, err := http.Get("http://localhost:8080")
	if err != nil {
		t.Errorf("Failed to send GET request: %v", err)
	}

	// Check if the server is listening
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, but got %d", http.StatusOK, resp.StatusCode)
	}
}
