/*
Package pubsub implements the necessary methods and types to work with
publish-subscribe messaging patterns. It offers support for various pubsub backends like kafka, avro ,
azure eventhub etc.
*/
package pubsub

import (
	"time"

	eventhub "github.com/Azure/azure-event-hubs-go/v3"

	"gofr.dev/pkg/gofr/types"
)

// Message struct represents a message with attributes such as schema ID,
// topic, partition, offset, key, value, headers, and an underlying event object.
type Message struct {
	SchemaID  int
	Topic     string
	Partition int
	Offset    int64
	Key       string
	Value     string
	Headers   map[string]string
	Event     *eventhub.Event
}

// PublishOptions provide additional configs which are required to publish messages
type PublishOptions struct {
	Topic     string    // default: reads topic from config, else empty string
	Partition int       // default: 0
	Timestamp time.Time // default: current timestamp
}

type TopicPartition struct {
	Topic     string
	Partition int
	Offset    int64
	Metadata  *string
	Error     error
}

// CommitFunc used to specify whether the message is to be committed,
// and if new message is to be consumed.
// first return bool value indicates whether the message has to be committed
// second return bool value indicates whether the next message is to be consumed.
// if second return bool value is set to false, the function would exit and return the control back
type CommitFunc func(message *Message) (bool, bool)

// SubscribeFunc used to defines the functionality for the push-based messages for mqtt publisher subscriber.
// This function would be executed whenever MQTT broker tries to push a message to the subscriber
type SubscribeFunc func(msg *Message) error

// PublisherSubscriber interface for publisher subscriber model
// also contains utility method for health-check and binding the messages
// received from Subscribe() method
type PublisherSubscriber interface {
	/*
		PublishEventWithOptions publishes message to the pubsub(kafka) configured.

			Ability to provide additional options as described in PublishOptions struct

			returns error if publish encounters a failure
	*/
	PublishEventWithOptions(key string, value interface{}, headers map[string]string, options *PublishOptions) error

	/*
		PublishEvent publishes message to the pubsub(kafka) configured.

			Information like topic is read from config, timestamp is set to current time
			other fields like offset and partition are set to it's default value
			if desire to overwrite these fields, refer PublishEventWithOptions() method above

			returns error if publish encounters a failure
	*/
	PublishEvent(string, interface{}, map[string]string) error

	/*
		Subscribe read messages from the pubsub(kafka) configured.

			If multiple topics are provided in the environment or
			in kafka config while creating the consumer, reads messages from multiple topics
			reads only one message at a time. If desire to read multiple messages
			call Subscribe in a for loop

			returns error if subscribe encounters a failure
			on success returns the message received in the Message struct format
	*/
	Subscribe() (*Message, error)

	/*
			SubscribeWithCommit read messages from the pubsub(kafka) configured.

				calls the CommitFunc after subscribing message from kafka and based on
		        the return values decides whether to commit message and consume another message
	*/
	SubscribeWithCommit(CommitFunc) (*Message, error)

	/*
		Bind converts message received from Subscribe to the specified target
			returns error, if messages doesn't adhere to the target structure
	*/
	Bind(message []byte, target interface{}) error

	CommitOffset(offsets TopicPartition)
	/*
		Ping checks for the health of the pubsub
			returns an error if the pubsub is down
	*/
	Ping() error

	// HealthCheck returns the health of the PubSub
	HealthCheck() types.Health

	// IsSet can be used to check if PubSub is initialized with a valid connection or not
	IsSet() bool
}

// PublisherSubscriberV2 interface for publisher subscriber model
// This one will implement the new function Pause and Resume
type PublisherSubscriberV2 interface {
	PublisherSubscriber

	// Pause will be used to pause the processing in kafka/sarama
	Pause() error

	// Resume will be used to resume all the consumer groups in kafka/sarama
	Resume() error
}

type MQTTPublisherSubscriber interface {
	Publish(payload []byte) error
	Subscribe(subscribeFunc SubscribeFunc) error
	Unsubscribe() error
	Disconnect(waitTime uint)
	Bind(message []byte, target interface{}) error
	Ping() error
	HealthCheck() types.Health
	IsSet() bool
}
