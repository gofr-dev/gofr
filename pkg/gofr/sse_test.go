package gofr

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gofr.dev/pkg/gofr/container"
	"gofr.dev/pkg/gofr/logging"
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

func TestSSEStream_Send(t *testing.T) {
	stream := newSSEStream()

	err := stream.Send(SSEEvent{Name: "test", Data: "hello"})
	require.NoError(t, err)

	msg := <-stream.events
	assert.Equal(t, "event: test\ndata: hello\n\n", msg)
}

func TestSSEStream_SendData(t *testing.T) {
	stream := newSSEStream()

	err := stream.SendData("simple")
	require.NoError(t, err)

	msg := <-stream.events
	assert.Equal(t, "data: simple\n\n", msg)
}

func TestSSEStream_SendEvent(t *testing.T) {
	stream := newSSEStream()

	err := stream.SendEvent("notification", map[string]int{"count": 5})
	require.NoError(t, err)

	msg := <-stream.events
	assert.Equal(t, "event: notification\ndata: {\"count\":5}\n\n", msg)
}

func TestSSEStream_SendComment(t *testing.T) {
	stream := newSSEStream()

	err := stream.SendComment("keep-alive")
	require.NoError(t, err)

	msg := <-stream.events
	assert.Equal(t, ": keep-alive\n\n", msg)
}

func TestSSEStream_SendComment_Multiline(t *testing.T) {
	stream := newSSEStream()

	err := stream.SendComment("line1\nline2")
	require.NoError(t, err)

	msg := <-stream.events
	assert.Equal(t, ": line1\n: line2\n\n", msg)
}

func TestSSEStream_Send_AfterDone(t *testing.T) {
	stream := newSSEStream()

	for i := 0; i < sseEventBufferSize; i++ {
		stream.events <- "fill"
	}

	close(stream.done)

	err := stream.Send(SSEEvent{Data: "too late"})
	assert.ErrorIs(t, err, errStreamClosed)
}

func TestSSEStream_SendComment_AfterDone(t *testing.T) {
	stream := newSSEStream()

	for i := 0; i < sseEventBufferSize; i++ {
		stream.events <- "fill"
	}

	close(stream.done)

	err := stream.SendComment("too late")
	assert.ErrorIs(t, err, errStreamClosed)
}

func TestSSEHTTPHandler_BlockedSenderNoLeak(t *testing.T) {
	c := &container.Container{Logger: logging.NewLogger(logging.FATAL)}

	handlerExited := make(chan struct{})

	h := sseHTTPHandler{
		function: func(_ *Context, stream *SSEStream) error {
			defer close(handlerExited)

			for i := 0; i < sseEventBufferSize+100; i++ {
				if err := stream.SendData(i); err != nil {
					return err
				}
			}

			return nil
		},
		container: c,
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/events", http.NoBody).WithContext(ctx)

	serveExited := make(chan struct{})

	go func() {
		h.ServeHTTP(w, r)
		close(serveExited)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-handlerExited:
	case <-time.After(5 * time.Second):
		t.Fatal("handler goroutine leaked: blocked on channel send after disconnect")
	}

	<-serveExited
}

func TestSSEStream_ConcurrentSend(t *testing.T) {
	stream := newSSEStream()
	count := 50

	var wg sync.WaitGroup

	wg.Add(count)

	for i := 0; i < count; i++ {
		go func(i int) {
			defer wg.Done()

			assert.NoError(t, stream.SendData(i))
		}(i)
	}

	received := make([]string, 0, count)
	done := make(chan struct{})

	go func() {
		for i := 0; i < count; i++ {
			received = append(received, <-stream.events)
		}

		close(done)
	}()

	wg.Wait()
	<-done

	assert.Len(t, received, count)
}

func TestSSEStream_Send_JSONStruct(t *testing.T) {
	type Notification struct {
		Title   string `json:"title"`
		Message string `json:"message"`
	}

	stream := newSSEStream()

	err := stream.Send(SSEEvent{
		Name: "notification",
		ID:   "1",
		Data: Notification{Title: "Hello", Message: "World"},
	})

	require.NoError(t, err)

	msg := <-stream.events
	assert.Equal(t, "id: 1\nevent: notification\ndata: {\"title\":\"Hello\",\"message\":\"World\"}\n\n", msg)
}

func TestSSEHTTPHandler_ServeHTTP(t *testing.T) {
	c := &container.Container{Logger: logging.NewLogger(logging.FATAL)}

	h := sseHTTPHandler{
		function: func(_ *Context, stream *SSEStream) error {
			return stream.SendData("hello")
		},
		container: c,
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/events", http.NoBody)
	h.ServeHTTP(w, r)

	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Body.String(), "data: hello\n\n")
}

func TestSSEHTTPHandler_ClientDisconnect(t *testing.T) {
	c := &container.Container{Logger: logging.NewLogger(logging.FATAL)}

	handlerStarted := make(chan struct{})

	h := sseHTTPHandler{
		function: func(ctx *Context, _ *SSEStream) error {
			close(handlerStarted)
			<-ctx.Done()

			return nil
		},
		container: c,
	}

	ctx, cancel := context.WithCancel(context.Background())

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/events", http.NoBody).WithContext(ctx)

	done := make(chan struct{})

	go func() {
		h.ServeHTTP(w, r)
		close(done)
	}()

	<-handlerStarted
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not exit after client disconnect")
	}

	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
}

