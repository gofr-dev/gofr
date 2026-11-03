package s3

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/datasource/file"
)

// testLogger is a datasource.Logger that records the format of every error line.
type testLogger struct {
	mu     sync.Mutex
	errors []string
}

func (l *testLogger) recordError(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.errors = append(l.errors, msg)
}

func (*testLogger) Debug(...any)          {}
func (*testLogger) Debugf(string, ...any) {}
func (*testLogger) Info(...any)           {}
func (*testLogger) Infof(string, ...any)  {}
func (*testLogger) Warn(...any)           {}
func (*testLogger) Warnf(string, ...any)  {}
func (l *testLogger) Error(...any)        { l.recordError("error") }
func (l *testLogger) Errorf(f string, _ ...any) {
	l.recordError(f)
}

func (l *testLogger) errorLines() []string {
	l.mu.Lock()
	defer l.mu.Unlock()

	return append([]string(nil), l.errors...)
}

// testMetrics records histogram creation and every recorded sample's labels.
type testMetrics struct {
	mu         sync.Mutex
	histograms []string
	samples    [][]string
}

func (m *testMetrics) NewHistogram(name, _ string, _ ...float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.histograms = append(m.histograms, name)
}

func (m *testMetrics) RecordHistogram(_ context.Context, name string, _ float64, labels ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.samples = append(m.samples, append([]string{name}, labels...))
}

func (m *testMetrics) registered() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]string(nil), m.histograms...)
}

func (m *testMetrics) hasSample(labels ...string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, s := range m.samples {
		if containsAll(s, labels) {
			return true
		}
	}

	return false
}

func containsAll(have, want []string) bool {
	for _, w := range want {
		found := false

		for _, h := range have {
			if h == w {
				found = true
				break
			}
		}

		if !found {
			return false
		}
	}

	return true
}

// memS3 is a minimal in-memory S3 (path-style), enough for end-to-end tests.
type memS3 struct {
	mu   sync.Mutex
	objs map[string][]byte
}

func newMemS3(t *testing.T) (*memS3, *fakeS3) {
	t.Helper()

	m := &memS3{objs: map[string][]byte{}}

	return m, newFakeS3(t, m.serve)
}

func (m *memS3) keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	keys := make([]string, 0, len(m.objs))
	for k := range m.objs {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	return keys
}

func (m *memS3) put(key, body string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.objs[key] = []byte(body)
}

func (m *memS3) body(key string) string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return string(m.objs[key])
}

type listContent struct {
	Key  string `xml:"Key"`
	Size int    `xml:"Size"`
}

type listPrefix struct {
	Prefix string `xml:"Prefix"`
}

type listResult struct {
	XMLName  xml.Name      `xml:"ListBucketResult"`
	Contents []listContent `xml:"Contents"`
	Prefixes []listPrefix  `xml:"CommonPrefixes"`
}

func (m *memS3) serve(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := strings.TrimPrefix(strings.TrimPrefix(r.URL.Path, "/"+testBucket), "/")

	switch {
	case key == "" && r.Method == http.MethodHead:
		w.WriteHeader(http.StatusOK)
	case key == "" && r.Method == http.MethodGet:
		m.list(w, r)
	case r.Method == http.MethodPut && r.Header.Get("X-Amz-Copy-Source") != "":
		src := strings.TrimPrefix(r.Header.Get("X-Amz-Copy-Source"), testBucket+"/")
		body, ok := m.objs[src]

		if !ok {
			writeS3Error(w, http.StatusNotFound, "NoSuchKey")
			return
		}

		m.objs[key] = body
		_, _ = io.WriteString(w, `<CopyObjectResult><ETag>"x"</ETag></CopyObjectResult>`)
	case r.Method == http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		m.objs[key] = body
	case r.Method == http.MethodDelete:
		delete(m.objs, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		m.object(w, r, key)
	}
}

func (m *memS3) object(w http.ResponseWriter, r *http.Request, key string) {
	body, ok := m.objs[key]
	if !ok {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		writeS3Error(w, http.StatusNotFound, "NoSuchKey")

		return
	}

	w.Header().Set("Last-Modified", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Format(http.TimeFormat))

	rng := r.Header.Get("Range")
	if rng == "" || r.Method != http.MethodGet {
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))

		if r.Method == http.MethodGet {
			_, _ = w.Write(body)
		}

		return
	}

	parts := strings.SplitN(strings.TrimPrefix(rng, "bytes="), "-", 2)
	start, _ := strconv.Atoi(parts[0])
	end, _ := strconv.Atoi(parts[1])

	if start >= len(body) {
		writeS3Error(w, http.StatusRequestedRangeNotSatisfiable, "InvalidRange")
		return
	}

	if end >= len(body) {
		end = len(body) - 1
	}

	w.Header().Set("Content-Range", "bytes "+strconv.Itoa(start)+"-"+strconv.Itoa(end)+"/"+strconv.Itoa(len(body)))
	w.Header().Set("Content-Length", strconv.Itoa(end-start+1))
	w.WriteHeader(http.StatusPartialContent)
	_, _ = w.Write(body[start : end+1])
}

