package jsonenc

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errFixture = errors.New("an error")

type plain struct {
	ID     int      `json:"id"`
	Name   string   `json:"name"`
	Active bool     `json:"active"`
	Tags   []string `json:"tags"`
}

type options struct {
	A  string         `json:"a,omitempty"`
	B  int            `json:"b,omitempty"`
	C  *int           `json:"c,omitempty"`
	D  []int          `json:"d,omitempty"`
	E  map[string]int `json:"e,omitempty"`
	F  any            `json:"f,omitempty"`
	G  float64        `json:"g,omitempty"`
	H  bool           `json:",omitempty"`
	I  uint8          `json:"i,omitempty,unknown"`
	J  string         `json:"-,"` //nolint:staticcheck // a field named "-", which encoding/json allows.
	K  string         `json:"-"`
	L  string         `json:"bad\"name"` //nolint:staticcheck // an invalid tag name, which falls back to the Go name.
	M  string         `json:"<&>"`
	N  struct{}       `json:"n,omitempty"`
	O  *plain         `json:"o"`
	P  time.Time      `json:"p"`
	Q  *time.Time     `json:"q"`
	u  string
	Up string
}

type named string

type namedMap map[named]any

type withMarshaler struct{ V int }

func (withMarshaler) MarshalJSON() ([]byte, error) { return []byte(`{"m":1}`), nil }

type withPtrMarshaler struct{ V int }

func (*withPtrMarshaler) MarshalJSON() ([]byte, error) { return []byte(`"ptr"`), nil }

type withText struct{ V int }

func (withText) MarshalText() ([]byte, error) { return []byte("text"), nil }

type embedded struct {
	plain
	X int
}

type quoted struct {
	N int `json:"n,string"`
}

type omitZero struct {
	T time.Time `json:"t,omitzero"`
}

// duplicate has two fields encoded as "X": a tagged one and an untagged one.
type duplicate struct {
	A int `json:"X"`
	X int
}

type recursive struct {
	Next *recursive `json:"next"`
}

type withArray struct {
	A [2]int `json:"a"`
}

type withBytes struct {
	B []byte `json:"b"`
}

type withNumber struct {
	N json.Number `json:"n"`
}

type withRaw struct {
	R json.RawMessage `json:"r"`
}

type withFunc struct {
	F func() `json:"f"`
}

type holder struct {
	V any `json:"v"`
}

// marshalJSON is the reference: json.Marshal, which writes what json.Encoder writes for the same
// value without its trailing newline.
func marshalJSON(v any) (string, error) {
	b, err := json.Marshal(v)

	return string(b), err
}

// assertSame checks that Append writes exactly json.Marshal's bytes whenever it reports true, and
// returns whether it did.
func assertSame(t *testing.T, v any) bool {
	t.Helper()

	got, ok := Append([]byte("prefix:"), v)
	if !ok {
		return false
	}

	want, err := marshalJSON(v)
	require.NoError(t, err, "Append reported true for a value json.Marshal rejects: %#v", v)
	assert.Equal(t, "prefix:"+want, string(got), "value %#v", v)

	return true
}

