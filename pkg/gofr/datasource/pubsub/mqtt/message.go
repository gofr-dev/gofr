package mqtt

import "github.com/eclipse/paho.golang/paho"

type message struct {
	msg *paho.Publish
}

func (*message) Commit() {
	// MQTT v5 QoS acknowledgment is handled by the autopaho library automatically.
	// For QoS 0: no ack needed.
	// For QoS 1/2: autopaho sends PUBACK/PUBREC automatically.
}
