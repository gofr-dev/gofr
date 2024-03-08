package mqtt

import mqtt "github.com/eclipse/paho.mqtt.golang"

type message struct {
	msg mqtt.Message
}

func (m *message) Commit() {
	m.msg.Ack()
}
