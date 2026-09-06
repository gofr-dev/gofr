package container

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
)

// typedNilPubSubContainer builds the state both tests below need: a container
// whose PubSub holds a TYPED nil.
//
// google.New returns (*googleClient)(nil) when it rejects an incomplete config --
// here a GOOGLE backend with no project id -- and createGooglePubSub assigns that
// straight into the interface. A typed nil is not equal to nil, so every plain
// "!= nil" guard downstream passes and then calls a method on a nil receiver.
func typedNilPubSubContainer(t *testing.T) *Container {
	t.Helper()

	return NewContainer(config.NewMockConfig(map[string]string{
		"PUBSUB_BACKEND": "GOOGLE",
		"LOG_LEVEL":      "FATAL",
	}))
}

// TestHealthSurvivesTypedNilPubSub pins the guard in Health, which called
// Health() on the nil receiver and dereferenced a Config embedded by value. Any
// request to the health endpoint took the process down.
func TestHealthSurvivesTypedNilPubSub(t *testing.T) {
	c := typedNilPubSubContainer(t)

	require.NotPanics(t, func() { _ = c.Health(context.Background()) },
		"Health must see through a typed nil PubSub")
}

// TestGetSubscriberFiltersTypedNil pins the more damaging one.
//
// App.Subscribe guards with `GetSubscriber() == nil` and then hands the client to
// a goroutine in an errgroup that has no recover, so the nil-receiver panic there
// killed the process at startup rather than on a request. Filtering in
// GetSubscriber fixes every caller at once, so this is the assertion that keeps
// them all honest.
func TestGetSubscriberFiltersTypedNil(t *testing.T) {
	c := typedNilPubSubContainer(t)

	// assert.Nil is deliberately NOT used: it reflects, so it reports a typed nil
	// as nil and would pass whether or not the filter exists. Callers write
	// `GetSubscriber() == nil` -- App.Subscribe does exactly that -- so the test
	// has to make the same plain comparison to mean anything.
	assert.True(t, c.GetSubscriber() == nil, //nolint:testifylint // see above: a reflective nil check cannot fail here
		"a typed nil must not escape GetSubscriber: callers compare it against nil")
}

// TestIsNilHandlesNonNillableKinds pins that isNil does not panic on a value
// whose kind cannot be nil.
//
// reflect.Value.IsNil panics for a struct, a string, an int and so on, and a
// datasource field can legitimately hold one: implementing pubsub.Client, Redis
// or DB on a value receiver is ordinary Go, and GoFr's own tests do it. Before
// the Kind check, every caller of isNil -- Close, Health, GetSubscriber -- took
// the process down for those users.
func TestIsNilHandlesNonNillableKinds(t *testing.T) {
	type valueImpl struct{ name string }

	present := []any{
		valueImpl{name: "struct value"},
		"a string",
		42,
		[2]int{1, 2},
	}

	for _, v := range present {
		assert.NotPanics(t, func() { _ = isNil(v) }, "%T must not panic", v)
		assert.False(t, isNil(v), "%T is present, not nil", v)
	}

	// The nillable kinds keep their meaning.
	var nilPtr *valueImpl

	assert.True(t, isNil(nilPtr), "a typed nil pointer is absent")
	assert.True(t, isNil(nil), "an unset interface is absent")
	assert.False(t, isNil(&valueImpl{}), "a real pointer is present")
}
