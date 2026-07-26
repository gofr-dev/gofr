package http

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/file"
)

func TestParam(t *testing.T) {
	req := NewRequest(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/abc?a=b", http.NoBody))
	if req.Param("a") != "b" {
		t.Error("Can not parse the request params")
	}
}

func TestBind(t *testing.T) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/abc", strings.NewReader(`{"a": "b", "b": 5}`))
	r.Header.Set("Content-Type", "application/json")
	req := NewRequest(r)

	x := struct {
		A string `json:"a"`
		B int    `json:"b"`
	}{}

	_ = req.Bind(&x)

	if x.A != "b" || x.B != 5 {
		t.Errorf("Bind error. Got: %v", x)
	}
}

func TestBind_FileSuccess(t *testing.T) {
	r := NewRequest(generateMultipartRequestZip(t))
	x := struct {
		// Zip file bind for zip struct
		Zip file.Zip `file:"zip"`

		// Zip file bind for zip pointer
		ZipPtr *file.Zip `file:"zip"`

		// FileHeader multipart.FileHeader bind(value)
		FileHeader multipart.FileHeader `file:"hello"`

		// FileHeaderPtr multipart.FileHeader bind for pointer
		FileHeaderPtr *multipart.FileHeader `file:"hello"`

		// Skip bind
		Skip *file.Zip `file:"-"`

		// Incompatible type cannot be bound
		Incompatible string `file:"hello"`

		// File not in multipart form
		FileNotPresent *multipart.FileHeader `file:"text"`

		// Additional form fields
		StringField string  `form:"stringField"`
		IntField    int     `form:"intField"`
		FloatField  float64 `form:"floatField"`
		BoolField   bool    `form:"boolField"`
	}{}

	err := r.Bind(&x)
	require.NoError(t, err)

	// Assert zip file bind
	assert.Len(t, x.Zip.Files, 2)
	assert.Equal(t, "Hello! This is file A.\n", string(x.Zip.Files["a.txt"].Bytes()))
	assert.Equal(t, "Hello! This is file B.\n\n", string(x.Zip.Files["b.txt"].Bytes()))

	// Assert zip file bind for pointer
	assert.NotNil(t, x.ZipPtr)
	assert.Len(t, x.ZipPtr.Files, 2)
	assert.Equal(t, "Hello! This is file A.\n", string(x.ZipPtr.Files["a.txt"].Bytes()))
	assert.Equal(t, "Hello! This is file B.\n\n", string(x.ZipPtr.Files["b.txt"].Bytes()))

	// Assert FileHeader struct type
	assert.Equal(t, "hello.txt", x.FileHeader.Filename)

	f, err := x.FileHeader.Open()
	require.NoError(t, err)
	assert.NotNil(t, f)

	content, err := io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "Test hello!", string(content))

	// Assert FileHeader pointer type
	assert.NotNil(t, x.FileHeader)
	assert.Equal(t, "hello.txt", x.FileHeader.Filename)

	f, err = x.FileHeader.Open()
	require.NoError(t, err)
	assert.NotNil(t, f)

	content, err = io.ReadAll(f)
	require.NoError(t, err)
	assert.Equal(t, "Test hello!", string(content))

	// Assert skipped field
	assert.Nil(t, x.Skip)

	// Assert incompatible
	assert.Empty(t, x.Incompatible)

	// Assert file not present
	assert.Nil(t, x.FileNotPresent)

	// Assert additional form fields
	assert.Equal(t, "testString", x.StringField)
	assert.Equal(t, 123, x.IntField)
	assert.InEpsilon(t, 123.456, x.FloatField, 0.01)

	assert.True(t, x.BoolField)
}

func TestBind_NoContentType(t *testing.T) {
	req := NewRequest(httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/abc", strings.NewReader(`{"a": "b", "b": 5}`)))
	x := struct {
		A string `json:"a"`
		B int    `json:"b"`
	}{}

	// A body with no Content-Type is now reported rather than silently ignored.
	require.ErrorIs(t, req.Bind(&x), errUnsupportedContentType)

	// The data still does not bind, so zero values are expected.
	if x.A != "" || x.B != 0 {
		t.Errorf("Bind error. Got: %v", x)
	}
}

func Test_GetContext(t *testing.T) {
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, "test/hello", http.NoBody)
	r := Request{req: req, pathParams: map[string]string{"key": "hello"}}

	assert.Equal(t, t.Context(), r.Context())
	assert.Equal(t, "http://", r.HostName())
	assert.Equal(t, "hello", r.PathParam("key"))
}

