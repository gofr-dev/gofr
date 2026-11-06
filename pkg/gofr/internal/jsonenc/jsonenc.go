// Package jsonenc writes the JSON of a handler's result without encoding/json's per-call setup.
// It builds one encoder per Go type, once, and reuses it for every value of that type.
//
// Every function here either appends exactly the bytes json.Marshal appends for the same value,
// with its default HTML escaping, or reports false. On false the caller discards what was appended
// and uses encoding/json, so a value this package does not cover still encodes as before.
//
// Covered: booleans, integers, floats, strings of printable ASCII, time.Time, pointers,
// interfaces, slices other than byte slices, maps with string keys, and structs whose fields carry
// no embedding and no ",string" or ",omitzero" option. Anything else reports false: a type with a
// MarshalJSON or MarshalText method (other than time.Time), json.Number, a byte slice (base64), an
// array, a recursive type, a NaN or infinite float, a non-ASCII string, and nesting deeper than
// maxDepth. The rules this package copies are those of encoding/json in Go 1.26 (encode.go,
// BSD-style license, Copyright 2010 The Go Authors); each copied rule says where it comes from.
package jsonenc

import (
	"encoding"
	"encoding/json"
	"math"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"gofr.dev/pkg/gofr/internal/logjson"
)

// encoderFunc appends the JSON of v, or reports false when v holds something only encoding/json
// can spell. depth counts the interfaces crossed so far.
type encoderFunc func(dst []byte, v reflect.Value, depth int) ([]byte, bool)

// maxDepth bounds how many interfaces one value may nest. A static type cannot recurse (recursive
// types are not covered), so only an interface can lead back into a value already being encoded,
// as a map[string]any holding itself does. Past the bound the caller falls back, and
// encoding/json reports the cycle or encodes the deep value as it always did.
const maxDepth = 64

const (
	decimal = 10
	bits32  = 32
	bits64  = 64
)

//nolint:gochecknoglobals // reflect.Type constants and the per-type cache, as encoding/json keeps them.
var (
	marshalerType     = reflect.TypeFor[json.Marshaler]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
	numberType        = reflect.TypeFor[json.Number]()
	timeType          = reflect.TypeFor[time.Time]()

	// encoders maps a reflect.Type to its encoderFunc. A nil func marks a type that is not
	// covered, so the decision is made once per type.
	encoders sync.Map
)

// Append appends the JSON of v, exactly as json.Marshal writes it, to dst. It reports false when v
// is not covered; dst may then hold a partial encoding past its original length, which the caller
// must discard.
func Append(dst []byte, v any) ([]byte, bool) {
	if !enabled {
		return dst, false
	}

	if v == nil {
		return append(dst, "null"...), true
	}

	rv := reflect.ValueOf(v)

	enc := encoderOf(rv.Type())
	if enc == nil {
		return dst, false
	}

	return enc(dst, rv, 0)
}

// encoderOf returns the cached encoder for t, building it on first use.
func encoderOf(t reflect.Type) encoderFunc {
	if f, ok := encoders.Load(t); ok {
		return f.(encoderFunc) //nolint:forcetypeassert // only encoderFuncs are stored.
	}

	f := build(t, map[reflect.Type]bool{})

	actual, _ := encoders.LoadOrStore(t, f)

	return actual.(encoderFunc) //nolint:forcetypeassert // only encoderFuncs are stored.
}

// build returns the encoder for t, or nil when t is not covered. building holds the types whose
// encoders are being built further up the stack; meeting one again means t is recursive.
func build(t reflect.Type, building map[reflect.Type]bool) encoderFunc {
	if building[t] {
		return nil
	}

	if enc, decided := buildSpecial(t, building); decided {
		return enc
	}

	if enc, ok := kindEncoders[t.Kind()]; ok {
		return enc
	}

	building[t] = true
	defer delete(building, t)

	return buildComposite(t, building)
}

// buildSpecial decides the types whose kind does not: time.Time and *time.Time, which marshal
// themselves but are common in results and which logjson spells exactly (a nil *time.Time is null,
// as encoding/json's marshalerEncoder writes it), and the types that fall back whatever their kind.
func buildSpecial(t reflect.Type, building map[reflect.Type]bool) (enc encoderFunc, decided bool) {
	switch {
	case t == timeType:
		return encodeTime, true
	case t.Kind() == reflect.Pointer && t.Elem() == timeType:
		return buildPointer(t, building), true
	case hasMarshaler(t) || t == numberType:
		return nil, true
	default:
		return nil, false
	}
}

