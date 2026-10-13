// Package rbaccheck compares an RBAC config against the route table an app serves, so that a rule
// guarding nothing, or a route no rule guards, is found at startup rather than by a request served
// without role checks. GoFr runs it from App.Run; it is internal because an app has no route table
// of its own to hand it.
package rbaccheck

import (
	"sort"
	"strings"

	"gofr.dev/pkg/gofr/rbac"
)

// Route is one route the router serves.
type Route struct {
	// Method is the route's method, or "*" for a route registered without a method matcher, such as
	// a static-file prefix.
	Method string

	// Path is the route's template in mux syntax. A prefix route is given a trailing catch-all
	// segment, as "/static/{path:.*}", so that it stands for every path under the prefix.
	Path string

	// BuiltIn is set for a route GoFr registered itself. It needs no rule, but keeps a rule written
	// for it live.
	BuiltIn bool
}

// Mismatches lists what Check found, each entry as "METHOD path", sorted.
type Mismatches struct {
	// DeadRules match no route - usually a typo in the path, or a method the route does not
	// register - so the route each was written for is unguarded.
	DeadRules []string

	// DeadPublicRules are public rules that match no route. They cannot leave a route unguarded.
	DeadPublicRules []string

	// Uncovered routes are matched by no rule, so they are served without role checks.
	Uncovered []string

	// PartlyCovered routes are matched by rules for some of their requests but not all of them.
	// Each entry names the rules, as "METHOD path (by METHOD rule and METHOD rule)".
	PartlyCovered []string
}

// Failed reports whether any mismatch can leave a request served without role checks: anything but
// a dead public rule.
func (m *Mismatches) Failed() bool {
	return len(m.DeadRules)+len(m.Uncovered)+len(m.PartlyCovered) > 0
}

// rule is one (method, path) an endpoint declares.
type rule struct {
	entry  routeEntry
	public bool
}

// Check compares the RBAC endpoints against the complete route table.
//
// Paths are compared as written, the way requests are matched: "/api/users/" and "/api/users" are
// different paths. The dead-rule check is lenient - a rule is live if some request could match it
// and a route - so that a live rule is never reported as dead. The coverage check is strict: a
// rule covers a route only when the check can show that it matches every request the route
// serves, and a route it cannot show that for is reported as partly covered.
func Check(endpoints []rbac.EndpointMapping, routes []Route) Mismatches {
	rc := newRouteChecker()
	rules := buildRules(rc, endpoints)

	var (
		m    Mismatches
		live = make([]bool, len(rules))
	)

	for i := range routes {
		entry := rc.newEntry(strings.ToUpper(routes[i].Method), routes[i].Path)

		if routes[i].BuiltIn {
			markLiveRules(rules, &entry, live)

			continue
		}

		switch covering := classifyRoute(rules, &entry, live); {
		case covering == nil:
			m.Uncovered = append(m.Uncovered, entry.label)
		case len(covering) > 0:
			m.PartlyCovered = append(m.PartlyCovered, entry.label+" (by "+strings.Join(sortedUnique(covering), " and ")+")")
		}
	}

	for i := range rules {
		switch {
		case live[i]:
		case rules[i].public:
			m.DeadPublicRules = append(m.DeadPublicRules, rules[i].entry.label)
		default:
			m.DeadRules = append(m.DeadRules, rules[i].entry.label)
		}
	}

	m.DeadRules = sortedUnique(m.DeadRules)
	m.DeadPublicRules = sortedUnique(m.DeadPublicRules)
	m.Uncovered = sortedUnique(m.Uncovered)
	m.PartlyCovered = sortedUnique(m.PartlyCovered)

	return m
}

// buildRules reads endpoints the way the rbac matcher does: an endpoint with no methods stands for
// every method, a method is read in upper case, and a (method, path) declared again replaces the
// earlier declaration. TestCheck_AgreesWithMiddleware fails if the two drift apart.
func buildRules(rc *routeChecker, endpoints []rbac.EndpointMapping) []rule {
	var (
		rules []rule
		index = make(map[string]int)
	)

	for i := range endpoints {
		methods := endpoints[i].Methods
		if len(methods) == 0 {
			methods = []string{anyMethod}
		}

		for _, method := range methods {
			r := rule{entry: rc.newEntry(strings.ToUpper(method), endpoints[i].Path), public: endpoints[i].Public}

			if at, seen := index[r.entry.label]; seen {
				rules[at] = r

				continue
			}

			index[r.entry.label] = len(rules)
			rules = append(rules, r)
		}
	}

	return rules
}

// classifyRoute marks every rule that overlaps route as live, and reports how route is covered: an
// empty non-nil slice when some rule covers it, the labels of the rules that overlap it when none
// does, and nil when no rule overlaps it.
func classifyRoute(rules []rule, route *routeEntry, live []bool) []string {
	covered, overlapping := false, []string{}

	for i := range rules {
		if !rules[i].entry.overlaps(route) {
			continue
		}

		live[i] = true

		switch {
		case covered:
		case rules[i].entry.covers(route):
			covered = true
		default:
			overlapping = append(overlapping, rules[i].entry.label)
		}
	}

	switch {
	case covered:
		return []string{}
	case len(overlapping) == 0:
		return nil
	default:
		return overlapping
	}
}

// markLiveRules marks every rule that overlaps route as live.
func markLiveRules(rules []rule, route *routeEntry, live []bool) {
	for i := range rules {
		if !live[i] && rules[i].entry.overlaps(route) {
			live[i] = true
		}
	}
}

// sortedUnique sorts entries and drops repeats, in place.
func sortedUnique(entries []string) []string {
	sort.Strings(entries)

	out := entries[:0]

	for i, entry := range entries {
		if i == 0 || entry != entries[i-1] {
			out = append(out, entry)
		}
	}

	return out
}
