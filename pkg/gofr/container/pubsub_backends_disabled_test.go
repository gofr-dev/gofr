//go:build gofr_nopubsub

package container

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/logging"
	"gofr.dev/pkg/gofr/testutil"
)

// The tagged build is the one where a mistake is silent, so it is the one that
// needs the assertion.
//
// Everywhere else the tag is handled by skipping: container_test.go skips the
// three backend tests because their clients are not linked. Skips prove nothing
// about what the stubs do. An edit that dropped the Errorf from any of them --
// or returned a non-nil client from the MQTT stub -- would leave every job green
// while producing exactly the failure the tag exists to avoid: a service that
// configured a backend, published to it, and never published anything.
//
// This file only compiles under the tag, so it runs in the Slim Build Tags job
// and nowhere else. That is the same shape graphql_disabled_test.go uses.
func newDisabledPubSubContainer(t *testing.T) *Container {
	t.Helper()

	return &Container{Logger: logging.NewLogger(logging.ERROR)}
}

func TestPubSubDisabled_MQTTReportsTheTag(t *testing.T) {
	var client any

	logs := testutil.StderrOutputForFunc(func() {
		c := newDisabledPubSubContainer(t)
		client = c.createMqttPubSub(config.NewMockConfig(map[string]string{"PUBSUB_BACKEND": "MQTT"}))
	})

	assert.Nil(t, client, "the stub must hand back no client, not a typed nil")
	assertNamesTheTag(t, logs, "MQTT")
}

func TestPubSubDisabled_KafkaReportsTheTag(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		c := newDisabledPubSubContainer(t)
		c.createKafkaPubSub(config.NewMockConfig(map[string]string{"PUBSUB_BACKEND": "KAFKA"}))

		assert.Nil(t, c.PubSub, "a disabled backend must leave PubSub unset")
	})

	assertNamesTheTag(t, logs, "Kafka")
}

func TestPubSubDisabled_GoogleReportsTheTag(t *testing.T) {
	logs := testutil.StderrOutputForFunc(func() {
		c := newDisabledPubSubContainer(t)
		c.createGooglePubSub(config.NewMockConfig(map[string]string{"PUBSUB_BACKEND": "GOOGLE"}))

		assert.Nil(t, c.PubSub, "a disabled backend must leave PubSub unset")
	})

	assertNamesTheTag(t, logs, "Google Pub/Sub")
}

// assertNamesTheTag checks the two things the message has to carry: which client
// is missing, and the tag that removed it. A user hitting this has a binary that
// behaves differently from the same source built normally, so the log line is the
// only place the explanation can come from.
func assertNamesTheTag(t *testing.T, logs, client string) {
	t.Helper()

	require.NotEmpty(t, logs, "a configured backend must not be dropped silently")
	assert.Contains(t, logs, client, "the message must name the client that is missing")
	assert.Contains(t, logs, "gofr_nopubsub", "the message must name the tag that removed it")
	assert.Contains(t, strings.ToLower(logs), "rebuild", "the message must say how to get it back")
}
