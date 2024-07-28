package mqtt

import (
	"testing"
)

func TestMessage(_ *testing.T) {
	m := message{msg: mockMessage{}}

	m.Commit()
}

type mockMessage struct {
	duplicate bool
	qos       int
	retained  bool
	topic     string
	messageID int
	pyload    string
}

func (m mockMessage) Duplicate() bool {
	return m.duplicate
}

func (m mockMessage) Qos() byte {
	return byte(m.qos)
}

func (m mockMessage) Retained() bool {
	return m.retained
}

func (m mockMessage) Topic() string {
	return m.topic
}

func (m mockMessage) MessageID() uint16 {
	return uint16(m.messageID)
}

func (m mockMessage) Payload() []byte {
	return []byte(m.pyload)
}

func (mockMessage) Ack() {
}