func generateMultipartRequestZip(t *testing.T) *http.Request {
	t.Helper()

	var buf bytes.Buffer

	writer := multipart.NewWriter(&buf)

	f, err := os.Open("../testutil/test.zip")
	if err != nil {
		t.Fatalf("Failed to open test.zip: %v", err)
	}
	defer f.Close()

	zipPart, err := writer.CreateFormFile("zip", "test.zip")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	_, err = io.Copy(zipPart, f)
	if err != nil {
		t.Fatalf("Failed to write file to form: %v", err)
	}

	fileHeader, err := writer.CreateFormFile("hello", "hello.txt")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}

	_, err = io.Copy(fileHeader, bytes.NewReader([]byte(`Test hello!`)))
	if err != nil {
		t.Fatalf("Failed to write file to form: %v", err)
	}

	// Add non-file fields
	err = writer.WriteField("stringField", "testString")
	require.NoError(t, err)

	err = writer.WriteField("intField", "123")
	require.NoError(t, err)

	err = writer.WriteField("floatField", "123.456")
	require.NoError(t, err)

	err = writer.WriteField("boolField", "true")
	require.NoError(t, err)

	// Close the multipart writer
	writer.Close()

	// Create a new HTTP request with the multipart data
	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	return req
}

func Test_bindMultipart_Fails(t *testing.T) {
	// Non-pointer bind
	r := NewRequest(generateMultipartRequestZip(t))
	input := struct {
		file *file.Zip
	}{}

	err := r.bindMultipart(input)
	require.Error(t, err)
	assert.Equal(t, errNonPointerBind, err)

	// unexported field cannot be binded
	err = r.bindMultipart(&input)
	require.ErrorIs(t, err, errNoFileFound)
}

func Test_bindMultipart_Fail_ParseMultiPart(t *testing.T) {
	r := NewRequest(generateMultipartRequestZip(t))
	input2 := struct {
		File *file.Zip `file:"zip"`
	}{}

	// Call the multipart reader to handle form from a multipart reader
	// This is called to invoke error while parsing Multipart form in bind
	_, _ = r.req.MultipartReader()

	err := r.bindMultipart(&input2)
	require.ErrorContains(t, err, "http: multipart handled by MultipartReader")
}

func Test_Params(t *testing.T) {
	req := &http.Request{
		URL: &url.URL{
			RawQuery: "category=books&category=electronics&tag=tech,science",
		},
	}
	r := NewRequest(req)

	expectedCategories := []string{"books", "electronics"}
	expectedTags := []string{"tech", "science"}

	assert.ElementsMatch(t, expectedCategories, r.Params("category"), "expected all values of 'category' to match")
	assert.ElementsMatch(t, expectedTags, r.Params("tag"), "expected all values of 'tag' to match")
	// Nil, not merely empty: assert.Empty alone would pass for both.
	assert.Nil(t, r.Params("nonexistent"), "absent key must return a nil slice")

	// Pin the actual response body, not just the marshaled value: a handler may return Params
	// directly, and this is what its client receives. Going through Responder means a future change
	// to how it normalizes nil values cannot silently alter the wire shape while this test passes.
	rec := httptest.NewRecorder()
	NewResponder(rec, http.MethodGet).Respond(r.Params("nonexistent"), nil)
	assert.JSONEq(t, `{"data":null}`, rec.Body.String(),
		"an absent key must reach the client as null, not [] or an omitted field")
}

func TestBind_FormURLEncoded(t *testing.T) {
	// Create a new HTTP request with form-encoded data
	req := NewRequest(httptest.NewRequest(http.MethodPost, "/abc", strings.NewReader("Name=John&Age=30")))
	req.req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	x := struct {
		Name string `form:"Name"`
		Age  int    `form:"Age"`
	}{}

	err := req.Bind(&x)
	if err != nil {
		t.Errorf("Bind error: %v", err)
	}

	// Check the results
	if x.Name != "John" || x.Age != 30 {
		t.Errorf("Bind error. Got: %v", x)
	}
}