func (m *memS3) list(w http.ResponseWriter, r *http.Request) {
	prefix := r.URL.Query().Get("prefix")
	delimiter := r.URL.Query().Get("delimiter")

	var res listResult

	seen := map[string]bool{}

	keys := make([]string, 0, len(m.objs))
	for k := range m.objs {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		if !strings.HasPrefix(k, prefix) {
			continue
		}

		rest := strings.TrimPrefix(k, prefix)

		if i := strings.Index(rest, delimiter); delimiter != "" && i >= 0 {
			p := prefix + rest[:i+1]
			if !seen[p] {
				seen[p] = true

				res.Prefixes = append(res.Prefixes, listPrefix{Prefix: p})
			}

			continue
		}

		res.Contents = append(res.Contents, listContent{Key: k, Size: len(m.objs[k])})
	}

	w.Header().Set("Content-Type", "application/xml")

	out, _ := xml.Marshal(res)
	_, _ = w.Write(out)
}

func testConfig(endpoint string) *Config {
	return &Config{
		EndPoint: endpoint, BucketName: testBucket, Region: "us-east-1",
		AccessKeyID: "AKID", SecretAccessKey: "secret", Flavor: FlavorMinIO,
	}
}

// newConnectedFS returns a connected S3 file system backed by an in-memory fake.
func newConnectedFS(t *testing.T) (*fileSystem, *memS3, *testMetrics) {
	t.Helper()

	mem, srv := newMemS3(t)
	metrics := &testMetrics{}

	fs := New(testConfig(srv.URL)).(*fileSystem)
	fs.UseLogger(&testLogger{})
	fs.UseMetrics(metrics)
	fs.Connect()

	require.True(t, fs.IsConnected(), "fake S3 must accept the connection")

	return fs, mem, metrics
}

func TestNew(t *testing.T) {
	tests := []struct {
		name       string
		config     *Config
		wantBucket string
	}{
		{name: "nil config is safe", config: nil, wantBucket: ""},
		{name: "bucket becomes the location", config: &Config{BucketName: "my-bucket"}, wantBucket: "my-bucket"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := New(tt.config)

			require.NotNil(t, fs)

			impl, ok := fs.(*fileSystem)
			require.True(t, ok)
			assert.Equal(t, tt.wantBucket, impl.Location)
			assert.Equal(t, "S3", impl.ProviderName)
			assert.NotNil(t, impl.adapter)
		})
	}
}

func TestFileSystem_UseLogger(t *testing.T) {
	tests := []struct {
		name       string
		logger     any
		wantLogger bool
	}{
		{name: "valid logger reaches fs and adapter", logger: &testLogger{}, wantLogger: true},
		{name: "wrong type is ignored", logger: "not a logger", wantLogger: false},
		{name: "nil is ignored", logger: nil, wantLogger: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := New(&Config{}).(*fileSystem)

			fs.UseLogger(tt.logger)

			assert.Equal(t, tt.wantLogger, fs.Logger != nil)
			assert.Equal(t, tt.wantLogger, fs.adapter.logger != nil)
		})
	}
}