// buildComposite builds the encoder of a type made of other types.
func buildComposite(t reflect.Type, building map[reflect.Type]bool) encoderFunc {
	//nolint:exhaustive // the scalar kinds are in kindEncoders; array, chan, func, complex and unsafe pointer are not covered.
	switch t.Kind() {
	case reflect.Interface:
		return encodeInterface
	case reflect.Pointer:
		return buildPointer(t, building)
	case reflect.Slice:
		return buildSlice(t, building)
	case reflect.Map:
		return buildMap(t, building)
	case reflect.Struct:
		return buildStruct(t, building)
	default:
		return nil
	}
}

// kindEncoders are the encoders of the scalar kinds, which need nothing from their type but the
// kind. Interface is not here: its encoder looks types up in turn, which a table initialized from
// it cannot reference.
//
//nolint:gochecknoglobals // a constant table, built once and only read.
var kindEncoders = map[reflect.Kind]encoderFunc{
	reflect.Bool:    encodeBool,
	reflect.Int:     encodeInt,
	reflect.Int8:    encodeInt,
	reflect.Int16:   encodeInt,
	reflect.Int32:   encodeInt,
	reflect.Int64:   encodeInt,
	reflect.Uint:    encodeUint,
	reflect.Uint8:   encodeUint,
	reflect.Uint16:  encodeUint,
	reflect.Uint32:  encodeUint,
	reflect.Uint64:  encodeUint,
	reflect.Uintptr: encodeUint,
	reflect.Float32: encodeFloat32,
	reflect.Float64: encodeFloat64,
	reflect.String:  encodeString,
}

// hasMarshaler reports whether encoding/json could call a MarshalJSON or MarshalText method for a
// value of type t: one declared on t, or on *t, which encoding/json calls when the value is
// addressable (encode.go newTypeEncoder).
func hasMarshaler(t reflect.Type) bool {
	if t.Implements(marshalerType) || t.Implements(textMarshalerType) {
		return true
	}

	if t.Kind() == reflect.Pointer {
		return false
	}

	p := reflect.PointerTo(t)

	return p.Implements(marshalerType) || p.Implements(textMarshalerType)
}

func encodeBool(dst []byte, v reflect.Value, _ int) ([]byte, bool) {
	return strconv.AppendBool(dst, v.Bool()), true
}

func encodeInt(dst []byte, v reflect.Value, _ int) ([]byte, bool) {
	return strconv.AppendInt(dst, v.Int(), decimal), true
}

func encodeUint(dst []byte, v reflect.Value, _ int) ([]byte, bool) {
	return strconv.AppendUint(dst, v.Uint(), decimal), true
}

func encodeFloat32(dst []byte, v reflect.Value, _ int) ([]byte, bool) {
	return appendFloat(dst, v.Float(), bits32)
}

func encodeFloat64(dst []byte, v reflect.Value, _ int) ([]byte, bool) {
	return appendFloat(dst, v.Float(), bits64)
}

// appendFloat is encoding/json's floatEncoder.encode (encode.go): ES6 number formatting, 'e' only
// below 1e-6 or from 1e21 up, with a one-digit negative exponent unpadded. encoding/json reports an
// error for NaN and the infinities; here they report false and the fallback reports that error.
func appendFloat(dst []byte, f float64, bits int) ([]byte, bool) {
	if math.IsInf(f, 0) || math.IsNaN(f) {
		return dst, false
	}

	format := floatFormat(math.Abs(f), bits)

	dst = strconv.AppendFloat(dst, f, format, -1, bits)

	if format == 'e' {
		// Clean up e-09 to e-9.
		n := len(dst)
		if n >= 4 && dst[n-4] == 'e' && dst[n-3] == '-' && dst[n-2] == '0' {
			dst[n-2] = dst[n-1]
			dst = dst[:n-1]
		}
	}

	return dst, true
}

