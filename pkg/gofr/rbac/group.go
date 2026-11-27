package rbac

import (
	"net/http"

	"gofr.dev/pkg/gofr/internal/rbaclink"
	"gofr.dev/pkg/gofr/internal/routemark"
)

// group is the set of configs one app installed with EnableRBAC. Each config's middleware sees only
// its own rules, so a decision that depends on all of them (is a route covered by any config? which
// config refuses what none covers?) is made through the group.
type group struct {
	configs []*Config

	// rejecter is the first config, in install order, whose own policy is deny. Only it refuses a
	// request no config covers; nil when no config denies.
	rejecter *Config
}

//nolint:gochecknoinits // self-registration of the hook gofr calls is the intended pattern.
func init() {
	rbaclink.Link = func(configs []any) bool {
		linked := make([]*Config, 0, len(configs))

		for _, c := range configs {
			// gofr passes only the *Config values it installed; anything else cannot be linked and is skipped.
			if config, ok := c.(*Config); ok {
				linked = append(linked, config)
			}
		}

		return linkGroup(linked)
	}
}

// linkGroup makes configs a group, in the order given, and reports whether any of them denies. It
// runs while the app is set up, before any request is served, so the group is read without a lock.
func linkGroup(configs []*Config) (deny bool) {
	g := &group{configs: configs}

	for _, c := range configs {
		if c.deny {
			g.rejecter = c

			break
		}
	}

	for _, c := range configs {
		c.group = g
	}

	return g.rejecter != nil
}

// rejectsUnmatched reports whether this config must refuse r, a request none of its own rules
// matched. Under allow, which is the default, it is false and costs a nil check and a bool read.
//
// A config that is not linked stands for itself: it refuses when its own policy is deny. In a group,
// only the rejecter refuses, and only when no other config has a rule for r, because that config's
// middleware enforces it. Requests GoFr's own handlers answer are never refused: the catch-all's 404
// and 405 run no application handler, and a built-in route is recognized by how it was registered.
func (c *Config) rejectsUnmatched(r *http.Request) bool {
	g := c.group
	if g == nil && !c.deny || g != nil && g.rejecter != c {
		return false
	}

	if routemark.CatchAllAnswered(r) || routemark.IsBuiltIn(routemark.Matched(r)) {
		return false
	}

	return g == nil || !g.coveredByOther(c, r)
}

// coveredByOther reports whether a config other than self has a rule for r, public or not.
func (g *group) coveredByOther(self *Config, r *http.Request) bool {
	for _, c := range g.configs {
		if c == self {
			continue
		}

		if endpoint, isPublic := getEndpointForRequest(r, c); endpoint != nil || isPublic {
			return true
		}
	}

	return false
}
