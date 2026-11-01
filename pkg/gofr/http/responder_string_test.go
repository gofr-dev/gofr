package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errStringEnvelope = errors.New("boom")

// encodedEnvelope is what Respond wrote for data before the string fast path: the envelope
// through json.Encoder.
func encodedEnvelope(t *testing.T, data any) string {
	t.Helper()

	var b bytes.Buffer

	require.NoError(t, json.NewEncoder(&b).Encode(&response{Data: data}))

	return b.String()
}

func stringCorpus() []string {
	const generated = 2000

	corpus := make([]string, 0, 20+generated)
	corpus = append(corpus,
		"Hello World!", "a", " ", `"`, `\`, `<script>&</script>`, "tab\there", "line\nbreak", "\r\b\f",
		"\x00\x01\x1f\x7f", "héllo", "日本語", "  ", "\xff\xfe", "a\xc3", "emoji 😀",
		`{"data":"x"}`, "'single'", "/slash/", "%41",
	)

	rng := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // reproducible test input
	for range generated {
		b := make([]byte, rng.IntN(24))

		for i := range b {
			// Weighted towards printable ASCII, with every byte reachable.
			if rng.IntN(4) == 0 {
				b[i] = byte(rng.UintN(256)) //nolint:gosec // UintN(256) fits a byte
			} else {
				b[i] = byte(0x20 + rng.UintN(0x5f)) //nolint:gosec // at most 0x7e
			}
		}

		corpus = append(corpus, string(b))
	}

	return corpus
}

// A plain string answers with the bytes json.Encoder wrote for its envelope.
func TestRespond_StringEnvelopeSameBytes(t *testing.T) {
	for _, s := range stringCorpus() {
		w := httptest.NewRecorder()
		NewResponder(w, http.MethodGet).Respond(s, nil)

		require.Equal(t, encodedEnvelope(t, s), w.Body.String(), "%q", s)
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "application/json", w.Header().Get("Content-Type"))
	}
}

// appendStringEnvelope declines what it cannot write as the encoder would, and what it accepts
// it writes exactly.
func TestAppendStringEnvelope(t *testing.T) {
	for _, s := range stringCorpus() {
		b, ok := appendStringEnvelope(nil, s, nil)
		if ok {
			require.Equal(t, encodedEnvelope(t, s), string(b), "%q", s)
		}
	}

	_, ok := appendStringEnvelope(nil, "Hello", nil)
	assert.True(t, ok)

	_, ok = appendStringEnvelope(nil, "", nil)
	assert.False(t, ok, "omitempty drops an empty string")

	_, ok = appendStringEnvelope(nil, "x", map[string]any{"message": "m"})
	assert.False(t, ok, "an error object")

	_, ok = appendStringEnvelope(nil, quotedString("x"), nil)
	assert.False(t, ok, "a named string type")

	_, ok = appendStringEnvelope(nil, "line\n", nil)
	assert.False(t, ok, "a control byte is the encoder's to spell")
}

type quotedString string

func (q quotedString) MarshalJSON() ([]byte, error) { return []byte(`"custom-` + string(q) + `"`), nil }

// Everything the fast path declines is answered exactly as before.
func TestRespond_StringEnvelopeFallbacks(t *testing.T) {
	for name, tc := range map[string]struct {
		data any
		err  error
	}{
		"empty":      {"", nil},
		"named":      {quotedString("x"), nil},
		"with error": {"partial", errStringEnvelope},
		"control":    {"a\nb", nil},
	} {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			NewResponder(w, http.MethodGet).Respond(tc.data, tc.err)

			if tc.err == nil {
				assert.Equal(t, encodedEnvelope(t, tc.data), w.Body.String())
				assert.Equal(t, http.StatusOK, w.Code)

				return
			}

			var got map[string]any

			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
			assert.Equal(t, "partial", got["data"])
			assert.NotNil(t, got["error"])
		})
	}
}
