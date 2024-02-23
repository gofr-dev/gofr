package kafka

import (
	"testing"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"

	"gofr.dev/pkg/gofr/testutil"
)

func TestNew(t *testing.T) {
	msg := new(kafka.Message)
	reader := new(kafka.Reader)
	k := newKafkaMessage(msg, reader, nil)

	assert.NotNil(t, k)
	assert.Equal(t, msg, k.msg)
	assert.Equal(t, reader, k.reader)
}

func TestKafkaMessage_Commit(t *testing.T) {
	out := testutil.StderrOutputForFunc(func() {
		logger := testutil.NewMockLogger(testutil.ERRORLOG)
		k := newKafkaMessage(&kafka.Message{Topic: "test", Value: []byte("hello")}, &kafka.Reader{}, logger)
		k.Commit()
	})

	assert.Contains(t, out, "unable to commit message on kafka")
}
