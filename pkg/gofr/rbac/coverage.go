package rbac

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/gorilla/mux"
)

// ErrDeadRules is returned by [Config.CheckRoutes] when a rule that requires permissions matches no
// registered route. Such a rule protects nothing, and it is usually a typo that leaves the route it
// was written for unguarded.
var ErrDeadRules = errors.New("RBAC rules match no registered route")

// anyMethod stands for "every method", on a rule declared with "*" and on a route registered
// without a method matcher, such as a static-file prefix.
const anyMethod = "*"

// faviconPath is the built-in favicon route GoFr registers itself.
const faviconPath = "/favicon.ico"

// registeredRoute is one (method, path) pair the router serves, as the coverage check sees it.
type registeredRoute struct {
	// method is the upper-cased method, or anyMethod for a route with no method matcher.
	method string

	// template is the path template. A PathPrefix route carries a trailing catch-all segment, so
	// that it overlaps every path under the prefix.
	template string

	// label is how the route is reported: "METHOD /path".
	label string
}

// CheckRoutes compares the config against the routes registered on router, which must hold the
// complete route table.
//
// A registered route that no rule covers is served without role checks. That is often intended,
// so the routes are reported in one warning line and are not an error. GoFr's own /.well-known/*
// routes and /favicon.ico are left out of it.
//
// A rule that matches no registered route - usually a typo in its path, or a method the route does
// not register - is returned as an error wrapping [ErrDeadRules] that lists every such rule. A dead
// public rule cannot leave a route unguarded, so it is only warned about.
//
// The PathPrefix("/") catch-all is ignored: it would otherwise make every rule look live.
func (c *Config) CheckRoutes(router *mux.Router) error {
	if router == nil {
		return nil
	}

	dead, uncovered := c.routeCoverage(collectRoutes(router))

	var deadGuards, deadPublic []string

	for i := range dead {
		label := dead[i].method + " " + dead[i].pattern
		if dead[i].isPublic {
			deadPublic = append(deadPublic, label)
		} else {
			deadGuards = append(deadGuards, label)
		}
	}

	uncoveredLabels := make([]string, 0, len(uncovered))
	for i := range uncovered {
		uncoveredLabels = append(uncoveredLabels, uncovered[i].label)
	}

	sort.Strings(deadGuards)
	sort.Strings(deadPublic)
	sort.Strings(uncoveredLabels)

	if c.Logger != nil {
		if len(uncoveredLabels) > 0 {
			c.Logger.Warnf("RBAC: %d registered route(s) are not covered by any rule and are served without role checks: %s",
				len(uncoveredLabels), strings.Join(uncoveredLabels, ", "))
		}

		if len(deadPublic) > 0 {
			c.Logger.Warnf("RBAC: public rule(s) match no registered route: %s", strings.Join(deadPublic, ", "))
		}
	}

	if len(deadGuards) > 0 {
		return fmt.Errorf("%w: %s", ErrDeadRules, strings.Join(deadGuards, ", "))
	}

	return nil
}

// routeCoverage splits the config against routes into the rules that match none of them and the
// routes that no rule matches.
func (c *Config) routeCoverage(routes []registeredRoute) (dead []endpointRule, uncovered []registeredRoute) {
	for i := range c.rules {
		live := false

		for j := range routes {
			if ruleMayMatchRoute(&c.rules[i], &routes[j]) {
				live = true

				break
			}
		}

		if !live {
			dead = append(dead, c.rules[i])
		}
	}

	for j := range routes {
		if isBuiltInRoute(routes[j].template) {
			continue
		}

		covered := false

		for i := range c.rules {
			if ruleMayMatchRoute(&c.rules[i], &routes[j]) {
				covered = true

				break
			}
		}

		if !covered {
			uncovered = append(uncovered, routes[j])
		}
	}

	return dead, uncovered
}

// ruleMayMatchRoute reports whether some request could be served by route and governed by rule.
func ruleMayMatchRoute(rule *endpointRule, route *registeredRoute) bool {
	methodsMatch := rule.method == anyMethod || route.method == anyMethod || strings.EqualFold(rule.method, route.method)

	return methodsMatch && patternsMayOverlap(rule.pattern, route.template)
}

// isBuiltInRoute reports whether a template belongs to a route GoFr registers itself, which is left
// out of the uncovered-route report because the application did not write it.
func isBuiltInRoute(template string) bool {
	return strings.HasPrefix(template, "/.well-known/") || template == faviconPath
}

// collectRoutes lists every (method, path) pair router serves, skipping the PathPrefix("/")
// catch-all and routes with no path. Duplicates collapse to one entry.
func collectRoutes(router *mux.Router) []registeredRoute {
	var routes []registeredRoute

	seen := make(map[string]bool)

	_ = router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		tmpl, err := route.GetPathTemplate()
		if err != nil {
			return nil
		}

		// mux anchors a Path regexp with "$" and leaves a PathPrefix one open.
		pathRegexp, _ := route.GetPathRegexp()
		isPrefix := !strings.HasSuffix(pathRegexp, "$")

		if isPrefix && tmpl == "/" {
			return nil
		}

		template := tmpl
		if isPrefix {
			template = strings.TrimSuffix(tmpl, "/") + "/{rest:.*}"
		}

		methods, err := route.GetMethods()
		if err != nil || len(methods) == 0 {
			methods = []string{anyMethod}
		}

		for _, method := range methods {
			method = strings.ToUpper(method)

			label := method + " " + tmpl
			if seen[label] {
				continue
			}

			seen[label] = true

			routes = append(routes, registeredRoute{method: method, template: template, label: label})
		}

		return nil
	})

	return routes
}

// patternsMayOverlap reports whether a request path could match both the rule pattern and the
// route template. Both use mux syntax.
//
// It errs toward "yes", because a wrong "no" reports a live rule as dead, and that stops startup.
// Two variables always overlap, even when their constraints could never agree, and a constraint
// that does not compile overlaps anything. The cost is that such a rule can go unreported.
func patternsMayOverlap(rule, template string) bool {
	if rule == "" || template == "" {
		return false
	}

	a := splitPatternSegments(strings.Trim(rule, "/"))
	b := splitPatternSegments(strings.Trim(template, "/"))

	for i := 0; ; i++ {
		// A catch-all absorbs the rest of the other path, including nothing at all.
		if isCatchAllAt(a, i) || isCatchAllAt(b, i) {
			return true
		}

		if i == len(a) || i == len(b) {
			return len(a) == len(b)
		}

		if !segmentsMayOverlap(a[i], b[i]) {
			return false
		}
	}
}

// segmentsMayOverlap reports whether one path segment could match both x and y.
func segmentsMayOverlap(x, y string) bool {
	xVar, yVar := isVariableSegment(x), isVariableSegment(y)

	switch {
	case xVar && yVar:
		return true
	case xVar:
		return variableMayMatch(x, y)
	case yVar:
		return variableMayMatch(y, x)
	default:
		return x == y
	}
}

// variableMayMatch reports whether the "{name}" or "{name:regex}" segment could match literal.
func variableMayMatch(variable, literal string) bool {
	inner := variable[1 : len(variable)-1]

	idx := strings.Index(inner, ":")
	if idx < 0 {
		return true
	}

	re, err := regexp.Compile("^(?:" + inner[idx+1:] + ")$")
	if err != nil {
		return true
	}

	return re.MatchString(literal)
}

func isVariableSegment(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}

// isCatchAllAt reports whether segments has a catch-all variable at index i.
func isCatchAllAt(segments []string, i int) bool {
	return i < len(segments) && isVariableSegment(segments[i]) && isCatchAllVariable(segments[i])
}