// floatFormat is the strconv format encoding/json picks for a float of magnitude abs. The float32
// comparisons are encoding/json's: they get the cutoffs right for a float32 value.
func floatFormat(abs float64, bits int) byte {
	if abs == 0 {
		return 'f'
	}

	if bits == bits64 && (abs < 1e-6 || abs >= 1e21) || bits == bits32 && (float32(abs) < 1e-6 || float32(abs) >= 1e21) {
		return 'e'
	}

	return 'f'
}

func encodeString(dst []byte, v reflect.Value, _ int) ([]byte, bool) {
	return logjson.AppendString(dst, v.String())
}

func encodeTime(dst []byte, v reflect.Value, _ int) ([]byte, bool) {
	// Only exported fields are walked, so v can always be read; the check keeps that a fallback
	// rather than a panic should it ever not hold.
	if !v.CanInterface() {
		return dst, false
	}

	t, ok := v.Interface().(time.Time)
	if !ok {
		return dst, false
	}

	return logjson.AppendTime(dst, t)
}

// encodeInterface encodes the value an interface holds with that value's own encoder, as
// encoding/json's interfaceEncoder does. The held value is not addressable, so only methods on its
// own type could apply, and those are excluded when its encoder is built.
func encodeInterface(dst []byte, v reflect.Value, depth int) ([]byte, bool) {
	if v.IsNil() {
		return append(dst, "null"...), true
	}

	if depth >= maxDepth {
		return dst, false
	}

	e := v.Elem()

	enc := encoderOf(e.Type())
	if enc == nil {
		return dst, false
	}

	return enc(dst, e, depth+1)
}

func buildPointer(t reflect.Type, building map[reflect.Type]bool) encoderFunc {
	elem := build(t.Elem(), building)
	if elem == nil {
		return nil
	}

	return func(dst []byte, v reflect.Value, depth int) ([]byte, bool) {
		if v.IsNil() {
			return append(dst, "null"...), true
		}

		return elem(dst, v.Elem(), depth)
	}
}

// buildSlice covers slices other than byte slices: encoding/json writes a slice whose element kind
// is uint8 as base64 (encode.go newSliceEncoder).
func buildSlice(t reflect.Type, building map[reflect.Type]bool) encoderFunc {
	if t.Elem().Kind() == reflect.Uint8 {
		return nil
	}

	elem := build(t.Elem(), building)
	if elem == nil {
		return nil
	}

	return func(dst []byte, v reflect.Value, depth int) ([]byte, bool) {
		if v.IsNil() {
			return append(dst, "null"...), true
		}

		dst = append(dst, '[')

		var ok bool

		for i := range v.Len() {
			if i > 0 {
				dst = append(dst, ',')
			}

			if dst, ok = elem(dst, v.Index(i), depth); !ok {
				return dst, false
			}
		}

		return append(dst, ']'), true
	}
}

// smallMap is the number of map entries sorted without a heap allocation.
const smallMap = 8

// mapEntry is one map element with its key already a string, as encoding/json's reflectWithString.
type mapEntry struct {
	key string
	val reflect.Value
}

// buildMap covers maps whose key kind is string. encoding/json uses such a key's string value as
// is, sorted by strings.Compare (encode.go mapEncoder.encode, resolveKeyName). A key type with a
// MarshalText method is left out even so, to keep to the plainest case.
func buildMap(t reflect.Type, building map[reflect.Type]bool) encoderFunc {
	key := t.Key()
	if key.Kind() != reflect.String || hasMarshaler(key) {
		return nil
	}

	elem := build(t.Elem(), building)
	if elem == nil {
		return nil
	}

	return func(dst []byte, v reflect.Value, depth int) ([]byte, bool) {
		if v.IsNil() {
			return append(dst, "null"...), true
		}

		// A small map's entries stay on the stack, and the iterator is a value rather than the
		// pointer MapRange allocates.
		var (
			small [smallMap]mapEntry
			iter  reflect.MapIter
		)

		entries := small[:0]

		iter.Reset(v)

		for iter.Next() {
			entries = append(entries, mapEntry{key: iter.Key().String(), val: iter.Value()})
		}

		slices.SortFunc(entries, func(a, b mapEntry) int { return strings.Compare(a.key, b.key) })

		dst = append(dst, '{')

		var ok bool

		for i := range entries {
			if i > 0 {
				dst = append(dst, ',')
			}

			if dst, ok = logjson.AppendString(dst, entries[i].key); !ok {
				return dst, false
			}

			dst = append(dst, ':')

			if dst, ok = elem(dst, entries[i].val, depth); !ok {
				return dst, false
			}
		}

		return append(dst, '}'), true
	}
}

