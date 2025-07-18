package ldapauth

import (
	"errors"
	"testing"

	"github.com/go-ldap/ldap/v3"
)

func TestAuthenticate_InvalidUser(t *testing.T) {
	auth := &Authenticator{
		cfg: Config{
			Addr:      "fakehost",
			BaseDN:    "dc=example,dc=com",
			JWTSecret: "testsecret",
		},
		dialFn: func(addr string, opts ...ldap.DialOpt) (*ldap.Conn, error) {
    return nil, errors.New("dial error")
    },

	}

	_, err := auth.Authenticate("testuser", "testpass")
	if err == nil {
		t.Fatal("expected error for invalid dial")
	}
}
