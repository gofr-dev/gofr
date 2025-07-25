package ldapauth

import (
    "fmt"
    "time"

    "github.com/go-ldap/ldap/v3"
    "github.com/golang-jwt/jwt/v4"
)

// Conn abstracts ldap.Conn methods for easier testing.
type Conn interface {
    Bind(username, password string) error
    Search(searchReq *ldap.SearchRequest) (*ldap.SearchResult, error)
    Close() error
}

// dialerFunc connects to LDAP and returns a Conn.
type dialerFunc func(addr string, opts ...ldap.DialOpt) (Conn, error)

// defaultDialer uses ldap.DialURL under the hood.
func defaultDialer(addr string, opts ...ldap.DialOpt) (Conn, error) {
    return ldap.DialURL(addr, opts...)
}

// Config holds LDAP and JWT settings.
type Config struct {
    Addr         string  // LDAP server address (e.g., "localhost:389" or "ldap.example.com:389")
    BaseDN       string  // Base distinguished name (DN) used to search for users (e.g., "dc=example,dc=com")
    BindUserDN   string // Optional: DN of a service account to perform search operations.
                        // Required if anonymous search is disabled on the LDAP server.

    BindPassword string // Optional: Password for the service account specified in BindUserDN.
                        // Required only if BindUserDN is set.
    JWTSecret    string // Secret key used to sign JWT tokens issued after successful authentication.
}

// Authenticator handles LDAP authentication and JWT issuance.
type Authenticator struct {
    cfg    Config
    dialFn dialerFunc
}

// New returns an Authenticator with the default dialer.
func New(cfg Config) *Authenticator {
    return &Authenticator{
        cfg:    cfg,
        dialFn: defaultDialer,
    }
}

// WithDialer allows injecting a custom dialer (for tests).
func (a *Authenticator) WithDialer(d dialerFunc) {
    a.dialFn = d
}

// Authenticate binds to LDAP, validates credentials, and returns a signed JWT.
func (a *Authenticator) Authenticate(username, password string) (string, error) {
    conn, err := a.dialFn("ldap://" + a.cfg.Addr)
    if err != nil {
        return "", fmt.Errorf("failed to connect LDAP: %w", err)
    }
    defer conn.Close()

    if a.cfg.BindUserDN != "" {
        if err := conn.Bind(a.cfg.BindUserDN, a.cfg.BindPassword); err != nil {
            return "", fmt.Errorf("service bind failed: %w", err)
        }
    }

    userDN, err := a.lookupUserDN(conn, username)
    if err != nil {
        return "", err
    }

    if err := a.bindUser(conn, userDN, password); err != nil {
        return "", err
    }

    token, err := a.generateToken(username)
    if err != nil {
        return "", err
    }

    return token, nil
}


func (a *Authenticator) lookupUserDN(conn Conn, username string) (string, error) {
    searchReq := ldap.NewSearchRequest(
        a.cfg.BaseDN,
        ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
        fmt.Sprintf("(uid=%s)", ldap.EscapeFilter(username)),
        []string{"dn"},
        nil,
    )

    result, err := conn.Search(searchReq)
    if err != nil {
        return "", fmt.Errorf("search error: %w", err)
    }

    if len(result.Entries) != 1 {
        return "", fmt.Errorf("user not found or multiple entries returned")
    }

    return result.Entries[0].DN, nil
}

func (a *Authenticator) bindUser(conn Conn, userDN, password string) error {
    if err := conn.Bind(userDN, password); err != nil {
        return fmt.Errorf("invalid credentials: %w", err)
    }
    return nil
}

func (a *Authenticator) generateToken(username string) (string, error) {
    if a.cfg.JWTSecret == "" {
        return "", fmt.Errorf("token signing error: no secret configured")
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


// ValidateToken parses and validates a JWT, returning the 'sub' claim.
func (a *Authenticator) ValidateToken(tokenStr string) (string, error) {
    token, err := jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
        }
        return []byte(a.cfg.JWTSecret), nil
    })
    if err != nil {
        return "", fmt.Errorf("token parse error: %w", err)
    }
    if !token.Valid {
        return "", fmt.Errorf("invalid token")
    }
    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok {
        return "", fmt.Errorf("invalid claims")
    }
    sub, ok := claims["sub"].(string)
    if !ok {
        return "", fmt.Errorf("missing sub claim")
    }
    return sub, nil
}