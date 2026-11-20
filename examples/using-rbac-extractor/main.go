package main

import (
	"net/http"
	"slices"
	"sync"

	"github.com/golang-jwt/jwt/v5"

	"gofr.dev/pkg/gofr"
	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/http/middleware"
	"gofr.dev/pkg/gofr/rbac"
)

// grants is the application's permission store. A real app keeps it in a database table or a policy
// service and queries it through the container (see permissionsFrom); an in-memory map keeps this
// example self-contained.
type grants struct {
	mu    sync.RWMutex
	perms map[string][]string // subject -> permissions
}

func (g *grants) of(subject string) []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	return slices.Clone(g.perms[subject])
}

// revoke removes permission from subject. The next request subject makes is authorized without it,
// even though the token it carries has not expired.
func (g *grants) revoke(subject, permission string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.perms[subject] = slices.DeleteFunc(g.perms[subject], func(p string) bool { return p == permission })
}

//nolint:gochecknoglobals // The example's store; the test revokes from it.
var store = &grants{perms: map[string][]string{
	"alice": {"orders:read", "orders:write"},
	"bob":   {"orders:read"},
}}

func main() {
	app := gofr.New()

	// OAuth verifies the token and puts its claims in the request. The extractor trusts the token's
	// subject, so validate the issuer, audience and expiry here. An empty issuer makes
	// jwt.WithIssuer a no-op, so refuse to start without one.
	issuer := app.Config.Get("TOKEN_ISSUER")
	if issuer == "" {
		app.Logger().Fatalf("TOKEN_ISSUER is not set")
	}

	if err := app.EnableOAuth(app.Config.Get("JWKS_URL"), 10,
		jwt.WithIssuer(issuer),
		jwt.WithAudience("orders-api"),
		jwt.WithExpirationRequired(),
	); err != nil {
		app.Logger().Fatalf("%v", err)
	}

	// Every route below has a rule in configs/rbac.json, and configs/.env sets
	// GOFR_RBAC_ROUTE_CHECK=fail: a route added without a rule stops startup instead of being served
	// with no authorization.
	if err := app.EnableRBACWithExtractor(permissionsFrom(store)); err != nil {
		app.Logger().Fatalf("%v", err)
	}

	app.GET("/status", status)
	app.GET("/orders", listOrders)
	app.POST("/orders", createOrder)

	app.Run()
}

// permissionsFrom returns the extractor: it reads the verified token's subject and looks up what that
// subject may do right now. With a database, query c.SQL with r.Context(), which carries the
// extractor's deadline.
func permissionsFrom(g *grants) rbac.Extractor {
	return func(_ *container.Container, r *http.Request) ([]string, error) {
		claims, ok := r.Context().Value(middleware.JWTClaim).(jwt.MapClaims)
		if !ok {
			return nil, rbac.ErrRoleNotFound // 401
		}

		subject, err := claims.GetSubject()
		if err != nil || subject == "" {
			return nil, rbac.ErrRoleNotFound // 401
		}

		return g.of(subject), nil // nil or empty is a 403
	}
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
