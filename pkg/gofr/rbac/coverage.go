package rbac

import (
	"regexp"
	"sort"
	"strings"
)

// anyMethod stands for "every method", on a rule declared with "*" and on a route registered
// without a method matcher, such as a static-file prefix.
const anyMethod = "*"

// faviconPath is the built-in favicon route GoFr registers itself.
const faviconPath = "/favicon.ico"

// ReportRouteMismatches compares the config against routes, the complete route table, and logs
// every mismatch it finds. Each route is "METHOD /template" in mux syntax, with "*" for a route
// registered without a method matcher; a prefix route is passed with a trailing catch-all segment,
// as "/static/{path:.*}". It reports whether any mismatch was logged as an error.
//
// It logs one line for each kind of mismatch, listing its entries as "METHOD path":
//
//   - a dead rule matches no route - usually a typo in its path, or a method the route does not
//     register - so the route it was written for is unguarded. A dead public rule cannot leave a
//     route unguarded, so it is only warned about.
//   - an uncovered route is matched by no rule, so it is served without role checks.
//   - a partly covered route is matched by rules for some of its requests but not all of them.
//
// GoFr's own /.well-known/* routes and /favicon.ico need no rule.
//
// Paths are compared as written, the way requests are matched: "/api/users/" and "/api/users" are
// different paths. The dead-rule check is lenient - a rule is live if some request could match it
// and a route - so that a live rule is never reported as dead. The coverage check is strict: a
// rule covers a route only when it matches every request the route serves.
func (c *Config) ReportRouteMismatches(routes []string) bool {
	rc := newRouteChecker()

	rules := make([]routeEntry, len(c.rules))
	for i := range c.rules {
		rules[i] = rc.newEntry(c.rules[i].method, c.rules[i].pattern)
	}

	parsed := make([]routeEntry, len(routes))
	for j, route := range routes {
		parsed[j] = rc.parseEntry(route)
	}

	var (
		live                 = make([]bool, len(rules))
		uncovered, partial   []string
		deadGuards, deadOpen []string
	)

	for j := range parsed {
		if isBuiltInRoute(parsed[j].rawPath) {
			markLiveRules(rules, &parsed[j], live)

			continue
		}

		switch covering := classifyRoute(rules, &parsed[j], live); {
		case covering == nil:
			uncovered = append(uncovered, parsed[j].label)
		case len(covering) > 0:
			partial = append(partial, parsed[j].label+" (by "+strings.Join(sortedUnique(covering), " and ")+")")
		}
	}

	for i := range rules {
		switch {
		case live[i]:
		case c.rules[i].isPublic:
			deadOpen = append(deadOpen, rules[i].label)
		default:
			deadGuards = append(deadGuards, rules[i].label)
		}
	}

	c.logRouteMismatches(sortedUnique(deadGuards), sortedUnique(deadOpen), sortedUnique(uncovered), sortedUnique(partial))

	return len(deadGuards)+len(uncovered)+len(partial) > 0
}

// logRouteMismatches writes one line for each non-empty kind of mismatch.
func (c *Config) logRouteMismatches(deadGuards, deadOpen, uncovered, partial []string) {
	if c.Logger == nil {
		return
	}

	if len(deadOpen) > 0 {
		c.Logger.Warnf("RBAC: public rules match no registered route: %s", strings.Join(deadOpen, ", "))
	}

	if len(deadGuards) > 0 {
		c.Logger.Errorf("RBAC route check failed: rules match no registered route: %s. These rules protect "+
			"nothing; fix each rule's path or methods, or remove it.", strings.Join(deadGuards, ", "))
	}

	if len(uncovered) > 0 {
		c.Logger.Errorf("RBAC route check failed: routes covered by no rule: %s. They are served without role "+
			"checks; add a rule for each, with \"public\": true for a route meant to be open.", strings.Join(uncovered, ", "))
	}

	if len(partial) > 0 {
		c.Logger.Errorf("RBAC route check failed: routes only partly covered: %s. Requests these rules do not "+
			"match are served without role checks; add a rule that matches each route in full.", strings.Join(partial, ", "))
	}
}