func TestBind_BinaryOctetStream(t *testing.T) {
	testCases := []struct {
		name string
		data []byte
	}{
		{"Raw Binary Data", []byte{0x42, 0x65, 0x6c, 0x6c, 0x61}},
		{"Text-Based Binary Data", []byte("This is some binary data")},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := NewRequest(httptest.NewRequest(http.MethodPost, "/binary", bytes.NewReader(tc.data)))
			req.req.Header.Set("Content-Type", "binary/octet-stream")

			var result []byte

			err := req.Bind(&result)
			if err != nil {
				t.Errorf("Bind error: %v", err)
			}

			if !bytes.Equal(result, tc.data) {
				t.Errorf("Bind error. Expected: %v, Got: %v", tc.data, result)
			}
		})
	}
}
func TestBind_BinaryOctetStream_NotPointerToByteSlice(t *testing.T) {
	req := &Request{
		req: httptest.NewRequest(http.MethodPost, "/binary", http.NoBody),
	}
	req.req.Header.Set("Content-Type", "binary/octet-stream")

	// A non-pointer target is now rejected up front by Bind with errNonPointerBind
	// rather than reaching bindBinary — binding into a value could never have
	// worked, and it used to be silent for the JSON and binary paths (the
	// form and multipart paths already returned errNonPointerBind).
	if err := req.Bind("invalid input"); !errors.Is(err, errNonPointerBind) {
		t.Fatalf("Expected error: %v, got: %v", errNonPointerBind, err)
	}

	// A pointer to something that is not a []byte still reaches bindBinary.
	var notBytes string

	err := req.Bind(&notBytes)

	if !errors.Is(err, errNonSliceBind) {
		t.Fatalf("Expected error: %v, got: %v", errNonSliceBind, err)
	}

	if !strings.Contains(err.Error(), "input is not a pointer to a byte slice") {
		t.Errorf("Expected error to contain: input is not a pointer to a byte slice, got: %v", err)
	}
}

func TestHostName_DefaultProto(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com/test", http.NoBody)
	r := NewRequest(req)

	hostname := r.HostName()

	assert.Equal(t, "http://example.com", hostname)
}

func TestHostName_WithForwardedProto(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://example.com/test", http.NoBody)
	req.Header.Set("X-Forwarded-Proto", "https")

	r := NewRequest(req)

	hostname := r.HostName()

	assert.Equal(t, "https://example.com", hostname)
}

func TestPathParam_NonExistent(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", http.NoBody)
	r := NewRequest(req)

	result := r.PathParam("nonexistent")

	assert.Empty(t, result)
}

