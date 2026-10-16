package rbac

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// errUnreadableClaim is returned when the role or permissions claim has a type RBAC cannot read: an
// object, or in permissions mode a number or boolean. It is a denial (403), not a missing credential
// (401): the token was verified and carries the claim, RBAC just cannot read it.
var errUnreadableClaim = errors.New("claim is not a string or an array of strings")

// held is what a request holds for authorization: role names, or - in permissions mode - the
// permissions themselves. It reads the claim value in place rather than merging it into a new list,
// so checking a request allocates nothing.
type held struct {
	list            []any  // an array claim; nil when the claim was a single string
	str             string // a string claim, or the header value
	permissionsMode bool   // entries are permissions (permissions mode), not role names
}

// heldFromClaim interprets a claim value. permissionsMode selects permissions mode, where a string is a
// space-separated list; in roles mode a string is one role name and is never split.
//
// A claim that holds nothing - "" or null in permissions mode, null in roles mode - is not an error:
// the token is valid and simply grants nothing, so the request is denied with a 403 (the status codes follow
// RFC 6750 §3.1; no WWW-Authenticate header is set).
// An empty role string stays a missing role (401), as it always was.
func heldFromClaim(value any, permissionsMode bool) (held, error) {
	switch v := value.(type) {
	case string:
		if v == "" && !permissionsMode {
			return held{}, ErrRoleNotFound
		}

		return held{str: v, permissionsMode: permissionsMode}, nil
	case []any:
		return held{list: v, permissionsMode: permissionsMode}, nil
	case nil:
		return held{permissionsMode: permissionsMode}, nil
	case map[string]any:
		return held{}, errUnreadableClaim
	default:
		if permissionsMode {
			return held{}, errUnreadableClaim
		}

		// A number or boolean role has always been read as its text (123 is role "123").
		return held{str: fmt.Sprint(v)}, nil
	}
}

// multi reports whether the request may hold more than one role or permission. Such requests get the
// privacy rules: a denial logs a count, never names.
func (h held) multi() bool {
	return h.permissionsMode || h.list != nil
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
	if !h.permissionsMode {
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

// grantFromList checks an array claim entry by entry, skipping empty and unusable entries.
func (h held) grantFromList(required []string, roles map[string]map[string]struct{}) (string, bool) {
	for _, entry := range h.list {
		name := entryName(entry, h.permissionsMode)
		if name == "" {
			continue
		}

		if h.permissionsMode {
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

// entryName returns the text of an array entry, or "" when the entry is unusable. A string is used as
// is. In roles mode a number or boolean is read as its text, the same as a scalar role claim (123 is
// role "123"); in permissions mode, where a scalar number is unreadable, such an entry is skipped.
// Objects and nulls are always skipped.
func entryName(entry any, permissionsMode bool) string {
	switch v := entry.(type) {
	case string:
		return v
	case float64, bool, int, int64, json.Number:
		if !permissionsMode {
			return fmt.Sprint(v)
		}
	}

	return ""
}

// count returns how many usable roles or permissions are held.
func (h held) count() int {
	if h.list == nil {
		if !h.permissionsMode {
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
		if entryName(entry, h.permissionsMode) != "" {
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