// classifyRoute marks every rule that overlaps route as live, and reports how route is covered: an
// empty non-nil slice when some rule covers it, the labels of the rules that overlap it when none
// does, and nil when no rule overlaps it.
func classifyRoute(rules []routeEntry, route *routeEntry, live []bool) []string {
	covered, overlapping := false, []string{}

	for i := range rules {
		if !rules[i].overlaps(route) {
			continue
		}

		live[i] = true

		switch {
		case covered:
		case rules[i].covers(route):
			covered = true
		default:
			overlapping = append(overlapping, rules[i].label)
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
func markLiveRules(rules []routeEntry, route *routeEntry, live []bool) {
	for i := range rules {
		if !live[i] && rules[i].overlaps(route) {
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

// isBuiltInRoute reports whether a template belongs to a route GoFr registers itself, which is left
// out of the coverage report because the application did not write it.
func isBuiltInRoute(template string) bool {
	return strings.HasPrefix(template, "/.well-known/") || template == faviconPath
}

// routeChecker holds what one check shares across its comparisons: each distinct constraint is
// compiled once, however many rules and routes carry it.
type routeChecker struct {
	constraints map[string]*regexp.Regexp
}

func newRouteChecker() *routeChecker {
	return &routeChecker{constraints: make(map[string]*regexp.Regexp)}
}

// routeEntry is a rule or a route, split into segments once for every comparison it takes part in.
type routeEntry struct {
	// method is the upper-cased method, or anyMethod.
	method string

	// rawPath is the path as written.
	rawPath string

	// label is how the entry is reported: "METHOD path".
	label string

	// segments is rawPath split on "/", empty segments kept. Nil for an empty path, which matches
	// nothing.
	segments []pathSegment
}

// pathSegment is one "/"-separated part of a mux path template.
type pathSegment struct {
	// text is the segment as written.
	text string

	// variable is set for a "{name}" or "{name:constraint}" segment.
	variable bool

	// catchAll is set for a variable that can span several segments. See isCatchAllVariable.
	catchAll bool

	// constraint is the variable's constraint as written, trimmed; empty for a free variable.
	constraint string

	// re is constraint compiled and anchored, or nil for a free variable or one that does not
	// compile.
	re *regexp.Regexp
}

// parseEntry parses "METHOD path". A string with no space is a path with anyMethod.
func (rc *routeChecker) parseEntry(s string) routeEntry {
	method, path, found := strings.Cut(s, " ")
	if !found {
		method, path = anyMethod, s
	}

	return rc.newEntry(strings.ToUpper(method), path)
}

func (rc *routeChecker) newEntry(method, path string) routeEntry {
	entry := routeEntry{method: method, rawPath: path, label: method + " " + path}

	if path == "" {
		return entry
	}

	parts := splitPatternSegments(path)
	entry.segments = make([]pathSegment, len(parts))

	for i, part := range parts {
		entry.segments[i] = rc.newSegment(part)
	}

	return entry
}

func (rc *routeChecker) newSegment(text string) pathSegment {
	seg := pathSegment{text: text}

	if !isVariableSegment(text) {
		return seg
	}

	seg.variable = true
	seg.catchAll = isCatchAllVariable(text)

	inner := text[1 : len(text)-1]

	idx := strings.Index(inner, ":")
	if idx < 0 {
		return seg
	}

	seg.constraint = strings.TrimSpace(inner[idx+1:])
	seg.re = rc.compile(seg.constraint)

	return seg
}

// compile returns constraint anchored to a whole segment, as mux applies it, or nil when it does
// not compile.
func (rc *routeChecker) compile(constraint string) *regexp.Regexp {
	if re, ok := rc.constraints[constraint]; ok {
		return re
	}

	re, err := regexp.Compile("^(?:" + constraint + ")$")
	if err != nil {
		re = nil
	}

	rc.constraints[constraint] = re

	return re
}

func isVariableSegment(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}

// overlaps reports whether some request could be served by route and governed by the rule e.
//
// It errs toward "yes", because a wrong "no" reports a live rule as dead, telling the operator its
// route is unguarded when it is not. Two variables always overlap, even when their constraints
// could never agree, and a constraint that does not compile overlaps anything.
func (e *routeEntry) overlaps(route *routeEntry) bool {
	methodsOverlap := e.method == anyMethod || route.method == anyMethod || e.method == route.method

	return methodsOverlap && segmentsOverlap(e.segments, route.segments)
}

// segmentsOverlap reports whether some request path could match both a and b.
func segmentsOverlap(a, b []pathSegment) bool {
	if a == nil || b == nil {
		return false
	}

	for i := 0; ; i++ {
		if i == len(a) || i == len(b) {
			return len(a) == len(b)
		}

		// A catch-all absorbs the rest of the other path, which may be empty but must be there: the
		// "/" before a catch-all is part of the template.
		if a[i].catchAll || b[i].catchAll {
			return true
		}

		if !a[i].mayMatch(&b[i]) {
			return false
		}
	}
}

// mayMatch reports whether one path segment could match both s and other.
func (s *pathSegment) mayMatch(other *pathSegment) bool {
	switch {
	case s.variable && other.variable:
		return true
	case s.variable:
		return s.matchesLiteral(other.text, true)
	case other.variable:
		return other.matchesLiteral(s.text, true)
	default:
		return s.text == other.text
	}
}

// matchesLiteral reports whether the variable s matches literal. A free variable matches any
// non-empty literal; a constraint that does not compile matches per ifUnknown.
func (s *pathSegment) matchesLiteral(literal string, ifUnknown bool) bool {
	switch {
	case s.constraint == "":
		return literal != "" && !strings.Contains(literal, "/")
	case s.re == nil:
		return ifUnknown
	default:
		return s.re.MatchString(literal)
	}
}

// covers reports whether the rule e governs every request route serves.
func (e *routeEntry) covers(route *routeEntry) bool {
	methodCovers := e.method == anyMethod || e.method == route.method

	return methodCovers && segmentsCover(e.segments, route.segments)
}

// segmentsCover reports whether the rule path a matches every request path the route path b does.
func segmentsCover(a, b []pathSegment) bool {
	if a == nil || b == nil {
		return false
	}

	for i := 0; ; i++ {
		if i == len(a) || i == len(b) {
			return len(a) == len(b)
		}

		if a[i].catchAll {
			return a[i].coversTail(b[i:])
		}

		if !a[i].covers(&b[i]) {
			return false
		}
	}
}

// covers reports whether the rule segment s matches everything the route segment other does.
func (s *pathSegment) covers(other *pathSegment) bool {
	switch {
	case other.catchAll:
		return false
	case !other.variable:
		return s.text == other.text || s.variable && s.matchesLiteral(other.text, false)
	case !s.variable:
		return false
	case s.constraint == "":
		// A free variable matches any non-empty segment, so it covers a constraint that cannot
		// match an empty one.
		return other.constraint == "" || other.re != nil && !other.re.MatchString("")
	default:
		return s.re != nil && s.constraint == other.constraint
	}
}

// coversTail reports whether the rule catch-all s matches every remainder the route segments tail
// can produce, joined with "/". tail is never empty.
func (s *pathSegment) coversTail(tail []pathSegment) bool {
	mayBeEmpty := len(tail) == 1 && tail[0].mayBeEmpty()

	switch {
	case s.constraint == ".*":
		return true
	case s.constraint == ".+":
		return !mayBeEmpty
	case s.re == nil:
		return false
	}

	if len(tail) == 1 && tail[0].catchAll {
		return s.constraint == tail[0].constraint
	}

	literals := make([]string, len(tail))

	for i := range tail {
		if tail[i].variable {
			return false
		}

		literals[i] = tail[i].text
	}

	return s.re.MatchString(strings.Join(literals, "/"))
}

// mayBeEmpty reports whether the route segment s can match an empty segment.
func (s *pathSegment) mayBeEmpty() bool {
	switch {
	case !s.variable:
		return s.text == ""
	case s.constraint == "":
		return false
	case s.re == nil:
		return true
	default:
		return s.re.MatchString("")
	}
}