func TestBody_MultipleReads(t *testing.T) {
	bodyContent := `{"key":"value"}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/test", strings.NewReader(bodyContent))
	r := NewRequest(req)

	// First read
	body1, err := r.body()
	require.NoError(t, err)
	assert.Equal(t, bodyContent, string(body1))

	// Second read should return same content due to NopCloser reset
	body2, err := r.body()
	require.NoError(t, err)
	assert.Equal(t, bodyContent, string(body2))
}

func TestBody_EmptyBody(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", http.NoBody)
	r := NewRequest(req)

	body, err := r.body()

	require.NoError(t, err)
	assert.Empty(t, body)
}

func TestBind_UnsupportedContentType(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/test", strings.NewReader("data"))
	req.Header.Set("Content-Type", "text/plain")

	r := NewRequest(req)

	err := r.Bind(&struct{}{})

	// A body the framework cannot decode is reported rather than silently
	// discarded; binding a bodyless request stays a no-op.
	require.ErrorIs(t, err, errUnsupportedContentType)
}

func TestParam_NonExistent(t *testing.T) {
	req := NewRequest(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", http.NoBody))

	result := req.Param("missing")

	assert.Empty(t, result)
}

func TestContext_ReturnsRequestContext(t *testing.T) {
	httpReq := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test", http.NoBody)
	r := NewRequest(httpReq)

	ctx := r.Context()

	assert.Equal(t, httpReq.Context(), ctx)
}

// ---------------------------------------------------------------------------
// Characterization suite.
//
// Pins the CURRENT param/binding contract of Request: exact return values and
// exact error strings, including the sharp edges. Assertions are literal on
// purpose — a refactor that changes any of these changes handler behavior.
// ---------------------------------------------------------------------------

// charBindTarget is the struct bound in the JSON characterization cases.
type charBindTarget struct {
	A string `json:"a"`
	B int    `json:"b"`
}

func newCharRequest(t *testing.T, target, contentType, body string) *Request {
	t.Helper()

	r := httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, strings.NewReader(body))
	if contentType != "" {
		r.Header.Set("Content-Type", contentType)
	}

	return NewRequest(r)
}

// TestRequest_Char_Param pins Param: it reads ONLY the URL query string (never
// the body), returns the FIRST value for a repeated key, and returns "" for a
// key that is absent or whose value is empty.
func TestRequest_Char_Param(t *testing.T) {
	tests := []struct {
		name   string
		target string
		key    string
		want   string
	}{
		{"single", "/x?a=b", "a", "b"},
		{"absent", "/x?a=b", "zzz", ""},
		{"no-query-string", "/x", "a", ""},
		{"empty-value", "/x?a=", "a", ""},
		{"bare-key", "/x?a", "a", ""},
		// A repeated key yields only the first value.
		{"repeated-returns-first", "/x?a=1&a=2&a=3", "a", "1"},
		// Commas are NOT split by Param (unlike Params).
		{"comma-not-split", "/x?a=1,2,3", "a", "1,2,3"},
		{"url-decoded", "/x?a=hello%20world%26", "a", "hello world&"},
		{"plus-is-space", "/x?a=hello+world", "a", "hello world"},
		// Keys are case sensitive.
		{"case-sensitive-key", "/x?Abc=1", "abc", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := NewRequest(httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.target, http.NoBody))

			assert.Equal(t, tc.want, req.Param(tc.key))
		})
	}
}

// TestRequest_Char_Params pins Params: every value for the key, each further
// split on commas. A missing key yields a nil slice (not an empty one).
func TestRequest_Char_Params(t *testing.T) {
	tests := []struct {
		name   string
		target string
		key    string
		want   []string
	}{
		{"single", "/x?a=b", "a", []string{"b"}},
		{"repeated", "/x?a=1&a=2", "a", []string{"1", "2"}},
		{"comma-split", "/x?a=1,2,3", "a", []string{"1", "2", "3"}},
		{"repeated-and-comma-split", "/x?a=1,2&a=3", "a", []string{"1", "2", "3"}},
		// An empty value still produces one empty element, because
		// strings.Split("", ",") returns []string{""}.
		{"empty-value-yields-empty-element", "/x?a=", "a", []string{""}},
		{"bare-key-yields-empty-element", "/x?a", "a", []string{""}},
		// Trailing/leading commas produce empty elements — no trimming.
		{"leading-trailing-commas", "/x?a=,1,", "a", []string{"", "1", ""}},
		{"absent-is-nil", "/x?a=b", "zzz", nil},
		{"no-query-string-is-nil", "/x", "a", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := NewRequest(httptest.NewRequestWithContext(t.Context(), http.MethodGet, tc.target, http.NoBody))

			got := req.Params(tc.key)

			assert.Equal(t, tc.want, got)

			if tc.want == nil {
				assert.Nil(t, got, "a missing key must yield nil, not an empty slice")
			}
		})
	}
}

// TestRequest_Char_PathParam pins PathParam against gorilla/mux vars: present
// keys return their value, everything else returns the empty string (never a
// panic), and lookups are case sensitive.
func TestRequest_Char_PathParam(t *testing.T) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/users/7", http.NoBody)
	r = mux.SetURLVars(r, map[string]string{"id": "7", "Empty": "", "Mixed": "AbC"})

	req := NewRequest(r)

	assert.Equal(t, "7", req.PathParam("id"))
	assert.Empty(t, req.PathParam("Empty"))
	assert.Equal(t, "AbC", req.PathParam("Mixed"))
	assert.Empty(t, req.PathParam("mixed"), "path params are case sensitive")
	assert.Empty(t, req.PathParam("missing"))
	assert.Empty(t, req.PathParam(""))
}

// TestRequest_Char_PathParamNoVars pins that a request never routed through mux
// has a nil pathParams map and every lookup safely returns "".
func TestRequest_Char_PathParamNoVars(t *testing.T) {
	req := NewRequest(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody))

	assert.Nil(t, req.pathParams)
	assert.Empty(t, req.PathParam("anything"))
}

// TestRequest_Char_HostName pins HostName: "<proto>://<Host>", where proto is
// X-Forwarded-Proto when set and "http" otherwise. The header value is trusted
// and echoed verbatim — no allow-list, no validation.
func TestRequest_Char_HostName(t *testing.T) {
	tests := []struct {
		name          string
		host          string
		forwardedProt string
		want          string
	}{
		{"default-proto", "example.com", "", "http://example.com"},
		{"forwarded-https", "example.com", "https", "https://example.com"},
		{"host-with-port", "example.com:8080", "", "http://example.com:8080"},
		// The proto header is echoed verbatim, whatever it says.
		{"arbitrary-proto-echoed", "example.com", "gopher", "gopher://example.com"},
		{"forwarded-proto-list-echoed", "example.com", "https, http", "https, http://example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/x", http.NoBody)
			r.Host = tc.host

			if tc.forwardedProt != "" {
				r.Header.Set("X-Forwarded-Proto", tc.forwardedProt)
			}

			assert.Equal(t, tc.want, NewRequest(r).HostName())
		})
	}
}

// TestRequest_Char_Context pins that Context returns the *same* context
// instance carried by the underlying http.Request.
func TestRequest_Char_Context(t *testing.T) {
	type ctxKey struct{}

	base := context.WithValue(t.Context(), ctxKey{}, "v")
	r := httptest.NewRequestWithContext(base, http.MethodGet, "/x", http.NoBody)

	got := NewRequest(r).Context()

	assert.Equal(t, base, got)
	assert.Equal(t, "v", got.Value(ctxKey{}))
}

// TestRequest_Char_BindJSON pins the JSON binding path, including the exact
// error strings produced by encoding/json for malformed input.
func TestRequest_Char_BindJSON(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantErr     string
		want        charBindTarget
	}{
		{"valid", "application/json", `{"a":"x","b":5}`, "", charBindTarget{A: "x", B: 5}},
		// Parameters are tolerated: the media type is parsed, not string-split.
		{"with-charset", "application/json; charset=utf-8", `{"a":"x"}`, "", charBindTarget{A: "x"}},
		// FIXED (was a silent no-op): the header used to be split on ";" without
		// trimming, so `application/json ; charset=utf-8` yielded "application/json "
		// and matched nothing. mime.ParseMediaType now handles the whitespace.
		{"with-space-before-semicolon", "application/json ; charset=utf-8", `{"a":"x"}`, "", charBindTarget{A: "x"}},
		// Unknown JSON keys are silently ignored.
		{"unknown-keys-ignored", "application/json", `{"a":"x","zz":1}`, "", charBindTarget{A: "x"}},
		// Absent keys leave the target's existing (zero) value untouched.
		{"partial-object", "application/json", `{"b":9}`, "", charBindTarget{B: 9}},
		{"json-null", "application/json", `null`, "", charBindTarget{}},
		{"empty-object", "application/json", `{}`, "", charBindTarget{}},

		// --- malformed input -------------------------------------------------
		{"empty-body", "application/json", ``, "unexpected end of JSON input", charBindTarget{}},
		{"whitespace-only-body", "application/json", "   ", "unexpected end of JSON input", charBindTarget{}},
		{"truncated", "application/json", `{"a":`, "unexpected end of JSON input", charBindTarget{}},
		{
			"not-json", "application/json", `hello`,
			"invalid character 'h' looking for beginning of value", charBindTarget{},
		},
		{
			"unquoted-key", "application/json", `{a:1}`,
			"invalid character 'a' looking for beginning of object key string", charBindTarget{},
		},
		{
			"trailing-garbage", "application/json", `{"a":"x"} junk`,
			"invalid character 'j' after top-level value", charBindTarget{},
		},

		// --- type mismatches --------------------------------------------------
		// NOTE: on a type mismatch encoding/json still populates the fields it
		// COULD decode, so the target is left partially bound alongside the error.
		{
			"wrong-type-for-string", "application/json", `{"a":5}`,
			"json: cannot unmarshal number into Go struct field charBindTarget.a of type string",
			charBindTarget{},
		},
		{
			"wrong-type-for-int", "application/json", `{"a":"x","b":"nope"}`,
			"json: cannot unmarshal string into Go struct field charBindTarget.b of type int",
			charBindTarget{A: "x"},
		},
		{
			"array-instead-of-object", "application/json", `[1,2]`,
			"json: cannot unmarshal array into Go value of type http.charBindTarget",
			charBindTarget{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := newCharRequest(t, "/x", tc.contentType, tc.body)

			var got charBindTarget

			err := req.Bind(&got)

			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, tc.wantErr, err.Error())
			}

			assert.Equal(t, tc.want, got)
		})
	}
}

// TestRequest_Char_BindNonPointerErrors pins the fix for a sharp edge: Bind
// takes `any` and unmarshals into `&i`, so a non-pointer target used to bind
// into a throwaway copy of the interface and a typo like `c.Bind(target)` failed
// completely silently. It now reports errNonPointerBind for every content type,
// matching what the form/multipart paths already did.
func TestRequest_Char_BindNonPointerErrors(t *testing.T) {
	for _, ct := range []string{"application/json", "binary/octet-stream", "text/plain", ""} {
		t.Run("ct="+ct, func(t *testing.T) {
			req := newCharRequest(t, "/x", ct, `{"a":"x","b":5}`)

			target := charBindTarget{}

			err := req.Bind(target)

			require.ErrorIs(t, err, errNonPointerBind)
			assert.Equal(t, "bind error, cannot bind to a non pointer type", err.Error())
			assert.Equal(t, charBindTarget{}, target, "the caller's value is still left untouched")
		})
	}
}

// TestRequest_Char_BindJSONIntoNonStructTargets pins binding into the
// non-struct pointer targets a handler may reasonably use.
func TestRequest_Char_BindJSONIntoNonStructTargets(t *testing.T) {
	t.Run("map", func(t *testing.T) {
		req := newCharRequest(t, "/x", "application/json", `{"a":1,"b":"s"}`)

		var got map[string]any

		require.NoError(t, req.Bind(&got))
		assert.Equal(t, map[string]any{"a": float64(1), "b": "s"}, got)
	})

	t.Run("slice", func(t *testing.T) {
		req := newCharRequest(t, "/x", "application/json", `[1,2,3]`)

		var got []int

		require.NoError(t, req.Bind(&got))
		assert.Equal(t, []int{1, 2, 3}, got)
	})

	t.Run("string", func(t *testing.T) {
		req := newCharRequest(t, "/x", "application/json", `"hello"`)

		var got string

		require.NoError(t, req.Bind(&got))
		assert.Equal(t, "hello", got)
	})

	t.Run("any", func(t *testing.T) {
		req := newCharRequest(t, "/x", "application/json", `{"a":1}`)

		var got any

		require.NoError(t, req.Bind(&got))
		assert.Equal(t, map[string]any{"a": float64(1)}, got)
	})
}

// TestRequest_Char_BindUnhandledContentTypes pins that a body Bind cannot decode
// is REPORTED rather than discarded. Returning nil here left the caller's target
// all-zero with no failure — the same silent no-op the non-pointer check rejects.
// A request with no body is the documented exception, covered separately below.
func TestRequest_Char_BindUnhandledContentTypes(t *testing.T) {
	for _, ct := range []string{
		"",
		"text/plain",
		"application/xml",
		"application/jsonx", // no prefix matching
		"text/json",
	} {
		t.Run("ct="+ct, func(t *testing.T) {
			req := newCharRequest(t, "/x", ct, `{"a":"x","b":5}`)

			var got charBindTarget

			err := req.Bind(&got)

			// A body that cannot be decoded is reported. Leaving the target
			// zeroed and returning nil was the same silent no-op the
			// non-pointer check rejects, and the likelier one in practice.
			require.ErrorIs(t, err, errUnsupportedContentType)
			assert.Equal(t, charBindTarget{}, got, "nothing is bound from an unhandled content type")
		})
	}
}

// TestRequest_Char_BindUnhandledContentTypeWithoutBody pins the carve-out: with
// no body there is nothing to decode and nothing lost, so binding an unhandled
// content type stays a no-op rather than erroring. This keeps handlers that Bind
// defensively on bodyless requests working.
func TestRequest_Char_BindUnhandledContentTypeWithoutBody(t *testing.T) {
	for _, ct := range []string{"", "text/plain", "application/xml"} {
		t.Run("ct="+ct, func(t *testing.T) {
			req := newCharRequest(t, "/x", ct, "")

			var got charBindTarget

			require.NoError(t, req.Bind(&got))
			assert.Equal(t, charBindTarget{}, got)
		})
	}
}

// TestRequest_Char_BindContentTypeIsNormalized pins the fix for case-sensitive,
// untrimmed content-type matching. The case- and whitespace-variant spellings
// below used to fall through to the unhandled branch and silently leave the
// target all-zero (the exactly-canonical ones always worked); the media type is
// now parsed per RFC 9110, so case and surrounding whitespace are irrelevant.
func TestRequest_Char_BindContentTypeIsNormalized(t *testing.T) {
	for _, ct := range []string{
		"application/json",
		"application/JSON",
		"Application/json",
		"APPLICATION/JSON",
		"application/json ",
		" application/json",
		"application/json;charset=utf-8",
		"application/json ; charset=UTF-8",
	} {
		t.Run("ct="+ct, func(t *testing.T) {
			req := newCharRequest(t, "/x", ct, `{"a":"x","b":5}`)

			var got charBindTarget

			require.NoError(t, req.Bind(&got))
			assert.Equal(t, charBindTarget{A: "x", B: 5}, got)
		})
	}

	// The same normalization applies to the form path.
	t.Run("form-urlencoded-trailing-space", func(t *testing.T) {
		type target struct{ Name string }

		req := newCharRequest(t, "/x", "application/x-www-form-urlencoded ", "Name=alice")

		var got target

		require.NoError(t, req.Bind(&got))
		assert.Equal(t, target{Name: "alice"}, got)
	})
}

// TestRequest_Char_BindFormURLEncoded pins the form-urlencoded path, including
// the exact sentinel errors.
func TestRequest_Char_BindFormURLEncoded(t *testing.T) {
	type target struct {
		Name string
		Age  int
		OK   bool
	}

	t.Run("valid", func(t *testing.T) {
		req := newCharRequest(t, "/x",
			"application/x-www-form-urlencoded", "Name=alice&Age=30&OK=true")

		var got target

		require.NoError(t, req.Bind(&got))
		assert.Equal(t, target{Name: "alice", Age: 30, OK: true}, got)
	})

	// Top-level form keys are matched against the exact Go field name (or the
	// `form`/`file` tag) — the match is CASE SENSITIVE, unlike the nested
	// struct-string parser in setStructValue.
	t.Run("field-names-are-case-sensitive", func(t *testing.T) {
		req := newCharRequest(t, "/x",
			"application/x-www-form-urlencoded", "name=alice&AGE=30")

		var got target

		require.ErrorIs(t, req.Bind(&got), errFieldsNotSet)
		assert.Equal(t, target{}, got)
	})

	t.Run("form-tag-overrides-field-name", func(t *testing.T) {
		type tagged struct {
			Name string `form:"user_name"`
		}

		req := newCharRequest(t, "/x",
			"application/x-www-form-urlencoded", "user_name=alice&Name=bob")

		var got tagged

		require.NoError(t, req.Bind(&got))
		assert.Equal(t, tagged{Name: "alice"}, got)
	})

	t.Run("no-matching-field-returns-errFieldsNotSet", func(t *testing.T) {
		req := newCharRequest(t, "/x",
			"application/x-www-form-urlencoded", "unknown=1")

		var got target

		err := req.Bind(&got)

		require.ErrorIs(t, err, errFieldsNotSet)
		assert.Equal(t, target{}, got)
	})

	t.Run("empty-body-returns-errFieldsNotSet", func(t *testing.T) {
		req := newCharRequest(t, "/x", "application/x-www-form-urlencoded", "")

		var got target

		require.ErrorIs(t, req.Bind(&got), errFieldsNotSet)
	})
}

// TestRequest_Char_BindFormURLEncodedErrors pins the form-urlencoded failure
// modes and the query-string leak.
func TestRequest_Char_BindFormURLEncodedErrors(t *testing.T) {
	type target struct {
		Name string
		Age  int
		OK   bool
	}

	t.Run("non-pointer-target-returns-errNonPointerBind", func(t *testing.T) {
		req := newCharRequest(t, "/x",
			"application/x-www-form-urlencoded", "Name=alice")

		err := req.Bind(target{})

		require.ErrorIs(t, err, errNonPointerBind)
		assert.Equal(t, "bind error, cannot bind to a non pointer type", err.Error())
	})

	t.Run("malformed-escape-returns-parse-error", func(t *testing.T) {
		req := newCharRequest(t, "/x",
			"application/x-www-form-urlencoded", "Name=%zz")

		var got target

		err := req.Bind(&got)

		require.Error(t, err)
		assert.Equal(t, `invalid URL escape "%zz"`, err.Error())
	})

	// SHARP EDGE (pinned as-is): ParseForm merges the URL query into r.Form, so
	// a query parameter can populate a body-bound field. A client can therefore
	// set form fields via the URL even when the body does not mention them.
	t.Run("query-string-leaks-into-form-binding", func(t *testing.T) {
		req := newCharRequest(t, "/x?Age=99",
			"application/x-www-form-urlencoded", "Name=alice")

		var got target

		require.NoError(t, req.Bind(&got))
		assert.Equal(t, target{Name: "alice", Age: 99}, got)
	})

	// The raw strconv error is surfaced verbatim to the handler: it names no
	// field, so the caller cannot tell WHICH input was bad.
	t.Run("type-mismatch-surfaces-raw-strconv-error", func(t *testing.T) {
		req := newCharRequest(t, "/x",
			"application/x-www-form-urlencoded", "Name=alice&Age=notanumber")

		var got target

		err := req.Bind(&got)

		require.Error(t, err)
		assert.Equal(t, `strconv.ParseInt: parsing "notanumber": invalid syntax`, err.Error())
	})
}

// TestRequest_Char_BindMultipartErrors pins the multipart failure modes.
func TestRequest_Char_BindMultipartErrors(t *testing.T) {
	type target struct {
		Name string
	}

	t.Run("non-pointer-target-returns-errNonPointerBind", func(t *testing.T) {
		req := newCharRequest(t, "/x", "multipart/form-data; boundary=xx", "")

		require.ErrorIs(t, req.Bind(target{}), errNonPointerBind)
	})

	t.Run("missing-boundary-returns-parse-error", func(t *testing.T) {
		req := newCharRequest(t, "/x", "multipart/form-data", "")

		var got target

		err := req.Bind(&got)

		require.Error(t, err)
		assert.Equal(t, "no multipart boundary param in Content-Type", err.Error())
	})

	t.Run("no-matching-field-returns-errNoFileFound", func(t *testing.T) {
		body := "--xx\r\nContent-Disposition: form-data; name=\"other\"\r\n\r\nv\r\n--xx--\r\n"
		req := newCharRequest(t, "/x", "multipart/form-data; boundary=xx", body)

		var got target

		err := req.Bind(&got)

		require.ErrorIs(t, err, errNoFileFound)
		assert.Equal(t, "no files were bounded", err.Error())
	})

	t.Run("valid-text-field-binds", func(t *testing.T) {
		body := "--xx\r\nContent-Disposition: form-data; name=\"Name\"\r\n\r\nalice\r\n--xx--\r\n"
		req := newCharRequest(t, "/x", "multipart/form-data; boundary=xx", body)

		var got target

		require.NoError(t, req.Bind(&got))
		assert.Equal(t, target{Name: "alice"}, got)
	})
}

// TestRequest_Char_BindBinary pins the binary/octet-stream path and its exact
// error for a target that is not a *[]byte.
func TestRequest_Char_BindBinary(t *testing.T) {
	t.Run("binds-raw-bytes", func(t *testing.T) {
		req := newCharRequest(t, "/x", "binary/octet-stream", "\x00\x01raw")

		var got []byte

		require.NoError(t, req.Bind(&got))
		assert.Equal(t, []byte("\x00\x01raw"), got)
	})

	t.Run("empty-body-binds-empty-slice", func(t *testing.T) {
		req := newCharRequest(t, "/x", "binary/octet-stream", "")

		var got []byte

		require.NoError(t, req.Bind(&got))
		assert.Empty(t, got)
	})

	t.Run("wrong-target-type-error-message", func(t *testing.T) {
		req := newCharRequest(t, "/x", "binary/octet-stream", "raw")

		var got string

		err := req.Bind(&got)

		require.ErrorIs(t, err, errNonSliceBind)
		// The message interpolates the target with %v, which for a pointer is a
		// non-deterministic address — so only the stable prefix is pinned.
		assert.True(t, strings.HasPrefix(err.Error(),
			"bind error: input is not a pointer to a byte slice: 0x"), err.Error())
	})
}

// TestRequest_Char_BodyIsReplayable pins that body() restores r.Body, so Bind
// can be called more than once and downstream readers still see the payload.
func TestRequest_Char_BodyIsReplayable(t *testing.T) {
	req := newCharRequest(t, "/x", "application/json", `{"a":"x","b":5}`)

	var first, second charBindTarget

	require.NoError(t, req.Bind(&first))
	require.NoError(t, req.Bind(&second))

	assert.Equal(t, charBindTarget{A: "x", B: 5}, first)
	assert.Equal(t, charBindTarget{A: "x", B: 5}, second)

	// The raw body is still readable afterwards.
	rest, err := io.ReadAll(req.req.Body)
	require.NoError(t, err)
	//nolint:testifylint // exact bytes are the contract.
	assert.Equal(t, `{"a":"x","b":5}`, string(rest))
}

// TestRequest_Char_BindNilBodyDoesNotPanic pins that a request built without a
// body — the standard handler unit-test construction, where http.NewRequest
// leaves Body nil — never panics.
//
// io.ReadAll(nil) panics, so every path that reads the body has to tolerate an
// absent one. A decodable content type reports an ordinary decode error and one
// with no decoder stays a no-op, matching the documented "no body, nothing
// lost" rule.
func TestRequest_Char_BindNilBodyDoesNotPanic(t *testing.T) {
	for _, tc := range []struct {
		contentType string
		wantErr     bool
	}{
		{"application/json", true}, // empty input is a JSON decode error
		{"text/plain", false},      // no decoder, no body: nothing to do
		{"", false},
	} {
		t.Run("ct="+tc.contentType, func(t *testing.T) {
			// nil, not http.NoBody: a nil Body is exactly what this pins.
			//nolint:gocritic // httpNoBody — the nil body IS the case under test.
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://dummy/x", nil)
			require.NoError(t, err)
			require.Nil(t, req.Body, "precondition: the request carries no body")

			if tc.contentType != "" {
				req.Header.Set("Content-Type", tc.contentType)
			}

			var got charBindTarget

			require.NotPanics(t, func() {
				bindErr := NewRequest(req).Bind(&got)
				if tc.wantErr {
					require.Error(t, bindErr)
				} else {
					require.NoError(t, bindErr)
				}
			})

			assert.Equal(t, charBindTarget{}, got)
		})
	}
}
