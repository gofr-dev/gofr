package middleware

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"gofr.dev/pkg/log"

	"github.com/go-ldap/ldap/v3"
)

// Ldap stores configuration options and an in-memory cache for LDAP authentication.
type Ldap struct {
	options *LDAPOptions
	cache   *LdapCache
}

// LdapCache represents the in-memory cache for LDAP authentication.
type LdapCache struct {
	mu    sync.RWMutex
	cache map[string]*CacheEntry
}

// CacheEntry represents entry in LdapCache.
type CacheEntry struct {
	authorized bool
	groups     map[string]bool
}

// MethodGroup is struct to specify HTTP methods and their associated groups.
type MethodGroup struct {
	Method string
	Group  string
}

// LDAPOptions stores the configurations for LDAP middleware options
type LDAPOptions struct {
	// ex: "^/v2/promises-composite/$": MethodGroup[{Method: "POST", Group: "promises-composite-write"}]
	// in case same method is present in multiple elements in the array, first occurrence would be considered
	RegexToMethodGroup map[string][]MethodGroup

	// ex: ldapstage.gofr.dev:636
	Addr string

	// TimeOut is the authentication timeout in seconds, if not set,
	// there is no timeout
	TimeOut int

	// CacheInvalidationFrequency sets the frequency(in seconds) with
	// which the in memory cache of authenticated users is reset;
	// if this is value not set, then there is no invalidation.
	// By default there is no cache invalidation
	CacheInvalidationFrequency int

	// InsecureSkipVerify controls whether a client verifies the
	// server's certificate chain and host name.
	// If InsecureSkipVerify is true, TLS accepts any certificate
	// presented by the server and any host name in that certificate.
	// In this mode, TLS is susceptible to man-in-the-middle attacks.
	// This should be used only for testing.
	InsecureSkipVerify bool
}

// NewLDAP is a factory function that creates and initializes an Ldap instance
func NewLDAP(logger log.Logger, options *LDAPOptions) (l *Ldap) {
	l = &Ldap{
		options: options,
		cache: &LdapCache{
			mu:    sync.RWMutex{},
			cache: map[string]*CacheEntry{},
		},
	}
	go l.invalidateCache(logger)

	return
}

// LDAP middleware enables basic authentication for inter-service calls using gofr.dev LDAP
func LDAP(logger log.Logger, options *LDAPOptions) func(inner http.Handler) http.Handler {
	l := NewLDAP(logger, options)

	return func(inner http.Handler) http.Handler {
		// in case LDAP Options are nil, then LDAP middleware is ignored
		if l.options == nil {
			logger.Warn("LDAP Middleware not enabled due to empty LDAP options")
			return inner
		}

		// in case the LDAP address is not configured, then LDAP middleware is ignored
		if l.options.Addr == "" {
			logger.Warn("LDAP Middleware not enabled due to empty LDAP Address. Set LDAP_ADDR env variable and use in LDAP Options ")
			return inner
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if ExemptPath(r) {
				inner.ServeHTTP(w, r)
				return
			}

			if err := l.Validate(logger, r); err != nil {
				description, code := GetDescription(err)
				e := FetchErrResponseWithCode(code, description, err.Error())
				ErrorResponse(w, r, logger, *e)
				return
			}

			inner.ServeHTTP(w, r)
		})
	}
}

// Validate an HTTP request against configured patterns based on the authentication and group validation results.
func (l *Ldap) Validate(logger log.Logger, r *http.Request) error {
	var requiredGroups []string

	for exp, mthGrps := range l.options.RegexToMethodGroup {
		ok, err := regexp.MatchString(exp, r.URL.EscapedPath())
		if err != nil {
			logger.Errorf("regex error: %v", err)

			return nil
		} else if !ok {
			continue
		}

		requiredGroups = getRequiredGroups(mthGrps, r.Method)
		if len(requiredGroups) > 0 {
			break
		}
	}

	if len(requiredGroups) == 0 {
		return nil
	}

	user, pass, err := getUsernameAndPassword(r.Header.Get("Authorization"))
	if err != nil {
		return err
	}

	entry := l.getEntry(user, pass)

	if !entry.authorized {
		err = ErrUnauthenticated
	} else if !validateGroups(requiredGroups, *entry) {
		err = ErrUnauthorised
	}

	return err
}

// getRequiredGroups performs all the  required parsing for groups and returns a slice of groups
func getRequiredGroups(methodGroups []MethodGroup, method string) []string {
	var groups []string

	for _, mthGrp := range methodGroups {
		if methodInCSV(method, mthGrp.Method) {
			groups = getListFromCSV(mthGrp.Group)
			break
		}
	}

	return groups
}