func TestFileSystem_UseMetrics(t *testing.T) {
	tests := []struct {
		name        string
		metrics     any
		wantMetrics bool
	}{
		{name: "valid metrics", metrics: &testMetrics{}, wantMetrics: true},
		{name: "wrong type is ignored", metrics: 42, wantMetrics: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs := New(&Config{}).(*fileSystem)

			fs.UseMetrics(tt.metrics)

			assert.Equal(t, tt.wantMetrics, fs.Metrics != nil)
		})
	}
}

func TestFileSystem_Connect(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		wantConnected bool
		wantRetryLog  bool
	}{
		{name: "reachable bucket connects", status: http.StatusOK, wantConnected: true},
		{name: "failure starts the background retry", status: http.StatusForbidden, wantRetryLog: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newFakeS3(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tt.status) })
			logger, metrics := &testLogger{}, &testMetrics{}

			fs := New(testConfig(srv.URL)).(*fileSystem)
			fs.UseLogger(logger)
			fs.UseMetrics(metrics)

			// Stops the retry goroutine at its first tick so it does not outlive the test.
			defer fs.SetDisableRetry(true)

			fs.Connect()

			assert.Equal(t, tt.wantConnected, fs.IsConnected())
			assert.Equal(t, []string{file.AppFileStats}, metrics.registered(), "histogram registered once")
			assert.True(t, metrics.hasSample(file.AppFileStats, "S3"))

			lines := logger.errorLines()

			if tt.wantRetryLog {
				require.NotEmpty(t, lines)
				assert.Contains(t, lines[0], "starting background retry")
			} else {
				assert.Empty(t, lines)
			}
		})
	}
}

func TestFileSystem_Connect_Idempotent(t *testing.T) {
	fs, _, _ := newConnectedFS(t)

	fs.Connect()

	assert.True(t, fs.IsConnected())
}

func TestFileSystem_Read(t *testing.T) {
	tests := []struct {
		name    string
		content string
		bufLen  int
		wantN   int
	}{
		{name: "buffer larger than object reports n and EOF", content: "hello", bufLen: 10, wantN: 5},
		{name: "buffer smaller than object", content: "hello", bufLen: 3, wantN: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs, mem, _ := newConnectedFS(t)
			mem.put("dir/a.txt", tt.content)

			f, err := fs.Open("dir/a.txt")
			require.NoError(t, err)

			defer f.Close()

			buf := make([]byte, tt.bufLen)
			n, err := f.Read(buf)

			assert.Equal(t, tt.wantN, n)
			assert.Equal(t, tt.content[:tt.wantN], string(buf[:n]))

			if err != nil {
				require.ErrorIs(t, err, io.EOF)
			}

			rest, err := io.ReadAll(f)
			require.NoError(t, err)
			assert.Equal(t, tt.content[tt.wantN:], string(rest))
		})
	}
}

