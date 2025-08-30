package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"gofr.dev/internal/auth/jwt"
	"gofr.dev/internal/auth/ldapauth"
)

func main() {
	jwtIssuer := jwt.NewIssuer("super-secret-key", 1*time.Hour, true)

	cfg := ldapauth.Config{
		Addr:         "localhost:389",
		BaseDN:       "dc=example,dc=com",
		BindUserDN:   "cn=admin,dc=example,dc=com",
		BindPassword: "admin",
		TokenIssuer:  jwtIssuer,
		UserAttributes: []string{
			"dn", "uid", "mail", "cn", "ou", "memberOf", "telephoneNumber",
		},
		UsernameAttr:   "uid",
		EmailAttr:      "mail",
		FullNameAttr:   "cn",
		DepartmentAttr: "ou",
	}

	authenticator := ldapauth.New(&cfg)

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var creds struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		token, err := authenticator.Authenticate(creds.Username, creds.Password)
		if err != nil {
			switch {
			case errors.Is(err, ldapauth.ErrUserNotFound):
				http.Error(w, err.Error(), http.StatusUnauthorized) // 401
			case errors.Is(err, ldapauth.ErrMultipleUsersFound):
				http.Error(w, err.Error(), http.StatusInternalServerError) // 500
			default:
				http.Error(w, err.Error(), http.StatusUnauthorized) // default to 401 for bad creds
			}

			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"token": token})
	})

	http.HandleFunc("/userinfo", func(w http.ResponseWriter, r *http.Request) {
		userInfoHandler(w, r, authenticator)
	})

	fmt.Println("Listening on :8080")

	srv := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())
}

func userInfoHandler(w http.ResponseWriter, r *http.Request, authenticator *ldapauth.Authenticator) {
	username := r.URL.Query().Get("username")
	if username == "" {
		http.Error(w, "username parameter required", http.StatusBadRequest)
		return
	}

	user, err := authenticator.GetUserInfo(username)
	if err != nil {
		statusCode := getHTTPStatusForError(err)
		http.Error(w, err.Error(), statusCode)

		return
	}

	if err := json.NewEncoder(w).Encode(user); err != nil {
		log.Println("encode error:", err)
		http.Error(w, "failed to write response", http.StatusInternalServerError)
	}
}

func getHTTPStatusForError(err error) int {
	switch {
	case errors.Is(err, ldapauth.ErrUserNotFound):
		return http.StatusUnauthorized
	case errors.Is(err, ldapauth.ErrMultipleUsersFound):
		return http.StatusInternalServerError
	default:
		return http.StatusUnauthorized
	}
}
