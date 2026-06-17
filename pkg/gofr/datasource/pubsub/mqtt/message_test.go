package mqtt

import (
	"testing"

	"github.com/eclipse/paho.golang/paho"
)

func TestMessage(_ *testing.T) {
	m := message{msg: &paho.Publish{}}

	m.Commit()
}