// validateGroups will validate the groups based on CacheEntry
func validateGroups(requiredGroups []string, entry CacheEntry) bool {
	if len(requiredGroups) == 0 {
		return true
	}

	for _, rg := range requiredGroups {
		if entry.groups[strings.TrimSpace(rg)] || rg == "" {
			return true
		}
	}

	return false
}

func getListFromCSV(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}

	list := strings.Split(csv, ",")
	for i, l := range list {
		list[i] = strings.TrimSpace(l)
	}

	return list
}

func methodInCSV(method, methodCSV string) bool {
	methods := strings.Split(methodCSV, ",")

	for _, m := range methods {
		if method == strings.TrimSpace(m) {
			return true
		}
	}

	return false
}

func (l *Ldap) invalidateCache(logger log.Logger) {
	for {
		l.cache.mu.Lock()
		l.cache.cache = make(map[string]*CacheEntry)
		l.cache.mu.Unlock()

		if l.options.CacheInvalidationFrequency <= 0 {
			logger.Debug("Caching disabled, exiting loop")
			break
		}

		time.Sleep(time.Second * time.Duration(l.options.CacheInvalidationFrequency))
	}
}

func (l *Ldap) getEntry(user, pass string) (entry *CacheEntry) {
	key := getCacheKey(user, pass)
	entry = l.getFromCache(key)

	if entry == nil {
		entryFromServer := l.getEntryFromLDAPServer(user, pass)
		entry = &entryFromServer
		l.addToCache(key, entry.groups)
	}

	return
}

func (l *Ldap) getEntryFromLDAPServer(user, pass string) (entry CacheEntry) {
	entries, err := l.callLdap(user, pass)
	if err != nil {
		return CacheEntry{authorized: false}
	}

	groups := getGroupsFromEntries(entries)

	entry = CacheEntry{authorized: true, groups: groups}

	return entry
}

func getCacheKey(user, pass string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(user + pass))

	return fmt.Sprintf("%d", h.Sum32())
}

func (l *Ldap) getFromCache(key string) (entry *CacheEntry) {
	l.cache.mu.RLock()

	entry = l.cache.cache[key]

	l.cache.mu.RUnlock()

	return entry
}

func (l *Ldap) addToCache(user string, groups map[string]bool) {
	if l.options.CacheInvalidationFrequency > 0 {
		entry := &CacheEntry{authorized: true, groups: groups}

		l.cache.mu.Lock()
		l.cache.cache[user] = entry

		l.cache.mu.Unlock()
	}
}

func getGroupsFromEntries(entries []*ldap.Entry) map[string]bool {
	validGroups := make(map[string]bool)

	for _, entry := range entries {
		groups := entry.GetAttributeValues("groupmembership")

		for _, group := range groups {
			for _, g := range strings.Split(group, ",") {
				if strings.Contains(g, "cn=") {
					g = strings.TrimLeft(g, "cn=")
					validGroups[g] = true

					break
				}
			}
		}
	}

	return validGroups
}

func (l *Ldap) callLdap(user, pass string) ([]*ldap.Entry, error) {
	options := l.options
	//nolint:gosec // TLS InsecureSkipVerify value will be provided by the user
	tlsConfig := &tls.Config{InsecureSkipVerify: options.InsecureSkipVerify}

	conn, er := ldap.DialTLS("tcp", options.Addr, tlsConfig)
	if er != nil {
		return nil, er
	}
	defer conn.Close()

	if err := conn.Bind(fmt.Sprintf("cn=%v,ou=svc,ou=people,o=gofr.dev", user), pass); err != nil {
		return nil, err
	}

	searchRequest := ldap.NewSearchRequest(
		fmt.Sprintf("cn=%v,ou=svc,ou=people,o=gofr.dev", user),
		ldap.ScopeBaseObject, ldap.NeverDerefAliases, 0, options.TimeOut, false,
		"(objectClass=*)", []string{"groupmembership"}, nil)

	sr, err := conn.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	return sr.Entries, nil
}

func getUsernameAndPassword(authHeader string) (user, pass string, err error) {
	const authLen = 2
	auth := strings.SplitN(authHeader, " ", authLen)

	if authHeader == "" {
		return "", "", ErrMissingHeader
	}

	if len(auth) != authLen || auth[0] != "Basic" {
		return "", "", ErrInvalidHeader
	}

	payload, _ := base64.StdEncoding.DecodeString(auth[1])
	pair := strings.SplitN(string(payload), ":", authLen)

	if len(pair) < authLen {
		return "", "", ErrInvalidToken
	}

	return pair[0], pair[1], nil
}
