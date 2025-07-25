package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"

    "gofr.dev/internal/auth/ldapauth"
)

var authenticator *ldapauth.Authenticator

func main() {
    cfg := ldapauth.Config{
    Addr:         "localhost:389",      // ← use localhost (or 127.0.0.1)
    BaseDN:       "dc=example,dc=com", 
    BindUserDN:   "cn=admin,dc=example,dc=com",
    BindPassword: "admin",
    JWTSecret:    "super-secret-key",
}

    authenticator = ldapauth.New(cfg)
    http.HandleFunc("/login", loginHandler)
    fmt.Println("Listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
    var creds struct{ Username, Password string }
    if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }
    token, err := authenticator.Authenticate(creds.Username, creds.Password)
    if err != nil {
        http.Error(w, err.Error(), http.StatusUnauthorized)
        return
    }
    json.NewEncoder(w).Encode(map[string]string{"token": token})
}