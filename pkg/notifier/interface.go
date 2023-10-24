/*
Package notifier defines Notifier interface and Message struct responsible for facilitating communication
and coordination between different components of an application.
*/
package notifier

import (
	"gofr.dev/pkg/gofr/types"
)

type Message struct {
	Value string
}

// Notifier interface containing useful methods needed to be implemented by any notifier
// also contains utility method for health-check and binding the messages
type Notifier interface {
	/*
		Publish publishes message to the notifier configured.
				Information like topic is read from configs
				returns error if publish encounters a failure.
		Attributes will be attached to original Message
				and it can be used to filter the messages
				at subscriber-side.
	*/
	Publish(value interface{}, attributes map[string]interface{}) error

	/*
		Subscribe read messages from the Notifier configured.
				returns error if subscribe encounters a failure.
				on success returns the message received in the Message struct format.
	*/
	Subscribe() (*Message, error)

	/*
		SubscribeWithResponse calls the subscribe function
			and binds the message's value to the target specified.
	*/
	SubscribeWithResponse(target interface{}) (*Message, error)

	/*
		Bind converts message received to the specified target
			returns error, if messages doesn't adhere to the target structure
	*/
	Bind(message []byte, target interface{}) error

	// HealthCheck returns the health of the Notifier
	HealthCheck() types.Health

	// IsSet can be used to check if Notifier is initialized with a valid connection or not
	IsSet() bool
}
