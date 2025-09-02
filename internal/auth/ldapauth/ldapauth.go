package ldapauth

import (
	"errors"
	"fmt"

	"github.com/go-ldap/ldap/v3"

	"gofr.dev/internal/auth/jwt"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrMultipleUsersFound = errors.New("multiple users found - LDAP configuration error")
)

// User represents LDAP user information.
type User struct {
	DN         string            `json:"dn"`
	Username   string            `json:"username"`
	Email      string            `json:"email,omitempty"`
	FullName   string            `json:"full_name,omitempty"`
	Groups     []string          `json:"groups,omitempty"`
	Department string            `json:"department,omitempty"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// GetDN returns the distinguished name.
func (u *User) GetDN() string { return u.DN }

// GetUsername returns the username.
func (u *User) GetUsername() string { return u.Username }

// GetEmail returns the email.
func (u *User) GetEmail() string { return u.Email }

// GetFullName returns the full name.
func (u *User) GetFullName() string { return u.FullName }

// GetGroups returns the groups.
func (u *User) GetGroups() []string { return u.Groups }

// GetDepartment returns the department.
func (u *User) GetDepartment() string { return u.Department }

// Conn abstracts ldap.Conn methods for easier testing.
type Conn interface {
	Bind(username, password string) error
	Search(searchReq *ldap.SearchRequest) (*ldap.SearchResult, error)
	Close() error
}

// TokenIssuer interface for token issuance - allows different implementations.
type TokenIssuer interface {
	IssueToken(user jwt.User) (string, error)
	ValidateToken(token string) (string, error)
}

// dialerFunc connects to LDAP and returns a Conn.
type dialerFunc func(addr string, opts ...ldap.DialOpt) (Conn, error)

// defaultDialer uses ldap.DialURL under the hood.
func defaultDialer(addr string, opts ...ldap.DialOpt) (Conn, error) {
	return ldap.DialURL(addr, opts...)
}

// Config holds LDAP settings and token issuer.
type Config struct {
	Addr           string
	BaseDN         string
	BindUserDN     string
	BindPassword   string
	TokenIssuer    TokenIssuer
	UserAttributes []string
	UsernameAttr   string
	EmailAttr      string
	FullNameAttr   string
	DepartmentAttr string
}

// Authenticator handles LDAP authentication and token issuance.
type Authenticator struct {
	cfg    Config
	dialFn dialerFunc
}

// New returns an Authenticator with the default dialer.
func New(cfg *Config) *Authenticator {
	// Set default attribute mappings if not specified.
	if cfg.UsernameAttr == "" {
		cfg.UsernameAttr = "uid"
	}

	if cfg.EmailAttr == "" {
		cfg.EmailAttr = "mail"
	}

	if cfg.FullNameAttr == "" {
		cfg.FullNameAttr = "cn"
	}

	if cfg.DepartmentAttr == "" {
		cfg.DepartmentAttr = "ou"
	}

	// Default attributes to retrieve if not specified.
	if len(cfg.UserAttributes) == 0 {
		cfg.UserAttributes = []string{
			"dn", cfg.UsernameAttr, cfg.EmailAttr,
			cfg.FullNameAttr, cfg.DepartmentAttr, "memberOf",
		}
	}

	return &Authenticator{
		cfg:    *cfg,
		dialFn: defaultDialer,
	}
}

// WithDialer allows injecting a custom dialer (for tests).
func (a *Authenticator) WithDialer(d dialerFunc) {
	a.dialFn = d
}

// Authenticate binds to LDAP, validates credentials, and returns a signed token.
func (a *Authenticator) Authenticate(username, password string) (string, error) {
	conn, err := a.dialFn("ldap://" + a.cfg.Addr)
	if err != nil {
		return "", fmt.Errorf("failed to connect LDAP: %w", err)
	}
	defer conn.Close()

	if a.cfg.BindUserDN != "" {
		bindErr := conn.Bind(a.cfg.BindUserDN, a.cfg.BindPassword)
		if bindErr != nil {
			return "", fmt.Errorf("service bind failed: %w", bindErr)
		}
	}

	user, err := a.lookupUser(conn, username)
	if err != nil {
		return "", err
	}

	authErr := bindUser(conn, user.DN, password)
	if authErr != nil {
		return "", authErr
	}

	token, err := a.cfg.TokenIssuer.IssueToken(user)
	if err != nil {
		return "", fmt.Errorf("token issuance failed: %w", err)
	}

	return token, nil
}

// GetUserInfo retrieves user information without authentication.
func (a *Authenticator) GetUserInfo(username string) (*User, error) {
	conn, err := a.dialFn("ldap://" + a.cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect LDAP: %w", err)
	}
	defer conn.Close()

	if a.cfg.BindUserDN != "" {
		bindErr := conn.Bind(a.cfg.BindUserDN, a.cfg.BindPassword)
		if bindErr != nil {
			return nil, fmt.Errorf("service bind failed: %w", bindErr)
		}
	}

	return a.lookupUser(conn, username)
}

// ValidateToken validates a token using the configured token issuer.
func (a *Authenticator) ValidateToken(token string) (string, error) {
	return a.cfg.TokenIssuer.ValidateToken(token)
}

// lookupUser retrieves comprehensive user information from LDAP.
func (a *Authenticator) lookupUser(conn Conn, username string) (*User, error) {
	searchReq := ldap.NewSearchRequest(
		a.cfg.BaseDN,
		ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 0, 0, false,
		fmt.Sprintf("(%s=%s)", a.cfg.UsernameAttr, ldap.EscapeFilter(username)),
		a.cfg.UserAttributes,
		nil,
	)

	result, err := conn.Search(searchReq)
	if err != nil {
		return nil, fmt.Errorf("search error: %w", err)
	}

	// Handle different result scenarios separately.
	switch len(result.Entries) {
	case 0:
		return nil, ErrUserNotFound
	case 1:
		return a.buildUserFromEntry(result.Entries[0], username), nil
	default:
		return nil, ErrMultipleUsersFound
	}
}

// buildUserFromEntry constructs a User object from LDAP entry.
func (a *Authenticator) buildUserFromEntry(entry *ldap.Entry, username string) *User {
	user := &User{
		DN:         entry.DN,
		Username:   username,
		Attributes: make(map[string]string),
	}

	// Extract standard attributes.
	if email := entry.GetAttributeValue(a.cfg.EmailAttr); email != "" {
		user.Email = email
	}

	if fullName := entry.GetAttributeValue(a.cfg.FullNameAttr); fullName != "" {
		user.FullName = fullName
	}

	if dept := entry.GetAttributeValue(a.cfg.DepartmentAttr); dept != "" {
		user.Department = dept
	}

	// Extract group memberships.
	if groups := entry.GetAttributeValues("memberOf"); len(groups) > 0 {
		user.Groups = groups
	}

	// Extract all other attributes into the Attributes map.
	for _, attr := range entry.Attributes {
		// Skip attributes we've already processed.
		switch attr.Name {
		case a.cfg.EmailAttr, a.cfg.FullNameAttr, a.cfg.DepartmentAttr, "memberOf":
			continue
		default:
			if len(attr.Values) > 0 {
				user.Attributes[attr.Name] = attr.Values[0]
			}
		}
	}

	return user
}

func bindUser(conn Conn, userDN, password string) error {
	if err := conn.Bind(userDN, password); err != nil {
		return fmt.Errorf("invalid credentials: %w", err)
	}

	return nil
}
