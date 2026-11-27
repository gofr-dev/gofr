package main

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/http/middleware"
)

// lookupTimeout bounds one store lookup. When it passes, the request is answered with a 503.
const lookupTimeout = 2 * time.Second

// permissionsClaim is the claim livePermissions adds and configs/rbac.json reads
// ("permissionsClaimPath": "permissions").
const permissionsClaim = "permissions"

var errStoreDown = errors.New("permission store unavailable")

// grants is the application's permission store. A real app keeps it in a database table or a policy
// service; an in-memory map keeps this example self-contained.
type grants struct {
	mu      sync.RWMutex
	perms   map[string][]string // subject -> permissions
	failing bool                // when set, every lookup fails, to simulate an outage
	lookups atomic.Int64        // how many lookups were made
}

// Permissions returns what subject may do right now. A real implementation queries its database with
// ctx, which carries the lookup deadline.
func (g *grants) Permissions(ctx context.Context, subject string) ([]string, error) {
	g.lookups.Add(1)

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	if g.failing {
		return nil, errStoreDown
	}

	return slices.Clone(g.perms[subject]), nil
}

// revoke removes permission from subject. The next request subject makes is authorized without it,
// even though the token it carries has not expired.
func (g *grants) revoke(subject, permission string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.perms[subject] = slices.DeleteFunc(g.perms[subject], func(p string) bool { return p == permission })
}

// grant adds permission to subject.
func (g *grants) grant(subject, permission string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !slices.Contains(g.perms[subject], permission) {
		g.perms[subject] = append(g.perms[subject], permission)
	}
}

func (g *grants) setFailing(failing bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.failing = failing
}

//nolint:gochecknoglobals // The example's store; the test changes it between requests.
var store = &grants{perms: map[string][]string{
	"alice": {"orders:read", "orders:write"},
	"bob":   {"orders:read"},
}}

// publicRoutes are the routes configs/rbac.json marks "public": true. They need no permission check,
// so livePermissions leaves them alone and never queries the store for them.
//
//nolint:gochecknoglobals // Read-only table, kept next to the rules it mirrors.
var publicRoutes = map[string]bool{"/status": true}

func main() {
	app := gofr.New()

	// OAuth verifies the token and puts its claims in the request. Validate the issuer, audience and
	// expiry here: the middleware below trusts the token's subject. An empty issuer makes
	// jwt.WithIssuer a no-op, so refuse to start without one.
	issuer := app.Config.Get("TOKEN_ISSUER")
	if issuer == "" {
		app.Logger().Fatalf("TOKEN_ISSUER is not set")
	}

	// The three calls append to one middleware chain in call order, so this order is the order they
	// run in: OAuth, then livePermissions, then RBAC.
	if err := app.EnableOAuth(app.Config.Get("JWKS_URL"), 10,
		jwt.WithIssuer(issuer),
		jwt.WithAudience("orders-api"),
		jwt.WithExpirationRequired(),
	); err != nil {
		app.Logger().Fatalf("%v", err)
	}

	app.UseMiddleware(livePermissions(store.Permissions, lookupTimeout))

	// configs/rbac.json sets "permissionsClaimPath": "permissions". Every route below has a rule there,
	// and configs/.env sets GOFR_RBAC_ROUTE_CHECK=fail: a route added without a rule stops startup
	// instead of being served with no authorization.
	if err := app.EnableRBAC(); err != nil {
		app.Logger().Fatalf("%v", err)
	}

	app.GET("/status", status)
	app.GET("/orders", listOrders)
	app.POST("/orders", createOrder)

	app.Run()
}

// livePermissions looks up the verified token's subject in the store and adds what it finds to the
// request's claims as a "permissions" claim, which EnableRBAC reads through permissionsClaimPath.
//
// It must be registered after EnableOAuth (which provides the claims) and before EnableRBAC (which
// reads the claim).
func livePermissions(lookup func(ctx context.Context, sub string) ([]string, error),
	timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// GoFr's own /.well-known/ routes (health, alive) and public routes need no permission
			// check, so a health probe never reaches the store.
			if strings.HasPrefix(r.URL.Path, "/.well-known/") || publicRoutes[r.URL.Path] {
				next.ServeHTTP(w, r)

				return
			}

			claims, _ := r.Context().Value(middleware.JWTClaim).(jwt.MapClaims)

			sub, _ := claims["sub"].(string)
			if sub == "" {
				// No subject to look up. Drop any permissions claim the token itself carries, so RBAC
				// sees none and answers 403 instead of trusting a list the store never confirmed.
				next.ServeHTTP(w, withoutPermissions(r, claims))

				return
			}

			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()

			perms, err := lookup(ctx, sub)
			if err != nil {
				// Fail closed: without an answer from the store, nothing is allowed.
				http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)

				return
			}

			list := make([]any, len(perms)) // RBAC reads the claim as []any
			for i, p := range perms {
				list[i] = p
			}

			// Copy the claims: the token's own map must not change under other readers.
			merged := maps.Clone(claims)
			merged[permissionsClaim] = list

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), middleware.JWTClaim, merged)))
		})
	}
}

// withoutPermissions returns r with the permissions claim removed from its claims. The claims are
// copied: the token's own map must not change under other readers.
func withoutPermissions(r *http.Request, claims jwt.MapClaims) *http.Request {
	if _, ok := claims[permissionsClaim]; !ok {
		return r
	}

	stripped := maps.Clone(claims)
	delete(stripped, permissionsClaim)

	return r.WithContext(context.WithValue(r.Context(), middleware.JWTClaim, stripped))
}

func status(_ *gofr.Context) (any, error) {
	return "ok", nil
}

func listOrders(_ *gofr.Context) (any, error) {
	return []string{"order-1", "order-2"}, nil
}

func createOrder(_ *gofr.Context) (any, error) {
	return "order created", nil
}