func TestAppend_Covered(t *testing.T) {
	seven := 7
	when := time.Date(2026, 11, 6, 3, 20, 1, 123456000, time.FixedZone("IST", 19800))

	values := []any{
		nil, true, false, 0, -1, math.MaxInt64, math.MinInt64, uint64(math.MaxUint64), uint8(255),
		int8(-128), uintptr(9), "", "plain", `quote " back \ slash`, "<script>&amp;</script>", "~ !",
		0.0, math.Copysign(0, -1), 1.0, 0.1, 1e-6, 9.99999e-7, 1e20, 1e21, 123456789.125, -1e-300, math.MaxFloat64,
		math.SmallestNonzeroFloat64, float32(0.1), float32(1e-7), float32(3.4e38), float32(1e21),
		[]int{}, []int(nil), []string{"a", "b"}, []any{1, "x", nil, []any{}, map[string]any{}},
		map[string]any{}, map[string]any(nil), map[string]any{"b": 1, "a": "x", "": nil, "A": true, "<": 1},
		map[named]int{"z": 1, "y": 2}, namedMap{"k": named("v")},
		plain{}, plain{ID: 1, Name: "gofr", Active: true, Tags: []string{"a", "b"}}, &plain{ID: 2},
		(*plain)(nil), options{}, options{
			A: "a", B: 1, C: &seven, D: []int{1}, E: map[string]int{"x": 1}, F: 0, G: 0.5, H: true, I: 1,
			J: "j", K: "k", L: "l", M: "m", O: &plain{}, P: when, Q: &when, u: "u", Up: "up",
		},
		struct{}{}, struct{ a int }{1}, holder{V: holder{V: []any{plain{}}}}, when, &when, (*time.Time)(nil),
		[]time.Time{when, {}}, map[string]*plain{"p": nil, "q": {Name: "q"}},
		errFixture, named("named"),
	}

	for i, v := range values {
		t.Run(fmt.Sprintf("%d_%T", i, v), func(t *testing.T) {
			assert.True(t, assertSame(t, v), "expected the fast path to cover %#v", v)
		})
	}
}

func TestAppend_FallsBack(t *testing.T) {
	selfMap := map[string]any{}
	selfMap["self"] = selfMap

	deep := any("leaf")
	for range maxDepth + 1 {
		deep = []any{deep}
	}

	values := []any{
		withMarshaler{}, &withMarshaler{}, withPtrMarshaler{}, &withPtrMarshaler{}, withText{},
		holder{V: withMarshaler{}}, []withPtrMarshaler{{}}, map[string]withText{"a": {}},
		embedded{}, quoted{}, omitZero{}, duplicate{}, recursive{}, withArray{}, withBytes{}, []byte("b"),
		withNumber{N: "1"}, json.Number("2"), withRaw{R: json.RawMessage(`{}`)}, withFunc{},
		math.NaN(), math.Inf(1), float32(math.Inf(-1)), []any{math.NaN()},
		"é", "line\nbreak", "\x00", " ", "\xff", map[string]int{"é": 1},
		map[int]string{1: "a"}, map[withText]int{{}: 1}, complex(1, 2), make(chan int), [2]int{},
		time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC), selfMap, deep,
		struct {
			X int `json:"é"`
		}{},
	}

	for i, v := range values {
		t.Run(fmt.Sprintf("%d_%T", i, v), func(t *testing.T) {
			_, ok := Append(nil, v)
			assert.False(t, ok, "expected %#v to fall back", v)
		})
	}
}

// TestAppend_Random compares Append with json.Marshal over random values: nested maps, slices and
// structs holding random strings, numbers and floats, including the bytes and float values whose
// spelling is easy to get wrong. Every value the fast path takes must match byte for byte, and it
// has to take most of them for the comparison to mean anything.
func TestAppend_Random(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2)) //nolint:gosec // a reproducible test input, not a secret.

	const n = 20000

	covered := 0

	for range n {
		if assertSame(t, randomValue(r, 0)) {
			covered++
		}
	}

	assert.Greater(t, covered, n/2, "the fast path covered too few random values to be tested")
}

//nolint:gocyclo // one case per kind of random value.
func randomValue(r *rand.Rand, depth int) any {
	kind := r.IntN(12)
	if depth > 3 {
		kind %= 6
	}

	switch kind {
	case 0:
		return randomString(r)
	case 1:
		return r.Int64() >> r.IntN(64)
	case 2:
		return randomFloat(r)
	case 3:
		return r.IntN(2) == 0
	case 4:
		return nil
	case 5:
		return float32(randomFloat(r))
	case 6, 7:
		m := map[string]any{}
		for range r.IntN(5) {
			m[randomString(r)] = randomValue(r, depth+1)
		}

		return m
	case 8:
		s := make([]any, r.IntN(4))
		for i := range s {
			s[i] = randomValue(r, depth+1)
		}

		return s
	case 9:
		return plain{ID: r.Int(), Name: randomString(r), Active: r.IntN(2) == 0, Tags: []string{randomString(r)}}
	case 10:
		return &options{A: randomString(r), G: randomFloat(r), F: randomValue(r, depth+1), M: randomString(r)}
	default:
		return holder{V: randomValue(r, depth+1)}
	}
}

