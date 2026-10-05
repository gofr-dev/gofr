package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeldFromClaim(t *testing.T) {
	roleSet := map[string]map[string]struct{}{
		"admin":     {"users:write": {}},
		"viewer":    {"users:read": {}},
		"Org Admin": {"org:manage": {}},
	}

	testCases := []struct {
		desc      string
		value     any
		split     bool
		required  []string
		wantErr   error
		allowed   bool
		granted   string
		heldCount int
		multi     bool
	}{
		{
			desc: "role array holds the permissions of every role", value: []any{"admin", "viewer"},
			required: []string{"users:read"}, allowed: true, granted: "viewer", heldCount: 2, multi: true,
		},
		{
			desc: "role array: names not in the config grant nothing", value: []any{"ghost", "viewer"},
			required: []string{"users:write"}, heldCount: 2, multi: true,
		},
		{
			desc: "single role string is not split", value: "Org Admin",
			required: []string{"org:manage"}, allowed: true, granted: "Org Admin", heldCount: 1,
		},
		{
			desc: "single role string with a space never matches its halves", value: "Org Admin",
			required: []string{"users:read"}, granted: "", heldCount: 1,
		},
		{
			desc: "scope string is split on spaces", value: "orders:read  orders:write", split: true,
			required: []string{"orders:write"}, allowed: true, granted: "orders:write", heldCount: 2, multi: true,
		},
		{
			desc: "permissions array entries are held as is", value: []any{"orders:read orders:write"}, split: true,
			required: []string{"orders:write"}, heldCount: 1, multi: true,
		},
		{
			desc: "empty and non-string array entries are ignored", value: []any{"", 42, true, "orders:read"}, split: true,
			required: []string{"orders:read"}, allowed: true, granted: "orders:read", heldCount: 1, multi: true,
		},
		{
			desc: "role array with only unusable entries holds nothing", value: []any{"", 7}, required: []string{"users:read"},
			heldCount: 0, multi: true,
		},
		{desc: "permission match is exact", value: "Orders:Read", split: true, required: []string{"orders:read"}, heldCount: 1, multi: true},
		{desc: "empty string is a missing role", value: "", wantErr: ErrRoleNotFound},
		{desc: "null claim is a missing role", value: nil, wantErr: ErrRoleNotFound},
		{desc: "number is unreadable", value: 42.0, wantErr: errUnreadableClaim},
		{desc: "boolean is unreadable", value: true, split: true, wantErr: errUnreadableClaim},
		{desc: "object is unreadable", value: map[string]any{"a": "b"}, wantErr: errUnreadableClaim},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			h, err := heldFromClaim(tc.value, tc.split)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr, "TEST[%d], Failed.\n%s", i, tc.desc)

				return
			}

			require.NoError(t, err, "TEST[%d], Failed.\n%s", i, tc.desc)

			granted, allowed := h.grant(tc.required, roleSet)
			assert.Equal(t, tc.allowed, allowed, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.granted, granted, "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.heldCount, h.count(), "TEST[%d], Failed.\n%s", i, tc.desc)
			assert.Equal(t, tc.multi, h.multi(), "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}

// TestHeld_GrantAllocs pins the per-request authorization check at zero allocations, so building a
// merged permission list per request cannot creep back in unnoticed.
func TestHeld_GrantAllocs(t *testing.T) {
	roles := make([]any, 0, 10)
	roleSet := make(map[string]map[string]struct{}, 10)

	for _, name := range []string{"r0", "r1", "r2", "r3", "r4", "r5", "r6", "r7", "r8", "r9"} {
		roles = append(roles, name)
		roleSet[name] = map[string]struct{}{name + ":read": {}}
	}

	scope := "s0:read s1:read s2:read s3:read s4:read s5:read s6:read s7:read s8:read s9:read"

	testCases := []struct {
		desc  string
		value any
		split bool
		need  []string
	}{
		{desc: "ten roles, last one grants", value: roles, need: []string{"r9:read"}},
		{desc: "ten-entry scope, last one grants", value: scope, split: true, need: []string{"s9:read"}},
		{desc: "ten-entry scope, none grants", value: scope, split: true, need: []string{"x:read"}},
	}

	for i, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			allocs := testing.AllocsPerRun(100, func() {
				h, _ := heldFromClaim(tc.value, tc.split)
				_, _ = h.grant(tc.need, roleSet)
			})

			assert.Zero(t, allocs, "TEST[%d], Failed.\n%s", i, tc.desc)
		})
	}
}
