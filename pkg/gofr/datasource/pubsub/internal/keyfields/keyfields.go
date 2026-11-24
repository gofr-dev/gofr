// Package keyfields resolves configured JSON payload fields into partition keys.
package keyfields

import (
	"encoding/json"
	"strings"
)

// Fields maps a topic to a top-level JSON field.
type Fields map[string]string

// Parse reads comma-separated topic:field mappings. Invalid entries are ignored.
func Parse(raw string) Fields {
	fields := make(Fields)

	for _, entry := range strings.Split(raw, ",") {
		topic, field, ok := strings.Cut(strings.TrimSpace(entry), ":")
		topic, field = strings.TrimSpace(topic), strings.TrimSpace(field)

		if ok && topic != "" && field != "" && !strings.Contains(field, ":") {
			fields[topic] = field
		}
	}

	return fields
}

// Key extracts a non-empty JSON string or scalar. Unsupported values fall back
// to an unkeyed record; debug logs intentionally omit payloads and key values.
func (f Fields) Key(topic string, payload []byte, logger interface{ Debugf(string, ...any) }) []byte {
	field, configured := f[topic]
	if !configured {
		return nil
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(payload, &object); err != nil {
		logger.Debugf("pubsub key unavailable for topic %q: payload is not a JSON object", topic)

		return nil
	}

	raw, found := object[field]
	if !found || string(raw) == "null" {
		logger.Debugf("pubsub key unavailable for topic %q: field %q is missing or null", topic, field)

		return nil
	}

	if key := scalarKey(raw); len(key) > 0 {
		return key
	}

	logger.Debugf("pubsub key unavailable for topic %q: field %q must be a non-empty string or scalar", topic, field)

	return nil
}

func scalarKey(raw json.RawMessage) []byte {
	switch raw[0] {
	case '"':
		var key string
		if err := json.Unmarshal(raw, &key); err != nil {
			return nil
		}

		return []byte(key)
	case '{', '[':
		return nil
	default:
		return raw
	}
}
