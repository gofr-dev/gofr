package rbaccheck

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// parseEntry parses "METHOD path" the way Check builds an entry from a route.
func parseEntry(rc *routeChecker, s string) routeEntry {
	method, path, _ := strings.Cut(s, " ")

	return rc.newEntry(method, path)
}

func Test_routeEntry_overlaps(t *testing.T) {
	testCases := []struct {
		desc  string
		rule  string
		route string
		want  bool
	}{
		{"identical literals", "GET /api/users", "GET /api/users", true},
		{"different literals", "GET /api/users", "GET /api/posts", false},
		{"typo in a literal segment", "GET /api/user/{id}", "GET /api/users/{id}", false},
		{"free variable against literal", "GET /api/users/{id}", "GET /api/users/me", true},
		{"literal against free variable", "GET /api/users/me", "GET /api/users/{id}", true},
		{"free variable against an empty segment", "GET /api/users/{id}", "GET /api/users/", false},
		{"constraint admits the literal", "GET /api/users/{id:[0-9]+}", "GET /api/users/42", true},
		{"constraint rejects the literal", "GET /api/users/{id:[0-9]+}", "GET /api/users/me", false},
		{"route constraint rejects the literal", "GET /api/users/me", "GET /api/users/{id:[0-9]+}", false},
		{"variable against variable", "GET /api/users/{id:[0-9]+}", "GET /api/users/{id:[a-z]+}", true},
		{"more segments in the rule", "GET /api/users/{id}/posts", "GET /api/users/{id}", false},
		{"fewer segments in the rule", "GET /api", "GET /api/users", false},
		{"catch-all in the rule absorbs the rest", "GET /api/{path:.*}", "GET /api/users/{id}/posts", true},
		{"catch-all in the rule needs its slash", "GET /api/{path:.*}", "GET /api", false},
		{"catch-all in the rule matches an empty tail", "GET /api/{path:.*}", "GET /api/", true},
		{"catch-all in the route absorbs the rest", "GET /static/css/site.css", "GET /static/{path:.*}", true},
		{"catch-all after a different literal", "GET /admin/{path:.*}", "GET /api/users", false},
		{"trailing slash is a different path", "GET /api/users/", "GET /api/users", false},
		{"relative path is a different path", "GET api/users", "GET /api/users", false},
		{"root against root", "GET /", "GET /", true},
		{"root against a path", "GET /", "GET /api", false},
		{"constraint containing a slash spans segments", "GET /files/{path:[a-z/]+}", "GET /files/a/b/c", true},
		{"uncompilable constraint is assumed to overlap", "GET /api/{id:[}", "GET /api/users", true},
		{"space in a constraint is part of it", "GET /orders/{id: [0-9]+}", "GET /orders/42", false},
		{"two variables in one segment admit the literal", "GET /items/{a}-{b}", "GET /items/x-y", true},
		{"two variables in one segment reject the literal", "GET /items/{a}-{b}", "GET /items/abc", false},
		{"route constraint matching a slash spans segments", "GET /files/a/b", "GET /files/{p:[^.]+}", true},
		{"empty rule matches nothing", "GET ", "GET /api", false},
		{"different methods", "GET /api/users", "POST /api/users", false},
		{"wildcard rule method", "* /api/users", "POST /api/users", true},
		{"wildcard route method", "GET /static/{path:.*}", "* /static/{path:.*}", true},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			rc := newRouteChecker()
			rule, route := parseEntry(rc, tc.rule), parseEntry(rc, tc.route)

			assert.Equal(t, tc.want, rule.overlaps(&route), "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

func Test_routeEntry_covers(t *testing.T) {
	testCases := []struct {
		desc  string
		rule  string
		route string
		want  bool
	}{
		{"identical literals", "GET /api/users", "GET /api/users", true},
		{"trailing slash in the rule", "GET /api/users/", "GET /api/users", false},
		{"relative rule path", "GET files", "GET /files", false},
		{"catch-all does not cover the path it hangs off", "GET /admin/{rest:.*}", "GET /admin", false},
		{"catch-all covers a path under it", "GET /admin/{rest:.*}", "GET /admin/users/{id}", true},
		{"catch-all covers an empty tail", "GET /api/{path:.*}", "GET /api/", true},
		{"non-empty catch-all does not cover an empty tail", "GET /api/{path:.+}", "GET /api/", false},
		{"catch-all covers a route catch-all", "GET /static/{p:.*}", "GET /static/{path:.*}", true},
		{"non-empty catch-all does not cover one that may be empty", "GET /static/{p:.+}", "GET /static/{path:.*}", false},
		{"free variable does not cover a route catch-all", "GET /static/{name}", "GET /static/{path:.*}", false},
		{"free variable covers a literal", "GET /api/users/{id}", "GET /api/users/me", true},
		{"free variable does not cover an empty segment", "GET /api/users/{id}", "GET /api/users/", false},
		{"free variable covers a free variable", "GET /api/users/{id}", "GET /api/users/{uid}", true},
		{"free variable covers a non-empty constraint", "GET /api/users/{id}", "GET /api/users/{id:[0-9]+}", true},
		{"free variable does not cover a constraint that admits empty", "GET /api/{id}", "GET /api/{id:[0-9]*}", false},
		{"constraint does not cover a free variable", "GET /api/users/{id:[0-9]+}", "GET /api/users/{id}", false},
		{"identical constraints", "GET /api/users/{id:[0-9]+}", "GET /api/users/{uid:[0-9]+}", true},
		{"different constraints", "GET /api/users/{id:[0-9]+}", "GET /api/users/{id:[a-z]+}", false},
		{"constraint admits the literal", "GET /api/users/{id:[0-9]+}", "GET /api/users/42", true},
		{"literal does not cover a variable", "GET /api/users/me", "GET /api/users/{id}", false},
		{"uncompilable constraint covers nothing", "GET /api/{id:[}", "GET /api/users", false},
		{"slash constraint covers a matching literal tail", "GET /files/{path:[a-z/]+}", "GET /files/a/b", true},
		{"slash constraint does not cover a variable", "GET /files/{path:[a-z/]+}", "GET /files/{name}", false},
		{"catch-all covers only as the rule's last segment", "GET /api/{rest:.*}/admin", "GET /api/users", false},
		{"two variables in one segment are not one free variable", "GET /items/{a}-{b}", "GET /items/{id}", false},
		{"two variables in one segment cover the same segment", "GET /items/{a}-{b}", "GET /items/{x}-{y}", true},
		{"two variables in one segment cover a literal they match", "GET /items/{a}-{b}", "GET /items/x-y", true},
		{"free variable covers two variables in one segment", "GET /items/{id}", "GET /items/{a}-{b}", true},
		{"free variable does not cover a constraint matching a slash", "GET /files/{x}", "GET /files/{p:[^.]+}", false},
		{"constraint matching a slash covers a literal tail", "GET /files/{p:[^.]+}", "GET /files/a/b", true},
		{"space in a constraint is part of it", "GET /orders/{id: [0-9]+}", "GET /orders/{id:[0-9]+}", false},
		{"same method", "GET /api/users", "GET /api/users", true},
		{"different method", "GET /api/users", "POST /api/users", false},
		{"wildcard rule covers any method", "* /api/users", "DELETE /api/users", true},
		{"wildcard rule covers a wildcard route", "* /static/{path:.*}", "* /static/{path:.*}", true},
		{"one method does not cover a wildcard route", "GET /static/{path:.*}", "* /static/{path:.*}", false},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			rc := newRouteChecker()
			rule, route := parseEntry(rc, tc.rule), parseEntry(rc, tc.route)

			assert.Equal(t, tc.want, rule.covers(&route), "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
