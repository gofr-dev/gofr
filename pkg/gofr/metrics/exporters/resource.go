package exporters

import (
	"net/url"
	"strings"

	"go.opentelemetry.io/otel/attribute"
)

// parseResourceAttributes parses the OpenTelemetry "key1=value1,key2=value2"
// resource attribute format. Values are percent-decoded per the W3C Baggage
// encoding OTEL_RESOURCE_ATTRIBUTES uses; PathUnescape rather than
// QueryUnescape, so a literal "+" survives as itself. Malformed pairs are
// skipped individually - one bad pair must not discard the rest.
func parseResourceAttributes(s string) []attribute.KeyValue {
	if strings.TrimSpace(s) == "" {
		return nil
	}

	var attrs []attribute.KeyValue

	for _, pair := range strings.Split(s, ",") {
		key, value, found := strings.Cut(pair, "=")
		if !found {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)

		if key == "" || value == "" {
			continue
		}

		if decoded, err := url.PathUnescape(value); err == nil {
			value = decoded
		}

		attrs = append(attrs, attribute.String(key, value))
	}

	return attrs
}
