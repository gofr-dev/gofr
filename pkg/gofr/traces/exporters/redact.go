package exporters

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// redactedPlaceholder stands in for credentials when a tracer endpoint is logged.
const redactedPlaceholder = "REDACTED"

// RedactURL returns a tracer endpoint that is safe to write to a log.
//
// Every log line in this package that names an endpoint goes through it, and a
// Builder registered from outside should do the same: TRACER_URL is operator
// input and routinely carries a credential.
//
// A scheme-bearing TRACER_URL is a real URL, so it can carry credentials in its
// userinfo (https://user:token@collector:4317) or its query
// (https://collector/api/v2/spans?api-key=...). Both are replaced rather than
// dropped, so the log still shows that something was configured there.
//
// Secrets placed in path segments (https://host/v1/TOKEN/spans) are not
// redacted: no tracer backend GoFr supports takes credentials there, and
// hiding the path would hide the part of the endpoint an operator debugs.
//
// Control characters are escaped on the unparsed path only: url.Parse rejects
// them, so such a value always lands there, and a raw newline would otherwise
// forge a second log line in the terminal's pretty output.
func RedactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return escapeControlChars(redactUnparsedURL(raw))
	}

	if u.User != nil {
		u.User = url.User(redactedPlaceholder)
	}

	if u.RawQuery != "" {
		u.RawQuery = redactedPlaceholder
	}

	u.Fragment = ""

	return u.String()
}

// escapeControlChars replaces every control character with its \x or \u
// escape, so a logged value always stays on one line.
func escapeControlChars(s string) string {
	if strings.IndexFunc(s, unicode.IsControl) < 0 {
		return s
	}

	var b strings.Builder

	for _, r := range s {
		switch {
		case !unicode.IsControl(r):
			b.WriteRune(r)
		case r <= unicode.MaxLatin1:
			fmt.Fprintf(&b, `\x%02x`, r)
		default:
			fmt.Fprintf(&b, `\u%04x`, r)
		}
	}

	return b.String()
}

// maxEchoedExporterNameLen bounds how much of an unsupported TRACE_EXPORTER is
// echoed back; every supported name is at most 6 characters.
const maxEchoedExporterNameLen = 16

// redactExporterName returns an unsupported TRACE_EXPORTER value in a form safe
// to log. A typo of a real exporter name -- short, letters and '-' or '_' only
// -- is echoed so it can be spotted; anything else, which is more likely a
// value pasted into the wrong variable, is replaced.
func redactExporterName(name string) string {
	if name == "" || len(name) > maxEchoedExporterNameLen {
		return redactedPlaceholder
	}

	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && r != '-' && r != '_' {
			return redactedPlaceholder
		}
	}

	return name
}

// redactUnparsedURL redacts an endpoint that url.Parse cannot split into a host
// and the rest: a schemeless host:port, host/path or user:pass@host, or input
// that does not parse at all. It works on the raw string and over-redacts
// rather than risk a leak.
//
// Userinfo is handled first, up to the last '@'. A '?' in a password, or an
// '@' in a query value, then falls inside the replaced prefix instead of
// splitting the credential. The query and fragment are handled after that.
func redactUnparsedURL(raw string) string {
	redacted := raw

	if i := strings.LastIndex(redacted, "@"); i >= 0 {
		redacted = redactedPlaceholder + redacted[i:]
	}

	if i := strings.IndexByte(redacted, '#'); i >= 0 {
		redacted = redacted[:i]
	}

	if i := strings.IndexByte(redacted, '?'); i >= 0 {
		redacted = redacted[:i+1] + redactedPlaceholder
	}

	return redacted
}

// RedactMessage returns msg with every occurrence of endpoint replaced by its
// RedactURL form. Use it on text that was produced elsewhere and may quote the
// configured endpoint back — an exporter's own error, an SDK diagnostic — where
// redacting at the point the endpoint is logged is not enough.
//
// This is not hypothetical on either count: an endpoint url.Parse rejects reaches
// zipkin.New, whose error is `invalid collector URL "<the raw URL>": parse
// "<the raw URL>"…`, and the OTel SDK reports a failed export as `request to
// <the raw URL> failed: …` long after startup.
//
// Both spellings are replaced. A URL reaches a message either verbatim or through
// %q — url.Error uses %q — and %q escapes exactly the characters (control, quote,
// backslash) that put a value on RedactURL's unparsed path to begin with, so
// matching the raw string alone would miss the case that produced the error. The
// whole endpoint is replaced rather than its credential alone: an endpoint that
// failed to parse is one RedactURL could not split, so there is no credential
// boundary to preserve inside it.
func RedactMessage(msg, endpoint string) string {
	if endpoint == "" {
		return msg
	}

	redacted := RedactURL(endpoint)

	// strconv.Quote wraps its result in quotes that the message supplies itself.
	quoted := strconv.Quote(endpoint)
	quoted = quoted[1 : len(quoted)-1]

	for _, form := range []string{endpoint, quoted} {
		msg = strings.ReplaceAll(msg, form, redacted)
	}

	return msg
}

// redactEndpointInError applies RedactMessage to a builder's error, so a failure
// cannot carry the credential the log line just hid. spanExporter logs that error
// and has no way to know an endpoint is inside it.
func redactEndpointInError(err error, endpoint string) error {
	if err == nil || endpoint == "" {
		return err
	}

	msg := RedactMessage(err.Error(), endpoint)
	if msg == err.Error() {
		return err
	}

	//nolint:err113 // a third-party constructor's message is being rewritten, not
	// wrapped: %w would re-expose the endpoint through Error().
	return errors.New(msg)
}
