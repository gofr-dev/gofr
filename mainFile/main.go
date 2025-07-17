package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"

    "ldap-auth-go/internal/ldapauth"
)

var authenticator *ldapauth.Authenticator

func main() {
    config := ldapauth.Config{
        Addr:         "localhost:389",                     // change this to your LDAP server
        BaseDN:       "dc=example,dc=com",                // adjust as per your directory
        BindUserDN:   "cn=admin,dc=example,dc=com",       // service account (optional)
        BindPassword: "admin",                            // service password
        JWTSecret:    "your-secret-key",                  // replace with strong key
    }

    authenticator = ldapauth.New(config)

    http.HandleFunc("/login", loginHandler)
    log.Println("Server running on http://localhost:8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
    var creds struct {
        Username string `json:"username"`
        Password string `json:"password"`
    }

    if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    token, err := authenticator.Authenticate(creds.Username, creds.Password)
    if err != nil {
        http.Error(w, fmt.Sprintf("Login failed: %v", err), http.StatusUnauthorized)
        return
    }

    // json.NewEncoder(w).Encode(map[string]string{
    //     "token": token,
    // })
    if err := json.NewEncoder(w).Encode(map[string]string{
    "token": token,
}); err != nil {
    http.Error(w, "Failed to write response", http.StatusInternalServerError)
    return
}

}
