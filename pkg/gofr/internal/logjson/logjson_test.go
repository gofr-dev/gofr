package logjson

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// encodingJSON is the reference: what json.Encoder (EscapeHTML on, as the logger uses it) writes
// for v, without the trailing newline.
func encodingJSON(tb testing.TB, v any) string {
	tb.Helper()

	var buf bytes.Buffer

	require.NoError(tb, json.NewEncoder(&buf).Encode(v))

	return string(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
}

func stringCorpus() []string {
	return []string{
		"", "plain", "GET", "/users/42?x=1&y=2", `quote " backslash \ slash /`,
		"<script>alert(1)</script>", "a&b", "tab\there", "nl\n", "nul\x00", "del\x7f",
		"héllo", "日本", "\u2028\u2029", "bad\xffutf8", "Mozilla/5.0 (X11; Linux x86_64)",
		"192.0.2.10", "0123456789abcdef0123456789abcdef", "~`!@#$%^*()_+-=[]{}|;':,.?",
	}
}

func TestAppendStringMatchesEncodingJSON(t *testing.T) {
	for _, s := range stringCorpus() {
		got, ok := AppendString(nil, s)
		if !ok {
			continue // the caller falls back to encoding/json, which is correct by definition
		}

		assert.Equal(t, encodingJSON(t, s), string(got), "string %q", s)
	}
}

// TestAppendStringFallsBackOutsidePrintableASCII pins which inputs leave the fast path: every byte
// whose escape the fast path does not spell itself.
func TestAppendStringFallsBackOutsidePrintableASCII(t *testing.T) {
	for _, s := range []string{"\n", "\x00", "\x1f", "\x7f", "é", "\u2028", "\xff"} {
		_, ok := AppendString(nil, s)
		assert.False(t, ok, "string %q must fall back", s)
	}

	for c := byte(0x20); c < 0x7f; c++ {
		_, ok := AppendString(nil, string(c))
		assert.True(t, ok, "byte %q must stay on the fast path", c)
	}
}

func FuzzAppendString(f *testing.F) {
	for _, s := range stringCorpus() {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		got, ok := AppendString(nil, s)
		if ok {
			require.Equal(t, encodingJSON(t, s), string(got), "string %q", s)
		}
	})
}

func TestAppendTimeMatchesEncodingJSON(t *testing.T) {
	ist := time.FixedZone("IST", 5*3600+30*60)
	west := time.FixedZone("W", -8*3600)

	times := []time.Time{
		time.Now(),
		time.Now().UTC(),
		time.Now().In(ist),
		time.Date(2026, 10, 30, 15, 4, 5, 0, time.UTC),
		time.Date(2026, 10, 30, 15, 4, 5, 120000000, west),
		time.Date(2026, 1, 2, 3, 4, 5, 999999999, ist),
		time.Date(0, 1, 1, 0, 0, 0, 1, time.UTC),
		time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC),
		{},
	}

	for _, tm := range times {
		got, ok := AppendTime(nil, tm)
		require.True(t, ok, "time %v", tm)
		assert.Equal(t, encodingJSON(t, tm), string(got), "time %v", tm)
	}
}

func TestAppendTimeFallsBackWhereEncodingJSONDiffers(t *testing.T) {
	times := []time.Time{
		time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(-1, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("S", 3600+30)),
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.FixedZone("H", 24*3600)),
	}

	for _, tm := range times {
		_, ok := AppendTime(nil, tm)
		assert.False(t, ok, "time %v must fall back", tm)
	}
}

type objectProbe struct {
	A string `json:"a,omitempty"`
	N int64  `json:"n,omitempty"`
	B string `json:"b,omitempty"`
	M int64  `json:"m,omitempty"`
}

func appendProbe(dst []byte, p objectProbe) ([]byte, bool) {
	o := BeginObject(dst)
	o.String("a", p.A)
	o.Int("n", p.N)
	o.String("b", p.B)
	o.Int("m", p.M)

	return o.End()
}

func TestObjectMatchesEncodingJSON(t *testing.T) {
	probes := []objectProbe{
		{}, {A: "x"}, {N: 1}, {B: "y"}, {M: -7}, {A: "x", M: 3}, {N: -9223372036854775808, B: "<&>"},
		{A: "a", N: 1, B: "b", M: 2},
	}

	for _, p := range probes {
		got, ok := appendProbe(nil, p)
		require.True(t, ok, "%+v", p)
		assert.Equal(t, encodingJSON(t, p), string(got), "%+v", p)
	}

	_, ok := appendProbe(nil, objectProbe{A: "x", B: "\n"})
	assert.False(t, ok, "a field outside the fast path must fail the whole object")
}

func TestAppendMessage(t *testing.T) {
	got, ok := AppendMessage([]byte("p:"), "hi <you>")
	require.True(t, ok)
	assert.Equal(t, `p:"hi \u003cyou\u003e"`, string(got))

	_, ok = AppendMessage(nil, 42)
	assert.False(t, ok, "an unregistered type must fall back")
}

type registeredProbe struct{ v string }

// TestRegisterConcurrentWithAppendMessage pins that registration can race with logging: -race
// must stay quiet and each registered encoder must become visible.
func TestRegisterConcurrentWithAppendMessage(t *testing.T) {
	var wg sync.WaitGroup

	for range 4 {
		wg.Add(2)

		go func() {
			defer wg.Done()

			Register(func(dst []byte, v any) ([]byte, bool) {
				p, ok := v.(registeredProbe)
				if !ok {
					return dst, false
				}

				return AppendString(dst, p.v)
			})
		}()

		go func() {
			defer wg.Done()

			_, _ = AppendMessage(nil, registeredProbe{v: "x"})
		}()
	}

	wg.Wait()

	got, ok := AppendMessage(nil, registeredProbe{v: "x"})
	require.True(t, ok)
	assert.Equal(t, `"x"`, string(got))
}
