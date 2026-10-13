package container

import (
	"context"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource"
	"gofr.dev/pkg/gofr/datasource/pubsub"
	"gofr.dev/pkg/gofr/logging"
)

// nilReceiverClient reproduces the shape the real constructors produce: a
// pubsub.Client whose methods are reached through a nil pointer.
//
// google.New and kafka.New return a typed nil when they reject an incomplete
// config -- (*googleClient)(nil), (*kafkaClient)(nil) -- and the createXPubSub
// helpers assign that straight into the interface. A typed nil is not equal to
// nil, so every plain "!= nil" guard downstream passes and then calls a method on
// a nil receiver.
//
// Health dereferences the receiver for the same reason googleClient.Health does
// (it reads a config field), so a guard that stops filtering is a panic here
// rather than a silently different result. Building the state directly keeps
// these tests independent of what google.New happens to validate.
type nilReceiverClient struct {
	status string
}

func (c *nilReceiverClient) Health() datasource.Health {
	return datasource.Health{Status: c.status}
}

func (*nilReceiverClient) Publish(context.Context, string, []byte) error { return nil }

func (*nilReceiverClient) Subscribe(context.Context, string) (*pubsub.Message, error) {
	return nil, nil
}

func (*nilReceiverClient) CreateTopic(context.Context, string) error { return nil }

func (*nilReceiverClient) DeleteTopic(context.Context, string) error { return nil }

func (*nilReceiverClient) Query(context.Context, string, ...any) ([]byte, error) { return nil, nil }

func (*nilReceiverClient) Close() error { return nil }

// typedNilPubSubContainer builds the state every test below needs: a container
// whose PubSub interface holds a typed nil.
func typedNilPubSubContainer(t *testing.T) *Container {
	t.Helper()

	var client *nilReceiverClient

	c := &Container{}
	c.PubSub = client

	// Stated as a plain comparison, not require.NotNil: that one reflects, and
	// would report this very value as nil -- which is the whole premise.
	if c.PubSub == nil {
		t.Fatal("the premise: a typed nil assigned into the interface must not compare equal to nil")
	}

	return c
}

// assertPlainNil asserts the way callers actually test the value.
//
// assert.Nil is deliberately NOT used: it reflects, so it reports a typed nil as
// nil and would pass whether or not the filter exists. App.Subscribe writes
// `GetSubscriber() == nil`, so the test has to make the same plain comparison to
// mean anything -- which is what this helper exists to say, so the next reader
// does not "fix" the assertion back.
func assertPlainNil[T any](t *testing.T, got T, msgAndArgs ...any) {
	t.Helper()

	//nolint:testifylint // a reflective nil check cannot fail here; see the doc comment
	assert.True(t, any(got) == nil, msgAndArgs...)
}

// TestHealthOmitsTypedNilPubSub pins the guard in checkPrimaryDatasources, which
// called Health() on the nil receiver.
//
// The assertion is on the health map, not on NotPanics. runCheck already recovers
// in its deferred func, so the panic never escaped -- it was converted into a pubsub
// entry reading "health check panicked", which is the visible defect: an app with
// no usable pub/sub reported a DOWN dependency, and any aggregator watching the
// endpoint saw a degraded service with a stack fragment for a reason. With the
// guard the key is absent, which is what "no pub/sub configured" has always
// looked like. A NotPanics assertion here would pass either way and prove
// nothing.
func TestHealthOmitsTypedNilPubSub(t *testing.T) {
	c := typedNilPubSubContainer(t)

	health, ok := c.Health(context.Background()).(map[string]any)
	require.True(t, ok, "Health returns a map")

	assert.NotContains(t, health, pubsubKey,
		"a typed nil pub/sub must not be reported as a dependency at all")
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

	assertPlainNil(t, c.GetSubscriber(),
		"a typed nil must not escape GetSubscriber: callers compare it against nil")
}

// TestGetPublisherFiltersTypedNil is the same guarantee on the publishing side.
//
// The two getters return the same field, so leaving one unfiltered would be an
// asymmetry a reader has to guess about. A handler writing
// ctx.GetPublisher().Publish(...) against a typed-nil client hits the identical
// nil receiver; it just surfaces on a request rather than at startup.
func TestGetPublisherFiltersTypedNil(t *testing.T) {
	c := typedNilPubSubContainer(t)

	assertPlainNil(t, c.GetPublisher(),
		"a typed nil must not escape GetPublisher either")
}

// TestIsNilHandlesNonNillableKinds pins that isNil does not panic on a value
// whose kind cannot be nil.
//
// reflect.Value.IsNil panics for a struct, a string, an int and so on, and a
// datasource field can legitimately hold one: App.AddPubSub and friends take an
// interface, so an implementation with value receivers can be handed over as a
// struct rather than a pointer. GoFr writes such implementations itself --
// sqlMockDB's methods are on a value receiver -- it just happens to pass them by
// address. Before the Kind check, isNil panicked on every one of those; in
// Container.Close that reached the shutdown goroutine, which has no recover.
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

	// Every nillable kind keeps its meaning. Listed exhaustively rather than testing the pointer
	// alone: the Kind switch enumerates them, so a kind dropped from it would otherwise fall through
	// to "not nillable, therefore present" silently, which is the wrong answer in the safe-looking
	// direction.
	var (
		nilPtr   *valueImpl
		nilMap   map[string]int
		nilSlice []int
		nilChan  chan int
		nilFunc  func()
		nilIface error
		nilUnsaf unsafe.Pointer
	)

	absent := []any{nilPtr, nilMap, nilSlice, nilChan, nilFunc, nilIface, nilUnsaf}
	for _, v := range absent {
		assert.True(t, isNil(v), "a nil %T is absent", v)
	}

	assert.True(t, isNil(nil), "an unset interface is absent")

	presentNillable := []any{
		&valueImpl{},
		map[string]int{},
		[]int{},
		make(chan int),
		func() {},
	}
	for _, v := range presentNillable {
		assert.False(t, isNil(v), "a non-nil %T is present", v)
	}
}

// TestRejectedPubSubConfigLeavesTheExportedFieldNil pins the root cause rather than the symptom.
//
// The getters filter a typed nil, but Container.PubSub is exported: ctx.Container.PubSub, and any
// future internal caller, reads it directly and gets no filtering at all. kafka.New and google.New
// both return a bare nil of their concrete type when they reject a config, and assigning that
// straight into the pubsub.Client interface is what manufactures the typed nil in the first place.
//
// Assigning only a real client means the bad value never exists, and the getters stay as defense in
// depth rather than as the only thing standing between a misconfiguration and a nil receiver.
//
// The plain == nil comparison is the point: assert.Nil reflects, so it would pass on a typed nil and
// this test would prove nothing.
func TestRejectedPubSubConfigLeavesTheExportedFieldNil(t *testing.T) {
	tests := []struct {
		desc   string
		create func(c *Container, conf config.Config)
		conf   map[string]string
	}{
		{
			desc:   "google with no project id",
			create: (*Container).createGooglePubSub,
			conf:   map[string]string{"PUBSUB_BACKEND": "GOOGLE"},
		},
		{
			desc:   "kafka with no broker",
			create: (*Container).createKafkaPubSub,
			conf:   map[string]string{"PUBSUB_BACKEND": "KAFKA", "PUBSUB_BROKER": ""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			c := &Container{Logger: logging.NewMockLogger(logging.ERROR)}

			tc.create(c, config.NewMockConfig(tc.conf))

			if c.PubSub != nil {
				t.Errorf("a rejected config must leave PubSub plainly nil, got a %T in the interface", c.PubSub)
			}
		})
	}
}
