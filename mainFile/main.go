package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"gofr.dev/internal/auth/ldapauth"
)

func main() {
	cfg := ldapauth.Config{
		Addr:         "localhost:389",
		BaseDN:       "dc=example,dc=com",
		BindUserDN:   "cn=admin,dc=example,dc=com",
		BindPassword: "admin",
		JWTSecret:    "super-secret-key",
	}

	authenticator := ldapauth.New(cfg)

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		loginHandler(w, r, authenticator)
	})

	fmt.Println("Listening on :8080")

	//  Add HTTP Timeouts
	srv := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}


func loginHandler(w http.ResponseWriter, r *http.Request, authenticator *ldapauth.Authenticator) {
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	token, err := authenticator.Authenticate(creds.Username, creds.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := json.NewEncoder(w).Encode(map[string]string{"token": token}); err != nil {
		log.Println("encode error:", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
	}
}


