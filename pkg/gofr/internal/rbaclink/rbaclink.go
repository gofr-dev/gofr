// Package rbaclink is how gofr hands the RBAC configs an app installed to package rbac, so that
// each config's middleware can see the others, without rbac exporting an API for it. It is
// internal, and it imports nothing from either side: rbac cannot be imported here, and gofr cannot
// call an unexported rbac function, so a variable both can reach carries the call.
package rbaclink

// Link is set by package rbac when it initializes. gofr calls it once, with the *rbac.Config of each
// EnableRBAC call in order, after every route is registered and before the first request is served.
// The values are typed any because this package cannot name rbac.Config. It reports whether any of
// the configs sets defaultPolicy deny.
//
//nolint:gochecknoglobals // a hook variable is the only way to reach an unexported rbac function from gofr.
var Link func(configs []any) (deny bool)
