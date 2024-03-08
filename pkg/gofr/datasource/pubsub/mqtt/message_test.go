package mqtt

import (
	"testing"
)

func TestMessage(_ *testing.T) {
	msg := message{msg: mockMessage{}}

	msg.Commit()
}

type mockMessage struct {
}

func (m mockMessage) Duplicate() bool {
	return false
}

func (m mockMessage) Qos() byte {
	return 0
}

func (m mockMessage) Retained() bool {
	return false
}

func (m mockMessage) Topic() string {
	return ""
}

func (m mockMessage) MessageID() uint16 {
	return 1
}

func (m mockMessage) Payload() []byte {
	return nil
}

func (m mockMessage) Ack() {
}