func TestFileSystem_Create(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		content string
	}{
		{name: "fresh prefix needs no parent (3805 regression)", key: "fresh/prefix/a.txt", content: "payload"},
		{name: "top level key", key: "top.json", content: `{"a":1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs, mem, _ := newConnectedFS(t)

			f, err := fs.Create(tt.key)
			require.NoError(t, err)

			_, err = f.Write([]byte(tt.content))
			require.NoError(t, err)
			require.NoError(t, f.Close())

			assert.Equal(t, []string{tt.key}, mem.keys())
			assert.Equal(t, tt.content, mem.body(tt.key))
		})
	}
}

func TestFileSystem_CreateWithOptions(t *testing.T) {
	tests := []struct {
		name string
		opts *file.FileOptions
	}{
		{name: "nil options (3805 regression)", opts: nil},
		{name: "with metadata", opts: &file.FileOptions{ContentType: "text/csv", Metadata: map[string]string{"k": "v"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs, mem, _ := newConnectedFS(t)

			f, err := fs.CreateWithOptions(context.Background(), "out/a.csv", tt.opts)
			require.NoError(t, err)
			require.NoError(t, f.Close())

			assert.Equal(t, []string{"out/a.csv"}, mem.keys())
		})
	}
}

func TestFileSystem_Stat(t *testing.T) {
	tests := []struct {
		name      string
		target    string
		wantDir   bool
		wantSize  int64
		wantErr   error
		wantLabel string
	}{
		{name: "file", target: "dir/a.txt", wantSize: 5},
		{name: "extensionless folder with keys under it", target: "dir", wantDir: true},
		{name: "folder holding only subfolders", target: "top", wantDir: true},
		{name: "missing object is NOT_FOUND", target: "nope.txt", wantErr: file.ErrFileNotFound, wantLabel: file.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs, mem, metrics := newConnectedFS(t)
			mem.put("dir/a.txt", "hello")
			mem.put("top/sub/b.txt", "x")

			info, err := fs.Stat(tt.target)

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.True(t, metrics.hasSample(file.AppFileStats, tt.wantLabel))

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantDir, info.IsDir())
			assert.Equal(t, tt.wantSize, info.Size())
		})
	}
}

func TestFileSystem_ReadDir(t *testing.T) {
	fs, mem, _ := newConnectedFS(t)
	mem.put("d/a.txt", "1")
	mem.put("d/sub/b.txt", "2")

	entries, err := fs.ReadDir("d/")

	require.NoError(t, err)

	got := map[string]bool{}
	for _, e := range entries {
		got[e.Name()] = e.IsDir()
	}

	assert.Equal(t, map[string]bool{"a.txt": false, "sub": true}, got)
}

func TestFileSystem_Rename(t *testing.T) {
	tests := []struct {
		name     string
		from, to string
		wantKeys []string
	}{
		{name: "same folder", from: "d/a.txt", to: "d/b.txt", wantKeys: []string{"d/b.txt"}},
		{name: "across folders and extensions", from: "d/a.txt", to: "e/f/a.csv", wantKeys: []string{"e/f/a.csv"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fs, mem, _ := newConnectedFS(t)
			mem.put("d/a.txt", "1")

			require.NoError(t, fs.Rename(tt.from, tt.to))
			assert.Equal(t, tt.wantKeys, mem.keys())
		})
	}
}

func TestFileSystem_Remove(t *testing.T) {
	fs, mem, _ := newConnectedFS(t)
	mem.put("d/a.txt", "1")

	require.NoError(t, fs.Remove("d/a.txt"))
	assert.Empty(t, mem.keys())
}

func TestFileSystem_Getwd(t *testing.T) {
	fs, _, _ := newConnectedFS(t)

	wd, err := fs.Getwd()

	require.NoError(t, err)
	assert.Equal(t, testBucket, wd)
}

func TestFileSystem_GenerateSignedURL(t *testing.T) {
	fs, _, _ := newConnectedFS(t)

	got, err := fs.GenerateSignedURL(context.Background(), "d/a.txt", time.Hour, nil)

	require.NoError(t, err)
	assert.Contains(t, got, "/"+testBucket+"/d/a.txt")
	assert.Contains(t, got, "X-Amz-Signature=")
}

// TestFileSystem_ReadAt_Parallel reads one open file from 32 goroutines at once; run with -race.
func TestFileSystem_ReadAt_Parallel(t *testing.T) {
	const workers = 32

	fs, mem, _ := newConnectedFS(t)

	var content strings.Builder

	for i := range workers {
		content.WriteString(strings.Repeat(string(rune('a'+i%26)), 8))
	}

	mem.put("big.bin", content.String())

	f, err := fs.Open("big.bin")
	require.NoError(t, err)

	defer f.Close()

	var wg sync.WaitGroup

	errs := make(chan error, workers)

	for i := range workers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			buf := make([]byte, 8)
			want := strings.Repeat(string(rune('a'+i%26)), 8)

			n, err := f.ReadAt(buf, int64(i*8))
			if err != nil && !errors.Is(err, io.EOF) {
				errs <- err
				return
			}

			if n != 8 || string(buf) != want {
				errs <- fmt.Errorf("%w at offset %d: %q", errWrongBytes, i*8, buf[:n])
			}
		}()
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}
}

var errWrongBytes = errors.New("wrong bytes")