func TestSSEHTTPHandler_HandlerError(t *testing.T) {
	c := &container.Container{Logger: logging.NewLogger(logging.FATAL)}

	h := sseHTTPHandler{
		function: func(_ *Context, stream *SSEStream) error {
			_ = stream.SendData("before error")
			return assert.AnError
		},
		container: c,
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/events", http.NoBody)
	h.ServeHTTP(w, r)

	assert.Contains(t, w.Body.String(), "data: before error\n\n")
}

func TestSSEHTTPHandler_Panic(t *testing.T) {
	c := &container.Container{Logger: logging.NewLogger(logging.FATAL)}

	h := sseHTTPHandler{
		function: func(_ *Context, _ *SSEStream) error {
			panic("test panic")
		},
		container: c,
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/events", http.NoBody)

	assert.NotPanics(t, func() {
		h.ServeHTTP(w, r)
	})
}

func TestSSEHTTPHandler_StreamingLoop(t *testing.T) {
	c := &container.Container{Logger: logging.NewLogger(logging.FATAL)}

	h := sseHTTPHandler{
		function: func(_ *Context, stream *SSEStream) error {
			for i := 0; i < 5; i++ {
				if err := stream.Send(SSEEvent{
					ID:   string(rune('0' + i)),
					Name: "tick",
					Data: map[string]int{"count": i},
				}); err != nil {
					return err
				}
			}

			return nil
		},
		container: c,
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/events", http.NoBody)
	h.ServeHTTP(w, r)

	body := w.Body.String()

	for i := 0; i < 5; i++ {
		expected, _ := json.Marshal(map[string]int{"count": i})
		assert.Contains(t, body, "data: "+string(expected)+"\n")
	}
}

func TestSSEHTTPHandler_NoFlusher(t *testing.T) {
	c := &container.Container{Logger: logging.NewLogger(logging.FATAL)}

	h := sseHTTPHandler{
		function: func(_ *Context, stream *SSEStream) error {
			return stream.SendData("should not arrive")
		},
		container: c,
	}

	w := &nonFlushableWriter{header: http.Header{}}
	r := httptest.NewRequest(http.MethodGet, "/events", http.NoBody)

	assert.NotPanics(t, func() {
		h.ServeHTTP(w, r)
	})
}

func TestSSEHTTPHandler_Heartbeat(t *testing.T) {
	c := &container.Container{Logger: logging.NewLogger(logging.FATAL)}

	handlerStarted := make(chan struct{})

	h := sseHTTPHandler{
		function: func(ctx *Context, _ *SSEStream) error {
			close(handlerStarted)
			<-ctx.Done()

			return nil
		},
		container: c,
	}

	w := httptest.NewRecorder()
	ctx, cancel := context.WithCancel(context.Background())
	r := httptest.NewRequest(http.MethodGet, "/events", http.NoBody).WithContext(ctx)

	done := make(chan struct{})

	go func() {
		h.ServeHTTP(w, r)
		close(done)
	}()

	<-handlerStarted
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("handler did not exit")
	}
}

// nonFlushableWriter is a ResponseWriter that does NOT implement http.Flusher.
type nonFlushableWriter struct {
	header http.Header
}

func (n *nonFlushableWriter) Header() http.Header     { return n.header }
func (*nonFlushableWriter) Write([]byte) (int, error) { return 0, nil }
func (*nonFlushableWriter) WriteHeader(int)           {}

func TestApp_SSE_Integration(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	app := New()

	app.SSE("/events", func(_ *Context, stream *SSEStream) error {
		for i := 0; i < 3; i++ {
			if err := stream.SendData(map[string]int{"count": i}); err != nil {
				return err
			}
		}

		return nil
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

func TestApp_SSE_Integration_ClientDisconnect(t *testing.T) {
	configs := testutil.NewServerConfigs(t)

	app := New()

	handlerExited := make(chan struct{})

	app.SSE("/stream", func(ctx *Context, stream *SSEStream) error {
		defer close(handlerExited)

		_ = stream.SendData("connected")

		<-ctx.Done()

		return nil
	})

	go app.Run()

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
