package ldapauth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/golang-jwt/jwt/v4"
)

// mockConn implements Conn for controlled test behavior.

type mockConn struct {
	bindErr map[string]error

	searchResult *ldap.SearchResult

	searchErr error

	closed bool
}

func (m *mockConn) Bind(username, password string) error {

	if err, ok := m.bindErr[username+"|"+password]; ok {

		return err

	}

	return nil

}

func (m *mockConn) Search(req *ldap.SearchRequest) (*ldap.SearchResult, error) {

	return m.searchResult, m.searchErr

}

func (m *mockConn) Close() error {

	m.closed = true

	return nil

}

func TestAuthenticate(t *testing.T) {
    t.Run("LDAP destination does not exist", func(t *testing.T) {
        auth := New(Config{Addr: "x", BaseDN: "d", JWTSecret: "s"})
        auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) {
            return nil, errors.New("dial error")
        })
        _, err := auth.Authenticate("jdoe", "ignored")
        if err == nil || !strings.Contains(err.Error(), "connect LDAP") {
            t.Fatalf("expected connection error, got %v", err)
        }
    })

    t.Run("Service bind unauthorized", func(t *testing.T) {
        mc := &mockConn{
            bindErr:      map[string]error{"cn=admin,dc=ex,dc=com|bad": errors.New("uhoh")},
            searchResult: &ldap.SearchResult{Entries: []*ldap.Entry{{DN: "uid=jdoe,dc=ex,dc=com"}}},
        }
        auth := New(Config{
            Addr:         "x",
            BaseDN:       "d",
            JWTSecret:    "secret",
            BindUserDN:   "cn=admin,dc=ex,dc=com",
            BindPassword: "bad",
        })
        auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) { return mc, nil })
        _, err := auth.Authenticate("jdoe", "secret")
        if err == nil || !strings.Contains(err.Error(), "service bind failed") {
            t.Fatalf("expected bind error, got %v", err)
        }
    })

    t.Run("Invalid credentials", func(t *testing.T) {
        mc := &mockConn{
            bindErr:      map[string]error{"uid=jdoe,dc=ex,dc=com|wrong": errors.New("bad")},
            searchResult: &ldap.SearchResult{Entries: []*ldap.Entry{{DN: "uid=jdoe,dc=ex,dc=com"}}},
        }
        auth := New(Config{Addr: "x", BaseDN: "d", JWTSecret: "secret"})
        auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) { return mc, nil })
        _, err := auth.Authenticate("jdoe", "wrong")
        if err == nil || !strings.Contains(err.Error(), "invalid credentials") {
            t.Fatalf("expected invalid credentials error, got %v", err)
        }
    })

    t.Run("Expired password", func(t *testing.T) {
        errExpired := ldap.NewError(ldap.LDAPResultInvalidCredentials, errors.New("expired"))
        mc := &mockConn{
            bindErr:      map[string]error{"uid=jdoe,dc=ex,dc=com|secret": errExpired},
            searchResult: &ldap.SearchResult{Entries: []*ldap.Entry{{DN: "uid=jdoe,dc=ex,dc=com"}}},
        }
        auth := New(Config{Addr: "x", BaseDN: "d", JWTSecret: "secret"})
        auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) { return mc, nil })
        _, err := auth.Authenticate("jdoe", "secret")
        if err == nil || !strings.Contains(err.Error(), "invalid credentials") {
            t.Fatalf("expected expired password error, got %v", err)
        }
    })

    t.Run("BindSuccess but info retrieval failed", func(t *testing.T) {
        mc := &mockConn{searchErr: errors.New("search down")}
        auth := New(Config{Addr: "x", BaseDN: "d", JWTSecret: "secret"})
        auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) { return mc, nil })
        _, err := auth.Authenticate("jdoe", "secret")
        if err == nil || !strings.Contains(err.Error(), "search error") {
            t.Fatalf("expected search failure error, got %v", err)
        }
    })

    t.Run("App unable to generate JWT", func(t *testing.T) {
        mc := &mockConn{searchResult: &ldap.SearchResult{Entries: []*ldap.Entry{{DN: "uid=jdoe,dc=ex,dc=com"}}}}
        auth := New(Config{Addr: "x", BaseDN: "d", JWTSecret: ""}) // no secret = JWT signing fails
        auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) { return mc, nil })
        _, err := auth.Authenticate("jdoe", "secret")
        if err == nil || !strings.Contains(err.Error(), "token signing error") {
            t.Fatalf("expected JWT signing error, got %v", err)
        }
    })

    t.Run("Successful authentication", func(t *testing.T) {
        mc := &mockConn{searchResult: &ldap.SearchResult{Entries: []*ldap.Entry{{DN: "uid=jdoe,dc=ex,dc=com"}}}}
        auth := New(Config{Addr: "x", BaseDN: "d", JWTSecret: "secret"})
        auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) { return mc, nil })
        token, err := auth.Authenticate("jdoe", "secret")
        if err != nil {
            t.Fatalf("unexpected error: %v", err)
        }
        sub, err := auth.ValidateToken(token)
        if err != nil {
            t.Fatalf("token validation failed: %v", err)
        }
        if sub != "jdoe" {
            t.Fatalf("expected sub=jdoe; got %s", sub)
        }
    })
}


func TestValidateTokenExpired(t *testing.T) {

	cfg := Config{JWTSecret: "secret"}

	auth := New(cfg)

	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "joe", "exp": time.Now().Add(-1 * time.Hour).Unix()})

	tok, _ := expired.SignedString([]byte(cfg.JWTSecret))

	_, err := auth.ValidateToken(tok)

	if err == nil || (!strings.Contains(err.Error(), "token parse error") && !strings.Contains(err.Error(), "invalid token")) {

		t.Fatalf("expected expired-token error, got %v", err)

	}

}
