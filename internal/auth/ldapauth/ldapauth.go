package ldapauth

import (
    // "crypto/tls"
    "fmt"
    "time"

    "github.com/go-ldap/ldap/v3"
    "github.com/golang-jwt/jwt/v4"
)

type Config struct {
    Addr         string
    BaseDN       string
    BindUserDN   string // optional
    BindPassword string // optional
    JWTSecret    string
}

type dialerFunc func(addr string, opts ...ldap.DialOpt) (*ldap.Conn, error)


type Authenticator struct {
    cfg Config
    dialFn dialerFunc
}

func New(cfg Config) *Authenticator {
    return &Authenticator{
        cfg: cfg,
        dialFn: ldap.DialURL,
    }
}

func (a *Authenticator) Authenticate(username, password string) (string, error) {
    // l, err := ldap.DialURL("ldap://" + a.cfg.Addr)
    l, err := a.dialFn("ldap://" + a.cfg.Addr)

    if err != nil {
        return "", fmt.Errorf("failed to connect LDAP: %w", err)
    }
    defer l.Close()

    // err = l.StartTLS(&tls.Config{InsecureSkipVerify: true})
    // if err != nil {
    //     return "", fmt.Errorf("TLS error: %w", err)
    // }

    if a.cfg.BindUserDN != "" {
        if err := l.Bind(a.cfg.BindUserDN, a.cfg.BindPassword); err != nil {
            return "", fmt.Errorf("bind failed: %w", err)
        }
    }

    searchReq := ldap.NewSearchRequest(
        a.cfg.BaseDN,
        ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
        fmt.Sprintf("(uid=%s)", ldap.EscapeFilter(username)),
        []string{"dn"},
        nil,
    )

    sr, err := l.Search(searchReq)
    if err != nil || len(sr.Entries) != 1 {
        return "", fmt.Errorf("user not found")
    }

    userDN := sr.Entries[0].DN

    if err := l.Bind(userDN, password); err != nil {
        return "", fmt.Errorf("invalid credentials")
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "sub": username,
        "exp": time.Now().Add(1 * time.Hour).Unix(),
    })

    signed, err := token.SignedString([]byte(a.cfg.JWTSecret))
    if err != nil {
        return "", fmt.Errorf("token signing error: %w", err)
    }

    return signed, nil
}
