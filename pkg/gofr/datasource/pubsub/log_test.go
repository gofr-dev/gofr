package pubsub

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLog_PrettyPrint(t *testing.T) {
	testCases := []struct {
		desc     string
		log      Log
		expParts []string
	}{
		{
			desc: "publish log",
			log: Log{
				Mode: "PUB", CorrelationID: "corr-123", MessageValue: `{"k":"v"}`,
				Topic: "orders", PubSubBackend: "KAFKA", Time: 42,
			},
			expParts: []string{"corr-123", "KAFKA", "42", "PUB", "orders", `{"k":"v"}`},
		},
		{
			desc: "subscribe log",
			log: Log{
				Mode: "SUB", CorrelationID: "corr-456", MessageValue: "hello",
				Topic: "events", PubSubBackend: "MQTT", Time: 7,
			},
			expParts: []string{"corr-456", "MQTT", "7", "SUB", "events", "hello"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			var buf bytes.Buffer

			tc.log.PrettyPrint(&buf)

			out := buf.String()
			for _, part := range tc.expParts {
				assert.Contains(t, out, part)
			}
		})
	}
}