// randomString is mostly printable ASCII, with the characters encoding/json escapes, and now and
// then a byte the fast path leaves to encoding/json.
func randomString(r *rand.Rand) string {
	const alphabet = `abcXYZ019 "\<>&/~` + "\t\x7fé "

	var b strings.Builder

	for range r.IntN(12) {
		if r.IntN(40) == 0 {
			b.WriteString(alphabet[len(alphabet)-7:])

			continue
		}

		b.WriteByte(alphabet[r.IntN(len(alphabet)-7)])
	}

	return b.String()
}

func randomFloat(r *rand.Rand) float64 {
	switch r.IntN(6) {
	case 0:
		return math.Float64frombits(r.Uint64())
	case 1:
		return float64(r.IntN(2000)-1000) / 8
	case 2:
		return math.Pow(10, float64(r.IntN(60)-30)) * (r.Float64() + 0.5)
	case 3:
		return []float64{1e-6, 1e21, 1e-7, 1e20, 0, math.Copysign(0, -1), 5e-324}[r.IntN(7)]
	default:
		return r.NormFloat64() * 1e6
	}
}

// FuzzAppend feeds arbitrary strings and numbers through the covered shapes.
func FuzzAppend(f *testing.F) {
	f.Add("gofr", int64(1), 0.5, uint64(2))
	f.Add("<&>\"\\", int64(-1), 1e21, uint64(0))
	f.Add(" \xff", int64(math.MinInt64), 1e-7, uint64(math.MaxUint64))

	f.Fuzz(func(t *testing.T, s string, i int64, fl float64, u uint64) {
		assertSame(t, s)
		assertSame(t, fl)
		assertSame(t, float32(fl))
		assertSame(t, plain{ID: int(i), Name: s, Tags: []string{s, s}})
		assertSame(t, map[string]any{s: fl, "i": i, "u": u, "f32": float32(fl)})
		assertSame(t, &options{A: s, B: int(i), G: fl, I: uint8(u), M: s, F: []any{s, u}}) //nolint:gosec // truncation is fine for test input.
	})
}

// TestAppend_Concurrent builds and uses encoders for the same types from many goroutines, for the
// race detector.
func TestAppend_Concurrent(t *testing.T) {
	type fresh struct {
		A string         `json:"a"`
		B map[string]any `json:"b"`
	}

	var wg sync.WaitGroup

	for g := range 8 {
		wg.Go(func() {
			for i := range 200 {
				v := fresh{A: fmt.Sprint(g, i), B: map[string]any{"p": plain{ID: i}}}

				got, ok := Append(nil, v)
				want, err := marshalJSON(v)

				assert.True(t, ok)
				require.NoError(t, err)
				assert.Equal(t, want, string(got))
			}
		})
	}

	wg.Wait()
}

func BenchmarkAppend(b *testing.B) {
	cases := []struct {
		name string
		v    any
	}{
		{"struct", holder{V: plain{ID: 1, Name: "gofr", Active: true, Tags: []string{"a", "b"}}}},
		{"map", map[string]any{"message": "route not registered"}},
		{"string", holder{V: "Hello World!"}},
	}

	for _, c := range cases {
		b.Run(c.name+"/jsonenc", func(b *testing.B) {
			buf := make([]byte, 0, 512)

			b.ReportAllocs()

			for b.Loop() {
				buf, _ = Append(buf[:0], c.v)
			}
		})

		b.Run(c.name+"/encoding_json", func(b *testing.B) {
			b.ReportAllocs()

			for b.Loop() {
				_, _ = json.Marshal(c.v)
			}
		})
	}
}
