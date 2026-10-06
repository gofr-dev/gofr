package rbac

import (
	"fmt"
	"slices"
	"strings"
)

// errUnreadableClaim is returned when the role or permissions claim has a type RBAC cannot read: an
// object, or in permissions mode a number or boolean. It is a denial (403), not a missing credential
// (401): the token was verified and carries the claim, RBAC just cannot read it.
var errUnreadableClaim = fmt.Errorf("%w: claim is not a string or an array of strings", ErrAccessDenied)

// held is what a request holds for authorization: role names, or - in permissions mode - the
// permissions themselves. It reads the claim value in place rather than merging it into a new list,
// so checking a request allocates nothing.
type held struct {
	list  []any  // an array claim; nil when the claim was a single string
	str   string // a string claim, or the header value
	perms bool   // entries are permissions (permissions mode), not role names
}

// heldFromClaim interprets a claim value. split selects permissions mode, where a string is a
// space-separated list; in roles mode a string is one role name and is never split.
//
// A claim that holds nothing - "" or null in permissions mode, null in roles mode - is not an error:
// the token is valid and simply grants nothing, so the request is denied with a 403 (RFC 6750 §3.1).
// An empty role string stays a missing role (401), as it always was.
func heldFromClaim(value any, split bool) (held, error) {
	switch v := value.(type) {
	case string:
		if v == "" && !split {
			return held{}, ErrRoleNotFound
		}

		return held{str: v, perms: split}, nil
	case []any:
		return held{list: v, perms: split}, nil
	case nil:
		return held{perms: split}, nil
	case map[string]any:
		return held{}, errUnreadableClaim
	default:
		if split {
			return held{}, errUnreadableClaim
		}

		// A number or boolean role has always been read as its text (123 is role "123").
		return held{str: fmt.Sprint(v)}, nil
	}
}

// multi reports whether the request may hold more than one role or permission. Such requests get the
// privacy rules: a denial logs a count, never names.
func (h held) multi() bool {
	return h.perms || h.list != nil
}

// grant returns the held role or permission that grants any of required, and whether one does.
func (h held) grant(required []string, roles map[string]map[string]struct{}) (string, bool) {
	if h.list == nil {
		return h.grantFromString(required, roles)
	}

	return h.grantFromList(required, roles)
}

// grantFromString checks a string claim: one role name, or a space-separated list of permissions.
func (h held) grantFromString(required []string, roles map[string]map[string]struct{}) (string, bool) {
	if !h.perms {
		if h.str != "" && roleGrants(roles[h.str], required) {
			return h.str, true
		}

		return "", false
	}

	for p := range strings.FieldsSeq(h.str) {
		if slices.Contains(required, p) {
			return p, true
		}
	}

	return "", false
}

// grantFromList checks an array claim entry by entry, skipping empty and non-string entries.
func (h held) grantFromList(required []string, roles map[string]map[string]struct{}) (string, bool) {
	for _, entry := range h.list {
		name, ok := entry.(string)
		if !ok || name == "" {
			continue
		}

		if h.perms {
			if slices.Contains(required, name) {
				return name, true
			}

			continue
		}

		if roleGrants(roles[name], required) {
			return name, true
		}
	}

	return "", false
}

// count returns how many usable roles or permissions are held.
func (h held) count() int {
	if h.list == nil {
		if !h.perms {
			if h.str == "" {
				return 0
			}

			return 1
		}

		n := 0
		for range strings.FieldsSeq(h.str) {
			n++
		}

		return n
	}

	n := 0

	for _, entry := range h.list {
		if name, ok := entry.(string); ok && name != "" {
			n++
		}
	}

	return n
}

// roleGrants reports whether a role's permission set holds any of required. Exact match only.
func roleGrants(permissions map[string]struct{}, required []string) bool {
	for _, p := range required {
		if _, ok := permissions[p]; ok {
			return true
		}
	}

	return false
}
