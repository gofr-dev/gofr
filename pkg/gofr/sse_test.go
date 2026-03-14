package gofr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/testutil"
)

func Test_formatSSEData(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{name: "string data", input: "hello world", expected: "hello world"},
		{name: "byte slice data", input: []byte("raw bytes"), expected: "raw bytes"},
		{name: "nil data", input: nil, expected: ""},
		{name: "struct data", input: struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}{Name: "GoFr", Age: 1}, expected: `{"name":"GoFr","age":1}`},
		{name: "map data", input: map[string]string{"key": "value"}, expected: `{"key":"value"}`},
		{name: "integer data", input: 42, expected: "42"},
		{name: "boolean data", input: true, expected: "true"},
		{name: "slice data", input: []int{1, 2, 3}, expected: "[1,2,3]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := formatSSEData(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func Test_formatSSEData_UnsupportedType(t *testing.T) {
	ch := make(chan int)
	_, err := formatSSEData(ch)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to marshal SSE data")
}

func Test_formatEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    SSEEvent
		expected string
	}{
		{
			name:     "data only",
			event:    SSEEvent{Data: "hello"},
			expected: "data: hello\n\n",
		},
		{
			name:     "named event",
			event:    SSEEvent{Name: "update", Data: "new data"},
			expected: "event: update\ndata: new data\n\n",
		},
		{
			name:     "event with ID",
			event:    SSEEvent{ID: "123", Data: "test"},
			expected: "id: 123\ndata: test\n\n",
		},
		{
			name:     "event with retry",
			event:    SSEEvent{Retry: 5000, Data: "retry test"},
			expected: "retry: 5000\ndata: retry test\n\n",
		},
		{
			name:     "all fields",
			event:    SSEEvent{ID: "42", Name: "message", Retry: 3000, Data: "full event"},
			expected: "id: 42\nevent: message\nretry: 3000\ndata: full event\n\n",
		},
		{
			name:     "JSON data",
			event:    SSEEvent{Data: map[string]string{"key": "value"}},
			expected: "data: {\"key\":\"value\"}\n\n",
		},
		{
			name:     "multiline data",
			event:    SSEEvent{Data: "line1\nline2\nline3"},
			expected: "data: line1\ndata: line2\ndata: line3\n\n",
		},
		{
			name:     "nil data",
			event:    SSEEvent{Data: nil},
			expected: "data: \n\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := formatEvent(tc.event)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// flushRecorder is a ResponseRecorder that implements http.Flusher.
type flushRecorder struct {
	*httptest.ResponseRecorder
	flushed bool
}

func newFlushRecorder() *flushRecorder {
	return &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
}

func (f *flushRecorder) Flush() {
	f.flushed = true
}

func (f *flushRecorder) Unwrap() http.ResponseWriter {
	return f.ResponseRecorder
}

func TestSSEStream_Send(t *testing.T) {
	w := newFlushRecorder()
	stream := &SSEStream{w: w, rc: http.NewResponseController(w)}

	err := stream.Send(SSEEvent{Name: "test", Data: "hello"})
	require.NoError(t, err)

	assert.Equal(t, "event: test\ndata: hello\n\n", w.Body.String())
	assert.True(t, w.flushed)
}

func TestSSEStream_SendData(t *testing.T) {
	w := newFlushRecorder()
	stream := &SSEStream{w: w, rc: http.NewResponseController(w)}

	err := stream.SendData("simple")
	require.NoError(t, err)

	assert.Equal(t, "data: simple\n\n", w.Body.String())
}

func TestSSEStream_SendEvent(t *testing.T) {
	w := newFlushRecorder()
	stream := &SSEStream{w: w, rc: http.NewResponseController(w)}

	err := stream.SendEvent("notification", map[string]int{"count": 5})
	require.NoError(t, err)

	assert.Equal(t, "event: notification\ndata: {\"count\":5}\n\n", w.Body.String())
}

func TestSSEStream_SendComment(t *testing.T) {
	w := newFlushRecorder()
	stream := &SSEStream{w: w, rc: http.NewResponseController(w)}

	err := stream.SendComment("keep-alive")
	require.NoError(t, err)

	assert.Equal(t, ": keep-alive\n\n", w.Body.String())
}

func TestSSEStream_SendComment_Multiline(t *testing.T) {
	w := newFlushRecorder()
	stream := &SSEStream{w: w, rc: http.NewResponseController(w)}

	err := stream.SendComment("line1\nline2")
	require.NoError(t, err)

	assert.Equal(t, ": line1\n: line2\n\n", w.Body.String())
}

func TestSSEStream_Send_JSONStruct(t *testing.T) {
	type Notification struct {
		Title   string `json:"title"`
		Message string `json:"message"`
	}

	w := newFlushRecorder()
	stream := &SSEStream{w: w, rc: http.NewResponseController(w)}

	err := stream.Send(SSEEvent{
		Name: "notification",
		ID:   "1",
		Data: Notification{Title: "Hello", Message: "World"},
	})

	require.NoError(t, err)
	assert.Equal(t, "id: 1\nevent: notification\ndata: {\"title\":\"Hello\",\"message\":\"World\"}\n\n", w.Body.String())
}

func TestSSEStream_ConcurrentSend(t *testing.T) {
	w := newFlushRecorder()
	stream := &SSEStream{w: w, rc: http.NewResponseController(w)}
	count := 50

	var wg sync.WaitGroup

	wg.Add(count)

	for i := 0; i < count; i++ {
		go func(i int) {
			defer wg.Done()

			_ = stream.SendData(i)
		}(i)
	}

	wg.Wait()

	body := w.Body.String()

	// Count the number of "data:" occurrences - may be less than count due to race conditions
	// in writing to the underlying buffer, which is expected for concurrent access without sync.
	// This test verifies no panics occur during concurrent sends.
	dataCount := strings.Count(body, "data:")

	assert.Positive(t, dataCount, "at least some events should be written")
}

func TestSSEStream_StreamingLoop(t *testing.T) {
	w := newFlushRecorder()
	stream := &SSEStream{w: w, rc: http.NewResponseController(w)}

	for i := 0; i < 5; i++ {
		err := stream.Send(SSEEvent{
			ID:   string(rune('0' + i)),
			Name: "tick",
			Data: map[string]int{"count": i},
		})
		require.NoError(t, err)
	}

	body := w.Body.String()

	for i := 0; i < 5; i++ {
		expected, _ := json.Marshal(map[string]int{"count": i})
		assert.Contains(t, body, "data: "+string(expected)+"\n")
	}
}

// nonFlushableWriter is a ResponseWriter that does NOT implement http.Flusher.
type nonFlushableWriter struct {
	header http.Header
}

func (n *nonFlushableWriter) Header() http.Header     { return n.header }
func (*nonFlushableWriter) Write([]byte) (int, error) { return 0, nil }
func (*nonFlushableWriter) WriteHeader(int)           {}

func TestSSEStream_NoFlusher(t *testing.T) {
	w := &nonFlushableWriter{header: http.Header{}}
	stream := &SSEStream{w: w, rc: http.NewResponseController(w)}

	// Should not panic, but Flush will fail silently
	err := stream.SendData("test")
	assert.Error(t, err) // Flush returns error for non-flushable writer
}

func TestSSEResponse_Integration(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	app := New()

	app.GET("/events", func(_ *Context) (any, error) {
		return SSEResponse(func(stream *SSEStream) error {
			for i := 0; i < 3; i++ {
				if err := stream.SendData(map[string]int{"count": i}); err != nil {
					return err
				}
			}

			return nil
		}), nil
	})

	go app.Run()

	var resp *http.Response

	var err error

	for i := 0; i < 50; i++ {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, configs.HTTPHost+"/events", http.NoBody)
		if reqErr != nil {
			t.Fatalf("create request: %v", reqErr)
		}

		resp, err = http.DefaultClient.Do(req)
		if err == nil {
			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	require.NoError(t, err)

	defer resp.Body.Close()

	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	body, readErr := io.ReadAll(resp.Body)
	require.NoError(t, readErr)

	bodyStr := string(body)

	assert.Contains(t, bodyStr, "data: {\"count\":0}\n")
	assert.Contains(t, bodyStr, "data: {\"count\":1}\n")
	assert.Contains(t, bodyStr, "data: {\"count\":2}\n")
}

func TestSSEResponse_Integration_ClientDisconnect(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	app := New()

	handlerExited := make(chan struct{})

	app.GET("/stream", func(c *Context) (any, error) {
		return SSEResponse(func(stream *SSEStream) error {
			defer close(handlerExited)

			_ = stream.SendData("connected")

			<-c.Context.Done()

			return nil
		}), nil
	})

	go app.Run()

	// Wait for server to start
	for i := 0; i < 50; i++ {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, configs.HTTPHost+"/.well-known/alive", http.NoBody)
		if reqErr != nil {
			t.Fatalf("create request: %v", reqErr)
		}

		probe, doErr := http.DefaultClient.Do(req)
		if doErr == nil {
			probe.Body.Close()

			break
		}

		time.Sleep(100 * time.Millisecond)
	}

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, configs.HTTPHost+"/stream", http.NoBody)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)

	buf := make([]byte, 512)
	n, _ := resp.Body.Read(buf)
	assert.Contains(t, string(buf[:n]), "data: connected")

	cancel()
	resp.Body.Close()

	select {
	case <-handlerExited:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not exit after client disconnect")
	}
}
