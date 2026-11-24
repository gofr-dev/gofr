package keyfields

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type debugLogger struct {
	calls int
}

func (l *debugLogger) Debugf(string, ...any) { l.calls++ }

func TestConfiguredKey(t *testing.T) {
	for _, tt := range []struct {
		name, payload, expected string
	}{
		{"string", `{"id":"abc"}`, "abc"},
		{"escaped string", `{"id":"a\u0062c"}`, "abc"},
		{"number", `{"id":1234567890123456789}`, "1234567890123456789"},
		{"boolean", `{"id":false}`, "false"},
		{"missing", `{}`, ""},
		{"null", `{"id":null}`, ""},
		{"empty", `{"id":""}`, ""},
		{"object", `{"id":{}}`, ""},
		{"array field", `{"id":[]}`, ""},
		{"invalid JSON", `not json`, ""},
		{"JSON array", `[]`, ""},
		{"JSON scalar", `"abc"`, ""},
		{"null object", `null`, ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			logger := &debugLogger{}
			fields := Parse("orders:id")
			key := fields.Key("orders", []byte(tt.payload), logger)

			if tt.expected == "" {
				assert.Nil(t, key)
				assert.Equal(t, 1, logger.calls)
			} else {
				assert.Equal(t, tt.expected, string(key))
				assert.Zero(t, logger.calls)
			}
		})
	}
}

func TestMappings(t *testing.T) {
	fields := Parse(" orders : id , events:eventId,invalid,:field,topic:,broken:a:b")
	assert.Equal(t, Fields{"orders": "id", "events": "eventId"}, fields)

	logger := &debugLogger{}
	assert.Equal(t, []byte("event-1"), fields.Key("events", []byte(`{"eventId":"event-1"}`), logger))
	assert.Nil(t, fields.Key("unmapped", []byte("not json"), logger))
	assert.Empty(t, Parse(""))
	assert.Zero(t, logger.calls)
}
