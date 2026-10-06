package rbac

import (
	"regexp"
	"regexp/syntax"
	"slices"
	"sort"
	"strings"
)

// anyMethod stands for "every method", on a rule declared with "*" and on a route registered
// without a method matcher, such as a static-file prefix.
const anyMethod = "*"

// freeVariablePattern is what mux matches a "{name}" variable against in a path.
const freeVariablePattern = "[^/]+"

// invalidPattern stands for the pattern of a segment mux would refuse. It does not compile, so the
// segment covers nothing.
const invalidPattern = "["

// ReportRouteMismatches compares the config against the complete route table and logs every
// mismatch it finds. routes are the routes the application registered and builtInRoutes the ones
// GoFr registered itself, which need no rule but keep a rule written for them live. Each route is
// "METHOD /template" in mux syntax, with "*" for a route registered without a method matcher; a
// prefix route is passed with a trailing catch-all segment, as "/static/{path:.*}". It reports
// whether any mismatch was logged as an error.
//
// It logs one line for each kind of mismatch, listing its entries as "METHOD path":
//
//   - a dead rule matches no route - usually a typo in its path, or a method the route does not
//     register - so the route it was written for is unguarded. A dead public rule cannot leave a
//     route unguarded, so it is only warned about.
//   - an uncovered route is matched by no rule, so it is served without role checks.
//   - a partly covered route is matched by rules for some of its requests but not all of them.
//
// Paths are compared as written, the way requests are matched: "/api/users/" and "/api/users" are
// different paths. The dead-rule check is lenient - a rule is live if some request could match it
// and a route - so that a live rule is never reported as dead. The coverage check is strict: a
// rule covers a route only when the check can show that it matches every request the route
// serves, and a route it cannot show that for is reported as partly covered.
func (c *Config) ReportRouteMismatches(routes, builtInRoutes []string) bool {
	rc := newRouteChecker()

	rules := make([]routeEntry, len(c.rules))
	for i := range c.rules {
		rules[i] = rc.newEntry(c.rules[i].method, c.rules[i].pattern)
	}

	var (
		live                 = make([]bool, len(rules))
		uncovered, partial   []string
		deadGuards, deadOpen []string
	)

	for _, route := range builtInRoutes {
		entry := rc.parseEntry(route)
		markLiveRules(rules, &entry, live)
	}

	for _, route := range routes {
		entry := rc.parseEntry(route)

		switch covering := classifyRoute(rules, &entry, live); {
		case covering == nil:
			uncovered = append(uncovered, entry.label)
		case len(covering) > 0:
			partial = append(partial, entry.label+" (by "+strings.Join(sortedUnique(covering), " and ")+")")
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

// routeChecker holds what one check shares across its comparisons: each distinct constraint is
// compiled once, however many rules and routes carry it.
type routeChecker struct {
	constraints map[string]compiledConstraint
}

// compiledConstraint is what one check knows about a constraint.
type compiledConstraint struct {
	// re is the constraint anchored to a whole segment, or nil when it does not compile.
	re *regexp.Regexp

	// spans is set when the constraint can match a "/". See admitsSlash.
	spans bool
}

func newRouteChecker() *routeChecker {
	return &routeChecker{constraints: make(map[string]compiledConstraint)}
}

// routeEntry is a rule or a route, split into segments once for every comparison it takes part in.
type routeEntry struct {
	// method is the upper-cased method, or anyMethod.
	method string

	// label is how the entry is reported: "METHOD path".
	label string

	// segments is the path split on "/", empty segments kept. Nil for an empty path, which matches
	// nothing.
	segments []pathSegment
}

// pathSegment is one "/"-separated part of a mux path template.
type pathSegment struct {
	// text is the segment as written.
	text string

	// variable is set for a segment holding any "{...}" variable.
	variable bool

	// catchAll is set for a variable that can span several segments: one whose pattern can match a
	// "/", as mux applies a variable's pattern to the whole path rather than to one segment.
	catchAll bool

	// constraint is what the segment matches. For a segment that is exactly one variable, it is the
	// variable's constraint as written - mux does not trim it - and empty for a free variable. For
	// any other segment holding variables, such as "{a}-{b}", it is the regexp mux builds for the
	// segment.
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
	entry := routeEntry{method: method, label: method + " " + path}

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

	if !strings.Contains(text, "{") {
		return seg
	}

	seg.variable = true
	seg.constraint = segmentPattern(text)

	if seg.constraint != "" {
		compiled := rc.compile(seg.constraint)
		seg.re, seg.catchAll = compiled.re, compiled.re != nil && compiled.spans
	}

	return seg
}

// segmentPattern returns what a segment holding variables matches, the way mux reads its template:
// the constraint as written for a segment that is exactly one "{name:constraint}", empty for one
// "{name}", and otherwise the segment's literal text and each variable's pattern - "[^/]+" for a
// free one - joined into one regexp. A segment mux would refuse, with unbalanced braces or an
// empty name or pattern, gets invalidPattern, which does not compile and so covers nothing.
func segmentPattern(text string) string {
	groups := braceGroups(text)
	if groups == nil {
		return invalidPattern
	}

	var (
		pattern strings.Builder
		end     int
	)

	for _, group := range groups {
		name, constraint, hasConstraint := strings.Cut(text[group[0]+1:group[1]], ":")
		if name == "" || hasConstraint && constraint == "" {
			return invalidPattern
		}

		if len(groups) == 1 && group[0] == 0 && group[1] == len(text)-1 {
			return constraint
		}

		if !hasConstraint {
			constraint = freeVariablePattern
		}

		pattern.WriteString(regexp.QuoteMeta(text[end:group[0]]) + "(?:" + constraint + ")")
		end = group[1] + 1
	}

	pattern.WriteString(regexp.QuoteMeta(text[end:]))

	return pattern.String()
}

// braceGroups returns the index of the opening and closing brace of each outermost "{...}" in text,
// or nil when its braces do not balance. Braces nest, as in "{id:[0-9]{3}}".
func braceGroups(text string) [][2]int {
	var (
		groups       [][2]int
		depth, start int
	)

	for i, r := range text {
		switch r {
		case '{':
			if depth == 0 {
				start = i
			}

			depth++
		case '}':
			depth--

			switch {
			case depth < 0:
				return nil
			case depth == 0:
				groups = append(groups, [2]int{start, i})
			}
		}
	}

	if depth != 0 {
		return nil
	}

	return groups
}

// admitsSlash reports whether the regexp pattern can match a string containing "/". It looks only
// at which characters the pattern names, so it can answer "yes" for a pattern that never actually
// produces one; that only makes the check stricter.
func admitsSlash(pattern string) bool {
	re, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return false
	}

	return regexpAdmits(re, '/')
}

// regexpAdmits reports whether re names r anywhere: as a literal, in a character class, or through
// a wildcard.
func regexpAdmits(re *syntax.Regexp, r rune) bool {
	if re.Op == syntax.OpAnyChar || re.Op == syntax.OpAnyCharNotNL {
		return true
	}

	if re.Op == syntax.OpLiteral {
		return slices.Contains(re.Rune, r)
	}

	if re.Op == syntax.OpCharClass {
		// Rune holds the class as inclusive [lo, hi] pairs.
		for i := 0; i+1 < len(re.Rune); i += 2 {
			if re.Rune[i] <= r && r <= re.Rune[i+1] {
				return true
			}
		}

		return false
	}

	for _, sub := range re.Sub {
		if regexpAdmits(sub, r) {
			return true
		}
	}

	return false
}

// compile returns constraint anchored to a whole segment, as mux applies it, and whether it can
// match a "/".
func (rc *routeChecker) compile(constraint string) compiledConstraint {
	if compiled, ok := rc.constraints[constraint]; ok {
		return compiled
	}

	var compiled compiledConstraint

	if re, err := regexp.Compile("^(?:" + constraint + ")$"); err == nil {
		compiled = compiledConstraint{re: re, spans: admitsSlash(constraint)}
	}

	rc.constraints[constraint] = compiled

	return compiled
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

		// A catch-all covers only as the rule's last segment: the rule's segments after it must match
		// too, and how much of the path the catch-all leaves them is not modeled.
		if a[i].catchAll {
			return i == len(a)-1 && a[i].coversTail(b[i:])
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
		// match an empty one. One that can match a "/" is a catch-all, turned away above.
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
