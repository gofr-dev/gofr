//go:build gofr_nopubsub

// Stubs for a build made with -tags gofr_nopubsub.
//
// Leaving Container.PubSub nil is an already-supported state -- it is what an
// unset PUBSUB_BACKEND produces, and Close guards it with isNil -- so a service
// that does not use pub/sub is unaffected. A service that DOES configure a
// backend gets a logged error naming the tag, rather than silence, because the
// alternative is a publisher that quietly never publishes.

package container

import (
	"gofr.dev/pkg/gofr/config"
	"gofr.dev/pkg/gofr/datasource/pubsub"
)

// pubsubBackendsLinked is false here: see the comment on the constant in
// pubsub_backends.go.
const pubsubBackendsLinked = false

// pubsubDisabledMsg names the tag so the fix is obvious from one log line.
const pubsubDisabledMsg = "PUBSUB_BACKEND=%s was configured, but this binary was " +
	"built with -tags gofr_nopubsub, which omits the %s client. Rebuild without the tag to use it."

func (c *Container) createMqttPubSub(config.Config) pubsub.Client {
	c.Logger.Errorf(pubsubDisabledMsg, "MQTT", "MQTT")

	return nil
}

func (c *Container) createKafkaPubSub(config.Config) {
	c.Logger.Errorf(pubsubDisabledMsg, "KAFKA", "Kafka")
}

func (c *Container) createGooglePubSub(config.Config) {
	c.Logger.Errorf(pubsubDisabledMsg, "GOOGLE", "Google Pub/Sub")
}
