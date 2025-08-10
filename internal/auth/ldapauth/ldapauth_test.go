package ldapauth

import (
	"errors"
	"testing"

	"github.com/go-ldap/ldap/v3"
	"gofr.dev/internal/auth/jwt"
)

// Static sentinel errors for tests (avoid dynamic errors per err113).
var (
	errDial             = errors.New("dial error")
	errBind             = errors.New("uhoh")
	errBad              = errors.New("bad")
	errExpiredCause     = errors.New("expired")
	errSearchDown       = errors.New("search down")
	errTokenIssueFailed = errors.New("token issue failed")
)

// mockTokenIssuer for testing.
type mockTokenIssuer struct {
	issueErr    error
	validateErr error
	token       string
	username    string
}

func (m *mockTokenIssuer) IssueToken(_ jwt.User) (string, error) {
	if m.issueErr != nil {
		return "", m.issueErr
	}

	return m.token, nil
}

func (m *mockTokenIssuer) ValidateToken(_ string) (string, error) {
	if m.validateErr != nil {
		return "", m.validateErr
	}

	return m.username, nil
}

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

func (m *mockConn) Search(_ *ldap.SearchRequest) (*ldap.SearchResult, error) {
	return m.searchResult, m.searchErr
}

func (m *mockConn) Close() error {
	m.closed = true
	return nil
}

func TestAuthenticate_LDAPDestinationDoesNotExist(t *testing.T) {
	tokenIssuer := &mockTokenIssuer{token: "test-token"}
	auth := New(&Config{
		Addr:        "x",
		BaseDN:      "d",
		TokenIssuer: tokenIssuer,
	})
	auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) {
		return nil, errDial
	})

	_, err := auth.Authenticate("jdoe", "ignored")
	if err == nil || err.Error() != "failed to connect LDAP: dial error" {
		t.Fatalf("expected connection error, got %v", err)
	}
}

func TestAuthenticate_ServiceBindUnauthorized(t *testing.T) {
	mc := &mockConn{
		bindErr:      map[string]error{"cn=admin,dc=ex,dc=com|bad": errBind},
		searchResult: &ldap.SearchResult{Entries: []*ldap.Entry{{DN: "uid=jdoe,dc=ex,dc=com"}}},
	}

	tokenIssuer := &mockTokenIssuer{token: "test-token"}
	auth := New(&Config{
		Addr:         "x",
		BaseDN:       "d",
		TokenIssuer:  tokenIssuer,
		BindUserDN:   "cn=admin,dc=ex,dc=com",
		BindPassword: "bad",
	})
	auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) { return mc, nil })

	_, err := auth.Authenticate("jdoe", "secret")
	if err == nil || err.Error() != "service bind failed: uhoh" {
		t.Fatalf("expected bind error, got %v", err)
	}
}

func TestAuthenticate_UserNotFound(t *testing.T) {
	mc := &mockConn{
		searchResult: &ldap.SearchResult{Entries: []*ldap.Entry{}},
	}

	tokenIssuer := &mockTokenIssuer{token: "test-token"}
	auth := New(&Config{
		Addr:        "x",
		BaseDN:      "d",
		TokenIssuer: tokenIssuer,
	})
	auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) { return mc, nil })

	_, err := auth.Authenticate("nonexistent", "secret")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestAuthenticate_MultipleUsersFound(t *testing.T) {
	mc := &mockConn{
		searchResult: &ldap.SearchResult{
			Entries: []*ldap.Entry{
				{DN: "uid=jdoe1,dc=ex,dc=com"},
				{DN: "uid=jdoe2,dc=ex,dc=com"},
			},
		},
	}

	tokenIssuer := &mockTokenIssuer{token: "test-token"}
	auth := New(&Config{
		Addr:        "x",
		BaseDN:      "d",
		TokenIssuer: tokenIssuer,
	})
	auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) { return mc, nil })

	_, err := auth.Authenticate("jdoe", "secret")
	if !errors.Is(err, ErrMultipleUsersFound) {
		t.Fatalf("expected ErrMultipleUsersFound, got %v", err)
	}
}

