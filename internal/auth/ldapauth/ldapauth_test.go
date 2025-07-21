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
    bindErr      map[string]error
    searchResult *ldap.SearchResult
    searchErr    error
    closed       bool
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
    goodEntry := &ldap.Entry{DN: "uid=jdoe,dc=ex,dc=com"}

    cases := []struct {
        name          string
        setupConn     func() *mockConn
        password      string
        wantErrSubstr string
    }{
        {"LDAP destination does not exist", func() *mockConn { return nil }, "ignored", "connect LDAP"},
        {"Service bind unauthorized", func() *mockConn {
            return &mockConn{bindErr: map[string]error{"cn=admin,dc=ex,dc=com|bad": errors.New("uhoh")}, searchResult: &ldap.SearchResult{Entries: []*ldap.Entry{goodEntry}}}
        }, "secret", "service bind failed"},
        {"Invalid credentials", func() *mockConn {
            return &mockConn{bindErr: map[string]error{"uid=jdoe,dc=ex,dc=com|wrong": errors.New("bad")}, searchResult: &ldap.SearchResult{Entries: []*ldap.Entry{goodEntry}}}
        }, "wrong", "invalid credentials"},
        {"Expired password", func() *mockConn {
            errExpired := ldap.NewError(ldap.LDAPResultInvalidCredentials, errors.New("expired"))
            return &mockConn{bindErr: map[string]error{"uid=jdoe,dc=ex,dc=com|secret": errExpired}, searchResult: &ldap.SearchResult{Entries: []*ldap.Entry{goodEntry}}}
        }, "secret", "invalid credentials"},
        {"BindSuccess but info retrieval failed", func() *mockConn { return &mockConn{searchErr: errors.New("search down")} }, "secret", "search error"},
        {"App unable to generate JWT", func() *mockConn { return &mockConn{searchResult: &ldap.SearchResult{Entries: []*ldap.Entry{goodEntry}}} }, "secret", "token signing error"},
        {"Successful authentication", func() *mockConn { return &mockConn{searchResult: &ldap.SearchResult{Entries: []*ldap.Entry{goodEntry}}} }, "secret", ""},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            var auth *Authenticator
            if tc.name == "LDAP destination does not exist" {
                auth = New(Config{Addr: "x", BaseDN: "d", JWTSecret: "s"})
                auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) {
                    return nil, errors.New("dial error")
                })
            } else {
                mc := tc.setupConn()
                auth = New(Config{Addr: "x", BaseDN: "d", JWTSecret: "secret"})
                auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) { return mc, nil })
                if tc.name == "Service bind unauthorized" {
                    auth.cfg.BindUserDN = "cn=admin,dc=ex,dc=com"
                    auth.cfg.BindPassword = "bad"
                }
                if tc.name == "App unable to generate JWT" {
                    auth.cfg.JWTSecret = ""
                }
            }

            token, err := auth.Authenticate("jdoe", tc.password)
            if tc.wantErrSubstr != "" {
                if err == nil || !strings.Contains(err.Error(), tc.wantErrSubstr) {
                    t.Fatalf("expected error containing %q, got %v", tc.wantErrSubstr, err)
                }
                return
            }
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