// structField is one encoded field of a struct: its index, its `"name":` prefix already escaped,
// and its encoder.
type structField struct {
	index     int
	prefix    string
	omitEmpty bool
	enc       encoderFunc
}

// buildStruct covers structs without embedded fields and without the ",string" and ",omitzero"
// options, so encoding/json's field list is the exported fields in declaration order, under their
// tag names (encode.go typeFields). A repeated name, which encoding/json resolves by dominance, is
// left out as well.
func buildStruct(t reflect.Type, building map[reflect.Type]bool) encoderFunc {
	fields := make([]structField, 0, t.NumField())
	names := make(map[string]bool, t.NumField())

	for i := range t.NumField() {
		sf := t.Field(i)

		f, use, ok := fieldFor(&sf, names, building)
		if !ok {
			return nil
		}

		if use {
			f.index = i
			fields = append(fields, f)
		}
	}

	return func(dst []byte, v reflect.Value, depth int) ([]byte, bool) {
		dst = append(dst, '{')

		var (
			ok    bool
			first = true
		)

		for i := range fields {
			f := &fields[i]
			fv := v.Field(f.index)

			if f.omitEmpty && isEmptyValue(fv) {
				continue
			}

			if !first {
				dst = append(dst, ',')
			}

			first = false

			dst = append(dst, f.prefix...)

			if dst, ok = f.enc(dst, fv, depth); !ok {
				return dst, false
			}
		}

		return append(dst, '}'), true
	}
}

// fieldFor returns the encoded field for sf. use is false for a field encoding/json skips: an
// unexported one, or one tagged "-". ok is false when sf is not covered, which leaves its whole
// struct to encoding/json. names holds the names taken by the fields before sf.
func fieldFor(sf *reflect.StructField, names map[string]bool, building map[reflect.Type]bool) (f structField, use, ok bool) {
	if sf.Anonymous {
		return f, false, false
	}

	tag := sf.Tag.Get("json")
	if !sf.IsExported() || tag == "-" {
		return f, false, true
	}

	name, opts, _ := strings.Cut(tag, ",")
	if !isValidTag(name) {
		name = sf.Name
	}

	omitEmpty, ok := parseOptions(opts)
	if !ok || names[name] {
		return f, false, false
	}

	names[name] = true

	prefix, ok := logjson.AppendString(nil, name)
	if !ok {
		return f, false, false
	}

	enc := build(sf.Type, building)
	if enc == nil {
		return f, false, false
	}

	return structField{prefix: string(append(prefix, ':')), omitEmpty: omitEmpty, enc: enc}, true, true
}

// parseOptions reads a field's tag options, reporting whether omitempty is among them and false
// when an option this package does not cover is. Other options are ignored, as encoding/json
// ignores them (tags.go tagOptions.Contains).
func parseOptions(opts string) (omitEmpty, ok bool) {
	for opts != "" {
		var opt string

		opt, opts, _ = strings.Cut(opts, ",")

		switch opt {
		case "omitempty":
			omitEmpty = true
		case "string", "omitzero":
			return false, false
		}
	}

	return omitEmpty, true
}

// isValidTag is encoding/json's isValidTag (encode.go): a tag name it accepts; others fall back to
// the field's Go name.
func isValidTag(s string) bool {
	if s == "" {
		return false
	}

	for _, c := range s {
		switch {
		case strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", c):
			// Backslash and quote chars are reserved, but otherwise any punctuation chars are allowed
			// in a tag name.
		case !unicode.IsLetter(c) && !unicode.IsDigit(c):
			return false
		}
	}

	return true
}

// isEmptyValue is encoding/json's isEmptyValue (encode.go), the omitempty test.
func isEmptyValue(v reflect.Value) bool {
	//nolint:exhaustive // a struct, array... of other kinds is never empty, as in encoding/json.
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.Interface, reflect.Pointer:
		return v.IsZero()
	}

	return false
}