func TestAuthenticate_InvalidCredentials(t *testing.T) {
	mc := &mockConn{
		bindErr:      map[string]error{"uid=jdoe,dc=ex,dc=com|wrong": errBad},
		searchResult: &ldap.SearchResult{Entries: []*ldap.Entry{{DN: "uid=jdoe,dc=ex,dc=com"}}},
	}

	tokenIssuer := &mockTokenIssuer{token: "test-token"}
	auth := New(&Config{
		Addr:        "x",
		BaseDN:      "d",
		TokenIssuer: tokenIssuer,
	})
	auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) { return mc, nil })

	_, err := auth.Authenticate("jdoe", "wrong")
	if err == nil || err.Error() != "invalid credentials: bad" {
		t.Fatalf("expected invalid credentials error, got %v", err)
	}
}

func TestAuthenticate_ExpiredPassword(t *testing.T) {
	errExpired := ldap.NewError(ldap.LDAPResultInvalidCredentials, errExpiredCause)

	mc := &mockConn{
		bindErr:      map[string]error{"uid=jdoe,dc=ex,dc=com|secret": errExpired},
		searchResult: &ldap.SearchResult{Entries: []*ldap.Entry{{DN: "uid=jdoe,dc=ex,dc=com"}}},
	}

	tokenIssuer := &mockTokenIssuer{token: "test-token"}
	auth := New(&Config{
		Addr:        "x",
		BaseDN:      "d",
		TokenIssuer: tokenIssuer,
	})
	auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) { return mc, nil })

	_, err := auth.Authenticate("jdoe", "secret")
	if err == nil || err.Error() != "invalid credentials: LDAP Result Code 49 \"Invalid Credentials\": expired" {
		t.Fatalf("expected expired password error, got %v", err)
	}
}

func TestAuthenticate_SearchError(t *testing.T) {
	mc := &mockConn{searchErr: errSearchDown}

	tokenIssuer := &mockTokenIssuer{token: "test-token"}
	auth := New(&Config{
		Addr:        "x",
		BaseDN:      "d",
		TokenIssuer: tokenIssuer,
	})
	auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) { return mc, nil })

	_, err := auth.Authenticate("jdoe", "secret")
	if err == nil || err.Error() != "search error: search down" {
		t.Fatalf("expected search failure error, got %v", err)
	}
}

func TestAuthenticate_TokenIssuanceFailed(t *testing.T) {
	mc := &mockConn{
		searchResult: &ldap.SearchResult{Entries: []*ldap.Entry{{DN: "uid=jdoe,dc=ex,dc=com"}}},
	}

	tokenIssuer := &mockTokenIssuer{issueErr: errTokenIssueFailed}
	auth := New(&Config{
		Addr:        "x",
		BaseDN:      "d",
		TokenIssuer: tokenIssuer,
	})
	auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) { return mc, nil })

	_, err := auth.Authenticate("jdoe", "secret")
	if err == nil || err.Error() != "token issuance failed: token issue failed" {
		t.Fatalf("expected token issuance error, got %v", err)
	}
}

func TestAuthenticate_Successful(t *testing.T) {
	mc := &mockConn{
		searchResult: &ldap.SearchResult{Entries: []*ldap.Entry{{DN: "uid=jdoe,dc=ex,dc=com"}}},
	}

	tokenIssuer := &mockTokenIssuer{token: "test-token", username: "jdoe"}
	auth := New(&Config{
		Addr:        "x",
		BaseDN:      "d",
		TokenIssuer: tokenIssuer,
	})
	auth.WithDialer(func(_ string, _ ...ldap.DialOpt) (Conn, error) { return mc, nil })

	token, err := auth.Authenticate("jdoe", "secret")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if token != "test-token" {
		t.Fatalf("expected token=test-token; got %s", token)
	}

	sub, err := auth.ValidateToken(token)
	if err != nil {
		t.Fatalf("token validation failed: %v", err)
	}

	if sub != "jdoe" {
		t.Fatalf("expected sub=jdoe; got %s", sub)
	}
}
