package gofr

import "net/http"

// graphQLRunner is everything App needs from the GraphQL subsystem.
//
// The field on App is this interface rather than the concrete *graphQLManager so
// that the implementation -- and with it the graphql-go and gqlparser trees, 20
// packages and 1.4 MB of binary -- can be left out of a build that does not use
// GraphQL. App holds five method calls' worth of dependency on it; naming those
// five is what turns a hard import into an optional one.
//
// This is the same shape that makes the pub/sub backends optional: nothing in
// GoFr's public API names a library type. GraphQLQuery and GraphQLMutation take
// a GoFr Handler, so the exported surface is identical in both builds and a user
// who does not set the tag cannot tell the difference.
//
// newGraphQLRunner has two definitions selected by the same tag -- the real one
// in graphql_enabled.go, a stub in graphql_disabled.go. It is a plain function
// rather than a variable assigned in init() so the choice is the compiler's and
// there is no package-level state to reason about.
//
// Whether the engine is linked at all is a constant, graphQLLinked, not a method
// on this interface: it is a property of the build rather than of any instance,
// the compiler can fold the branch away, and it matches pubsubBackendsLinked,
// which the pub/sub tag already uses for exactly the same question.
type graphQLRunner interface {
	RegisterQuery(name string, handler Handler)
	RegisterMutation(name string, handler Handler)
	buildSchema() error
	GetHandler() http.Handler
}
